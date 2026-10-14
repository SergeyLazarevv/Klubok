package transfer

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

type fakeRouter struct {
	leg *road.Leg
	err error
}

func (r fakeRouter) Table(context.Context, road.Point, []road.Point) ([]*road.Leg, error) {
	return []*road.Leg{r.leg}, r.err
}

func TestOptionsFarAway(t *testing.T) {
	// Аэропорт Алматы → центр: 20 км, пешком не предлагаем
	s := NewService(fakeRouter{leg: &road.Leg{DistanceM: 20400, DurationS: 2040}}, fakeRouter{err: errors.New("unused")}, slog.Default())
	opts := s.Options(context.Background(), Point{43.3521, 77.0405}, Point{43.2420, 76.9462}, "KZ", 2)
	if len(opts) != 2 || opts[0].Mode != ModeTaxi || opts[1].Mode != ModePublicTransport {
		t.Fatalf("opts = %+v", opts)
	}
	if opts[0].DurationMin != 34 || opts[0].DistanceKm != 20.4 {
		t.Errorf("taxi = %+v", opts[0])
	}
	if !strings.HasPrefix(opts[1].Links[1].URL, "https://2gis.kz/directions/points/77.040500%2C43.352100%7C") {
		t.Errorf("2gis link = %s", opts[1].Links[1].URL)
	}
}

func TestOptionsWalkAndFallback(t *testing.T) {
	// Маршрутизатор машины недоступен — оценка по прямой; пешком близко — предлагаем и ставим первым
	s := NewService(fakeRouter{err: errors.New("down")}, fakeRouter{leg: &road.Leg{DistanceM: 1200, DurationS: 900}}, slog.Default())
	opts := s.Options(context.Background(), Point{43.2711, 76.9417}, Point{43.2620, 76.9440}, "KZ", 5)
	if len(opts) != 3 {
		t.Fatalf("opts = %+v", opts)
	}
	if opts[0].Mode != ModeTaxi || !opts[0].Approximate || !strings.Contains(opts[0].PriceNote, "минивэн") {
		t.Errorf("taxi = %+v", opts[0])
	}
	if opts[1].Mode != ModeWalk || opts[1].DurationMin != 15 {
		t.Errorf("walk = %+v", opts[1])
	}
	if opts[2].Mode != ModePublicTransport {
		t.Errorf("unknown duration must be last: %+v", opts[2])
	}
}

func TestAddressLabel(t *testing.T) {
	cases := []struct{ name, street, house, city, typ, label, kind string }{
		{"", "Абай даңғылы", "10", "Алматы", "house", "Абай даңғылы 10, Алматы", "house"},
		{"Абай даңғылы", "", "", "Алматы", "street", "Абай даңғылы, Алматы", "street"},
		{"Eurasian Bank", "Абай даңғылы", "10", "Алматы", "house", "Абай даңғылы 10, Алматы", "house"},
	}
	for _, c := range cases {
		label, kind := addressLabel(c.name, c.street, c.house, c.city, "", c.typ)
		if label != c.label || kind != c.kind {
			t.Errorf("addressLabel = %q, %q; want %q, %q", label, kind, c.label, c.kind)
		}
	}
}
