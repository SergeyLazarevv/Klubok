package search

import (
	"context"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

// DriveSkip — почему вариант «только на машине» не предложен.
type DriveSkip string

const (
	DriveTooLong       DriveSkip = "too_long"           // дольше, чем пользователь готов ехать в одну сторону
	DriveBorderBlocked DriveSkip = "border_not_allowed" // другая страна, а ехать за границу пользователь не готов
	DriveNoRoad        DriveSkip = "no_road"            // по дороге не доехать (остров, другой континент)
)

// flightTimeGain — во сколько раз перелёт (с дорогой до аэропорта) должен быть быстрее машины,
// чтобы его стоило предлагать, если он не дешевле: 0.75 — минимум на четверть быстрее.
const flightTimeGain = 0.75

// hideUnworthyFlights — скрывать перелёты, которые не дешевле машины и не заметно быстрее.
// Пока выключено: пользователь смотрит все варианты, потом решим — скрывать или не считать вовсе.
const hideUnworthyFlights = false

// DriveOption — поездка на своей машине от дома до места и обратно, без самолёта.
type DriveOption struct {
	Leg        road.Leg
	DriveHours float64 // в одну сторону, с учётом границы
	Border     *carcost.BorderRule
	Cost       carcost.Cost // топливо туда и обратно и страховка; парковка на месте не считается
	Total      carcost.Kop
	PerPerson  carcost.Kop
	TripDays   int // для расчёта; даты у поездки на машине свободные
	Warnings   []Warning
}

// drive считает вариант «только на машине». nil и причина — вариант не подходит.
func (s *Service) drive(ctx context.Context, home, dest geo.Place, q TripsQuery, fuelPrice carcost.Kop) (*DriveOption, DriveSkip, error) {
	drives, skips, err := s.drives(ctx, home, []geo.Place{dest}, q, fuelPrice)
	if err != nil {
		return nil, "", err
	}
	return drives[dest.ID], skips[dest.ID], nil
}

// drives — варианты «только на машине» до нескольких мест: дорога — одним запросом к маршрутизатору (с кэшем).
// Места без варианта — в skips с причиной.
func (s *Service) drives(ctx context.Context, home geo.Place, dests []geo.Place, q TripsQuery, fuelPrice carcost.Kop) (map[uuid.UUID]*DriveOption, map[uuid.UUID]DriveSkip, error) {
	out := make(map[uuid.UUID]*DriveOption, len(dests))
	skips := make(map[uuid.UUID]DriveSkip)
	var rules map[string]carcost.BorderRule
	var points []road.Point
	for _, d := range dests {
		if d.CountryCode == home.CountryCode {
			points = append(points, roadPoint(d))
			continue
		}
		if !q.Car.AllowBorder {
			skips[d.ID] = DriveBorderBlocked
			continue
		}
		if rules == nil {
			var err error
			if rules, err = s.costs.BorderRules(ctx, home.CountryCode); err != nil {
				return nil, nil, err
			}
		}
		points = append(points, roadPoint(d))
	}
	legs, err := s.road.Legs(ctx, roadPoint(home), points)
	if err != nil {
		return nil, nil, err
	}
	for _, dest := range dests {
		if _, blocked := skips[dest.ID]; blocked {
			continue
		}
		leg, ok := legs[dest.ID]
		if !ok {
			skips[dest.ID] = DriveNoRoad
			continue
		}
		var border *carcost.BorderRule
		if dest.CountryCode != home.CountryCode {
			rule, known := rules[dest.CountryCode]
			if !known {
				rule = carcost.BorderRule{Note: "Нет данных о правилах въезда на машине в эту страну — проверьте самостоятельно."}
			}
			border = &rule
		}
		d, skip := newDrive(leg, border, q, fuelPrice)
		if d == nil {
			skips[dest.ID] = skip
			continue
		}
		out[dest.ID] = d
	}
	return out, skips, nil
}

func newDrive(leg road.Leg, border *carcost.BorderRule, q TripsQuery, fuelPrice carcost.Kop) (*DriveOption, DriveSkip) {
	d := &DriveOption{Leg: leg, DriveHours: float64(leg.DurationS) / 3600, Border: border, TripDays: q.MinDays}
	var insurance carcost.Kop
	if border != nil {
		d.DriveHours += border.CrossingHours
		insurance = border.Insurance
		d.Warnings = append(d.Warnings, WarnBorder)
	}
	if q.Car.MaxDriveHours > 0 && d.DriveHours > q.Car.MaxDriveHours {
		return nil, DriveTooLong
	}
	if leg.Approximate {
		d.Warnings = append(d.Warnings, WarnRoadApproximate)
	}
	d.Cost = carcost.Calc(carcost.Input{
		OneWayKm:        float64(leg.DistanceM) / 1000,
		ConsumptionL100: q.Car.ConsumptionL100,
		FuelPrice:       fuelPrice,
		BorderInsurance: insurance,
	})
	d.Total = d.Cost.Total
	d.PerPerson = d.Total / carcost.Kop(max(q.Travelers, 1))
	return d, ""
}

// doorToDoorHours — время туда от двери до места: дорога до аэропорта + перелёт.
// ok = false — длительность перелёта неизвестна.
func (o TripOption) doorToDoorHours() (float64, bool) {
	if o.Quote.DurationTo <= 0 {
		return 0, false
	}
	h := o.Quote.DurationTo.Hours()
	if o.ByCar {
		h += o.Hub.DriveHours
	}
	return h, true
}

// worthFlying — стоит ли предлагать перелёт, если можно доехать на машине: он дешевле
// или заметно быстрее (большое расстояние).
func worthFlying(o TripOption, d *DriveOption) bool {
	if d == nil || o.Total < d.Total {
		return true
	}
	h, ok := o.doorToDoorHours()
	return ok && h <= d.DriveHours*flightTimeGain
}
