package osmimport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
)

func TestOSMID(t *testing.T) {
	cases := map[string]string{
		"n123": "node/123", "w45": "way/45", "r7": "relation/7",
		"a90": "way/45",     // площадь из линии: id×2
		"a15": "relation/7", // площадь из отношения: id×2+1
	}
	for in, want := range cases {
		if got, err := osmID(in); err != nil || got != want {
			t.Errorf("osmID(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := osmID("x1"); err == nil {
		t.Error("bad prefix must fail")
	}
}

func TestRepresentativePoint(t *testing.T) {
	square := `[[[0,0],[2,0],[2,2],[0,2],[0,0]]]`
	// «П»-образная фигура: центр масс снаружи — берём вершину
	uShape := `[[[0,0],[3,0],[3,3],[2,3],[2,1],[1,1],[1,3],[0,3],[0,0]]]`
	cases := []struct {
		geom     string
		lon, lat float64
	}{
		{`{"type":"Point","coordinates":[44.5,40.1]}`, 44.5, 40.1},
		{`{"type":"LineString","coordinates":[[0,0],[1,1],[2,2]]}`, 1, 1},
		{`{"type":"Polygon","coordinates":` + square + `}`, 1, 1},
		{`{"type":"MultiPolygon","coordinates":[[[[10,10],[10.1,10],[10.1,10.1],[10,10],[10,10]]],` + square + `]}`, 1, 1},
	}
	for _, c := range cases {
		var g geometry
		if err := json.Unmarshal([]byte(c.geom), &g); err != nil {
			t.Fatal(err)
		}
		lon, lat, err := representativePoint(g)
		if err != nil || lon != c.lon || lat != c.lat {
			t.Errorf("%s → %v,%v,%v; want %v,%v", c.geom, lon, lat, err, c.lon, c.lat)
		}
	}
	var g geometry
	_ = json.Unmarshal([]byte(`{"type":"Polygon","coordinates":`+uShape+`}`), &g)
	lon, lat, _ := representativePoint(g)
	if !pointInRing(lon, lat, [][2]float64{{0, 0}, {3, 0}, {3, 3}, {2, 3}, {2, 1}, {1, 1}, {1, 3}, {0, 3}, {0, 0}}) &&
		!(lon == 0 && lat == 0) {
		t.Errorf("U-shape point %v,%v is outside", lon, lat)
	}
}

func TestParsePoly(t *testing.T) {
	poly := `armenia
1
   44.0E+00   40.0E+00
   45.0E+00   40.0E+00
   45.0E+00   41.0E+00
END
!2
   44.2   40.2
   44.4   40.2
   44.4   40.4
END
END
`
	got, err := parsePoly(strings.NewReader(poly))
	if err != nil {
		t.Fatal(err)
	}
	want := "MULTIPOLYGON(((44.0E+00 40.0E+00,45.0E+00 40.0E+00,45.0E+00 41.0E+00,44.0E+00 40.0E+00),(44.2 40.2,44.4 40.2,44.4 40.4,44.2 40.2)))"
	if got != want {
		t.Errorf("parsePoly =\n%s\nwant\n%s", got, want)
	}
}

func TestStateAndSequencePath(t *testing.T) {
	seq, err := parseState(strings.NewReader("# original OSM minutely replication sequence number 7324813\ntimestamp=2026-10-11T20\\:21\\:06Z\nsequenceNumber=3072\n"))
	if err != nil || seq != 3072 {
		t.Errorf("parseState = %d, %v", seq, err)
	}
	if p := sequencePath(4918); p != "000/004/918" {
		t.Errorf("sequencePath = %q", p)
	}
}

func TestReadFeatures(t *testing.T) {
	in := strings.Join([]string{
		`{"type":"Feature","id":"n1","geometry":{"type":"Point","coordinates":[44.5,40.1]},"properties":{"amenity":"fuel","fuel:lpg":"yes","name":"АЗС"}}`,
		`{"type":"Feature","id":"n2","geometry":{"type":"Point","coordinates":[44.5,40.1]},"properties":{"highway":"primary","wikidata":"Q1"}}`,
		`{"type":"Feature","id":"a90","geometry":{"type":"Polygon","coordinates":[[[0,0],[2,0],[2,2],[0,2],[0,0]]]},"properties":{"natural":"beach","name":"Пляж","wikidata":"Q2"}}`,
		`{"type":"Feature","id":"n3","geometry":{"type":"Point","coordinates":[1,1]},"properties":{"amenity":"parking"}}`,
	}, "\n")
	var got []row
	total, err := readFeatures(strings.NewReader(in),
		func(r row) bool { return r.Layer != poi.LayerParking },
		func(r row) error { got = append(got, r); return nil })
	if err != nil || total != 2 {
		t.Fatalf("total = %d, err = %v", total, err)
	}
	if got[0].POI.OSMID != "node/1" || got[0].Layer != poi.LayerFuel || got[0].Tags["fuel:lpg"] != "yes" {
		t.Errorf("fuel = %+v", got[0])
	}
	if got[1].POI.OSMID != "way/45" || got[1].Layer != poi.LayerNature || got[1].POI.Category != poi.CategoryAttraction {
		t.Errorf("beach = %+v", got[1])
	}
	if got[0].Hash == "" || got[0].Hash == got[1].Hash {
		t.Error("hash must be set and differ")
	}
}

func TestAirportIndex(t *testing.T) {
	idx := airportIndex{}
	kuf := [2]float64{53.5078, 50.1474}
	idx[cell(kuf[0], kuf[1])] = append(idx[cell(kuf[0], kuf[1])], kuf)
	if !idx.near(53.52, 50.16) { // ~1,6 км
		t.Error("parking 1.6 km from airport must be near")
	}
	if idx.near(53.60, 50.15) { // ~10 км
		t.Error("parking 10 km away must not be near")
	}
}
