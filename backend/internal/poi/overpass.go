package poi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Overpass — клиент Overpass API (https://wiki.openstreetmap.org/wiki/Overpass_API).
// Публичные серверы — только для разработки: лимиты и нестабильная скорость.
type Overpass struct {
	urls       []string
	client     *http.Client
	retryDelay time.Duration
}

// Публичные серверы часто отвечают 429/504 под нагрузкой — пробуем ещё раз, по очереди на каждом.
const overpassAttempts = 3

// NewOverpass принимает один адрес или несколько через запятую — запасные серверы.
func NewOverpass(urls string) *Overpass {
	var list []string
	for _, u := range strings.Split(urls, ",") {
		if u = strings.TrimSpace(u); u != "" {
			list = append(list, u)
		}
	}
	return &Overpass{urls: list, client: &http.Client{Timeout: 90 * time.Second}, retryDelay: 2 * time.Second}
}

// errRetry — временная ошибка сервера, имеет смысл повторить.
var errRetry = errors.New("temporary")

// Фильтры OSM-тегов по категориям. Кафе и достопримечательности без названия не берём —
// на карте от них мало толку.
var overpassFilters = map[Category][]string{
	CategoryFuel: {`[amenity=fuel]`},
	CategoryFood: {`[amenity~"^(cafe|restaurant|fast_food)$"][name]`},
	CategoryAttraction: {
		`[tourism~"^(attraction|museum|viewpoint|gallery|zoo|theme_park)$"][name]`,
		`[historic~"^(castle|fort|monument|ruins|archaeological_site|manor|monastery)$"][name]`,
	},
}

type overpassResponse struct {
	Remark   string            `json:"remark"`
	Elements []overpassElement `json:"elements"`
}

type overpassElement struct {
	Type   string                      `json:"type"`
	ID     int64                       `json:"id"`
	Lat    float64                     `json:"lat"`
	Lon    float64                     `json:"lon"`
	Center *struct{ Lat, Lon float64 } `json:"center"`
	Tags   map[string]string           `json:"tags"`
}

func (o *Overpass) Fetch(ctx context.Context, boxes []BBox, categories []Category) ([]POI, error) {
	query := buildQuery(boxes, categories)
	var err error
	for attempt := range overpassAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(o.retryDelay * time.Duration(attempt)):
			}
		}
		var elements []overpassElement
		elements, err = o.fetch(ctx, o.urls[attempt%len(o.urls)], query)
		if err == nil {
			return parseElements(elements), nil
		}
		if !errors.Is(err, errRetry) {
			return nil, err
		}
	}
	return nil, err
}

func (o *Overpass) fetch(ctx context.Context, endpoint, query string) ([]overpassElement, error) {
	form := url.Values{"data": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)")
	resp, err := o.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("overpass %s: %w: %w", endpoint, errRetry, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, fmt.Errorf("overpass %s: %w: status %d", endpoint, errRetry, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("overpass %s: status %d", endpoint, resp.StatusCode)
	}
	var r overpassResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("overpass %s: %w: decode: %w", endpoint, errRetry, err)
	}
	// По таймауту Overpass отвечает 200 с пустым или неполным списком — такой ответ не сохраняем
	if strings.Contains(r.Remark, "error") {
		return nil, fmt.Errorf("overpass %s: %w: %s", endpoint, errRetry, r.Remark)
	}
	return r.Elements, nil
}

func buildQuery(boxes []BBox, categories []Category) string {
	var b strings.Builder
	b.WriteString("[out:json][timeout:60];\n(\n")
	for _, box := range boxes {
		bbox := fmt.Sprintf("(%s,%s,%s,%s)", coord(box.South), coord(box.West), coord(box.North), coord(box.East))
		for _, c := range categories {
			for _, f := range overpassFilters[c] {
				b.WriteString("  nwr" + f + bbox + ";\n")
			}
		}
	}
	b.WriteString(");\nout center tags;\n")
	return b.String()
}

func parseElements(elements []overpassElement) []POI {
	seen := make(map[string]bool, len(elements))
	out := make([]POI, 0, len(elements))
	for _, e := range elements {
		id := e.Type + "/" + strconv.FormatInt(e.ID, 10)
		category, kind := classify(e.Tags)
		if category == "" || seen[id] {
			continue
		}
		seen[id] = true
		lat, lon := e.Lat, e.Lon
		if e.Center != nil { // линии и полигоны — по центру
			lat, lon = e.Center.Lat, e.Center.Lon
		}
		name := e.Tags["name:ru"]
		if name == "" {
			name = e.Tags["name"]
		}
		out = append(out, POI{
			OSMID: id, Category: category, Kind: kind, Name: name, Brand: e.Tags["brand"],
			Lat: lat, Lon: lon,
			OpeningHours: e.Tags["opening_hours"], Website: firstTag(e.Tags, "website", "contact:website"),
			Wikipedia: e.Tags["wikipedia"], Wikidata: e.Tags["wikidata"],
			CommonsFile: commonsFile(firstTag(e.Tags, "wikimedia_commons", "image")),
			Cuisine:     e.Tags["cuisine"],
			Phone:       firstTag(e.Tags, "phone", "contact:phone"),
			Fee:         firstTag(e.Tags, "charge", "fee"),
		})
	}
	return out
}

func classify(tags map[string]string) (Category, string) {
	switch a := tags["amenity"]; a {
	case "fuel":
		return CategoryFuel, a
	case "cafe", "restaurant", "fast_food":
		return CategoryFood, a
	}
	switch t := tags["tourism"]; t {
	case "attraction", "museum", "viewpoint", "gallery", "zoo", "theme_park":
		return CategoryAttraction, t
	}
	switch h := tags["historic"]; h {
	case "castle", "fort", "monument", "ruins", "archaeological_site", "manor", "monastery":
		return CategoryAttraction, h
	}
	return "", ""
}

// commonsFile достаёт имя файла Wikimedia Commons из тега: «File:X.jpg» или ссылка на Commons.
// Ссылки на другие сайты не берём — лицензия фото там неизвестна.
func commonsFile(tag string) string {
	switch {
	case strings.HasPrefix(tag, "File:"):
		return strings.TrimPrefix(tag, "File:")
	case strings.Contains(tag, "commons.wikimedia.org/wiki/File:"):
		name := tag[strings.Index(tag, "File:")+len("File:"):]
		if decoded, err := url.PathUnescape(name); err == nil {
			name = decoded
		}
		return strings.ReplaceAll(name, "_", " ")
	}
	return ""
}

func firstTag(tags map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := tags[k]; v != "" {
			return v
		}
	}
	return ""
}

func coord(v float64) string {
	return strconv.FormatFloat(v, 'f', 5, 64)
}
