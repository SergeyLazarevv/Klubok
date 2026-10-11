// Команда klubok — единая точка входа: HTTP API, миграции, импорт справочников.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/config"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/database"
	"github.com/SergeyLazarevv/Klubok/backend/internal/providers/travelpayouts"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
)

const usage = `Использование: klubok <команда>

Команды:
  api                 запустить HTTP API
  migrate [up|down|status]
                      миграции БД (по умолчанию up)
  import-places       загрузить справочник городов и аэропортов`

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
	default:
		return fmt.Errorf("неизвестная команда %q\n\n%s", cmd, usage)
	}
}

func serveAPI(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	geoSvc := geo.NewService(pool)
	roadSvc := road.NewService(pool, road.NewOSRM(cfg.OSRMURL), log)
	handler, err := httpapi.NewRouter(httpapi.Deps{
		Log:  log,
		Pool: pool,
		Geo:  geoSvc,
		Search: search.NewService(geoSvc, roadSvc, carcost.NewRefs(pool),
			travelpayouts.New(cfg.TravelpayoutsToken, cfg.TravelpayoutsMarker), fares.NewHistory(pool), log),
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

func newLogger(format string) *slog.Logger {
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
