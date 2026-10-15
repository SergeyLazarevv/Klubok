package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
)

// searchHorizon — насколько вперёд можно начать диапазон: дальше в кэше цен мало данных (см. docs/07-integrations.md).
const searchHorizon = 90 * 24 * time.Hour

// maxRangeDays — самый длинный диапазон дат; каждый захваченный месяц — отдельный запрос к источнику.
const maxRangeDays = 60

func (h *handlers) SearchTrips(ctx context.Context, req apigen.SearchTripsRequestObject) (apigen.SearchTripsResponseObject, error) {
	p := req.Params
	today := time.Now().Truncate(24 * time.Hour).UTC()
	dateFrom, dateTo := p.DateFrom.Time, p.DateTo.Time
	switch {
	case dateFrom.Before(today):
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(fieldProblem("date_from", "дата вылета должна быть сегодня или позже")), nil
	case dateFrom.After(today.Add(searchHorizon)):
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(fieldProblem("date_from", "начало поездки — не дальше чем через 3 месяца")), nil
	case dateTo.Before(dateFrom):
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(fieldProblem("date_to", "не может быть раньше даты начала")), nil
	case dateTo.After(dateFrom.AddDate(0, 0, maxRangeDays)):
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(fieldProblem("date_to", "диапазон дат — не больше 60 дней")), nil
	}
	q := search.TripsQuery{
		HomeID:        p.From,
		DestinationID: p.To,
		DateFrom:      dateFrom,
		DateTo:        dateTo,
		MinDays:       valueOr(p.MinDays, 3),
		MaxDays:       valueOr(p.MaxDays, 14),
		Travelers:     valueOr(p.Travelers, 1),
		UseCar:        valueOr(p.UseCar, false),
		Car:           carParams(p.RadiusKm, p.MaxDriveHours, p.ConsumptionL100, p.FuelType, p.FuelPriceRub, p.AllowBorder),
	}
	if q.MinDays > q.MaxDays {
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(fieldProblem("min_days", "не может быть больше max_days")), nil
	}

	res, err := h.search.Trips(ctx, q)
	switch {
	case errors.Is(err, geo.ErrNotFound):
		return apigen.SearchTrips404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case errors.Is(err, carcost.ErrNoFuelPrice):
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(noFuelPrice()), nil
	case errors.Is(err, search.ErrSameCity):
		return apigen.SearchTrips422ApplicationProblemPlusJSONResponse(fieldProblem("to", "направление совпадает с домашним городом")), nil
	case err != nil:
		h.log.ErrorContext(ctx, "trips search failed", "err", err)
		detail := "источник цен недоступен, попробуйте позже"
		return apigen.SearchTrips502ApplicationProblemPlusJSONResponse(apigen.Problem{
			Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail,
		}), nil
	}

	resp := apigen.SearchTrips200JSONResponse{
		Home:          toAPIPlace(res.Home),
		Destination:   toAPIPlace(res.Destination),
		Options:       make([]apigen.TripOption, len(res.Options)),
		NoData:        make([]apigen.Place, len(res.NoData)),
		Partial:       res.Partial,
		HiddenFlights: res.HiddenFlights,
	}
	if res.Drive != nil {
		d := toAPIDrive(*res.Drive, res.HomeBest)
		resp.Drive = &d
	}
	if res.DriveSkip != "" {
		skip := apigen.TripsResponseDriveSkipped(res.DriveSkip)
		resp.DriveSkipped = &skip
	}
	if res.FuelPrice > 0 {
		fp := rub(res.FuelPrice)
		resp.FuelPrice = &fp
	}
	for i, o := range res.Options {
		resp.Options[i] = toAPITrip(o)
	}
	if res.HomeBest != nil {
		hb := toAPITrip(*res.HomeBest)
		resp.HomeBest = &hb
	}
	for i, pl := range res.NoData {
		resp.NoData[i] = toAPIPlace(pl)
	}
	return resp, nil
}

func toAPITrip(o search.TripOption) apigen.TripOption {
	t := apigen.TripOption{
		Hub:             toAPIPlace(o.Hub.Hub.Place),
		DepartAt:        o.Quote.DepartAt,
		ReturnAt:        o.Quote.ReturnAt,
		TripDays:        o.TripDays,
		Airline:         o.Quote.Airline,
		Transfers:       o.Quote.Transfers,
		ReturnTransfers: o.Quote.ReturnTransfers,
		DurationToMin:   int(math.Round(o.Quote.DurationTo.Minutes())),
		DurationBackMin: int(math.Round(o.Quote.DurationBack.Minutes())),
		BookingUrl:      optional(o.Quote.BookingURL),
		TicketPerPerson: rub(o.Quote.Price),
		Tickets:         rub(o.Tickets),
		Total:           rub(o.Total),
		PerPerson:       rub(o.PerPerson),
		Warnings:        make([]apigen.TripOptionWarnings, len(o.Warnings)),
	}
	if !o.Quote.FoundOn.IsZero() {
		t.FoundOn = &openapi_types.Date{Time: o.Quote.FoundOn}
	}
	if o.Arrival != nil {
		a := toAPIPlace(*o.Arrival)
		t.Arrival = &a
	}
	if o.ByCar {
		t.Car = &apigen.TripCar{
			Road:       toAPIRoad(o.Hub.Leg),
			DriveHours: o.Hub.DriveHours,
			Cost:       toAPICarCost(o.Car),
			Border:     toAPIBorder(o.Hub.Border),
		}
	}
	if o.Savings != nil {
		s := rub(*o.Savings)
		t.SavingsVsHome = &s
	}
	for i, w := range o.Warnings {
		t.Warnings[i] = apigen.TripOptionWarnings(w)
	}
	return t
}

func toAPIDrive(d search.DriveOption, homeBest *search.TripOption) apigen.DriveTrip {
	t := apigen.DriveTrip{
		Road:       toAPIRoad(d.Leg),
		DriveHours: d.DriveHours,
		Cost:       toAPICarCost(d.Cost),
		Total:      rub(d.Total),
		PerPerson:  rub(d.PerPerson),
		TripDays:   d.TripDays,
		Border:     toAPIBorder(d.Border),
		Warnings:   make([]apigen.DriveTripWarnings, len(d.Warnings)),
	}
	for i, w := range d.Warnings {
		t.Warnings[i] = apigen.DriveTripWarnings(w)
	}
	// Сравнение с лучшим перелётом из своего города — как у вариантов с самолётом
	if homeBest != nil {
		s := rub(homeBest.Total - d.Total)
		t.SavingsVsHome = &s
	}
	return t
}
