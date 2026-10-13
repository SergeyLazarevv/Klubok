package road

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OSRM — клиент table- и route-сервисов OSRM (https://project-osrm.org).
// В разработке — публичный демо-сервер, в продакшене — свой экземпляр.
type OSRM struct {
	baseURL string
	client  *http.Client
}

// Сколько точек назначения отправлять в одном запросе.
const osrmBatch = 50

func NewOSRM(baseURL string) *OSRM {
	return &OSRM{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

type osrmTable struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Distances [][]*float64 `json:"distances"`
	Durations [][]*float64 `json:"durations"`
}

func (o *OSRM) Table(ctx context.Context, from Point, to []Point) ([]*Leg, error) {
	out := make([]*Leg, 0, len(to))
	for start := 0; start < len(to); start += osrmBatch {
		end := min(start+osrmBatch, len(to))
		legs, err := o.table(ctx, from, to[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, legs...)
	}
	return out, nil
}

func (o *OSRM) table(ctx context.Context, from Point, to []Point) ([]*Leg, error) {
	coords := make([]string, 0, len(to)+1)
	for _, p := range append([]Point{from}, to...) {
		coords = append(coords, lonLat(p))
	}
	url := fmt.Sprintf("%s/table/v1/driving/%s?sources=0&annotations=distance,duration", o.baseURL, strings.Join(coords, ";"))

	var t osrmTable
	if err := o.get(ctx, url, &t); err != nil {
		return nil, err
	}
	if t.Code != "Ok" {
		return nil, fmt.Errorf("osrm: %s: %s", t.Code, t.Message)
	}
	if len(t.Distances) != 1 || len(t.Durations) != 1 || len(t.Distances[0]) != len(to)+1 {
		return nil, fmt.Errorf("osrm: unexpected table shape")
	}

	legs := make([]*Leg, len(to))
	for i := range to {
		d, dur := t.Distances[0][i+1], t.Durations[0][i+1]
		if d == nil || dur == nil {
			continue
		}
		legs[i] = &Leg{DistanceM: int(*d), DurationS: int(*dur)}
	}
	return legs, nil
}

type osrmRoute struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Routes  []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Geometry string  `json:"geometry"`
	} `json:"routes"`
}

func (o *OSRM) Route(ctx context.Context, from, to Point) (*Route, error) {
	// simplified — линия упрощена под масштаб всего маршрута: для мини-карты достаточно
	url := fmt.Sprintf("%s/route/v1/driving/%s;%s?overview=simplified&geometries=polyline6", o.baseURL, lonLat(from), lonLat(to))
	var r osrmRoute
	if err := o.get(ctx, url, &r); err != nil {
		return nil, err
	}
	if r.Code == "NoRoute" {
		return nil, nil
	}
	if r.Code != "Ok" {
		return nil, fmt.Errorf("osrm: %s: %s", r.Code, r.Message)
	}
	if len(r.Routes) == 0 {
		return nil, nil
	}
	best := r.Routes[0]
	return &Route{
		Leg:      Leg{DistanceM: int(best.Distance), DurationS: int(best.Duration)},
		Polyline: best.Geometry,
	}, nil
}

func lonLat(p Point) string {
	return strconv.FormatFloat(p.Lon, 'f', 6, 64) + "," + strconv.FormatFloat(p.Lat, 'f', 6, 64)
}

// get выполняет запрос к OSRM и разбирает JSON. OSRM отвечает JSON и на ошибки (code, message).
func (o *OSRM) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)")
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("osrm: %w", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("osrm: decode (status %d): %w", resp.StatusCode, err)
	}
	return nil
}
