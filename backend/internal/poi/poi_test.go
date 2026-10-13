package poi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorridorBoxes(t *testing.T) {
	// ~314 км на юг: должно получиться несколько кусков, каждый — с отступом
	var line []Point
	for i := 0; i <= 30; i++ {
		line = append(line, Point{Lat: 53.2 - float64(i)*0.0945, Lon: 50.15})
	}
	boxes := corridorBoxes(line, 5)
	if len(boxes) < 5 || len(boxes) > 8 {
		t.Fatalf("boxes = %d, want 5–8", len(boxes))
	}
	if boxes[0].North < 53.2+0.04 || boxes[0].West > 50.15-0.05 {
		t.Errorf("first box has no padding: %+v", boxes[0])
	}
	last := boxes[len(boxes)-1]
	if last.South > line[len(line)-1].Lat {
		t.Errorf("last box misses the end of the line: %+v", last)
	}

	if got := corridorBoxes([]Point{{Lat: 1, Lon: 1}}, 5); len(got) != 1 {
		t.Errorf("single point: boxes = %d", len(got))
	}
}

func TestOverpassFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.PostForm.Get("data")
		if !strings.Contains(q, "[amenity=fuel](51.00000,50.00000,52.00000,51.00000)") || strings.Contains(q, "tourism") {
			t.Errorf("unexpected query:\n%s", q)
		}
		_, _ = w.Write([]byte(`{"elements":[
			{"type":"node","id":1,"lat":51.5,"lon":50.5,"tags":{"amenity":"fuel","brand":"Лукойл"}},
			{"type":"way","id":2,"center":{"lat":51.6,"lon":50.6},"tags":{"amenity":"cafe","name":"Cafe","name:ru":"Кафе"}},
			{"type":"node","id":1,"lat":51.5,"lon":50.5,"tags":{"amenity":"fuel"}},
			{"type":"node","id":3,"lat":51.7,"lon":50.7,"tags":{"amenity":"bench"}}
		]}`))
	}))
	defer srv.Close()

	pois, err := NewOverpass(srv.URL).Fetch(context.Background(),
		[]BBox{{South: 51, West: 50, North: 52, East: 51}}, []Category{CategoryFuel, CategoryFood})
	if err != nil {
		t.Fatal(err)
	}
	if len(pois) != 2 {
		t.Fatalf("pois = %+v, want 2 (duplicate and bench dropped)", pois)
	}
	if pois[0].OSMID != "node/1" || pois[0].Category != CategoryFuel || pois[0].Brand != "Лукойл" {
		t.Errorf("pois[0] = %+v", pois[0])
	}
	if pois[1].OSMID != "way/2" || pois[1].Name != "Кафе" || pois[1].Lat != 51.6 {
		t.Errorf("pois[1] = %+v", pois[1])
	}
}

func TestOverpassTimeoutRemark(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"remark":"runtime error: Query timed out","elements":[]}`))
	}))
	defer srv.Close()

	o := NewOverpass(srv.URL)
	o.retryDelay = 0
	if _, err := o.Fetch(context.Background(), []BBox{{}}, AllCategories); err == nil {
		t.Error("timeout remark must be an error, otherwise an empty area gets cached")
	}
	if calls != overpassAttempts {
		t.Errorf("calls = %d, want %d retries", calls, overpassAttempts)
	}
}

func TestOverpassFallbackServer(t *testing.T) {
	busy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer busy.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[{"type":"node","id":1,"lat":1,"lon":1,"tags":{"amenity":"fuel"}}]}`))
	}))
	defer ok.Close()

	o := NewOverpass(busy.URL + ", " + ok.URL)
	o.retryDelay = 0
	pois, err := o.Fetch(context.Background(), []BBox{{}}, AllCategories)
	if err != nil || len(pois) != 1 {
		t.Errorf("pois = %+v, err = %v; want the fallback server's answer", pois, err)
	}
}

func TestOverpassBadRequestNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	o := NewOverpass(srv.URL)
	o.retryDelay = 0
	if _, err := o.Fetch(context.Background(), []BBox{{}}, AllCategories); err == nil || calls != 1 {
		t.Errorf("err = %v, calls = %d; want one failed call", err, calls)
	}
}
