package search

import (
	"testing"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
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

func TestLongestDay(t *testing.T) {
	h := func(hours float64) road.Leg { return road.Leg{DurationS: int(hours * 3600)} }
	via := h(11)
	d := &DriveOption{
		Leg: h(10), DriveHours: 10.5, // полчаса на границе
		ViaLeg: &via,
		parts:  []road.Leg{h(4), h(3), h(4)},
		Stops:  []DriveStop{{Nights: 0}, {Nights: 1}},
	}
	// Заезд на пару часов день не делит: 4 + 3 = 7 ч, ночёвка, затем 4 ч + граница
	if got := longestDay(d); got != 7 {
		t.Errorf("longestDay = %v, want 7", got)
	}
	d.Stops[1].Nights = 0
	if got := longestDay(d); got != 11.5 {
		t.Errorf("без ночёвки longestDay = %v, want 11.5", got)
	}
	// Маршрутизатор не вернул участки — вся дорога через остановки
	d.parts = nil
	if got := longestDay(d); got != 11.5 {
		t.Errorf("без участков longestDay = %v, want 11.5", got)
	}
}
