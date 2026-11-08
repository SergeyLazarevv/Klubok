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
	via, back := h(11), h(10)
	// Обратно с ночёвкой: 0,5 на границе + 5 ч и 5 ч — короче, чем туда
	d := &DriveOption{
		Leg: h(10), DriveHours: 10.5, // полчаса на границе
		ViaLeg: &via, BackLeg: &back,
		parts:     []road.Leg{h(4), h(3), h(4)},
		backParts: []road.Leg{h(5), h(5)},
		Stops:     []DriveStop{{Nights: 0}, {Nights: 1}, {Nights: 1, Back: true}},
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

	// Обратно с ночёвкой: граница в первый день (0,5 + 6), затем 6 ч; туда напрямую — 10,5 ч
	back = h(12)
	d = &DriveOption{
		Leg: h(10), DriveHours: 10.5,
		BackLeg:   &back,
		backParts: []road.Leg{h(6), h(6)},
		Stops:     []DriveStop{{Nights: 1, Back: true}},
	}
	if got := longestDay(d); got != 10.5 {
		t.Errorf("обратно с ночёвкой longestDay = %v, want 10.5 (прямая туда)", got)
	}
	// И туда с ночёвкой: 5 + 5,5 с границей; обратно 6,5 и 6
	via = h(10.5)
	d.ViaLeg, d.parts = &via, []road.Leg{h(5), h(5.5)}
	d.Stops = []DriveStop{{Nights: 1}, {Nights: 1, Back: true}}
	if got := longestDay(d); got != 6.5 {
		t.Errorf("с ночёвками в обе стороны longestDay = %v, want 6.5", got)
	}
}
