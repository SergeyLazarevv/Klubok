package search

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

// destinationRadiusKm — в каком радиусе от точки назначения показывать достопримечательности.
const destinationRadiusKm = 15

// RoadPOIs — заправки, кафе и достопримечательности вдоль дороги между точками справочника.
func (s *Service) RoadPOIs(ctx context.Context, fromID, toID uuid.UUID) ([]poi.POI, error) {
	route, err := s.RoadRoute(ctx, fromID, toID)
	if err != nil {
		return nil, err
	}
	points, err := road.DecodePolyline(route.Polyline)
	if err != nil {
		return nil, fmt.Errorf("decode road polyline: %w", err)
	}
	line := make([]poi.Point, len(points))
	for i, p := range points {
		line[i] = poi.Point{Lat: p.Lat, Lon: p.Lon}
	}
	return s.pois.AlongLine(ctx, fmt.Sprintf("road:%s:%s", fromID, toID), line)
}

// POI — место с описанием и фото; см. poi.Service.Get.
func (s *Service) POI(ctx context.Context, osmID string) (poi.POI, error) {
	return s.pois.Get(ctx, osmID)
}

// PlacePOIs — достопримечательности вокруг точки справочника.
func (s *Service) PlacePOIs(ctx context.Context, id uuid.UUID) ([]poi.POI, error) {
	p, err := s.geo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.pois.Around(ctx, "place:"+id.String(), poi.Point{Lat: p.Lat, Lon: p.Lon},
		destinationRadiusKm, []poi.Category{poi.CategoryAttraction})
}
