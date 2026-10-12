// Package geo — справочник точек: города и транспортные узлы, автодополнение, поиск хабов в радиусе.
package geo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo/store"
)

// ErrNotFound — точки нет или она неактивна.
var ErrNotFound = errors.New("place not found")

type Kind string

const (
	KindCity    Kind = "city"
	KindAirport Kind = "airport"
)

type Place struct {
	ID          uuid.UUID
	Kind        Kind
	IATA        string
	CityIATA    string
	CityName    string // название города для аэропорта: «Гагарин» → «Саратов»
	Name        string
	NameEn      string
	NameTo      string // «в Самару»
	CountryCode string
	Lat, Lon    float64
	Timezone    string
}

// Hub — транспортный узел рядом с исходной точкой.
type Hub struct {
	Place         Place
	StraightKm    float64
	CrossesBorder bool
}

type Service struct {
	q *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: store.New(pool)}
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Place, error) {
	r, err := s.q.GetPlace(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Place{}, ErrNotFound
	}
	if err != nil {
		return Place{}, fmt.Errorf("get place: %w", err)
	}
	return placeFromRow(store.SearchPlacesRow(r)), nil
}

// Search ищет точки по началу названия или IATA-коду. kind пустой — любые.
func (s *Service) Search(ctx context.Context, query string, kind Kind, limit int) ([]Place, error) {
	rows, err := s.q.SearchPlaces(ctx, store.SearchPlacesParams{
		Query: query,
		Kind:  pgtype.Text{String: string(kind), Valid: kind != ""},
		Lim:   int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search places: %w", err)
	}
	places := make([]Place, len(rows))
	for i, r := range rows {
		places[i] = placeFromRow(r)
	}
	return places, nil
}

// AirportsByIATA возвращает аэропорты по кодам. Неизвестных кодов в ответе нет.
func (s *Service) AirportsByIATA(ctx context.Context, codes []string) (map[string]Place, error) {
	rows, err := s.q.ListAirportsByIATA(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("list airports by iata: %w", err)
	}
	out := make(map[string]Place, len(rows))
	for _, r := range rows {
		out[r.Iata] = placeFromRow(store.SearchPlacesRow(r))
	}
	return out, nil
}

// AirportsWithin возвращает аэропорты в радиусе от точки по прямой, ближайшие первыми.
func (s *Service) AirportsWithin(ctx context.Context, originID uuid.UUID, radiusKm int) ([]Hub, error) {
	origin, err := s.Get(ctx, originID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListAirportsWithin(ctx, store.ListAirportsWithinParams{
		OriginID: originID,
		RadiusM:  float64(radiusKm) * 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("list airports within: %w", err)
	}
	hubs := make([]Hub, len(rows))
	for i, r := range rows {
		p := placeFromRow(store.SearchPlacesRow{
			ID: r.ID, Kind: r.Kind, Iata: r.Iata, CityIata: r.CityIata, NameRu: r.NameRu, NameEn: r.NameEn,
			NameTo: r.NameTo, CountryCode: r.CountryCode, Lat: r.Lat, Lon: r.Lon, Timezone: r.Timezone,
			CityName: r.CityName,
		})
		hubs[i] = Hub{Place: p, StraightKm: r.DistanceKm, CrossesBorder: p.CountryCode != origin.CountryCode}
	}
	return hubs, nil
}

func placeFromRow(r store.SearchPlacesRow) Place {
	return Place{
		ID:          r.ID,
		Kind:        Kind(r.Kind),
		IATA:        r.Iata,
		CityIATA:    r.CityIata.String,
		CityName:    r.CityName.String,
		Name:        r.NameRu,
		NameEn:      r.NameEn.String,
		NameTo:      r.NameTo.String,
		CountryCode: r.CountryCode,
		Lat:         r.Lat,
		Lon:         r.Lon,
		Timezone:    r.Timezone,
	}
}
