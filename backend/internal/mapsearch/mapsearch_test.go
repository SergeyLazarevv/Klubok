package mapsearch

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
)

type fakePhoton struct {
	items []transfer.Found
	err   error
}

func (f fakePhoton) Find(context.Context, string, *transfer.Point, int, int) ([]transfer.Found, error) {
	return f.items, f.err
}

type fakeWiki struct {
	items []transfer.Found
	err   error
	calls *int
}

func (f fakeWiki) Find(context.Context, string, int) ([]transfer.Found, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.items, f.err
}

func names(fs []transfer.Found) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}

func TestSearchMerge(t *testing.T) {
	bbox := &[4]float64{103.7, 51.4, 110, 55.8}
	s := &Service{
		log: slog.New(slog.DiscardHandler),
		wiki: fakeWiki{items: []transfer.Found{
			{Name: "Байкал", Kind: "nature", Point: transfer.Point{Lat: 53.5, Lon: 108}},
			{Name: "Байкальск", Kind: "city", Point: transfer.Point{Lat: 51.5, Lon: 104.1}},
		}},
		photon: fakePhoton{items: []transfer.Found{
			{Name: "Байкал", Kind: "other", Point: transfer.Point{Lat: 53.2, Lon: 50.1}}, // кафе рядом
			{Name: "озеро Байкал", Kind: "nature", Point: transfer.Point{Lat: 53.9, Lon: 108.3}, BBox: bbox},
			{Name: "Байкал", Kind: "village", Point: transfer.Point{Lat: 55, Lon: 55}},
		}},
	}
	got, err := s.Search(context.Background(), "Байкал", nil, 6, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Байкал", "Байкальск", "Байкал", "Байкал"}
	if n := names(got); len(n) != len(want) {
		t.Fatalf("got %v", n)
	}
	if got[0].BBox != bbox {
		t.Errorf("lake should take OSM bounds: %+v", got[0])
	}
	if got[2].Kind != "village" || got[3].Kind != "other" {
		t.Errorf("village before cafe: %+v", got)
	}
}

func TestSearchCityOnRiver(t *testing.T) {
	s := &Service{
		log: slog.New(slog.DiscardHandler),
		wiki: fakeWiki{items: []transfer.Found{
			{Name: "Самара", Kind: "city", Point: transfer.Point{Lat: 53.2, Lon: 50.1}},
			{Name: "Самара", Kind: "sight", Subtitle: "станция", Point: transfer.Point{Lat: 53.18, Lon: 50.12}},
		}},
		photon: fakePhoton{items: []transfer.Found{
			{Name: "Самара", Kind: "nature", Point: transfer.Point{Lat: 52.9, Lon: 51.5}, BBox: &[4]float64{49, 52, 56, 54}},
			{Name: "Самара", Kind: "city", Point: transfer.Point{Lat: 53.19, Lon: 50.12}, BBox: &[4]float64{49.7, 53.1, 50.4, 53.5}},
		}},
	}
	got, _ := s.Search(context.Background(), "Самара", nil, 6, 10)
	if len(got) != 3 || got[0].Kind != "city" || got[0].BBox[0] != 49.7 || got[1].BBox != nil || got[2].Kind != "nature" {
		t.Fatalf("city takes city bounds, river stays separate: %+v", got)
	}
}

func TestSearchFallbacks(t *testing.T) {
	calls := 0
	s := &Service{
		log:    slog.New(slog.DiscardHandler),
		wiki:   fakeWiki{err: errors.New("down"), calls: &calls},
		photon: fakePhoton{items: []transfer.Found{{Name: "Ленинградская улица 1", Kind: "house"}}},
	}
	got, err := s.Search(context.Background(), "Ленинградская 1", nil, 6, 10)
	if err != nil || len(got) != 1 || calls != 0 {
		t.Fatalf("address goes only to Photon: %v %v calls=%d", got, err, calls)
	}
	if _, err := s.Search(context.Background(), "Кижи", nil, 6, 10); err != nil || calls != 1 {
		t.Fatalf("wikidata failure is not fatal: %v", err)
	}
	s.photon = fakePhoton{err: errors.New("down")}
	if _, err := s.Search(context.Background(), "Кижи", nil, 6, 10); err == nil {
		t.Fatal("both sources down — error")
	}
}

func TestKindFromDescription(t *testing.T) {
	for d, want := range map[string]string{
		"город в России":     "city",
		"Село в Карелии":     "village",
		"озеро в Сибири":     "nature",
		"музей в Петербурге": "sight",
		"":                   "sight",
	} {
		if got := kindFromDescription(d); got != want {
			t.Errorf("%q: got %s, want %s", d, got, want)
		}
	}
}
