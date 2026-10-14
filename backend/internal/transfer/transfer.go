// Package transfer — как добраться внутри города: из аэропорта или с вокзала до жилья.
// Варианты: такси, общественный транспорт, пешком. Без ключей внешних сервисов считаем дорогу сами
// (OSRM), а цену такси и маршруты транспорта отдаём ссылками на Яндекс Go, Яндекс Карты и 2ГИС.
package transfer

import (
	"context"
	"log/slog"
	"math"
	"sort"

	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

type Point struct{ Lat, Lon float64 }

type Mode string

const (
	ModeTaxi            Mode = "taxi"
	ModePublicTransport Mode = "public_transport"
	ModeWalk            Mode = "walk"
)

// Link — где открыть или заказать маршрут.
type Link struct{ Title, URL string }

// Option — один способ добраться.
type Option struct {
	Mode        Mode
	Title       string
	DurationMin int     // 0 — неизвестно
	DistanceKm  float64 // 0 — неизвестно
	PriceKop    int64   // 0 — неизвестно
	PriceNote   string
	Approximate bool
	Links       []Link
}

const (
	// Пешком предлагаем, только если идти не дольше часа; маршрут строим, если по прямой не дальше 5 км
	walkMaxStraightKm = 5.0
	walkMaxMin        = 60
	taxiSeats         = 4
)

// Router — расстояние и время от точки до точек (OSRM table: машина или пешком).
type Router interface {
	Table(ctx context.Context, from road.Point, to []road.Point) ([]*road.Leg, error)
}

type Service struct {
	car  Router // маршруты на машине
	foot Router // пешие маршруты
	log  *slog.Logger
}

func NewService(car, foot Router, log *slog.Logger) *Service {
	return &Service{car: car, foot: foot, log: log}
}

// Options возвращает варианты, быстрые первыми; неизвестное время — в конце.
func (s *Service) Options(ctx context.Context, from, to Point, country string, travelers int) []Option {
	links := linksFor(from, to, country)
	var out []Option

	// Такси: время и расстояние по дороге; цена — в приложении такси, пока нет ключа API прогноза стоимости
	taxi := Option{Mode: ModeTaxi, Title: "Такси", PriceNote: "точную цену покажет Яндекс Go",
		Links: []Link{links.yandexGo, links.yandexMapsCar}}
	if leg, ok := s.leg(ctx, s.car, from, to); ok {
		taxi.DurationMin, taxi.DistanceKm, taxi.Approximate = minutes(leg), km(leg), leg.Approximate
	}
	if travelers > taxiSeats {
		taxi.PriceNote = "вас больше четырёх — понадобится минивэн или две машины; " + taxi.PriceNote
	}
	out = append(out, taxi)

	// Общественный транспорт: маршрут строят Яндекс Карты и 2ГИС
	out = append(out, Option{Mode: ModePublicTransport, Title: "Общественный транспорт",
		PriceNote: "время и пересадки — в Яндекс Картах или 2ГИС",
		Links:     []Link{links.yandexMapsTransit, links.twoGISTransit}})

	// Пешком — если близко
	if haversineKm(from, to) <= walkMaxStraightKm {
		if leg, ok := s.leg(ctx, s.foot, from, to); ok && minutes(leg) <= walkMaxMin {
			out = append(out, Option{Mode: ModeWalk, Title: "Пешком", DurationMin: minutes(leg), DistanceKm: km(leg),
				Approximate: leg.Approximate, Links: []Link{links.yandexMapsWalk}})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].DurationMin, out[j].DurationMin
		if a == 0 || b == 0 {
			return a != 0
		}
		return a < b
	})
	return out
}

// leg — маршрут через router; не ответил — оценка по прямой.
func (s *Service) leg(ctx context.Context, router Router, from, to Point) (road.Leg, bool) {
	a, b := road.Point{Lat: from.Lat, Lon: from.Lon}, road.Point{Lat: to.Lat, Lon: to.Lon}
	legs, err := router.Table(ctx, a, []road.Point{b})
	if err != nil {
		s.log.WarnContext(ctx, "transfer routing failed, using estimate", slog.Any("err", err))
		return road.Estimate(a, b), true
	}
	if len(legs) == 0 || legs[0] == nil {
		return road.Leg{}, false // проезда или прохода нет
	}
	return *legs[0], true
}

func minutes(l road.Leg) int { return int(math.Round(float64(l.DurationS) / 60)) }
func km(l road.Leg) float64  { return math.Round(float64(l.DistanceM)/100) / 10 }

func haversineKm(a, b Point) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (b.Lat-a.Lat)*rad, (b.Lon-a.Lon)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
