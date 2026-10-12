package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
)

func (h *handlers) ListRoadPois(ctx context.Context, req apigen.ListRoadPoisRequestObject) (apigen.ListRoadPoisResponseObject, error) {
	pois, err := h.search.RoadPOIs(ctx, req.Params.From, req.Params.To)
	switch {
	case errors.Is(err, geo.ErrNotFound):
		return apigen.ListRoadPois404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case errors.Is(err, poi.ErrSource):
		h.log.WarnContext(ctx, "road pois", "err", err)
		return apigen.ListRoadPois502ApplicationProblemPlusJSONResponse(poiSourceProblem()), nil
	case err != nil:
		return nil, err
	}
	return apigen.ListRoadPois200JSONResponse{Items: toAPIPois(pois)}, nil
}

func (h *handlers) ListPlacePois(ctx context.Context, req apigen.ListPlacePoisRequestObject) (apigen.ListPlacePoisResponseObject, error) {
	pois, err := h.search.PlacePOIs(ctx, req.Id)
	switch {
	case errors.Is(err, geo.ErrNotFound):
		return apigen.ListPlacePois404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case errors.Is(err, poi.ErrSource):
		h.log.WarnContext(ctx, "place pois", "err", err)
		return apigen.ListPlacePois502ApplicationProblemPlusJSONResponse(poiSourceProblem()), nil
	case err != nil:
		return nil, err
	}
	return apigen.ListPlacePois200JSONResponse{Items: toAPIPois(pois)}, nil
}

func (h *handlers) GetPoi(ctx context.Context, req apigen.GetPoiRequestObject) (apigen.GetPoiResponseObject, error) {
	p, err := h.search.POI(ctx, fmt.Sprintf("%s/%d", req.Type, req.Id))
	if errors.Is(err, poi.ErrNotFound) {
		return apigen.GetPoi404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("poi")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.GetPoi200JSONResponse(toAPIPoi(p)), nil
}

func poiSourceProblem() apigen.Problem {
	detail := "OpenStreetMap сейчас не отвечает, попробуйте позже"
	return apigen.Problem{Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail}
}

func toAPIPois(pois []poi.POI) []apigen.Poi {
	out := make([]apigen.Poi, len(pois))
	for i, p := range pois {
		out[i] = toAPIPoi(p)
	}
	return out
}

func toAPIPoi(p poi.POI) apigen.Poi {
	a := apigen.Poi{
		Id:             p.OSMID,
		Category:       apigen.PoiCategory(p.Category),
		Kind:           p.Kind,
		Name:           optional(p.Name),
		Brand:          optional(p.Brand),
		Location:       apigen.Point{Lat: p.Lat, Lon: p.Lon},
		OpeningHours:   optional(p.OpeningHours),
		Website:        optional(p.Website),
		Wikipedia:      optional(p.Wikipedia),
		Wikidata:       optional(p.Wikidata),
		Phone:          optional(p.Phone),
		Fee:            optional(p.Fee),
		Description:    optional(p.Description),
		DetailsPending: p.HasWikiRefs() && !p.DetailsLoaded,
	}
	if p.Cuisine != "" {
		var cuisine []string
		for _, c := range strings.Split(p.Cuisine, ";") {
			if c = strings.TrimSpace(c); c != "" {
				cuisine = append(cuisine, c)
			}
		}
		a.Cuisine = &cuisine
	}
	if p.PhotoURL != "" {
		a.Photo = &apigen.Photo{Url: p.PhotoURL, Page: p.PhotoPage, Credit: optional(p.PhotoCredit)}
	}
	return a
}
