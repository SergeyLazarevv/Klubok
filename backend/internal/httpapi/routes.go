package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

func (h *handlers) GetRoadRoute(ctx context.Context, req apigen.GetRoadRouteRequestObject) (apigen.GetRoadRouteResponseObject, error) {
	var via []road.Point
	for _, v := range derefSlice(req.Params.Via) {
		p, ok := parseLatLon(v)
		if !ok {
			return apigen.GetRoadRoute422ApplicationProblemPlusJSONResponse(fieldProblem("via", "нужно «широта,долгота»")), nil
		}
		via = append(via, p)
	}
	route, err := h.search.RoadRoute(ctx, req.Params.From, req.Params.To, via...)
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.GetRoadRoute404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.GetRoadRoute200JSONResponse{Road: toAPIRoad(route.Leg), Polyline: route.Polyline}, nil
}

// parseLatLon разбирает «53.2415,50.2212» — точку, куда заехать по пути.
func parseLatLon(s string) (road.Point, bool) {
	lat, lon, ok := strings.Cut(s, ",")
	if !ok {
		return road.Point{}, false
	}
	la, err1 := strconv.ParseFloat(lat, 64)
	lo, err2 := strconv.ParseFloat(lon, 64)
	if err1 != nil || err2 != nil || la < -90 || la > 90 || lo < -180 || lo > 180 {
		return road.Point{}, false
	}
	return road.Point{Lat: la, Lon: lo}, true
}

func derefSlice[T any](s *[]T) []T {
	if s == nil {
		return nil
	}
	return *s
}
