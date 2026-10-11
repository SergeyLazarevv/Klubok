// Package carcost — стоимость поездки на своей машине: топливо, парковка, страховка за границей.
// Формулы — docs/06-route-search.md, раздел 6.3.
package carcost

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost/store"
)

// Деньги — в копейках.
type Kop = int64

type FuelType string

const (
	AI92 FuelType = "ai92"
	AI95 FuelType = "ai95"
	DT   FuelType = "dt"
	LPG  FuelType = "lpg"
)

// DefaultParkingPerDay — если для хаба нет данных о парковке.
const DefaultParkingPerDay Kop = 30000

// ErrNoFuelPrice — в справочнике нет цены топлива для страны.
var ErrNoFuelPrice = errors.New("no fuel price")

// Input — данные для расчёта поездки дом → хаб → дом.
type Input struct {
	OneWayKm        float64
	ConsumptionL100 float64
	FuelPrice       Kop // за литр
	ParkingPerDay   Kop
	Days            int
	BorderInsurance Kop
}

type Cost struct {
	Fuel    Kop
	Parking Kop
	Border  Kop
	Total   Kop
}

// Calc считает расходы на машину туда и обратно. Это расходы на всю группу, на человека делит вызывающий.
func Calc(in Input) Cost {
	litres := in.OneWayKm * 2 * in.ConsumptionL100 / 100
	c := Cost{
		Fuel:    Kop(math.Round(litres * float64(in.FuelPrice))),
		Parking: in.ParkingPerDay * Kop(in.Days),
		Border:  in.BorderInsurance,
	}
	c.Total = c.Fuel + c.Parking + c.Border
	return c
}

// BorderRule — условия въезда на машине в другую страну.
type BorderRule struct {
	Insurance     Kop
	CrossingHours float64
	Note          string
}

// Refs — справочные данные для расчёта.
type Refs struct {
	q *store.Queries
}

func NewRefs(pool *pgxpool.Pool) *Refs {
	return &Refs{q: store.New(pool)}
}

func (r *Refs) FuelPrice(ctx context.Context, country, region string, fuel FuelType) (Kop, error) {
	p, err := r.q.GetFuelPrice(ctx, store.GetFuelPriceParams{CountryCode: country, RegionCode: region, FuelType: string(fuel)})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoFuelPrice
	}
	if err != nil {
		return 0, fmt.Errorf("fuel price: %w", err)
	}
	return p, nil
}

// Parking возвращает известные цены парковки; для остальных хабов вызывающий берёт DefaultParkingPerDay.
func (r *Refs) Parking(ctx context.Context, placeIDs []uuid.UUID) (map[uuid.UUID]Kop, error) {
	rows, err := r.q.ListParking(ctx, placeIDs)
	if err != nil {
		return nil, fmt.Errorf("parking: %w", err)
	}
	m := make(map[uuid.UUID]Kop, len(rows))
	for _, row := range rows {
		m[row.PlaceID] = row.ParkingPerDayKop
	}
	return m, nil
}

// BorderRules — правила въезда из страны дома, по стране назначения.
func (r *Refs) BorderRules(ctx context.Context, fromCountry string) (map[string]BorderRule, error) {
	rows, err := r.q.ListBorderRules(ctx, fromCountry)
	if err != nil {
		return nil, fmt.Errorf("border rules: %w", err)
	}
	m := make(map[string]BorderRule, len(rows))
	for _, row := range rows {
		m[row.ToCountry] = BorderRule{Insurance: row.InsuranceKop, CrossingHours: row.CrossingHours, Note: row.Note}
	}
	return m, nil
}
