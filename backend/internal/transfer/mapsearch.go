package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Found — место для поиска по карте: город, село, достопримечательность, улица.
type Found struct {
	Name     string
	Subtitle string // «Карелия, Россия»
	Kind     string // city | village | sight | nature | street | house | other
	Point    Point
	BBox     *[4]float64 // запад, юг, восток, север — у протяжённых объектов, чтобы показать целиком
}

type photonFindResponse struct {
	Features []struct {
		Geometry struct {
			Coordinates [2]float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Name        string    `json:"name"`
			Street      string    `json:"street"`
			HouseNumber string    `json:"housenumber"`
			City        string    `json:"city"`
			County      string    `json:"county"`
			State       string    `json:"state"`
			Country     string    `json:"country"`
			Type        string    `json:"type"`
			OsmKey      string    `json:"osm_key"`
			OsmValue    string    `json:"osm_value"`
			Extent      []float64 `json:"extent"` // запад, север, восток, юг
		} `json:"properties"`
	} `json:"features"`
}

// Сколько кандидатов брать у Photon: магазины и кафе с тем же названием он ставит высоко,
// а нужные путешественнику места (гора, село, музей) поднимаем сами — их надо успеть получить.
const findCandidates = 40

// Find ищет по всему миру; near — центр карты: при равной известности ближние места выше.
// zoom — масштаб карты: чем крупнее, тем сильнее тянет к центру.
func (p *Photon) Find(ctx context.Context, q string, near *Point, zoom, limit int) ([]Found, error) {
	params := url.Values{"q": {q}, "limit": {strconv.Itoa(findCandidates)}}
	if near != nil {
		params.Set("lat", f(near.Lat))
		params.Set("lon", f(near.Lon))
		params.Set("zoom", strconv.Itoa(zoom))
		params.Set("location_bias_scale", "0.2")
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
	var r photonFindResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("photon: decode: %w", err)
	}

	seen := map[string]bool{}
	out := []Found{}
	for _, ft := range r.Features {
		pr := ft.Properties
		name := pr.Name
		if pr.Street != "" && pr.HouseNumber != "" && (name == "" || pr.OsmKey == "building" || pr.OsmKey == "place") {
			name = pr.Street + " " + pr.HouseNumber
		}
		if name == "" {
			continue
		}
		kind := foundKind(pr.OsmKey, pr.OsmValue, pr.Type)
		sub := joinNonEmpty(", ", subtitleHead(kind, pr.Street, pr.HouseNumber, pr.City, name), pr.State, pr.Country)
		// Одинаковые объекты из разных кусков OSM (улица из нескольких линий) — один результат
		key := name + "|" + sub
		if seen[key] {
			continue
		}
		seen[key] = true
		it := Found{Name: name, Subtitle: sub, Kind: kind,
			Point: Point{Lat: ft.Geometry.Coordinates[1], Lon: ft.Geometry.Coordinates[0]}}
		if e := pr.Extent; len(e) == 4 {
			it.BBox = &[4]float64{e[0], e[3], e[2], e[1]}
		}
		out = append(out, it)
	}
	// Порядок Photon (известность и близость) сохраняем внутри группы
	sort.SliceStable(out, func(i, j int) bool { return kindRank[out[i].Kind] < kindRank[out[j].Kind] })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Сначала то, куда едут, затем адреса, в конце организации (магазины, кафе, гостиницы)
var kindRank = map[string]int{"city": 0, "village": 0, "sight": 0, "nature": 0, "street": 1, "house": 1, "other": 2}

// subtitleHead — что писать перед регионом: у улицы и дома город, у места в городе — улица и город.
func subtitleHead(kind, street, house, city, name string) string {
	switch kind {
	case "city", "village", "nature":
		return ""
	case "house", "street":
		return city
	}
	addr := street
	if street != "" && house != "" {
		addr += " " + house
	}
	if addr == name {
		addr = ""
	}
	return joinNonEmpty(", ", addr, city)
}

func foundKind(key, value, typ string) string {
	switch key {
	case "place":
		switch value {
		case "city", "town", "municipality":
			return "city"
		case "village", "hamlet", "suburb", "neighbourhood", "quarter", "locality", "isolated_dwelling":
			return "village"
		case "island", "islet", "archipelago":
			return "nature"
		case "house":
			return "house"
		}
		return "other"
	case "tourism":
		switch value {
		case "hotel", "guest_house", "hostel", "motel", "apartment", "chalet", "information", "camp_pitch":
			return "other"
		}
		return "sight"
	case "historic", "heritage", "memorial":
		return "sight"
	case "natural", "waterway", "water", "mountain_pass":
		return "nature"
	case "leisure":
		if value == "park" || value == "nature_reserve" || value == "garden" || value == "beach_resort" {
			return "nature"
		}
		return "other"
	case "amenity":
		if value == "place_of_worship" || value == "theatre" || value == "fountain" {
			return "sight"
		}
	case "building":
		if value == "church" || value == "cathedral" || value == "chapel" || value == "monastery" || value == "mosque" {
			return "sight"
		}
	case "highway":
		return "street"
	case "boundary":
		return "city"
	}
	if typ == "street" {
		return "street"
	}
	// Дом без названия — адрес; организация в доме (магазин, офис) — не адрес
	if typ == "house" && (key == "building" || key == "place") {
		return "house"
	}
	return "other"
}

func joinNonEmpty(sep string, parts ...string) string {
	var b []string
	for _, s := range parts {
		if s != "" && (len(b) == 0 || b[len(b)-1] != s) {
			b = append(b, s)
		}
	}
	return strings.Join(b, sep)
}
