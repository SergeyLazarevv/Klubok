package fares

import (
	"testing"
	"time"
)

func TestBookingURLFor(t *testing.T) {
	cases := []struct {
		in     string
		adults int
		want   string
	}{
		{"https://www.aviasales.ru/search/KUF2311ALA28111?t=WZ1&search_date=11102026&marker=123", 2,
			"https://www.aviasales.ru/search/KUF2311ALA28112?t=WZ1&search_date=11102026&marker=123"},
		{"https://www.aviasales.ru/search/URA2011ALA26111?t=FS1", 4, "https://www.aviasales.ru/search/URA2011ALA26114?t=FS1"},
		{"https://www.aviasales.ru/search/KUF2311ALA1?t=X", 3, "https://www.aviasales.ru/search/KUF2311ALA3?t=X"}, // в одну сторону
		{"https://www.aviasales.ru/search/KUF2311ALA28111?t=X", 1, "https://www.aviasales.ru/search/KUF2311ALA28111?t=X"},
		{"https://example.com/other", 2, "https://example.com/other"},
		{"", 2, ""},
	}
	for _, c := range cases {
		if got := (Quote{BookingURL: c.in}).BookingURLFor(c.adults); got != c.want {
			t.Errorf("BookingURLFor(%q, %d) =\n %q\nwant\n %q", c.in, c.adults, got, c.want)
		}
	}
}

func TestDaytimeHours(t *testing.T) {
	samara := time.FixedZone("UTC+4", 4*3600)
	moscow := time.FixedZone("UTC+3", 3*3600)
	at := func(loc *time.Location, s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", s, loc)
		return v
	}
	cases := []struct {
		name     string
		dep, ret string
		flight   time.Duration
		want     float64
	}{
		// Вылет из Ульяновска 22:30 (UTC+4), прилёт в Москву в полночь, обратно в 8 утра — на месте только ночь
		{"прилёт в полночь, обратно в 8 утра", "2026-10-24 22:30", "2026-10-25 08:00", 90 * time.Minute, 0},
		// Прилёт 10:00 по Москве → в городе с 11:30; обратно в 20:00 → из города в 17:30: 10,5 + 9,5
		{"выходные: суббота утром — воскресенье вечером", "2026-10-24 09:30", "2026-10-25 20:00", 90 * time.Minute, 20},
		// День в городе: 11:30–17:30
		{"однодневная поездка", "2026-10-24 09:30", "2026-10-24 20:00", 90 * time.Minute, 6},
		// Длительность неизвестна — считаем от вылета: 10:00 по Самаре = 9:00 по Москве → с 10:30 до 17:30
		{"длительность перелёта неизвестна", "2026-10-24 10:00", "2026-10-24 20:00", 0, 7},
	}
	for _, c := range cases {
		q := Quote{DepartAt: at(samara, c.dep), ReturnAt: at(moscow, c.ret), DurationTo: c.flight}
		if got := q.DaytimeHours(); got != c.want {
			t.Errorf("%s: DaytimeHours = %v, want %v", c.name, got, c.want)
		}
	}
}
