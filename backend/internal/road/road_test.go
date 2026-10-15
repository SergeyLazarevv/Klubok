package road

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEstimate(t *testing.T) {
	samara := Point{Lat: 53.2038, Lon: 50.1606}
	uralsk := Point{Lat: 51.1537, Lon: 51.5386}
	leg := Estimate(samara, uralsk)
	// по прямой ~247 км, с коэффициентом ~321 км; реальная дорога — 314 км
	if km := leg.DistanceM / 1000; km < 300 || km > 340 {
		t.Errorf("distance = %d km", km)
	}
	if !leg.Approximate {
		t.Error("estimate must be approximate")
	}
}

func TestOSRMTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sources") != "0" {
			t.Errorf("sources = %q", r.URL.Query().Get("sources"))
		}
		_, _ = w.Write([]byte(`{"code":"Ok","distances":[[0,43000,null]],"durations":[[0,2880,null]]}`))
	}))
	defer srv.Close()

	legs, err := NewOSRM(srv.URL).Table(context.Background(), Point{}, []Point{{}, {}})
	if err != nil {
		t.Fatal(err)
	}
	if legs[0] == nil || legs[0].DistanceM != 43000 || legs[0].DurationS != 2880 {
		t.Errorf("legs[0] = %+v", legs[0])
	}
	if legs[1] != nil {
		t.Errorf("unreachable point must be nil, got %+v", legs[1])
	}
}

func TestEncodePolyline(t *testing.T) {
	// пример из описания формата
	points := []Point{{Lat: 38.5, Lon: -120.2}, {Lat: 40.7, Lon: -120.95}, {Lat: 43.252, Lon: -126.453}}
	if got, want := encodePolyline(points, 1e5), "_p~iF~ps|U_ulLnnqC_mqNvxq`@"; got != want {
		t.Errorf("encodePolyline = %q, want %q", got, want)
	}
}

func TestOSRMRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("geometries") != "polyline6" {
			t.Errorf("geometries = %q", r.URL.Query().Get("geometries"))
		}
		_, _ = w.Write([]byte(`{"code":"Ok","routes":[{"distance":314000,"duration":14400,"geometry":"abc"}]}`))
	}))
	defer srv.Close()

	route, err := NewOSRM(srv.URL).Route(context.Background(), Point{}, Point{})
	if err != nil {
		t.Fatal(err)
	}
	if route == nil || route.DistanceM != 314000 || route.DurationS != 14400 || route.Polyline != "abc" {
		t.Errorf("route = %+v", route)
	}
}

func TestOSRMRouteVia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Заезды — между началом и концом, в том же порядке, долгота первой
		if want := "/route/v1/driving/50.100000,53.200000;50.500000,54.000000;49.100000,55.800000"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		_, _ = w.Write([]byte(`{"code":"Ok","routes":[{"distance":380000,"duration":20000,"geometry":"abc"}]}`))
	}))
	defer srv.Close()

	from, via, to := Point{Lat: 53.2, Lon: 50.1}, Point{Lat: 54, Lon: 50.5}, Point{Lat: 55.8, Lon: 49.1}
	route, err := NewOSRM(srv.URL).Route(context.Background(), from, to, via)
	if err != nil || route == nil || route.DistanceM != 380000 {
		t.Fatalf("route = %+v, err = %v", route, err)
	}
}

func TestOSRMNoRoute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"NoRoute","message":"Impossible route between points"}`))
	}))
	defer srv.Close()

	route, err := NewOSRM(srv.URL).Route(context.Background(), Point{}, Point{})
	if err != nil || route != nil {
		t.Errorf("route = %+v, err = %v; want nil, nil", route, err)
	}
}

func TestDecodePolyline(t *testing.T) {
	points, err := decodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@", 1e5)
	if err != nil {
		t.Fatal(err)
	}
	want := []Point{{Lat: 38.5, Lon: -120.2}, {Lat: 40.7, Lon: -120.95}, {Lat: 43.252, Lon: -126.453}}
	if len(points) != len(want) {
		t.Fatalf("points = %+v", points)
	}
	for i := range want {
		if math.Abs(points[i].Lat-want[i].Lat) > 1e-9 || math.Abs(points[i].Lon-want[i].Lon) > 1e-9 {
			t.Errorf("points[%d] = %+v, want %+v", i, points[i], want[i])
		}
	}
	if _, err := DecodePolyline("_p~iF~ps|"); err == nil {
		t.Error("truncated polyline must fail")
	}
}
