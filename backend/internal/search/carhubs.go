// Package search — подбор вариантов поездки: хабы, дорога до них, стоимость.
package search

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

type Service struct {
	geo   *geo.Service
	road  *road.Service
	costs *carcost.Refs
}

func NewService(g *geo.Service, r *road.Service, c *carcost.Refs) *Service {
	return &Service{geo: g, road: r, costs: c}
}

// CarHubsQuery — откуда и на каких условиях пользователь готов ехать на машине.
type CarHubsQuery struct {
	HomeID          uuid.UUID
	RadiusKm        int
	MaxDriveHours   float64 // 0 — без ограничения
	Days            int
	Travelers       int
	ConsumptionL100 float64
	Fuel            carcost.FuelType
	FuelPrice       carcost.Kop // 0 — взять из справочника
	AllowBorder     bool
}

// CarHub — хаб, до которого можно доехать, и во что обойдётся машина.
type CarHub struct {
	Hub              geo.Hub
	Leg              road.Leg
	DriveHours       float64 // в одну сторону, с учётом границы
	Cost             carcost.Cost
	PerPerson        carcost.Kop
	ParkingEstimated bool
	Border           *carcost.BorderRule // nil — граница не пересекается
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
	res := CarHubsResult{Home: home, FuelPrice: q.FuelPrice, FuelSource: "user"}
	if q.FuelPrice == 0 {
		if res.FuelPrice, err = s.costs.FuelPrice(ctx, home.CountryCode, "", q.Fuel); err != nil {
			return CarHubsResult{}, err
		}
		res.FuelSource = "reference"
	}

	hubs, err := s.geo.AirportsWithin(ctx, home.ID, q.RadiusKm)
	if err != nil {
		return CarHubsResult{}, err
	}
	if !q.AllowBorder {
		hubs = filter(hubs, func(h geo.Hub) bool { return !h.CrossesBorder })
	}
	if len(hubs) == 0 {
		return res, nil
	}

	points := make([]road.Point, len(hubs))
	ids := make([]uuid.UUID, len(hubs))
	for i, h := range hubs {
		points[i] = road.Point{ID: h.Place.ID, Lat: h.Place.Lat, Lon: h.Place.Lon}
		ids[i] = h.Place.ID
	}
	legs, err := s.road.Legs(ctx, road.Point{ID: home.ID, Lat: home.Lat, Lon: home.Lon}, points)
	if err != nil {
		return CarHubsResult{}, err
	}
	parking, err := s.costs.Parking(ctx, ids)
	if err != nil {
		return CarHubsResult{}, err
	}
	borders, err := s.costs.BorderRules(ctx, home.CountryCode)
	if err != nil {
		return CarHubsResult{}, err
	}

	for _, h := range hubs {
		leg, ok := legs[h.Place.ID]
		if !ok {
			continue // по дороге не доехать
		}
		ch := CarHub{Hub: h, Leg: leg, DriveHours: float64(leg.DurationS) / 3600}

		var insurance carcost.Kop
		if h.CrossesBorder {
			rule, known := borders[h.Place.CountryCode]
			if !known {
				rule = carcost.BorderRule{Note: "Нет данных о правилах въезда на машине в эту страну — проверьте самостоятельно."}
			}
			ch.Border = &rule
			ch.DriveHours += rule.CrossingHours
			insurance = rule.Insurance
		}
		if q.MaxDriveHours > 0 && ch.DriveHours > q.MaxDriveHours {
			continue
		}

		perDay, known := parking[h.Place.ID]
		if !known {
			perDay, ch.ParkingEstimated = carcost.DefaultParkingPerDay, true
		}
		ch.Cost = carcost.Calc(carcost.Input{
			OneWayKm:        float64(leg.DistanceM) / 1000,
			ConsumptionL100: q.ConsumptionL100,
			FuelPrice:       res.FuelPrice,
			ParkingPerDay:   perDay,
			Days:            q.Days,
			BorderInsurance: insurance,
		})
		ch.PerPerson = ch.Cost.Total / carcost.Kop(max(q.Travelers, 1))
		res.Hubs = append(res.Hubs, ch)
	}
	sort.SliceStable(res.Hubs, func(i, j int) bool { return res.Hubs[i].DriveHours < res.Hubs[j].DriveHours })
	return res, nil
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
