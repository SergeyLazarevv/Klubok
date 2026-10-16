package fares

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/fares/store"
)

// Cached — источник цен со снимками в БД для запросов «во все города» (пустое направление): их отдаёт
// ежедневный прогрев (fares-warm), а живой запрос идёт, только если снимка нет или он старше maxAge.
// Запросы по конкретному направлению всегда живые — по ним уточняется цена перед покупкой.
type Cached struct {
	inner  Provider
	q      *store.Queries
	source string
	maxAge time.Duration
	log    *slog.Logger
}

func NewCached(inner Provider, pool *pgxpool.Pool, source string, maxAge time.Duration, log *slog.Logger) *Cached {
	return &Cached{inner: inner, q: store.New(pool), source: source, maxAge: maxAge, log: log}
}

func (c *Cached) RoundTrips(ctx context.Context, q RoundTripQuery) ([]Quote, error) {
	if q.Destination != "" {
		return c.inner.RoundTrips(ctx, q)
	}
	quotes, fetchedAt, err := c.snapshot(ctx, q)
	switch {
	case err != nil:
		c.log.WarnContext(ctx, "read fare snapshot", slog.String("origin", q.Origin), slog.Any("err", err))
	case !fetchedAt.IsZero() && time.Since(fetchedAt) < c.maxAge:
		return quotes, nil
	}
	return c.Refresh(ctx, q)
}

// Refresh запрашивает источник и сохраняет снимок. Ошибка сохранения не мешает вернуть цены.
func (c *Cached) Refresh(ctx context.Context, q RoundTripQuery) ([]Quote, error) {
	quotes, err := c.inner.RoundTrips(ctx, q)
	if err != nil {
		return nil, err
	}
	if err := c.save(ctx, q, quotes); err != nil {
		c.log.WarnContext(ctx, "save fare snapshot", slog.String("origin", q.Origin), slog.Any("err", err))
	}
	return quotes, nil
}

func (c *Cached) snapshot(ctx context.Context, q RoundTripQuery) ([]Quote, time.Time, error) {
	row, err := c.q.GetFareSnapshot(ctx, store.GetFareSnapshotParams{Source: c.source, OriginIata: q.Origin, Month: month(q.Month)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	var quotes []Quote
	if err := json.Unmarshal(row.Quotes, &quotes); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode snapshot: %w", err)
	}
	return quotes, row.FetchedAt.Time, nil
}

func (c *Cached) save(ctx context.Context, q RoundTripQuery, quotes []Quote) error {
	if quotes == nil {
		quotes = []Quote{} // «цен нет» — тоже ответ, его не нужно перезапрашивать до следующего прогрева
	}
	data, err := json.Marshal(quotes)
	if err != nil {
		return err
	}
	return c.q.UpsertFareSnapshot(ctx, store.UpsertFareSnapshotParams{
		Source: c.source, OriginIata: q.Origin, Month: month(q.Month), Quotes: data,
	})
}

func month(t time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), Valid: true}
}

// WarmStats — итог прогрева снимков.
type WarmStats struct {
	Requests int
	Failed   int
	Quotes   int
}

// Warm обновляет снимки «во все города» для городов вылета на месяцы вперёд и пишет котировки в историю.
// Запросы идут по одному с паузой — источник бесплатный, нагружать его не нужно.
func (c *Cached) Warm(ctx context.Context, origins []string, months []time.Time, history *History, pause time.Duration) (WarmStats, error) {
	var st WarmStats
	for _, origin := range origins {
		for _, m := range months {
			if err := ctx.Err(); err != nil {
				return st, err
			}
			st.Requests++
			quotes, err := c.Refresh(ctx, RoundTripQuery{Origin: origin, Month: m})
			if err != nil {
				st.Failed++
				c.log.WarnContext(ctx, "warm fares", slog.String("origin", origin), slog.String("month", m.Format("2006-01")), slog.Any("err", err))
			} else {
				st.Quotes += len(quotes)
				if err := history.Save(ctx, quotes); err != nil {
					c.log.WarnContext(ctx, "save price history", slog.Any("err", err))
				}
			}
			time.Sleep(pause)
		}
	}
	if st.Requests > 0 && st.Failed == st.Requests {
		return st, errors.New("all fare requests failed")
	}
	return st, nil
}
