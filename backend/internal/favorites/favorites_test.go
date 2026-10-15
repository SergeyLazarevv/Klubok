package favorites

import "testing"

func TestKey(t *testing.T) {
	params := `{"from":"a","to":"b","date_from":"2026-11-16","date_to":"2026-11-23","min_days":5,"max_days":10,"travelers":2,"use_car":true,"consumption_l100":8}`
	pick := `{"kind":"flight","hub_iata":"URA","depart_date":"2026-11-16","return_date":"2026-11-21"}`
	got, err := Key([]byte(params), []byte(pick))
	if err != nil {
		t.Fatal(err)
	}
	if want := "a|b|2026-11-16|2026-11-23|5|10|2|car|flight|URA|2026-11-16|2026-11-21"; got != want {
		t.Errorf("Key = %q, want %q", got, want)
	}
	// Настройки машины не влияют: тот же маршрут с другим расходом — тот же отпечаток
	other := `{"from":"a","to":"b","date_from":"2026-11-16","date_to":"2026-11-23","min_days":5,"max_days":10,"travelers":2,"use_car":true,"consumption_l100":12}`
	f1, _ := Fingerprint([]byte(params), []byte(pick))
	f2, _ := Fingerprint([]byte(other), []byte(pick))
	if f1 != f2 {
		t.Error("car settings must not change the fingerprint")
	}
	// Значения по умолчанию, как у GET /trips
	got, _ = Key([]byte(`{"from":"a","to":"b","date_from":"2026-11-16","date_to":"2026-11-23"}`), []byte(`{"kind":"drive"}`))
	if want := "a|b|2026-11-16|2026-11-23|3|14|1|nocar|drive|||"; got != want {
		t.Errorf("defaults: Key = %q, want %q", got, want)
	}
}
