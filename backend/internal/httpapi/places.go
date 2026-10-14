package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/guides"
	"github.com/SergeyLazarevv/Klubok/backend/internal/hotels"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
)

type handlers struct {
	geo    *geo.Service
	search *search.Service
	hotels *hotels.Ostrovok
	guides *guides.Service
	// Как добраться внутри города
	transfers *transfer.Service
	geocoder  *transfer.Photon
	favorites *favorites.Service
	log       *slog.Logger
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

func toAPIPlace(p geo.Place) apigen.Place {
	return apigen.Place{
		Id:          p.ID,
		Kind:        apigen.PlaceKind(p.Kind),
		Iata:        p.IATA,
		CityIata:    optional(p.CityIATA),
		CityName:    optional(p.CityName),
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
