package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/guides"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
)

func (h *handlers) GetPlaceGuide(ctx context.Context, req apigen.GetPlaceGuideRequestObject) (apigen.GetPlaceGuideResponseObject, error) {
	place, err := h.geo.Get(ctx, req.Id)
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.GetPlaceGuide404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	if err != nil {
		return nil, err
	}
	city, err := h.geo.City(ctx, place)
	if err != nil {
		return nil, err
	}
	g, err := h.guides.Get(ctx, city)
	switch {
	case errors.Is(err, guides.ErrNotFound):
		return apigen.GetPlaceGuide404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("guide")}, nil
	case errors.Is(err, guides.ErrSource):
		h.log.WarnContext(ctx, "place guide", "err", err)
		detail := "Викигид сейчас не отвечает, попробуйте позже"
		return apigen.GetPlaceGuide502ApplicationProblemPlusJSONResponse(apigen.Problem{
			Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail,
		}), nil
	case err != nil:
		return nil, err
	}

	resp := apigen.GetPlaceGuide200JSONResponse{
		Source:   apigen.Wikivoyage,
		Title:    g.Title,
		Url:      g.URL,
		Overview: optional(g.Overview),
		License:  g.License,
		Listings: make([]apigen.GuideListing, len(g.Listings)),
	}
	for i, l := range g.Listings {
		a := apigen.GuideListing{
			Type:        apigen.GuideListingType(l.Type),
			Name:        l.Name,
			Alt:         optional(l.Alt),
			Description: optional(l.Description),
			Address:     optional(l.Address),
			Url:         optional(l.URL),
			Hours:       optional(l.Hours),
			Price:       optional(l.Price),
			Wikidata:    optional(l.Wikidata),
		}
		if l.HasLocation {
			a.Location = &apigen.Point{Lat: l.Lat, Lon: l.Lon}
		}
		if l.Photo != nil {
			a.Photo = &apigen.Photo{Url: l.Photo.URL, Page: l.Photo.Page, Credit: optional(l.Photo.Credit)}
		}
		resp.Listings[i] = a
	}
	return resp, nil
}
