// Package search — подбор вариантов поездки: хабы, дорога до них, стоимость.
package search

import (
	"context"
	"log/slog"
	"sort"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

type Service struct {
	geo     *geo.Service
	road    *road.Service
	costs   *carcost.Refs
	fares   fares.Provider
	history *fares.History
	pois    *poi.Service
	log     *slog.Logger
}

func NewService(g *geo.Service, r *road.Service, c *carcost.Refs, f fares.Provider, h *fares.History, p *poi.Service, log *slog.Logger) *Service {
	return &Service{geo: g, road: r, costs: c, fares: f, history: h, pois: p, log: log}
}

// CarParams — на каких условиях пользователь готов ехать на своей машине.
type CarParams struct {
	RadiusKm        int
	MaxDriveHours   float64 // 0 — без ограничения
	ConsumptionL100 float64
	Fuel            carcost.FuelType
	FuelPrice       carcost.Kop // 0 — взять из справочника
	AllowBorder     bool
}

// CarHubsQuery — запрос хабов с расходами на машину на поездку фиксированной длительности.
type CarHubsQuery struct {
	HomeID    uuid.UUID
	Car       CarParams
	Days      int
	Travelers int
}

// CarHub — хаб, до которого можно доехать, и во что обойдётся машина.
type CarHub struct {
	reachable
	Cost      carcost.Cost
	PerPerson carcost.Kop
}

// reachable — хаб, до которого пользователь готов доехать; стоимость зависит от длительности поездки.
type reachable struct {
	Hub              geo.Hub
	Leg              road.Leg
	DriveHours       float64 // в одну сторону, с учётом границы
	ParkingPerDay    carcost.Kop
	ParkingEstimated bool
	Border           *carcost.BorderRule // nil — граница не пересекается
}

func (r reachable) carCost(fuelPrice carcost.Kop, consumption float64, days int) carcost.Cost {
	var insurance carcost.Kop
	if r.Border != nil {
		insurance = r.Border.Insurance
	}
	return carcost.Calc(carcost.Input{
		OneWayKm:        float64(r.Leg.DistanceM) / 1000,
		ConsumptionL100: consumption,
		FuelPrice:       fuelPrice,
		ParkingPerDay:   r.ParkingPerDay,
		Days:            days,
		BorderInsurance: insurance,
	})
}

type CarHubsResult struct {
	Home       geo.Place
	FuelPrice  carcost.Kop
	FuelSource string // reference | user
	Hubs       []CarHub
}

// CarHubs возвращает аэропорты, до которых пользователь готов доехать, с расходами на машину.
func (s *Service) CarHubs(ctx context.Context, q CarHubsQuery) (CarHubsResult, error) {
	home, err := s.geo.Get(ctx, q.HomeID)
	if err != nil {
		return CarHubsResult{}, err
	}
	fuelPrice, fuelSource, err := s.fuelPrice(ctx, home, q.Car)
	if err != nil {
		return CarHubsResult{}, err
	}
	hubs, err := s.reachableByCar(ctx, home, q.Car)
	if err != nil {
		return CarHubsResult{}, err
	}
	res := CarHubsResult{Home: home, FuelPrice: fuelPrice, FuelSource: fuelSource}
	for _, r := range hubs {
		cost := r.carCost(fuelPrice, q.Car.ConsumptionL100, q.Days)
		res.Hubs = append(res.Hubs, CarHub{reachable: r, Cost: cost, PerPerson: cost.Total / carcost.Kop(max(q.Travelers, 1))})
	}
	return res, nil
}

func (s *Service) fuelPrice(ctx context.Context, home geo.Place, car CarParams) (carcost.Kop, string, error) {
	if car.FuelPrice > 0 {
		return car.FuelPrice, "user", nil
	}
	p, err := s.costs.FuelPrice(ctx, home.CountryCode, "", car.Fuel)
	return p, "reference", err
}

// reachableByCar — аэропорты в радиусе, до которых можно доехать за отведённое время, ближайшие по времени первыми.
func (s *Service) reachableByCar(ctx context.Context, home geo.Place, car CarParams) ([]reachable, error) {
	hubs, err := s.geo.AirportsWithin(ctx, home.ID, car.RadiusKm)
	if err != nil {
		return nil, err
	}
	if !car.AllowBorder {
		hubs = filter(hubs, func(h geo.Hub) bool { return !h.CrossesBorder })
	}
	if len(hubs) == 0 {
		return nil, nil
	}

	points := make([]road.Point, len(hubs))
	ids := make([]uuid.UUID, len(hubs))
	for i, h := range hubs {
		points[i] = roadPoint(h.Place)
		ids[i] = h.Place.ID
	}
	legs, err := s.road.Legs(ctx, roadPoint(home), points)
	if err != nil {
		return nil, err
	}
	parking, err := s.costs.Parking(ctx, ids)
	if err != nil {
		return nil, err
	}
	borders, err := s.costs.BorderRules(ctx, home.CountryCode)
	if err != nil {
		return nil, err
	}

	var out []reachable
	for _, h := range hubs {
		leg, ok := legs[h.Place.ID]
		if !ok {
			continue // по дороге не доехать
		}
		r := reachable{Hub: h, Leg: leg, DriveHours: float64(leg.DurationS) / 3600}
		if h.CrossesBorder {
			rule, known := borders[h.Place.CountryCode]
			if !known {
				rule = carcost.BorderRule{Note: "Нет данных о правилах въезда на машине в эту страну — проверьте самостоятельно."}
			}
			r.Border = &rule
			r.DriveHours += rule.CrossingHours
		}
		if car.MaxDriveHours > 0 && r.DriveHours > car.MaxDriveHours {
			continue
		}
		perDay, known := parking[h.Place.ID]
		if !known {
			perDay, r.ParkingEstimated = carcost.DefaultParkingPerDay, true
		}
		r.ParkingPerDay = perDay
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DriveHours < out[j].DriveHours })
	return out, nil
}

func filter[T any](in []T, keep func(T) bool) []T {
	out := in[:0:0]
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func roadPoint(p geo.Place) road.Point {
	return road.Point{ID: p.ID, Lat: p.Lat, Lon: p.Lon}
}
