package osmimport

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
)

// feature — строка GeoJSON Lines из osmium export.
type feature struct {
	ID         string            `json:"id"`
	Geometry   geometry          `json:"geometry"`
	Properties map[string]string `json:"properties"`
}

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// row — место, готовое к загрузке в staging.
type row struct {
	POI   poi.POI
	Layer poi.Layer
	Tags  map[string]string
	Hash  string
}

// readFeatures читает выгрузку построчно и отдаёт места, прошедшие классификацию.
func readFeatures(r io.Reader, keep func(row) bool, emit func(row) error) (total int, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20) // полигоны заповедников бывают в десятки мегабайт
	for sc.Scan() {
		var f feature
		if err := json.Unmarshal(sc.Bytes(), &f); err != nil {
			return total, fmt.Errorf("parse feature: %w", err)
		}
		c, ok := poi.Classify(f.Properties)
		if !ok {
			continue
		}
		id, err := osmID(f.ID)
		if err != nil {
			return total, err
		}
		lon, lat, err := representativePoint(f.Geometry)
		if err != nil {
			continue // пустая или непонятная геометрия
		}
		r := row{POI: poi.FromTags(id, c, f.Properties, lat, lon), Layer: c.Layer, Tags: poi.LayerTags(c.Layer, f.Properties)}
		if !keep(r) {
			continue
		}
		r.Hash = hashRow(r)
		total++
		if err := emit(r); err != nil {
			return total, err
		}
	}
	return total, sc.Err()
}

// osmID: n123 → node/123, w45 → way/45, r7 → relation/7; площади a<id>: чётные — из линий (id/2), нечётные — из отношений.
func osmID(id string) (string, error) {
	if len(id) < 2 {
		return "", fmt.Errorf("bad osm id %q", id)
	}
	n, err := strconv.ParseInt(id[1:], 10, 64)
	if err != nil {
		return "", fmt.Errorf("bad osm id %q", id)
	}
	switch id[0] {
	case 'n':
		return "node/" + id[1:], nil
	case 'w':
		return "way/" + id[1:], nil
	case 'r':
		return "relation/" + id[1:], nil
	case 'a':
		if n%2 == 0 {
			return "way/" + strconv.FormatInt(n/2, 10), nil
		}
		return "relation/" + strconv.FormatInt((n-1)/2, 10), nil
	}
	return "", fmt.Errorf("bad osm id %q", id)
}

// representativePoint — точка для показа на карте: для линии — средняя вершина,
// для полигона — центр самого большого контура, если он внутри, иначе вершина контура.
func representativePoint(g geometry) (lon, lat float64, err error) {
	switch g.Type {
	case "Point":
		var c [2]float64
		if err := json.Unmarshal(g.Coordinates, &c); err != nil {
			return 0, 0, err
		}
		return c[0], c[1], nil
	case "LineString":
		var c [][2]float64
		if err := json.Unmarshal(g.Coordinates, &c); err != nil || len(c) == 0 {
			return 0, 0, fmt.Errorf("empty line")
		}
		m := c[len(c)/2]
		return m[0], m[1], nil
	case "Polygon":
		var c [][][2]float64
		if err := json.Unmarshal(g.Coordinates, &c); err != nil || len(c) == 0 {
			return 0, 0, fmt.Errorf("empty polygon")
		}
		return ringPoint(c[0])
	case "MultiPolygon":
		var c [][][][2]float64
		if err := json.Unmarshal(g.Coordinates, &c); err != nil || len(c) == 0 {
			return 0, 0, fmt.Errorf("empty multipolygon")
		}
		best, bestArea := c[0][0], -1.0
		for _, p := range c {
			if len(p) > 0 {
				if a := math.Abs(ringArea(p[0])); a > bestArea {
					best, bestArea = p[0], a
				}
			}
		}
		return ringPoint(best)
	}
	return 0, 0, fmt.Errorf("unsupported geometry %q", g.Type)
}

func ringPoint(ring [][2]float64) (float64, float64, error) {
	if len(ring) == 0 {
		return 0, 0, fmt.Errorf("empty ring")
	}
	a := ringArea(ring)
	if a == 0 {
		return ring[0][0], ring[0][1], nil
	}
	var cx, cy float64
	for i := 0; i < len(ring)-1; i++ {
		x0, y0, x1, y1 := ring[i][0], ring[i][1], ring[i+1][0], ring[i+1][1]
		f := x0*y1 - x1*y0
		cx += (x0 + x1) * f
		cy += (y0 + y1) * f
	}
	cx, cy = cx/(6*a), cy/(6*a)
	if pointInRing(cx, cy, ring) {
		return cx, cy, nil
	}
	return ring[0][0], ring[0][1], nil // вогнутая фигура: центр снаружи
}

// ringArea — ориентированная площадь замкнутого кольца в квадратных градусах.
func ringArea(ring [][2]float64) float64 {
	var s float64
	for i := 0; i < len(ring)-1; i++ {
		s += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return s / 2
}

func pointInRing(x, y float64, ring [][2]float64) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		xi, yi, xj, yj := ring[i][0], ring[i][1], ring[j][0], ring[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// hashRow — хеш значимых полей: совпал с сохранённым — строку в pois не трогаем.
func hashRow(r row) string {
	p := r.POI
	tags, _ := json.Marshal(r.Tags) // ключи map сортируются
	s := strings.Join([]string{
		string(r.Layer), p.Kind, p.Name, p.Brand, p.OpeningHours, p.Website, p.Wikipedia, p.Wikidata,
		p.CommonsFile, p.Cuisine, p.Phone, p.Fee, string(tags),
		strconv.FormatFloat(p.Lat, 'f', 6, 64), strconv.FormatFloat(p.Lon, 'f', 6, 64),
	}, "\x1f")
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
