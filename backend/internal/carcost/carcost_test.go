package carcost

import "testing"

// Пример из docs/06-route-search.md, раздел 6.4: Самара → Уральск, 7 дней.
func TestCalcDocExample(t *testing.T) {
	got := Calc(Input{
		OneWayKm:        330,
		ConsumptionL100: 8,
		FuelPrice:       6200,
		ParkingPerDay:   30000,
		Days:            7,
		BorderInsurance: 150000,
	})
	want := Cost{Fuel: 327360, Parking: 210000, Border: 150000, Total: 687360}
	if got != want {
		t.Errorf("Calc = %+v, want %+v", got, want)
	}
}

func TestCalcHomeAirport(t *testing.T) {
	got := Calc(Input{OneWayKm: 43, ConsumptionL100: 10, FuelPrice: 6500, ParkingPerDay: 0, Days: 5})
	// 43 × 2 × 10 / 100 = 8,6 л × 65 ₽ = 559 ₽
	if got.Fuel != 55900 || got.Total != 55900 {
		t.Errorf("Calc = %+v", got)
	}
}
