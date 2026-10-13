package search

import (
	"context"

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
	var border *carcost.BorderRule
	if dest.CountryCode != home.CountryCode {
		if !q.Car.AllowBorder {
			return nil, DriveBorderBlocked, nil
		}
		rules, err := s.costs.BorderRules(ctx, home.CountryCode)
		if err != nil {
			return nil, "", err
		}
		rule, known := rules[dest.CountryCode]
		if !known {
			rule = carcost.BorderRule{Note: "Нет данных о правилах въезда на машине в эту страну — проверьте самостоятельно."}
		}
		border = &rule
	}

	legs, err := s.road.Legs(ctx, roadPoint(home), []road.Point{roadPoint(dest)})
	if err != nil {
		return nil, "", err
	}
	leg, ok := legs[dest.ID]
	if !ok {
		return nil, DriveNoRoad, nil
	}
	d := &DriveOption{Leg: leg, DriveHours: float64(leg.DurationS) / 3600, Border: border, TripDays: q.MinDays}
	var insurance carcost.Kop
	if border != nil {
		d.DriveHours += border.CrossingHours
		insurance = border.Insurance
		d.Warnings = append(d.Warnings, WarnBorder)
	}
	if q.Car.MaxDriveHours > 0 && d.DriveHours > q.Car.MaxDriveHours {
		return nil, DriveTooLong, nil
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
	return d, "", nil
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
