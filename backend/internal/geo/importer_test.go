package geo

import (
	"encoding/json"
	"testing"
)

func TestBuildPlaces(t *testing.T) {
	var cities []tpCity
	mustUnmarshal(t, `[
		{"code":"KUF","name":"Самара","name_translations":{"en":"Samara"},"cases":{"vi":"в Самару"},
		 "country_code":"RU","time_zone":"Europe/Samara","coordinates":{"lat":53.2,"lon":50.16}},
		{"code":"XXX","name":null,"country_code":"RU","time_zone":"Europe/Moscow","coordinates":{"lat":1,"lon":1}}
	]`, &cities)
	var airports []tpAirport
	mustUnmarshal(t, `[
		{"code":"KUF","city_code":"KUF","name":"Курумоч","country_code":"RU","time_zone":"Europe/Samara",
		 "coordinates":{"lat":53.5,"lon":50.15},"iata_type":"airport","flightable":true},
		{"code":"ZZZ","city_code":"KUF","name":"Закрытый","country_code":"RU","time_zone":"Europe/Samara",
		 "coordinates":{"lat":53.5,"lon":50.15},"iata_type":"airport","flightable":false},
		{"code":"RRR","city_code":"KUF","name":"Вокзал","country_code":"RU","time_zone":"Europe/Samara",
		 "coordinates":{"lat":53.5,"lon":50.15},"iata_type":"railway","flightable":true}
	]`, &airports)

	got, stats := buildPlaces(cities, airports)

	if stats != (ImportStats{Cities: 1, Airports: 1, Skipped: 1}) {
		t.Fatalf("stats = %+v", stats)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	city := got[0]
	if city.Kind != "city" || city.NameTo.String != "в Самару" || city.NameEn.String != "Samara" || city.Lat != 53.2 {
		t.Errorf("city = %+v", city)
	}
	airport := got[1]
	if airport.Kind != "airport" || airport.CityIata.String != "KUF" || airport.NameTo.Valid {
		t.Errorf("airport = %+v", airport)
	}
}

func mustUnmarshal(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), v); err != nil {
		t.Fatal(err)
	}
}
