package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
)

func (h *handlers) ListCarHubs(ctx context.Context, req apigen.ListCarHubsRequestObject) (apigen.ListCarHubsResponseObject, error) {
	p := req.Params
	q := search.CarHubsQuery{
		HomeID:    p.From,
		Car:       carParams(p.RadiusKm, p.MaxDriveHours, p.ConsumptionL100, p.FuelType, p.FuelPriceRub, p.AllowBorder),
		Days:      valueOr(p.Days, 7),
		Travelers: valueOr(p.Travelers, 1),
	}

	res, err := h.search.CarHubs(ctx, q)
	switch {
	case errors.Is(err, geo.ErrNotFound):
		return apigen.ListCarHubs404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case errors.Is(err, carcost.ErrNoFuelPrice):
		return apigen.ListCarHubs422ApplicationProblemPlusJSONResponse(noFuelPrice()), nil
	case err != nil:
		return nil, err
	}

	items := make([]apigen.CarHub, len(res.Hubs))
	for i, ch := range res.Hubs {
		items[i] = apigen.CarHub{
			Place:            toAPIPlace(ch.Hub.Place),
			StraightKm:       ch.Hub.StraightKm,
			Road:             toAPIRoad(ch.Leg),
			DriveHours:       ch.DriveHours,
			Cost:             toAPICarCost(ch.Cost),
			PerPerson:        rub(ch.PerPerson),
			ParkingEstimated: ch.ParkingEstimated,
			Border:           toAPIBorder(ch.Border),
		}
	}
	return apigen.ListCarHubs200JSONResponse{
		Home:            toAPIPlace(res.Home),
		FuelPrice:       rub(res.FuelPrice),
		FuelPriceSource: apigen.CarHubsResponseFuelPriceSource(res.FuelSource),
		Items:           items,
	}, nil
}

func rub(kop carcost.Kop) apigen.Money {
	return apigen.Money{Amount: kop, Currency: apigen.RUB}
}

func valueOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

func fieldProblem(field, msg string) apigen.Problem {
	errs := []struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}{{Field: field, Message: msg}}
	return apigen.Problem{Status: http.StatusUnprocessableEntity, Title: "Validation failed", Errors: &errs}
}

func carParams(radius *int, hours *float64, consumption *float64, fuel *apigen.FuelType, price *float64, border *bool) search.CarParams {
	c := search.CarParams{
		RadiusKm:        valueOr(radius, 100),
		MaxDriveHours:   valueOr(hours, 8),
		ConsumptionL100: valueOr(consumption, 8),
		Fuel:            carcost.FuelType(valueOr(fuel, apigen.Ai95)),
		AllowBorder:     valueOr(border, false),
	}
	if price != nil {
		c.FuelPrice = carcost.Kop(math.Round(*price * 100))
	}
	return c
}

func toAPIRoad(l road.Leg) apigen.RoadLeg {
	return apigen.RoadLeg{
		DistanceKm:  float64(l.DistanceM) / 1000,
		DurationMin: int(math.Round(float64(l.DurationS) / 60)),
		Approximate: l.Approximate,
	}
}

func toAPICarCost(c carcost.Cost) apigen.CarCost {
	return apigen.CarCost{Fuel: rub(c.Fuel), Parking: rub(c.Parking), Border: rub(c.Border), Total: rub(c.Total)}
}

func toAPIBorder(b *carcost.BorderRule) *apigen.BorderInfo {
	if b == nil {
		return nil
	}
	return &apigen.BorderInfo{Insurance: rub(b.Insurance), CrossingHours: b.CrossingHours, Note: b.Note}
}

func noFuelPrice() apigen.Problem {
	return fieldProblem("fuel_price_rub", "нет справочной цены топлива для страны — укажите свою цену")
}
