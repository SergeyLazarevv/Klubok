package fares

import "testing"

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
