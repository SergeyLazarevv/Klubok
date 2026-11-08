// Package mapsearch — поиск по карте: известные места из Wikidata и всё остальное (сёла, улицы, адреса)
// из OpenStreetMap через Photon.
package mapsearch

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/redis/go-redis/v9"

	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
)

type photon interface {
	Find(ctx context.Context, q string, near *transfer.Point, zoom, limit int) ([]transfer.Found, error)
}

type wikidata interface {
	Find(ctx context.Context, q string, limit int) ([]transfer.Found, error)
}

type Service struct {
	photon photon
	wiki   wikidata
	cache  *redis.Client // nil — без кэша
	log    *slog.Logger
}

func NewService(p *transfer.Photon, w *Wikidata, cache *redis.Client, log *slog.Logger) *Service {
	return &Service{photon: p, wiki: w, cache: cache, log: log}
}

const (
	// Ответ Wikidata не зависит от карты — один на всех; известные места меняются редко
	wikiCacheTTL = 7 * 24 * time.Hour
	// Wikidata отвечает за два запроса; дольше не ждём — хватит и Photon
	wikiBudget = 3 * time.Second
	wikiLimit  = 10
	// Сколько известных мест ставить перед ближними из OSM
	wikiTop = 3
)

// Search — места по запросу; near и zoom — вид карты, ближние места при равной известности выше.
func (s *Service) Search(ctx context.Context, q string, near *transfer.Point, zoom, limit int) ([]transfer.Found, error) {
	var (
		wg              sync.WaitGroup
		wiki, osm       []transfer.Found
		wikiErr, osmErr error
	)
	// В адресе есть номер дома — это не достопримечательность, Wikidata не нужна
	if !hasDigit(q) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wiki, wikiErr = s.wikiCached(ctx, q)
		}()
	}
	osm, osmErr = s.photon.Find(ctx, q, near, zoom, limit*2)
	wg.Wait()
	if wikiErr != nil {
		s.log.WarnContext(ctx, "map search: wikidata failed", "err", wikiErr)
	}
	if osmErr != nil && (wikiErr != nil || hasDigit(q)) {
		return nil, osmErr
	}
	if osmErr != nil {
		s.log.WarnContext(ctx, "map search: photon failed", "err", osmErr)
	}
	return merge(wiki, osm, limit), nil
}

func (s *Service) wikiCached(ctx context.Context, q string) ([]transfer.Found, error) {
	key := "mapsearch:wd:" + strings.ToLower(strings.Join(strings.Fields(q), " "))
	if s.cache != nil {
		if b, err := s.cache.Get(ctx, key).Bytes(); err == nil {
			var out []transfer.Found
			if json.Unmarshal(b, &out) == nil {
				return out, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			s.log.WarnContext(ctx, "map search: cache read failed", "err", err)
		}
	}
	wctx, cancel := context.WithTimeout(ctx, wikiBudget)
	defer cancel()
	out, err := s.wiki.Find(wctx, q, wikiLimit)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		if b, err := json.Marshal(out); err == nil {
			_ = s.cache.Set(ctx, key, b, wikiCacheTTL).Err()
		}
	}
	return out, nil
}

// merge: сначала самые известные места, затем города, сёла и природа рядом, затем остальное известное,
// в конце улицы, адреса и организации. Одно и то же место из двух источников — одной строкой.
func merge(wiki, osm []transfer.Found, limit int) []transfer.Found {
	var near, rest []transfer.Found
	for _, o := range osm {
		if i := sameIndex(wiki, o); i >= 0 {
			// У OSM есть границы — по ним карта покажет место целиком
			if wiki[i].BBox == nil {
				wiki[i].BBox = o.BBox
			}
			continue
		}
		if o.Kind == "city" || o.Kind == "village" || o.Kind == "nature" || o.Kind == "sight" {
			near = append(near, o)
		} else {
			rest = append(rest, o)
		}
	}
	top := min(wikiTop, len(wiki))
	out := append([]transfer.Found{}, wiki[:top]...)
	out = append(out, near...)
	out = append(out, wiki[top:]...)
	out = append(out, rest...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// sameIndex — то же место в Wikidata: похожее название, тот же вид и рядом (или внутри границ из OSM).
// Вид важен: город Самара стоит на реке Самаре и лежит внутри её границ.
func sameIndex(wiki []transfer.Found, o transfer.Found) int {
	on := strings.ToLower(o.Name)
	for i, w := range wiki {
		wn := strings.ToLower(w.Name)
		if !strings.Contains(on, wn) && !strings.Contains(wn, on) {
			continue
		}
		if distanceKm(w.Point, o.Point) < 3 && (w.Kind == o.Kind || w.Kind == "sight" || o.Kind == "sight") {
			return i
		}
		// Внутри границ — только тот же вид: «достопримечательность» в Wikidata — вид по умолчанию,
		// и вокзал внутри границ реки — не река
		if w.Kind == o.Kind && inside(o.BBox, w.Point) {
			return i
		}
	}
	return -1
}

func inside(b *[4]float64, p transfer.Point) bool {
	return b != nil && p.Lon >= b[0] && p.Lat >= b[1] && p.Lon <= b[2] && p.Lat <= b[3]
}

func hasDigit(s string) bool { return strings.ContainsFunc(s, unicode.IsDigit) }

func distanceKm(a, b transfer.Point) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (b.Lat-a.Lat)*rad, (b.Lon-a.Lon)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
