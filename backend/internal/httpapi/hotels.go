package httpapi

import (
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/hotels"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
)

func (h *handlers) GetHotelsLink(ctx context.Context, req apigen.GetHotelsLinkRequestObject) (apigen.GetHotelsLinkResponseObject, error) {
	p := req.Params
	provider := valueOr(p.Provider, apigen.Ostrovok)
	url, partner, problem, err := h.hotelsLink(ctx, provider, p.Place, p.CheckIn, p.CheckOut, p.Travelers)
	switch {
	case err != nil:
		return nil, err
	case problem == errNotFound:
		return apigen.GetHotelsLink404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case problem == errBadDates:
		return apigen.GetHotelsLink422ApplicationProblemPlusJSONResponse(badDatesProblem()), nil
	case problem == errBadProvider:
		return apigen.GetHotelsLink422ApplicationProblemPlusJSONResponse(badProviderProblem()), nil
	}
	return apigen.GetHotelsLink200JSONResponse{Url: url, Provider: provider, Partner: partner}, nil
}

func (h *handlers) GoToHotels(ctx context.Context, req apigen.GoToHotelsRequestObject) (apigen.GoToHotelsResponseObject, error) {
	p := req.Params
	url, _, problem, err := h.hotelsLink(ctx, valueOr(p.Provider, apigen.Ostrovok), p.Place, p.CheckIn, p.CheckOut, p.Travelers)
	switch {
	case err != nil:
		return nil, err
	case problem == errNotFound:
		return apigen.GoToHotels404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	case problem == errBadDates:
		return apigen.GoToHotels422ApplicationProblemPlusJSONResponse(badDatesProblem()), nil
	case problem == errBadProvider:
		return apigen.GoToHotels422ApplicationProblemPlusJSONResponse(badProviderProblem()), nil
	}
	return apigen.GoToHotels302Response{Headers: apigen.GoToHotels302ResponseHeaders{Location: &url}}, nil
}

type linkProblem int

const (
	noProblem linkProblem = iota
	errNotFound
	errBadDates
	errBadProvider
)

// hotelsLink — ссылка на жильё в городе точки (для аэропорта — в его городе) на даты поездки:
// отели Островка, жильё на Суточно.ру (Россия и СНГ) или квартиры посуточно на Авито (только Россия).
func (h *handlers) hotelsLink(ctx context.Context, provider apigen.HotelsProvider, placeID openapi_types.UUID, checkIn, checkOut openapi_types.Date, travelers *int) (string, bool, linkProblem, error) {
	if !provider.Valid() {
		return "", false, errBadProvider, nil
	}
	if !checkOut.After(checkIn.Time) {
		return "", false, errBadDates, nil
	}
	place, err := h.geo.Get(ctx, placeID)
	if errors.Is(err, geo.ErrNotFound) {
		return "", false, errNotFound, nil
	}
	if err != nil {
		return "", false, noProblem, err
	}
	city, err := h.geo.City(ctx, place)
	if err != nil {
		return "", false, noProblem, err
	}
	switch provider {
	case apigen.Avito:
		url, ok := hotels.AvitoLink(city)
		if !ok {
			return "", false, errNotFound, nil
		}
		return url, false, noProblem, nil
	case apigen.Sutochno:
		url, ok := hotels.SutochnoLink(city, checkIn.Time, checkOut.Time, valueOr(travelers, 1))
		if !ok {
			return "", false, errNotFound, nil
		}
		url, partner := h.hotels.Partner(ctx, url)
		return url, partner, noProblem, nil
	}
	// Ссылку отдаём, даже если Островок не успел подтвердить город: лучше догадка, чем ожидание
	lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	url, partner := h.hotels.Link(lctx, city, checkIn.Time, checkOut.Time, valueOr(travelers, 1))
	return url, partner, noProblem, nil
}

func badProviderProblem() apigen.Problem {
	return fieldProblem("provider", "ostrovok, sutochno или avito")
}

func badDatesProblem() apigen.Problem {
	return fieldProblem("check_out", "должна быть позже check_in")
}
