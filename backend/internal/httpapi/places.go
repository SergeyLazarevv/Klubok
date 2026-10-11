package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
)

type handlers struct {
	geo *geo.Service
	log *slog.Logger
}

var _ apigen.StrictServerInterface = (*handlers)(nil)

func (h *handlers) SearchPlaces(ctx context.Context, req apigen.SearchPlacesRequestObject) (apigen.SearchPlacesResponseObject, error) {
	limit := 10
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	var kind geo.Kind
	if req.Params.Kind != nil {
		kind = geo.Kind(*req.Params.Kind)
	}
	places, err := h.geo.Search(ctx, req.Params.Q, kind, limit)
	if err != nil {
		return nil, err
	}
	items := make([]apigen.Place, len(places))
	for i, p := range places {
		items[i] = toAPIPlace(p)
	}
	return apigen.SearchPlaces200JSONResponse{Items: items}, nil
}

func (h *handlers) GetPlace(ctx context.Context, req apigen.GetPlaceRequestObject) (apigen.GetPlaceResponseObject, error) {
	p, err := h.geo.Get(ctx, req.Id)
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.GetPlace404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.GetPlace200JSONResponse(toAPIPlace(p)), nil
}

func (h *handlers) ListPlaceHubs(ctx context.Context, req apigen.ListPlaceHubsRequestObject) (apigen.ListPlaceHubsResponseObject, error) {
	radius := 100
	if req.Params.RadiusKm != nil {
		radius = *req.Params.RadiusKm
	}
	hubs, err := h.geo.AirportsWithin(ctx, req.Id, radius)
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.ListPlaceHubs404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]apigen.Hub, len(hubs))
	for i, hb := range hubs {
		crosses := hb.CrossesBorder
		items[i] = apigen.Hub{Place: toAPIPlace(hb.Place), StraightKm: hb.StraightKm, CrossesBorder: &crosses}
	}
	return apigen.ListPlaceHubs200JSONResponse{Items: items}, nil
}

func toAPIPlace(p geo.Place) apigen.Place {
	return apigen.Place{
		Id:          p.ID,
		Kind:        apigen.PlaceKind(p.Kind),
		Iata:        p.IATA,
		CityIata:    optional(p.CityIATA),
		Name:        p.Name,
		NameEn:      optional(p.NameEn),
		NameTo:      optional(p.NameTo),
		CountryCode: p.CountryCode,
		Location:    apigen.Point{Lat: p.Lat, Lon: p.Lon},
		Timezone:    p.Timezone,
	}
}

func notFound(what string) apigen.ProblemApplicationProblemPlusJSONResponse {
	detail := what + " not found"
	return apigen.ProblemApplicationProblemPlusJSONResponse{Status: http.StatusNotFound, Title: "Not Found", Detail: &detail}
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
