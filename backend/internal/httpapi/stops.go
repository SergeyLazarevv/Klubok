package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
	"github.com/SergeyLazarevv/Klubok/backend/internal/stay"
)

func (h *handlers) SuggestStops(ctx context.Context, req apigen.SuggestStopsRequestObject) (apigen.SuggestStopsResponseObject, error) {
	p := req.Params
	from, err := h.geo.Get(ctx, p.From)
	if err == nil {
		var to geo.Place
		if to, err = h.geo.Get(ctx, p.To); err == nil {
			return h.suggestStops(ctx, from, to, valueOr(p.MaxDriveHours, 8))
		}
	}
	if errors.Is(err, geo.ErrNotFound) {
		return apigen.SuggestStops404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("place")}, nil
	}
	return nil, err
}

func (h *handlers) suggestStops(ctx context.Context, from, to geo.Place, maxDriveHours float64) (apigen.SuggestStopsResponseObject, error) {
	direct, err := h.search.RoadRoute(ctx, from.ID, to.ID)
	if err != nil {
		return nil, err
	}
	resp := apigen.SuggestStops200JSONResponse{Direct: toAPIRoad(direct.Leg), Items: []apigen.StopSuggestion{}}
	if h.stops == nil {
		return resp, nil // справочник городов карты не загружен
	}
	list, err := h.stops.Suggest(ctx, placePoint(from), placePoint(to), direct, maxDriveHours)
	if err != nil {
		return nil, err
	}
	prices, err := h.stops.StayPerNight(ctx)
	if err != nil {
		h.log.WarnContext(ctx, "stay prices unavailable, stops without stay", "err", err)
	}
	for _, sg := range list {
		item := apigen.StopSuggestion{
			City:           toAPIStopCity(sg.City),
			Detour:         toAPIRoad(sg.Detour),
			Position:       sg.Position,
			DriveToHours:   sg.ToHours,
			DriveFromHours: sg.FromHours,
			Overnight:      sg.Overnight,
		}
		if kop := prices[sg.City.ID]; kop > 0 {
			m := rub(carcost.Kop(kop))
			item.StayPerNight = &m
		}
		resp.Items = append(resp.Items, item)
	}
	return resp, nil
}

func placePoint(p geo.Place) road.Point {
	return road.Point{ID: p.ID, Lat: p.Lat, Lon: p.Lon}
}

func toAPIStopCity(c stay.MapCity) apigen.StopCity {
	return apigen.StopCity{
		Id:         c.ID,
		Name:       c.Name,
		Location:   apigen.Point{Lat: c.Lat, Lon: c.Lon},
		Population: c.Pop,
		Arms:       c.Arms,
		Photo:      optional(c.Photo),
		Landmark:   optional(c.Landmark),
	}
}

// parseVia разбирает остановки «Q900:1» (формат проверил валидатор по схеме).
func parseVia(via *[]string) []search.ViaStop {
	var out []search.ViaStop
	for _, v := range derefSlice(via) {
		id, n, ok := strings.Cut(v, ":")
		nights, err := strconv.Atoi(n)
		if !ok || err != nil {
			continue
		}
		out = append(out, search.ViaStop{CityID: id, Nights: nights})
	}
	return out
}

func toAPIDriveStops(d search.DriveOption, t *apigen.DriveTrip) {
	if len(d.Stops) == 0 {
		return
	}
	stops := make([]apigen.DriveStop, len(d.Stops))
	for i, st := range d.Stops {
		stops[i] = apigen.DriveStop{City: toAPIStopCity(st.City), Nights: st.Nights}
		if st.PerNight > 0 {
			per := rub(st.PerNight)
			stops[i].StayPerNight = &per
			if st.Stay > 0 {
				total := rub(st.Stay)
				stops[i].Stay = &total
			}
		}
	}
	t.Stops = &stops
	if d.ViaLeg != nil {
		v := toAPIRoad(*d.ViaLeg)
		t.ViaRoad = &v
	}
	if d.Stay > 0 {
		s := rub(d.Stay)
		t.Stay = &s
	}
}
