// Package road — расстояние и время в пути на машине между точками.
package road

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/road/store"
)

// Point — точка маршрута.
type Point struct {
	ID       uuid.UUID
	Lat, Lon float64
}

// Leg — путь на машине в одну сторону.
type Leg struct {
	DistanceM int
	DurationS int
	// Approximate — оценка по прямой, а не реальный маршрут.
	Approximate bool
}

// Route — путь на машине вместе с линией для карты.
type Route struct {
	Leg
	Polyline string // Encoded Polyline, точность 6 знаков
}

// Router строит маршруты. nil в ответе — проезда нет.
type Router interface {
	// Table — расстояния от одной точки до многих, без линий.
	Table(ctx context.Context, from Point, to []Point) ([]*Leg, error)
	// Route — путь между двумя точками с линией.
	Route(ctx context.Context, from, to Point) (*Route, error)
}

const (
	sourceOSRM     = "osrm"
	detourFactor   = 1.3  // дороги длиннее прямой примерно на 30%
	estimatedSpeed = 70.0 // км/ч, средняя по трассе с населёнными пунктами
)

type Service struct {
	q      *store.Queries
	router Router
	log    *slog.Logger
}

func NewService(pool *pgxpool.Pool, router Router, log *slog.Logger) *Service {
	return &Service{q: store.New(pool), router: router, log: log}
}

// Legs возвращает путь от from до каждой точки to. Точки без проезда в ответ не попадают.
// Сначала смотрим кэш, недостающее запрашиваем у маршрутизатора; если он недоступен — оцениваем по прямой.
func (s *Service) Legs(ctx context.Context, from Point, to []Point) (map[uuid.UUID]Leg, error) {
	result := make(map[uuid.UUID]Leg, len(to))
	if len(to) == 0 {
		return result, nil
	}
	ids := make([]uuid.UUID, len(to))
	for i, p := range to {
		ids[i] = p.ID
	}
	cached, err := s.q.ListRoadDistances(ctx, store.ListRoadDistancesParams{FromID: from.ID, ToIds: ids})
	if err != nil {
		return nil, fmt.Errorf("list road distances: %w", err)
	}
	for _, c := range cached {
		result[c.ToPlaceID] = Leg{DistanceM: int(c.DistanceM), DurationS: int(c.DurationS)}
	}

	var missing []Point
	for _, p := range to {
		if p.ID == from.ID {
			result[p.ID] = Leg{}
			continue
		}
		if _, ok := result[p.ID]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}

	legs, err := s.router.Table(ctx, from, missing)
	if err != nil {
		s.log.WarnContext(ctx, "router failed, using estimate", slog.Any("err", err))
		for _, p := range missing {
			result[p.ID] = Estimate(from, p)
		}
		return result, nil
	}
	for i, leg := range legs {
		if leg == nil {
			continue // проезда нет: остров, другой континент
		}
		result[missing[i].ID] = *leg
		err := s.q.UpsertRoadDistance(ctx, store.UpsertRoadDistanceParams{
			FromID: from.ID, ToID: missing[i].ID,
			DistanceM: int32(leg.DistanceM), DurationS: int32(leg.DurationS), Source: sourceOSRM,
		})
		if err != nil {
			return nil, fmt.Errorf("cache road distance: %w", err)
		}
	}
	return result, nil
}

// Route возвращает путь с линией для карты. Сначала смотрим кэш; если маршрутизатор недоступен
// или проезда нет — прямая между точками с оценкой по прямой.
func (s *Service) Route(ctx context.Context, from, to Point) (Route, error) {
	cached, err := s.q.GetRoadRoute(ctx, store.GetRoadRouteParams{FromID: from.ID, ToID: to.ID})
	if err == nil {
		return Route{
			Leg:      Leg{DistanceM: int(cached.DistanceM), DurationS: int(cached.DurationS)},
			Polyline: cached.Geometry,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Route{}, fmt.Errorf("get road route: %w", err)
	}

	route, err := s.router.Route(ctx, from, to)
	if err != nil || route == nil {
		if err != nil {
			s.log.WarnContext(ctx, "router failed, using straight line", slog.Any("err", err))
		}
		return Route{Leg: Estimate(from, to), Polyline: EncodePolyline([]Point{from, to})}, nil
	}
	err = s.q.UpsertRoadRoute(ctx, store.UpsertRoadRouteParams{
		FromID: from.ID, ToID: to.ID,
		DistanceM: int32(route.DistanceM), DurationS: int32(route.DurationS), Source: sourceOSRM,
		Geometry: pgtype.Text{String: route.Polyline, Valid: true},
	})
	if err != nil {
		return Route{}, fmt.Errorf("cache road route: %w", err)
	}
	return *route, nil
}

// Estimate — грубая оценка пути, когда маршрутизатор недоступен. В кэш не пишется.
func Estimate(from, to Point) Leg {
	km := haversineKm(from, to) * detourFactor
	return Leg{
		DistanceM:   int(km * 1000),
		DurationS:   int(km / estimatedSpeed * 3600),
		Approximate: true,
	}
}

func haversineKm(a, b Point) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (b.Lat - a.Lat) * rad
	dLon := (b.Lon - a.Lon) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
