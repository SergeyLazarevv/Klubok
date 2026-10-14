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
	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
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
                      загрузить описания и фото мест из Wikidata и Википедии; для крона — после osm-update`

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
	default:
		return fmt.Errorf("неизвестная команда %q\n\n%s", cmd, usage)
	}
}

func serveAPI(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	geoSvc := geo.NewService(pool)
	roadSvc := road.NewService(pool, road.NewOSRM(cfg.OSRMURL), log)
	tp := travelpayouts.New(cfg.TravelpayoutsToken, cfg.TravelpayoutsMarker)
	// Без ID проекта ссылки на Островок обычные, без партнёрской метки
	var ostrovokLinker hotels.PartnerLinker
	if cfg.TravelpayoutsTRS != "" {
		ostrovokLinker = tp.Linker(cfg.TravelpayoutsTRS)
	}
	handler, err := httpapi.NewRouter(httpapi.Deps{
		Log:  log,
		Pool: pool,
		Geo:  geoSvc,
		Search: search.NewService(geoSvc, roadSvc, carcost.NewRefs(pool), tp, fares.NewHistory(pool),
			poi.NewService(pool, poi.NewOverpass(cfg.OverpassURL), poi.NewWiki(), log), log),
		Hotels: hotels.NewOstrovok(ostrovokLinker, log),
		Guides: guides.NewService(pool, guides.NewWikivoyage(), log),

		Transfers: transfer.NewService(road.NewOSRM(cfg.OSRMURL), road.NewOSRM(cfg.OSRMFootURL), log),
		Geocoder:  transfer.NewPhoton(cfg.PhotonURL),

		Sessions:  session.NewManager(pool, log),
		Favorites: favorites.NewService(pool),
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
