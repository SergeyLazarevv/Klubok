package httpapi

import (
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
)

func TestMatchFavorite(t *testing.T) {
	almaty := time.FixedZone("+05", 5*3600)
	opt := func(iata string, dep, ret time.Time, total int64) apigen.TripOption {
		return apigen.TripOption{Hub: apigen.Place{Iata: iata}, DepartAt: dep, ReturnAt: ret, Total: apigen.Money{Amount: total}}
	}
	// Вылет поздно вечером по местному времени: в UTC это уже другой день — сравнивать надо местные даты
	dep := time.Date(2026, 11, 16, 23, 30, 0, 0, almaty)
	ret := time.Date(2026, 11, 21, 10, 0, 0, 0, almaty)
	res := apigen.TripsResponse{Options: []apigen.TripOption{
		opt("KUF", dep, ret, 2000000),
		opt("URA", dep, ret, 3000000),
	}}
	date := func(s string) *openapi_types.Date {
		d, _ := time.Parse("2006-01-02", s)
		return &openapi_types.Date{Time: d}
	}
	ura := "URA"
	pick := apigen.FavoritePick{Kind: apigen.Flight, HubIata: &ura, DepartDate: date("2026-11-16"), ReturnDate: date("2026-11-21")}

	got := matchFavorite(res, pick)
	if got.Status != apigen.Found || got.Option.Hub.Iata != "URA" || got.Total.Amount != 3000000 {
		t.Errorf("found: %+v", got)
	}

	pick.DepartDate = date("2026-11-17") // таких дат больше нет — лучший вариант выдачи
	got = matchFavorite(res, pick)
	if got.Status != apigen.Replaced || got.Option.Hub.Iata != "KUF" {
		t.Errorf("replaced: %+v", got)
	}

	drive := &apigen.DriveTrip{Total: apigen.Money{Amount: 1500000}}
	res.Drive = drive
	got = matchFavorite(res, pick) // машина дешевле самого дешёвого перелёта
	if got.Status != apigen.Replaced || got.Drive == nil {
		t.Errorf("replaced by drive: %+v", got)
	}
	got = matchFavorite(res, apigen.FavoritePick{Kind: apigen.Drive})
	if got.Status != apigen.Found || got.Drive == nil {
		t.Errorf("drive found: %+v", got)
	}
	if got := matchFavorite(apigen.TripsResponse{}, pick); got.Status != apigen.Empty {
		t.Errorf("empty: %+v", got)
	}
}

func TestMonthPassed(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	if monthPassed("2026-10", now) || !monthPassed("2026-09", now) || monthPassed("2026-11", now) {
		t.Error("monthPassed is wrong")
	}
}
