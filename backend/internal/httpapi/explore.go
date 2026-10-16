package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
)

func (h *handlers) ExploreDestinations(ctx context.Context, req apigen.ExploreDestinationsRequestObject) (apigen.ExploreDestinationsResponseObject, error) {
	p := req.Params
	if problem := checkDates(p.DateFrom.Time, p.DateTo.Time); problem != nil {
		return apigen.ExploreDestinations422ApplicationProblemPlusJSONResponse(*problem), nil
	}
	q := search.TripsQuery{
		HomeID:    p.From,
		DateFrom:  p.DateFrom.Time,
		DateTo:    p.DateTo.Time,
		MinDays:   valueOr(p.MinDays, 3),
		MaxDays:   valueOr(p.MaxDays, 14),
		Travelers: valueOr(p.Travelers, 1),
		UseCar:    valueOr(p.UseCar, false),
		Car:       carParams(p.RadiusKm, p.MaxDriveHours, p.ConsumptionL100, p.FuelType, p.FuelPriceRub, p.AllowBorder),
		Weekends:  valueOr(p.Weekends, false),
	}
	if q.MinDays > q.MaxDays {
		return apigen.ExploreDestinations422ApplicationProblemPlusJSONResponse(fieldProblem("min_days", "не может быть больше max_days")), nil
	}

	res, err := h.search.Explore(ctx, q)
	switch {
	case errors.Is(err, geo.ErrNotFound):
		return apigen.ExploreDestinations404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case errors.Is(err, carcost.ErrNoFuelPrice):
		return apigen.ExploreDestinations422ApplicationProblemPlusJSONResponse(noFuelPrice()), nil
	case err != nil:
		h.log.ErrorContext(ctx, "explore failed", "err", err)
		detail := "источник цен недоступен, попробуйте позже"
		return apigen.ExploreDestinations502ApplicationProblemPlusJSONResponse(apigen.Problem{
			Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail,
		}), nil
	}

	resp := apigen.ExploreDestinations200JSONResponse{
		Home:        toAPIPlace(res.Home),
		Partial:     res.Partial,
		DriveBetter: res.DriveBetter,
		Items:       make([]apigen.ExploreItem, len(res.Destinations)),
	}
	if res.FuelPrice > 0 {
		fp := rub(res.FuelPrice)
		resp.FuelPrice = &fp
	}
	if len(res.BorderRules) > 0 {
		rules := make([]apigen.BorderRule, 0, len(res.BorderRules))
		for cc, r := range res.BorderRules {
			rules = append(rules, apigen.BorderRule{CountryCode: cc, Insurance: rub(r.Insurance), CrossingHours: r.CrossingHours, Note: r.Note})
		}
		resp.BorderRules = &rules
	}
	for i, d := range res.Destinations {
		resp.Items[i] = apigen.ExploreItem{City: toAPIPlace(d.City), Best: toAPITrip(d.Best)}
		if d.HomeBest != nil {
			hb := toAPITrip(*d.HomeBest)
			resp.Items[i].HomeBest = &hb
		}
		if d.Drive != nil {
			dr := toAPIDrive(*d.Drive, d.HomeBest)
			resp.Items[i].Drive = &dr
		}
	}
	return resp, nil
}
