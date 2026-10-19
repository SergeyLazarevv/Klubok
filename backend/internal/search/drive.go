package search

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/stay"
)

// DriveSkip — почему вариант «только на машине» не предложен.
type DriveSkip string

const (
	DriveTooLong       DriveSkip = "too_long"           // дольше, чем пользователь готов ехать в одну сторону
	DriveBorderBlocked DriveSkip = "border_not_allowed" // другая страна, а ехать за границу пользователь не готов
	DriveNoRoad        DriveSkip = "no_road"            // по дороге не доехать (остров, другой континент)
)

// maxRoadDays — дорога дольше стольких дней за рулём (по лимиту в день) не предлагается: это уже не «доехать»,
// а путешествие по дороге.
const maxRoadDays = 4

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

	// С остановками по пути (TripsQuery.Via): туда — через них, обратно — напрямую
	Stops  []DriveStop
	ViaLeg *road.Leg   // дорога туда через остановки
	Stay   carcost.Kop // ночёвки в остановках на всех; входит в Total
	parts  []road.Leg  // участки дороги туда между остановками
}

// DriveStop — остановка по пути с ценой ночёвок.
type DriveStop struct {
	City     stay.MapCity
	Nights   int
	PerNight carcost.Kop // сутки с человека; 0 — цены нет
	Stay     carcost.Kop // ночёвки на всех
}

// drive считает вариант «только на машине». nil и причина — вариант не подходит.
func (s *Service) drive(ctx context.Context, home, dest geo.Place, q TripsQuery, fuelPrice carcost.Kop) (*DriveOption, DriveSkip, error) {
	stops := s.viaStops(q.Via)
	// Длинную дорогу не прячем и сами её не делим: пользователь видит города по пути и сам выбирает,
	// где заехать и где переночевать (ADR-0023). С ночёвками лимит «за рулём» — на каждый день
	relaxed := s.stops != nil && q.Car.MaxDriveHours > 0
	dq := q
	if relaxed {
		dq.Car.MaxDriveHours = 0
	}
	drives, skips, err := s.drives(ctx, home, []geo.Place{dest}, dq, fuelPrice)
	if err != nil {
		return nil, "", err
	}
	d := drives[dest.ID]
	if d == nil {
		return nil, skips[dest.ID], nil
	}
	if len(stops) > 0 {
		if err := s.applyStops(ctx, d, home, dest, stops, q, fuelPrice); err != nil {
			return nil, "", err
		}
	}
	if !relaxed {
		return d, "", nil
	}
	if d.DriveHours > q.Car.MaxDriveHours*maxRoadDays {
		return nil, DriveTooLong, nil
	}
	if longestDay(d) > q.Car.MaxDriveHours {
		d.Warnings = append(d.Warnings, WarnLongDriveDay)
	}
	return d, "", nil
}

// viaStops — остановки из запроса; неизвестные города (справочник обновился) пропускаем.
func (s *Service) viaStops(via []ViaStop) []DriveStop {
	if s.stops == nil {
		return nil
	}
	var out []DriveStop
	for _, v := range via {
		if c, ok := s.stops.City(v.CityID); ok {
			out = append(out, DriveStop{City: c, Nights: v.Nights})
		}
	}
	return out
}

func overnight(stops []DriveStop) bool {
	for _, st := range stops {
		if st.Nights > 0 {
			return true
		}
	}
	return false
}

// applyStops — дорога туда через остановки и ночёвки в них. Бензин — туда через остановки и обратно напрямую,
// ночёвка — сутки в гостинице 3* × люди × ночи (Росстат, ADR-0021).
func (s *Service) applyStops(ctx context.Context, d *DriveOption, home, dest geo.Place, stops []DriveStop, q TripsQuery, fuelPrice carcost.Kop) error {
	via := make([]road.Point, len(stops))
	for i, st := range stops {
		via[i] = road.Point{Lat: st.City.Lat, Lon: st.City.Lon}
	}
	route, err := s.road.Route(ctx, roadPoint(home), roadPoint(dest), via...)
	if err != nil {
		return err
	}
	prices, err := s.stops.StayPerNight(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "stay prices unavailable, stops without stay", "err", err)
	}
	travelers := carcost.Kop(max(q.Travelers, 1))
	d.Stay = 0
	for i := range stops {
		st := &stops[i]
		st.PerNight = carcost.Kop(prices[st.City.ID])
		st.Stay = st.PerNight * travelers * carcost.Kop(st.Nights)
		d.Stay += st.Stay
	}
	d.Stops = stops
	d.ViaLeg = &route.Leg
	d.parts = route.Parts
	if route.Approximate && !slices.Contains(d.Warnings, WarnRoadApproximate) {
		d.Warnings = append(d.Warnings, WarnRoadApproximate)
	}
	d.Cost = carcost.Calc(carcost.Input{
		OneWayKm:        float64(route.DistanceM+d.Leg.DistanceM) / 2 / 1000, // Calc считает туда и обратно
		ConsumptionL100: q.Car.ConsumptionL100,
		FuelPrice:       fuelPrice,
		BorderInsurance: d.Cost.Border,
	})
	d.Total = d.Cost.Total + d.Stay
	d.PerPerson = d.Total / travelers
	return nil
}

// longestDay — самый долгий день за рулём на пути туда: дорога делится на дни ночёвками в остановках.
// Без деления на участки (маршрутизатор их не вернул) — вся дорога через остановки.
func longestDay(d *DriveOption) float64 {
	if d.ViaLeg == nil {
		return d.DriveHours
	}
	border := d.DriveHours - float64(d.Leg.DurationS)/3600 // время на границе
	if len(d.parts) != len(d.Stops)+1 {
		return float64(d.ViaLeg.DurationS)/3600 + border
	}
	var longest, day float64
	for i, p := range d.parts {
		day += float64(p.DurationS) / 3600
		if i < len(d.Stops) && d.Stops[i].Nights > 0 {
			longest = max(longest, day)
			day = 0
		}
	}
	return max(longest, day+border)
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
