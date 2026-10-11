package travelpayouts

import (
	"testing"
	"time"
)

func TestFoundOn(t *testing.T) {
	got := foundOn("/search/URA2011ALA26111?t=FS17951736&search_date=09102026&expected_price=11892")
	if want := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("foundOn = %v, want %v", got, want)
	}
	if !foundOn("/search/x?t=1").IsZero() {
		t.Error("missing search_date must give zero time")
	}
}

func TestBookingURL(t *testing.T) {
	c := New("token", "12345")
	got := c.bookingURL("/search/URA2011ALA26111?t=FS1")
	if want := "https://www.aviasales.ru/search/URA2011ALA26111?t=FS1&marker=12345"; got != want {
		t.Errorf("bookingURL = %q, want %q", got, want)
	}
}
