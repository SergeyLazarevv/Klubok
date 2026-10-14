package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
)

func (h *handlers) Geocode(ctx context.Context, req apigen.GeocodeRequestObject) (apigen.GeocodeResponseObject, error) {
	city, err := h.geo.Get(ctx, req.Params.Near)
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.Geocode404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	if err != nil {
		return nil, err
	}
	found, err := h.geocoder.Search(ctx, req.Params.Q, transfer.Point{Lat: city.Lat, Lon: city.Lon}, 6)
	if err != nil {
		h.log.WarnContext(ctx, "geocode failed", "err", err)
		detail := "поиск адресов временно недоступен, попробуйте позже или поставьте точку на карте"
		return apigen.Geocode502ApplicationProblemPlusJSONResponse(apigen.Problem{
			Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail,
		}), nil
	}
	items := make([]apigen.Address, len(found))
	for i, a := range found {
		d := a.DistanceKm
		items[i] = apigen.Address{
			Label:      a.Label,
			Location:   apigen.Point{Lat: a.Point.Lat, Lon: a.Point.Lon},
			Kind:       optional(a.Kind),
			DistanceKm: &d,
		}
	}
	return apigen.Geocode200JSONResponse{Items: items}, nil
}

func (h *handlers) ListTransfers(ctx context.Context, req apigen.ListTransfersRequestObject) (apigen.ListTransfersResponseObject, error) {
	p := req.Params
	opts := h.transfers.Options(ctx,
		transfer.Point{Lat: p.FromLat, Lon: p.FromLon}, transfer.Point{Lat: p.ToLat, Lon: p.ToLon},
		valueOr(p.Country, ""), valueOr(p.Travelers, 1))

	items := make([]apigen.TransferOption, len(opts))
	for i, o := range opts {
		t := apigen.TransferOption{
			Mode:      apigen.TransferOptionMode(o.Mode),
			Title:     o.Title,
			PriceNote: optional(o.PriceNote),
			Links: make([]struct {
				Title string `json:"title"`
				Url   string `json:"url"`
			}, len(o.Links)),
		}
		for j, l := range o.Links {
			t.Links[j].Title, t.Links[j].Url = l.Title, l.URL
		}
		if o.DurationMin > 0 {
			d := o.DurationMin
			t.DurationMin = &d
		}
		if o.DistanceKm > 0 {
			d := o.DistanceKm
			t.DistanceKm = &d
		}
		if o.PriceKop > 0 {
			m := rub(o.PriceKop)
			t.Price = &m
		}
		if o.Approximate {
			a := true
			t.Approximate = &a
		}
		items[i] = t
	}
	return apigen.ListTransfers200JSONResponse{Items: items}, nil
}
