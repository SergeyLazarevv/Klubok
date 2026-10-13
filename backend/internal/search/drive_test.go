package search

import (
	"testing"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
)

func TestWorthFlying(t *testing.T) {
	drive := &DriveOption{DriveHours: 5, Total: 500000} // 5 ч, 5 000 ₽
	flight := func(total int64, flight time.Duration, toAirport float64) TripOption {
		return TripOption{
			Total: total, ByCar: toAirport > 0,
			Hub:   reachable{DriveHours: toAirport},
			Quote: fares.Quote{DurationTo: flight},
		}
	}
	cases := []struct {
		name string
		o    TripOption
		want bool
	}{
		{"дешевле машины", flight(400000, 0, 0), true},
		{"дороже, но вдвое быстрее", flight(900000, 90*time.Minute, 1), true},
		{"дороже и почти так же долго", flight(900000, 3*time.Hour, 1), false},
		{"дороже, длительность неизвестна", flight(900000, 0, 0), false},
	}
	for _, c := range cases {
		if got := worthFlying(c.o, drive); got != c.want {
			t.Errorf("%s: worthFlying = %v, want %v", c.name, got, c.want)
		}
	}
	if !worthFlying(flight(9e9, 0, 0), nil) {
		t.Error("without a drive option every flight is worth showing")
	}
}
