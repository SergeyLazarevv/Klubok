package httpapi

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
)

func (h *handlers) ListStayPrices(ctx context.Context, _ apigen.ListStayPricesRequestObject) (apigen.ListStayPricesResponseObject, error) {
	prices, err := h.stay.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]apigen.StayPrice, len(prices))
	for i, p := range prices {
		items[i] = apigen.StayPrice{
			CityId:   p.CityID,
			PerNight: rub(p.Kop),
			Category: apigen.StayPriceCategory(p.Category),
			Source:   apigen.StayPriceSource(p.Source),
			Basis:    p.Basis,
			Period:   openapi_types.Date{Time: p.Period},
		}
	}
	return apigen.ListStayPrices200JSONResponse{Items: items}, nil
}
