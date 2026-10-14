package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Address — найденный адрес.
type Address struct {
	Label      string
	Point      Point
	Kind       string  // house | street | place
	DistanceKm float64 // от центра города
}

// Photon — поиск адресов по данным OpenStreetMap (https://photon.komoot.io).
// Публичный сервер — для разработки; в продакшене — свой экземпляр или платный геокодер.
type Photon struct {
	baseURL string
	client  *http.Client
}

func NewPhoton(baseURL string) *Photon {
	return &Photon{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

// Дальше этого от центра города результат считаем ошибкой поиска (адрес в другом городе с той же улицей).
const maxAddressKm = 60

type photonResponse struct {
	Features []struct {
		Geometry struct {
			Coordinates [2]float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Name        string `json:"name"`
			Street      string `json:"street"`
			HouseNumber string `json:"housenumber"`
			City        string `json:"city"`
			District    string `json:"district"`
			Type        string `json:"type"`
		} `json:"properties"`
	} `json:"features"`
}

// Search ищет адрес рядом с городом: результаты ближе к центру — выше.
func (p *Photon) Search(ctx context.Context, q string, near Point, limit int) ([]Address, error) {
	params := url.Values{
		"q": {q}, "limit": {"15"},
		"lat": {f(near.Lat)}, "lon": {f(near.Lon)}, "zoom": {"12"}, "location_bias_scale": {"0.1"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("photon: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photon: status %d", resp.StatusCode)
	}
	var r photonResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("photon: decode: %w", err)
	}

	seen := map[string]bool{}
	var out []Address
	for _, ft := range r.Features {
		pr := ft.Properties
		pt := Point{Lat: ft.Geometry.Coordinates[1], Lon: ft.Geometry.Coordinates[0]}
		d := haversineKm(near, pt)
		if d > maxAddressKm {
			continue
		}
		label, kind := addressLabel(pr.Name, pr.Street, pr.HouseNumber, pr.City, pr.District, pr.Type)
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, Address{Label: label, Point: pt, Kind: kind, DistanceKm: d})
	}
	// Точный адрес с номером дома — выше улицы целиком: к улице маршрут строится до её середины
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind == "house" && out[j].Kind != "house" })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// addressLabel: «Абай даңғылы 10, Алматы»; для улицы без дома — kind=street, точку стоит поправить на карте.
func addressLabel(name, street, house, city, district, typ string) (string, string) {
	var head string
	kind := "place"
	switch {
	case street != "" && house != "":
		// Организации в доме (магазин, банк) не нужны: маршрут строим до дома, одинаковые адреса склеиваются
		head, kind = street+" "+house, "house"
	case typ == "street":
		head, kind = name, "street"
	case name != "":
		head = name
		if street != "" {
			head += ", " + street
		}
	}
	if head == "" {
		return "", ""
	}
	place := city
	if place == "" {
		place = district
	}
	if place != "" && place != head {
		head += ", " + place
	}
	return head, kind
}
