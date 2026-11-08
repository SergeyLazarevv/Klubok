package transfer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("lat") != "53.200000" || r.URL.Query().Get("zoom") != "5" {
			t.Errorf("bias not passed: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"features":[
{"geometry":{"coordinates":[50.1,53.2]},"properties":{"name":"Кижи","osm_key":"shop","osm_value":"convenience","type":"house","street":"Беломорская улица","housenumber":"2","city":"Москва","country":"Россия"}},
{"geometry":{"coordinates":[35.22,62.07]},"properties":{"name":"Кижский погост","osm_key":"historic","osm_value":"church","state":"Карелия","country":"Россия","extent":[35.2,62.08,35.3,62.06]}},
{"geometry":{"coordinates":[50.1,53.2]},"properties":{"street":"Ленинградская улица","housenumber":"1","osm_key":"building","osm_value":"yes","type":"house","city":"Самара","state":"Самарская область","country":"Россия"}},
{"geometry":{"coordinates":[50.1,53.2]},"properties":{"name":"Ленинградская улица","osm_key":"highway","osm_value":"pedestrian","type":"street","city":"Самара","state":"Самарская область","country":"Россия"}},
{"geometry":{"coordinates":[50.2,53.2]},"properties":{"name":"Ленинградская улица","osm_key":"highway","osm_value":"pedestrian","type":"street","city":"Самара","state":"Самарская область","country":"Россия"}},
{"geometry":{"coordinates":[0,0]},"properties":{"osm_key":"natural"}}
]}`))
	}))
	defer srv.Close()

	got, err := NewPhoton(srv.URL).Find(context.Background(), "x", &Point{Lat: 53.2, Lon: 50.1}, 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 results (street deduped, nameless dropped), got %+v", got)
	}
	if got[3].Kind != "other" || got[3].Subtitle != "Беломорская улица 2, Москва, Россия" {
		t.Errorf("shop goes last: %+v", got[3])
	}
	if got[0].Kind != "sight" || got[0].Subtitle != "Карелия, Россия" || got[0].BBox == nil || got[0].BBox[1] != 62.06 {
		t.Errorf("sight: %+v %v", got[0], got[0].BBox)
	}
	if got[1].Name != "Ленинградская улица 1" || got[1].Kind != "house" || got[1].Subtitle != "Самара, Самарская область, Россия" {
		t.Errorf("house: %+v", got[1])
	}
	if got[2].Kind != "street" {
		t.Errorf("street: %+v", got[2])
	}
}
