package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo/store"
)

// Открытые справочники Travelpayouts: коды совпадают с кодами в ценах на билеты.
const (
	citiesURL   = "https://api.travelpayouts.com/data/ru/cities.json"
	airportsURL = "https://api.travelpayouts.com/data/ru/airports.json"
)

type tpCoordinates struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type tpCity struct {
	Code             string            `json:"code"`
	Name             *string           `json:"name"`
	NameTranslations map[string]string `json:"name_translations"`
	Cases            map[string]string `json:"cases"`
	CountryCode      string            `json:"country_code"`
	TimeZone         string            `json:"time_zone"`
	Coordinates      *tpCoordinates    `json:"coordinates"`
}

type tpAirport struct {
	Code             string            `json:"code"`
	CityCode         string            `json:"city_code"`
	Name             *string           `json:"name"`
	NameTranslations map[string]string `json:"name_translations"`
	CountryCode      string            `json:"country_code"`
	TimeZone         string            `json:"time_zone"`
	Coordinates      *tpCoordinates    `json:"coordinates"`
	IATAType         string            `json:"iata_type"`
	Flightable       bool              `json:"flightable"`
}

// ImportStats — итог импорта.
type ImportStats struct {
	Cities, Airports, Skipped int
}

// Importer загружает справочник городов и аэропортов.
type Importer struct {
	pool   *pgxpool.Pool
	client *http.Client
}

func NewImporter(pool *pgxpool.Pool) *Importer {
	return &Importer{pool: pool, client: &http.Client{Timeout: 60 * time.Second}}
}

// Run скачивает справочники и обновляет таблицу places одной транзакцией.
func (im *Importer) Run(ctx context.Context) (ImportStats, error) {
	var cities []tpCity
	if err := im.fetch(ctx, citiesURL, &cities); err != nil {
		return ImportStats{}, err
	}
	var airports []tpAirport
	if err := im.fetch(ctx, airportsURL, &airports); err != nil {
		return ImportStats{}, err
	}
	params, stats := buildPlaces(cities, airports)

	tx, err := im.pool.Begin(ctx)
	if err != nil {
		return stats, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit — no-op
	q := store.New(tx)
	for _, p := range params {
		if err := q.UpsertPlace(ctx, p); err != nil {
			return stats, fmt.Errorf("upsert %s %s: %w", p.Kind, p.Iata, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

func (im *Importer) fetch(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := im.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("fetch %s: status %d: %s", url, resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// buildPlaces превращает справочники в строки для upsert.
// Берём все города (домашний город может быть без аэропорта) и только действующие аэропорты.
func buildPlaces(cities []tpCity, airports []tpAirport) ([]store.UpsertPlaceParams, ImportStats) {
	var stats ImportStats
	out := make([]store.UpsertPlaceParams, 0, len(cities)+len(airports))

	for _, c := range cities {
		if c.Code == "" || c.Name == nil || *c.Name == "" || c.Coordinates == nil || c.CountryCode == "" || c.TimeZone == "" {
			stats.Skipped++
			continue
		}
		out = append(out, store.UpsertPlaceParams{
			ID:          uuid.Must(uuid.NewV7()),
			Kind:        string(KindCity),
			Iata:        c.Code,
			CityIata:    text(c.Code),
			NameRu:      *c.Name,
			NameEn:      text(c.NameTranslations["en"]),
			NameTo:      text(c.Cases["vi"]),
			CountryCode: c.CountryCode,
			Lat:         c.Coordinates.Lat,
			Lon:         c.Coordinates.Lon,
			Timezone:    c.TimeZone,
			IsActive:    true,
		})
		stats.Cities++
	}

	for _, a := range airports {
		if a.IATAType != "airport" || !a.Flightable {
			continue
		}
		if a.Code == "" || a.Name == nil || *a.Name == "" || a.Coordinates == nil || a.CountryCode == "" || a.TimeZone == "" {
			stats.Skipped++
			continue
		}
		out = append(out, store.UpsertPlaceParams{
			ID:          uuid.Must(uuid.NewV7()),
			Kind:        string(KindAirport),
			Iata:        a.Code,
			CityIata:    text(a.CityCode),
			NameRu:      *a.Name,
			NameEn:      text(a.NameTranslations["en"]),
			CountryCode: a.CountryCode,
			Lat:         a.Coordinates.Lat,
			Lon:         a.Coordinates.Lon,
			Timezone:    a.TimeZone,
			IsActive:    true,
		})
		stats.Airports++
	}
	return out, stats
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
