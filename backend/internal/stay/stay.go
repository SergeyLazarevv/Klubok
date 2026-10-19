// Package stay — ориентировочная цена жилья по городам карты (ADR-0021): средняя цена суток в гостинице 3*
// с человека по данным Росстата. Хранится в БД, отдаётся через кэш Redis; обновляет команда stay-update.
package stay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/SergeyLazarevv/Klubok/backend/internal/stay/store"
)

const (
	SourceRosstat  = "rosstat"
	CategoryHotel3 = "hotel3"
	BasisCountry   = "Россия"

	cacheKey = "klubok:stay-prices:v1"
	cacheTTL = 7 * 24 * time.Hour // страховка: обычно кэш сбрасывает stay-update

	// nearestKm — до какого расстояния брать цену ближайшего города наблюдения; дальше — средняя по России
	nearestKm = 250
)

// Price — цена суток с человека для города карты.
type Price struct {
	CityID   string    `json:"city_id"`
	CityName string    `json:"city_name"`
	Kop      int64     `json:"kop"`
	Category string    `json:"category"`
	Source   string    `json:"source"`
	Basis    string    `json:"basis"`
	Period   time.Time `json:"period"`
}

type Service struct {
	q     *store.Queries
	pool  *pgxpool.Pool
	cache *redis.Client // nil — без кэша, всегда из БД
	log   *slog.Logger
}

func NewService(pool *pgxpool.Pool, cache *redis.Client, log *slog.Logger) *Service {
	return &Service{q: store.New(pool), pool: pool, cache: cache, log: log}
}

// List — все цены: из кэша, а если его нет — из БД с записью в кэш. Недоступный Redis не мешает отдать данные.
func (s *Service) List(ctx context.Context) ([]Price, error) {
	if s.cache != nil {
		data, err := s.cache.Get(ctx, cacheKey).Bytes()
		switch {
		case err == nil:
			var prices []Price
			if err := json.Unmarshal(data, &prices); err == nil {
				return prices, nil
			}
		case !errors.Is(err, redis.Nil):
			s.log.WarnContext(ctx, "stay cache get", slog.Any("err", err))
		}
	}
	rows, err := s.q.ListStayPrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stay prices: %w", err)
	}
	prices := make([]Price, len(rows))
	for i, r := range rows {
		prices[i] = Price{CityID: r.CityID, CityName: r.CityName, Kop: r.PriceKop, Category: r.Category,
			Source: r.Source, Basis: r.Basis, Period: r.Period.Time}
	}
	if s.cache != nil {
		if data, err := json.Marshal(prices); err == nil {
			if err := s.cache.Set(ctx, cacheKey, data, cacheTTL).Err(); err != nil {
				s.log.WarnContext(ctx, "stay cache set", slog.Any("err", err))
			}
		}
	}
	return prices, nil
}

// Replace записывает новый расчёт в БД (удаляя города, которых в нём нет) и сбрасывает кэш.
func (s *Service) Replace(ctx context.Context, prices []Price) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit — no-op
	q := s.q.WithTx(tx)
	var started pgtype.Timestamptz
	if err := tx.QueryRow(ctx, "SELECT now()").Scan(&started); err != nil {
		return err
	}
	params := make([]store.UpsertStayPricesParams, len(prices))
	for i, p := range prices {
		params[i] = store.UpsertStayPricesParams{CityID: p.CityID, CityName: p.CityName, PriceKop: p.Kop, Category: p.Category,
			Source: p.Source, Basis: p.Basis, Period: pgtype.Date{Time: p.Period, Valid: true}}
	}
	var batchErr error
	q.UpsertStayPrices(ctx, params).Exec(func(_ int, err error) {
		if err != nil && batchErr == nil {
			batchErr = err
		}
	})
	if batchErr != nil {
		return fmt.Errorf("upsert stay prices: %w", batchErr)
	}
	// now() в транзакции одно и то же: обновлённые строки получили updated_at = started, остальные — старше
	if err := q.DeleteStalePrices(ctx, started); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if s.cache != nil {
		if err := s.cache.Del(ctx, cacheKey).Err(); err != nil {
			s.log.WarnContext(ctx, "stay cache invalidate", slog.Any("err", err))
		}
	}
	return nil
}

// MapCity — город карты из cities.geojson.
type MapCity struct {
	ID, Name, Country string
	Pop               int
	Lat, Lon          float64
	Arms              bool   // есть герб: /data/arms/<id>.webp
	Photo, Landmark   string // фото на Викискладе и главная достопримечательность; могут быть пустыми
}

// LoadCities читает справочник городов карты (frontend/public/data/cities.geojson).
func LoadCities(path string) ([]MapCity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fc struct {
		Features []struct {
			Geometry struct {
				Coordinates [2]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Pop      int    `json:"pop"`
				Country  string `json:"country"`
				Arms     bool   `json:"arms"`
				Photo    string `json:"photo"`
				Landmark string `json:"landmark"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("cities geojson: %w", err)
	}
	out := make([]MapCity, 0, len(fc.Features))
	for _, f := range fc.Features {
		p := f.Properties
		out = append(out, MapCity{ID: p.ID, Name: p.Name, Country: p.Country, Pop: p.Pop, Lon: f.Geometry.Coordinates[0], Lat: f.Geometry.Coordinates[1],
			Arms: p.Arms, Photo: p.Photo, Landmark: p.Landmark})
	}
	return out, nil
}

// Assign — цена для каждого города карты в России: своя, если Росстат его наблюдает, иначе ближайшего
// города наблюдения в пределах nearestKm, иначе средняя по России. Зарубежные города — без цены.
func Assign(cities []MapCity, territories []Territory) []Price {
	type observed struct {
		name     string
		lat, lon float64
		obs      Observation
	}
	// Города наблюдения получают координаты своего тёзки на карте (крупнейшего, если тёзок несколько)
	byName := make(map[string]MapCity)
	for _, c := range cities {
		if c.Country != "" {
			continue
		}
		k := norm(c.Name)
		if cur, ok := byName[k]; !ok || c.Pop > cur.Pop {
			byName[k] = c
		}
	}
	var country Observation
	var points []observed
	for _, t := range territories {
		obs := t.Hotel3
		if t.Level == LevelCountry && obs.Kop > 0 && country.Kop == 0 {
			country = obs
		}
		if t.Level != LevelCity || obs.Kop == 0 {
			continue
		}
		if c, ok := byName[norm(cityName(t.Name))]; ok {
			points = append(points, observed{name: cityName(t.Name), lat: c.Lat, lon: c.Lon, obs: obs})
		}
	}

	var out []Price
	for _, c := range cities {
		if c.Country != "" {
			continue
		}
		best, bestKm := -1, math.Inf(1)
		for i, p := range points {
			if km := distanceKm(c.Lat, c.Lon, p.lat, p.lon); km < bestKm {
				best, bestKm = i, km
			}
		}
		price := Price{CityID: c.ID, CityName: c.Name, Category: CategoryHotel3, Source: SourceRosstat}
		switch {
		case best >= 0 && bestKm <= nearestKm:
			price.Kop, price.Period, price.Basis = points[best].obs.Kop, points[best].obs.Month, points[best].name
		case country.Kop > 0:
			price.Kop, price.Period, price.Basis = country.Kop, country.Month, BasisCountry
		default:
			continue
		}
		out = append(out, price)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CityID < out[j].CityID })
	return out
}

// cityName — «Город Москва столица … федерального значения» → «Москва».
func cityName(title string) string {
	for _, c := range []string{"Москва", "Санкт-Петербург", "Севастополь"} {
		if strings.Contains(title, c) {
			return c
		}
	}
	return title
}

func norm(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "ё", "е")
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * 6371 * math.Asin(math.Sqrt(h))
}
