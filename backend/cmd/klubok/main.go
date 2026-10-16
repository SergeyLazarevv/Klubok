// Команда klubok — единая точка входа: HTTP API, миграции, импорт справочников.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/guides"
	"github.com/SergeyLazarevv/Klubok/backend/internal/hotels"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/config"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/database"
	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
	"github.com/SergeyLazarevv/Klubok/backend/internal/poi/osmimport"
	"github.com/SergeyLazarevv/Klubok/backend/internal/providers/travelpayouts"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
	"github.com/SergeyLazarevv/Klubok/backend/internal/session"
	"github.com/SergeyLazarevv/Klubok/backend/internal/stay"
	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
	"github.com/SergeyLazarevv/Klubok/backend/internal/weather"
)

const usage = `Использование: klubok <команда>

Команды:
  api                 запустить HTTP API
  migrate [up|down|status]
                      миграции БД (по умолчанию up)
  import-places       загрузить справочник городов и аэропортов
  osm-update [--regions r1,r2] [--full]
                      обновить места на карте из выгрузок OpenStreetMap (Geofabrik): скачать новые
                      дифы, применить, загрузить изменившееся; для крона — раз в неделю
  poi-enrich [--limit N] [--max-age-days N] [--pause D]
                      загрузить описания и фото мест из Wikidata и Википедии; для крона — после osm-update
  stay-update         обновить цены на жильё по городам карты (Росстат, гостиница 3*) и сбросить кэш;
                      для крона — раз в месяц
  fares-warm [--origins KUF,MOW] [--months N] [--pause D]
                      обновить снимки цен «во все города» для «Куда угодно»: по каждому городу вылета
                      (по умолчанию — все города России с аэропортом) и месяцу; для крона — раз в сутки`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	cfg := config.Load()
	log := newLogger(cfg.LogFormat)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, log, os.Args[1], os.Args[2:]); err != nil {
		log.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, log *slog.Logger, cmd string, args []string) error {
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "api":
		if cfg.TravelpayoutsToken == "" {
			log.Warn("TRAVELPAYOUTS_TOKEN не задан — поиск билетов работать не будет")
		}
		return serveAPI(ctx, cfg, log, pool)
	case "migrate":
		command := "up"
		if len(args) > 0 {
			command = args[0]
		}
		return database.Migrate(ctx, pool, command)
	case "import-places":
		start := time.Now()
		stats, err := geo.NewImporter(pool).Run(ctx)
		if err != nil {
			return err
		}
		log.Info("places imported", slog.Int("cities", stats.Cities), slog.Int("airports", stats.Airports),
			slog.Int("skipped", stats.Skipped), slog.Duration("took", time.Since(start)))
		return nil
	case "osm-update":
		return osmUpdate(ctx, cfg, log, pool, args)
	case "poi-enrich":
		return poiEnrich(ctx, cfg, log, pool, args)
	case "fares-warm":
		return faresWarm(ctx, cfg, log, pool, args)
	case "stay-update":
		return stayUpdate(ctx, cfg, log, pool)
	default:
		return fmt.Errorf("неизвестная команда %q\n\n%s", cmd, usage)
	}
}

func serveAPI(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	cache, err := newRedis(cfg)
	if err != nil {
		return err
	}
	defer cache.Close()
	geoSvc := geo.NewService(pool)
	roadSvc := road.NewService(pool, road.NewOSRM(cfg.OSRMURL), log)
	roadSvc.UseCache(cache)
	tp := travelpayouts.New(cfg.TravelpayoutsToken, cfg.TravelpayoutsMarker)
	prices := fares.NewCached(tp, pool, travelpayouts.Source, snapshotMaxAge, log)
	// Без ID проекта ссылки на Островок обычные, без партнёрской метки
	var ostrovokLinker hotels.PartnerLinker
	if cfg.TravelpayoutsTRS != "" {
		ostrovokLinker = tp.Linker(cfg.TravelpayoutsTRS)
	}
	handler, err := httpapi.NewRouter(httpapi.Deps{
		Log:  log,
		Pool: pool,
		Geo:  geoSvc,
		Search: search.NewService(geoSvc, roadSvc, carcost.NewRefs(pool), prices, fares.NewHistory(pool),
			poi.NewService(pool, poi.NewOverpass(cfg.OverpassURL), poi.NewWiki(), log), log),
		Hotels: hotels.NewOstrovok(ostrovokLinker, log),
		Guides: guides.NewService(pool, guides.NewWikivoyage(), log),

		Transfers: transfer.NewService(road.NewOSRM(cfg.OSRMURL), road.NewOSRM(cfg.OSRMFootURL), log),
		Geocoder:  transfer.NewPhoton(cfg.PhotonURL),

		Sessions:  session.NewManager(pool, log),
		Favorites: favorites.NewService(pool),
		Stay:      stay.NewService(pool, cache, log),
		Road:      roadSvc,
		Weather:   weather.NewService(weather.NewOpenMeteo(), weather.NewRedisCache(cache), log),
	})
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("api listening", slog.String("addr", cfg.HTTPAddr))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func osmUpdate(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("osm-update", flag.ContinueOnError)
	regions := fs.String("regions", "", "регионы Geofabrik через запятую (по умолчанию — из KLUBOK_OSM_REGIONS)")
	full := fs.Bool("full", false, "скачать выгрузки заново, не применяя дифы")
	if err := fs.Parse(args); err != nil {
		return err
	}
	all := osmimport.DefaultRegions
	if cfg.OSMRegions != "" {
		all = splitList(cfg.OSMRegions)
	}
	im := osmimport.New(
		osmimport.Config{Dir: cfg.OSMDir, Regions: all, MaxLag: cfg.OSMMaxLag},
		pool, osmimport.NewGeofabrik(cfg.GeofabrikURL), osmimport.NewOsmium(cfg.Osmium), log)
	start := time.Now()
	results, err := im.Run(ctx, osmimport.Options{Regions: splitList(*regions), Full: *full})
	var created, updated, deleted int
	for _, r := range results {
		created, updated, deleted = created+r.Created, updated+r.Updated, deleted+r.Deleted
	}
	log.Info("osm-update finished", slog.Int("regions", len(results)), slog.Int("created", created),
		slog.Int("updated", updated), slog.Int("deleted", deleted), slog.Duration("took", time.Since(start)))
	return err
}

func poiEnrich(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("poi-enrich", flag.ContinueOnError)
	limit := fs.Int("limit", 2000, "сколько мест обработать за запуск")
	maxAge := fs.Int("max-age-days", 90, "описания старше — загрузить заново")
	pause := fs.Duration("pause", 300*time.Millisecond, "пауза между местами")
	if err := fs.Parse(args); err != nil {
		return err
	}
	svc := poi.NewService(pool, poi.NewOverpass(cfg.OverpassURL), poi.NewWiki(), log)
	start := time.Now()
	st, err := svc.Enrich(ctx, poi.EnrichOptions{Limit: *limit, MaxAgeDays: *maxAge, Pause: *pause})
	log.Info("poi-enrich finished", slog.Int("processed", st.Processed), slog.Int("with_photo", st.WithPhoto),
		slog.Int("with_description", st.WithDescription), slog.Int("failed", st.Failed), slog.Duration("took", time.Since(start)))
	return err
}

// snapshotMaxAge — снимок цен «во все города» старше этого перезапрашивается: прогрев идёт раз в сутки,
// запас — на случай, если ночной запуск задержался.
const snapshotMaxAge = 36 * time.Hour

func faresWarm(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("fares-warm", flag.ContinueOnError)
	origins := fs.String("origins", "", "коды городов вылета через запятую (по умолчанию — все города России с аэропортом)")
	monthsAhead := fs.Int("months", 4, "сколько месяцев, начиная с текущего")
	pause := fs.Duration("pause", 400*time.Millisecond, "пауза между запросами к источнику")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.TravelpayoutsToken == "" {
		return errors.New("TRAVELPAYOUTS_TOKEN не задан")
	}
	list := splitList(*origins)
	if len(list) == 0 {
		var err error
		if list, err = geo.NewService(pool).CitiesWithAirports(ctx, "RU"); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	months := make([]time.Time, *monthsAhead)
	for i := range months {
		months[i] = time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
	}
	cached := fares.NewCached(travelpayouts.New(cfg.TravelpayoutsToken, cfg.TravelpayoutsMarker), pool, travelpayouts.Source, snapshotMaxAge, log)
	start := time.Now()
	st, err := cached.Warm(ctx, list, months, fares.NewHistory(pool), *pause)
	log.Info("fares-warm finished", slog.Int("origins", len(list)), slog.Int("requests", st.Requests),
		slog.Int("failed", st.Failed), slog.Int("quotes", st.Quotes), slog.Duration("took", time.Since(start)))
	return err
}

// newRedis — клиент кэша. Подключение ленивое: недоступный Redis не мешает запуску, кэш просто не работает.
func newRedis(cfg config.Config) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("KLUBOK_REDIS_URL: %w", err)
	}
	return redis.NewClient(opts), nil
}

func stayUpdate(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	cities, err := stay.LoadCities(cfg.CitiesFile)
	if err != nil {
		return fmt.Errorf("справочник городов карты (KLUBOK_CITIES_FILE): %w", err)
	}
	start := time.Now()
	territories, err := stay.NewRosstat("https://fedstat.ru").Fetch(ctx, time.Now())
	if err != nil {
		return err
	}
	prices := stay.Assign(cities, territories)
	if len(prices) == 0 {
		return errors.New("stay-update: ни одной цены — проверьте ответ ЕМИСС, старые данные не тронуты")
	}
	cache, err := newRedis(cfg)
	if err != nil {
		return err
	}
	defer cache.Close()
	if err := stay.NewService(pool, cache, log).Replace(ctx, prices); err != nil {
		return err
	}
	own := 0
	for _, p := range prices {
		if p.Basis == p.CityName {
			own++
		}
	}
	log.Info("stay-update finished", slog.Int("territories", len(territories)), slog.Int("cities", len(prices)),
		slog.Int("own_price", own), slog.Duration("took", time.Since(start)))
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func newLogger(format string) *slog.Logger {
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
