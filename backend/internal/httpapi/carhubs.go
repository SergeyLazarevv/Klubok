package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
)

func (h *handlers) ListCarHubs(ctx context.Context, req apigen.ListCarHubsRequestObject) (apigen.ListCarHubsResponseObject, error) {
	p := req.Params
	q := search.CarHubsQuery{
		HomeID:          p.From,
		RadiusKm:        valueOr(p.RadiusKm, 100),
		MaxDriveHours:   valueOr(p.MaxDriveHours, 8),
		Days:            valueOr(p.Days, 7),
		Travelers:       valueOr(p.Travelers, 1),
		ConsumptionL100: valueOr(p.ConsumptionL100, 8),
		Fuel:            carcost.FuelType(valueOr(p.FuelType, apigen.Ai95)),
		AllowBorder:     valueOr(p.AllowBorder, false),
	}
	if p.FuelPriceRub != nil {
		q.FuelPrice = carcost.Kop(math.Round(*p.FuelPriceRub * 100))
	}

	res, err := h.search.CarHubs(ctx, q)
	switch {
	case errors.Is(err, geo.ErrNotFound):
		return apigen.ListCarHubs404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case errors.Is(err, carcost.ErrNoFuelPrice):
		return apigen.ListCarHubs422ApplicationProblemPlusJSONResponse(fieldProblem("fuel_price_rub",
			"нет справочной цены топлива для страны — укажите свою цену")), nil
	case err != nil:
		return nil, err
	}

	items := make([]apigen.CarHub, len(res.Hubs))
	for i, ch := range res.Hubs {
		items[i] = apigen.CarHub{
			Place:      toAPIPlace(ch.Hub.Place),
			StraightKm: ch.Hub.StraightKm,
			Road: apigen.RoadLeg{
				DistanceKm:  float64(ch.Leg.DistanceM) / 1000,
				DurationMin: int(math.Round(float64(ch.Leg.DurationS) / 60)),
				Approximate: ch.Leg.Approximate,
			},
			DriveHours: ch.DriveHours,
			Cost: apigen.CarCost{
				Fuel:    rub(ch.Cost.Fuel),
				Parking: rub(ch.Cost.Parking),
				Border:  rub(ch.Cost.Border),
				Total:   rub(ch.Cost.Total),
			},
			PerPerson:        rub(ch.PerPerson),
			ParkingEstimated: ch.ParkingEstimated,
		}
		if ch.Border != nil {
			items[i].Border = &apigen.BorderInfo{
				Insurance:     rub(ch.Border.Insurance),
				CrossingHours: ch.Border.CrossingHours,
				Note:          ch.Border.Note,
			}
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
