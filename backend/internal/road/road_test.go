package road

import (
	"context"
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
