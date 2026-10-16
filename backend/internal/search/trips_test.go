package search

import (
	"testing"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
)

func TestFits(t *testing.T) {
	day := func(d int) time.Time { return time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, d) }
	q := TripsQuery{DateFrom: day(10), DateTo: day(20), MinDays: 3, MaxDays: 5}
	quote := func(dep, ret int) fares.Quote {
		return fares.Quote{DepartAt: day(dep).Add(10 * time.Hour), ReturnAt: day(ret).Add(18 * time.Hour)}
	}
	cases := []struct {
		name     string
		q        fares.Quote
		wantDays int
		want     bool
	}{
		{"внутри дат и длительности", quote(12, 15), 4, true},
		{"вылет до начала", quote(9, 12), 4, false},
		{"возврат после конца — даже если вылет внутри", quote(18, 21), 4, false},
		{"слишком коротко", quote(12, 12), 1, false},
		{"слишком долго", quote(10, 19), 10, false},
	}
	for _, c := range cases {
		days, ok := fits(c.q, q)
		if ok != c.want || days != c.wantDays {
			t.Errorf("%s: got (%d, %v), want (%d, %v)", c.name, days, ok, c.wantDays, c.want)
		}
	}
}

func TestFitsWeekends(t *testing.T) {
	samara := time.FixedZone("UTC+4", 4*3600)
	at := func(s string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", s, samara)
		return v
	}
	q := TripsQuery{DateFrom: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), DateTo: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		MinDays: 1, MaxDays: 5, Weekends: true}
	cases := []struct {
		name     string
		dep, ret string
		want     bool
	}{
		{"пятница вечером — воскресенье", "2099-11-06 23:15", "2099-11-08 19:30", true},
		{"суббота — понедельник", "2099-11-07 08:00", "2099-11-09 10:00", true},
		{"вторник — пятница", "2099-11-03 08:00", "2099-11-06 10:00", false},
		{"пятница — вторник", "2099-11-06 08:00", "2099-11-10 10:00", false},
	}
	for _, c := range cases {
		if _, ok := fits(fares.Quote{DepartAt: at(c.dep), ReturnAt: at(c.ret)}, q); ok != c.want {
			t.Errorf("%s: got %v, want %v", c.name, ok, c.want)
		}
	}
}
