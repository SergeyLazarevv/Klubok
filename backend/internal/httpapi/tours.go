package httpapi

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/tours"
)

// maxDepartureKm — дальше этого от ближайшего города вылета горящие туры не предлагаем
// (например, для городов за границей).
const maxDepartureKm = 1000

func (h *handlers) ListHotTours(ctx context.Context, req apigen.ListHotToursRequestObject) (apigen.ListHotToursResponseObject, error) {
	p := req.Params
	q, km, ok, err := h.toursQuery(ctx, p.From, p.Travelers)
	if err != nil {
		return nil, err
	}
	if !ok {
		return apigen.ListHotTours404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("departure")}, nil
	}
	offers, err := h.tours.Hot(ctx, q)
	if err != nil {
		return nil, err
	}
	items := make([]apigen.HotTour, len(offers))
	for i, o := range offers {
		items[i] = apigen.HotTour{
			Country:       o.Destination.Country,
			Name:          o.Destination.Name,
			Nights:        o.Nights,
			DepartureDate: openapi_types.Date{Time: o.Departure},
			Url:           toursGoURL(p.From, o.Destination.Country, q.Travelers),
		}
		if o.PriceKop > 0 {
			price := rub(carcost.Kop(o.PriceKop))
			items[i].Price = &price
		}
	}
	return apigen.ListHotTours200JSONResponse{
		Departure: apigen.TourDeparture{Name: q.From.Name, NameFrom: q.From.NameFrom, DistanceKm: km},
		AllUrl:    toursGoURL(p.From, "", q.Travelers),
		Items:     items,
	}, nil
}

func (h *handlers) GoToTours(ctx context.Context, req apigen.GoToToursRequestObject) (apigen.GoToToursResponseObject, error) {
	p := req.Params
	q, _, ok, err := h.toursQuery(ctx, p.From, p.Travelers)
	if err != nil {
		return nil, err
	}
	if !ok {
		return apigen.GoToTours404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("departure")}, nil
	}
	link := h.tours.AllURL(q.From)
	if p.Country != nil {
		offers, err := h.tours.Hot(ctx, q)
		if err != nil {
			return nil, err
		}
		link = ""
		for _, o := range offers {
			if o.Destination.Country == *p.Country {
				link = o.URL
				break
			}
		}
		if link == "" {
			return apigen.GoToTours404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("country")}, nil
		}
	}
	lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	link, _ = h.tours.Partner(lctx, link)
	return apigen.GoToTours302Response{Headers: apigen.GoToTours302ResponseHeaders{Location: &link}}, nil
}

// toursQuery — город вылета, ближайший к городу пользователя; false — города нет или вылет слишком далеко.
func (h *handlers) toursQuery(ctx context.Context, from openapi_types.UUID, travelers *int) (tours.Query, float64, bool, error) {
	place, err := h.geo.Get(ctx, from)
	if errors.Is(err, geo.ErrNotFound) {
		return tours.Query{}, 0, false, nil
	}
	if err != nil {
		return tours.Query{}, 0, false, err
	}
	dep, km := h.tours.Nearest(place.Lat, place.Lon)
	if km > maxDepartureKm {
		return tours.Query{}, 0, false, nil
	}
	return tours.Query{From: dep, Travelers: valueOr(travelers, 1), Today: time.Now()}, km, true, nil
}

func toursGoURL(from openapi_types.UUID, country string, travelers int) string {
	v := url.Values{"from": {from.String()}, "travelers": {strconv.Itoa(travelers)}}
	if country != "" {
		v.Set("country", country)
	}
	return basePath + "/tours/go?" + v.Encode()
}
