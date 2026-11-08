package tours

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNearest(t *testing.T) {
	s := NewService(LevelTravelLinks{}, Departures, nil, slog.Default())
	cases := []struct {
		name     string
		lat, lon float64
		want     string
	}{
		{"Казань", 55.79, 49.12, "Kazan"},
		{"Тольятти → Самара", 53.51, 49.42, "Samara"},
		{"Подольск → Москва", 55.43, 37.54, "Moscow"},
		{"Анапа → Краснодар", 44.89, 37.32, "Krasnodar"},
	}
	for _, c := range cases {
		got, km := s.Nearest(c.lat, c.lon)
		if got.Slug != c.want {
			t.Errorf("%s: got %s (%.0f км), want %s", c.name, got.Slug, km, c.want)
		}
	}
}

func TestLevelTravelHot(t *testing.T) {
	kazan := Departures[2]
	today := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	offers, err := LevelTravelLinks{Destinations: HotDestinations}.Hot(context.Background(), Query{From: kazan, Travelers: 6, Today: today})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) == 0 || len(offers) > maxOffers {
		t.Fatalf("got %d offers", len(offers))
	}
	for _, o := range offers {
		if !o.Destination.InSeason(time.November) {
			t.Errorf("%s не в сезон в ноябре", o.Destination.Name)
		}
		if o.Destination.Country == "TR" {
			t.Error("Турция в ноябре не горящее направление")
		}
	}
	first := offers[0]
	if first.Destination.Country != "EG" {
		t.Fatalf("first = %s, want EG", first.Destination.Country)
	}
	want := "https://level.travel/search/Kazan-RU-to-Any-EG-departure-11.11.2026-for-7-nights-4-adults-0-kids-1..5-stars-package-type"
	if first.URL != want {
		t.Errorf("url = %s\nwant  %s", first.URL, want)
	}
	if got := (LevelTravelLinks{}).AllURL(kazan); got != "https://level.travel/explore/Kazan-Russia/any/hot" {
		t.Errorf("all url = %s", got)
	}
}

type fakeLinker struct {
	calls int
	err   error
}

func (f *fakeLinker) PartnerLink(_ context.Context, link string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return "https://tp.media/r?u=" + link, nil
}

func TestPartner(t *testing.T) {
	l := &fakeLinker{}
	s := NewService(LevelTravelLinks{}, Departures, l, slog.Default())
	for range 2 {
		got, ok := s.Partner(context.Background(), "https://level.travel/x")
		if !ok || !strings.HasPrefix(got, "https://tp.media/") {
			t.Fatalf("got %s %v", got, ok)
		}
	}
	if l.calls != 1 {
		t.Errorf("linker called %d times, want 1 (cache)", l.calls)
	}

	s = NewService(LevelTravelLinks{}, Departures, &fakeLinker{err: errors.New("boom")}, slog.Default())
	if got, ok := s.Partner(context.Background(), "https://level.travel/x"); ok || got != "https://level.travel/x" {
		t.Errorf("on error got %s %v, want original link", got, ok)
	}
}
