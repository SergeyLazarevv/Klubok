package httpapi

import (
	"context"
	"errors"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
)

func (h *handlers) GetRoadRoute(ctx context.Context, req apigen.GetRoadRouteRequestObject) (apigen.GetRoadRouteResponseObject, error) {
	route, err := h.search.RoadRoute(ctx, req.Params.From, req.Params.To)
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.GetRoadRoute404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.GetRoadRoute200JSONResponse{Road: toAPIRoad(route.Leg), Polyline: route.Polyline}, nil
}
