// Package fares — цены на билеты: доменная модель котировки и история цен.
package fares

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/fares/store"
)

// Quote — цена на перелёт туда и обратно для одного взрослого.
type Quote struct {
	Source          string
	Origin          string // IATA аэропорта вылета
	Destination     string // IATA аэропорта прилёта
	DepartAt        time.Time
	ReturnAt        time.Time
	Price           int64 // копейки
	Transfers       int
	ReturnTransfers int
	Airline         string
	DurationTo      time.Duration
	DurationBack    time.Duration
	FoundOn         time.Time // дата, когда источник видел эту цену
	BookingURL      string
}

// TripDays — длительность поездки в календарных днях по местным датам вылета и возвращения.
func (q Quote) TripDays() int {
	dep := time.Date(q.DepartAt.Year(), q.DepartAt.Month(), q.DepartAt.Day(), 0, 0, 0, 0, time.UTC)
	ret := time.Date(q.ReturnAt.Year(), q.ReturnAt.Month(), q.ReturnAt.Day(), 0, 0, 0, 0, time.UTC)
	return int(ret.Sub(dep).Hours()/24) + 1
}

// RoundTripQuery — направление и месяц вылета; дата возвращения любая.
type RoundTripQuery struct {
	Origin      string // IATA города или аэропорта
	Destination string
	Month       time.Time // берутся год и месяц
}

// Provider — источник цен.
type Provider interface {
	RoundTrips(ctx context.Context, q RoundTripQuery) ([]Quote, error)
}

// History сохраняет котировки для расчёта базовых цен.
type History struct {
	q *store.Queries
}

func NewHistory(pool *pgxpool.Pool) *History {
	return &History{q: store.New(pool)}
}

func (h *History) Save(ctx context.Context, quotes []Quote) error {
	if len(quotes) == 0 {
		return nil
	}
	params := make([]store.SaveQuotesParams, len(quotes))
	for i, q := range quotes {
		params[i] = store.SaveQuotesParams{
			Source:          q.Source,
			OriginIata:      q.Origin,
			DestinationIata: q.Destination,
			DepartAt:        pgtype.Timestamptz{Time: q.DepartAt, Valid: true},
			ReturnAt:        pgtype.Timestamptz{Time: q.ReturnAt, Valid: !q.ReturnAt.IsZero()},
			PriceKop:        q.Price,
			Transfers:       int16(q.Transfers),
			ReturnTransfers: pgtype.Int2{Int16: int16(q.ReturnTransfers), Valid: !q.ReturnAt.IsZero()},
			Airline:         q.Airline,
			FoundOn:         pgtype.Date{Time: q.FoundOn, Valid: !q.FoundOn.IsZero()},
		}
	}
	var firstErr error
	h.q.SaveQuotes(ctx, params).Exec(func(_ int, err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	})
	if firstErr != nil {
		return fmt.Errorf("save quotes: %w", firstErr)
	}
	return nil
}

// aviasalesSearch — путь выдачи Aviasales: KUF2311ALA2811 + пассажиры (взрослые[дети[младенцы]]).
// Без даты возвращения — перелёт в одну сторону: KUF2311ALA1.
var aviasalesSearch = regexp.MustCompile(`^(/search/[A-Z]{3}\d{4}[A-Z]{3}(?:\d{4})?)\d{1,3}$`)

// BookingURLFor — ссылка на покупку с нужным числом взрослых. Источник отдаёт ссылку на одного пассажира,
// а котировки кэшируются для всех поисков, поэтому число людей подставляется при сборке варианта.
func (q Quote) BookingURLFor(adults int) string {
	if q.BookingURL == "" || adults < 1 {
		return q.BookingURL
	}
	u, err := url.Parse(q.BookingURL)
	if err != nil {
		return q.BookingURL
	}
	m := aviasalesSearch.FindStringSubmatch(u.Path)
	if m == nil {
		return q.BookingURL // не выдача Aviasales — не трогаем
	}
	u.Path = m[1] + strconv.Itoa(min(adults, 9))
	return u.String()
}
