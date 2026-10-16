package stay

import (
	"testing"
	"time"
)

func TestParseFiltersAndCollect(t *testing.T) {
	page := `<script>window['grid'] = new FGrid({
                        id: 31448,
                        filters: {
                            57831: {
                                title: 'ОКАТО',
                                values: {
                                    1: { title: 'Российская Федерация', order: 0 },
                                    2: { title: 'Самарская область', order: 1 },
                                    3: { title: 'Самара', order: 2 },
                                    4: { title: 'Город Москва столица Российской Федерации город федерального значения', order: 3 }
                                }
                            },
                            33560: {
                                title: 'Период',
                                values: {
                                    11: { title: 'август', order: 0 },
                                    12: { title: 'сентябрь', order: 1 }
                                }
                            }
                        }
                    })</script>`
	f := parseFilters(page)
	if len(f["57831"]) != 4 || f["57831"][0].title != "Российская Федерация" || len(f["33560"]) != 2 {
		t.Fatalf("filters: %+v", f)
	}
	months := map[string]time.Month{"11": time.August, "12": time.September}
	rows := []map[string]string{
		{"dim57831": "Самара", "dim58273": "Проживание в гостинице 3*, сутки с человека", "dim2026_11_d1_i1": "2500,5", "dim2026_12_d1_i2": "2559,51"},
		{"dim57831": "Российская Федерация", "dim58273": "Проживание в гостинице 3*, сутки с человека", "dim2026_11_d1_i1": "2691,61"},
	}
	ts := collect(rows, f["57831"], months)
	if len(ts) != 2 {
		t.Fatalf("territories: %+v", ts)
	}
	samara := ts[1]
	if samara.Level != LevelCity || samara.Region != "Самарская область" || samara.Hotel3.Kop != 255951 || samara.Hotel3.Month.Month() != time.September {
		t.Errorf("samara: %+v", samara)
	}
	if levelOf(f["57831"][3].title) != LevelCity {
		t.Error("Москва — город")
	}
}

func TestAssign(t *testing.T) {
	sep := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	territories := []Territory{
		{Name: "Российская Федерация", Level: LevelCountry, Hotel3: Observation{Kop: 269161, Month: sep}},
		{Name: "Самара", Level: LevelCity, Hotel3: Observation{Kop: 255951, Month: sep}},
		{Name: "Жигулевск", Level: LevelCity}, // без цены 3* — не город наблюдения
	}
	cities := []MapCity{
		{ID: "Q1", Name: "Самара", Pop: 1_100_000, Lat: 53.2, Lon: 50.15},
		{ID: "Q2", Name: "Жигулёвск", Pop: 50_000, Lat: 53.4, Lon: 49.5},   // ~50 км — цена Самары
		{ID: "Q3", Name: "Владивосток", Pop: 600_000, Lat: 43.1, Lon: 131.9}, // далеко — средняя по России
		{ID: "Q4", Name: "Рим", Country: "Италия", Lat: 41.9, Lon: 12.5},     // за рубежом — без цены
	}
	got := map[string]Price{}
	for _, p := range Assign(cities, territories) {
		got[p.CityID] = p
	}
	if len(got) != 3 {
		t.Fatalf("prices: %+v", got)
	}
	if got["Q1"].Basis != "Самара" || got["Q2"].Basis != "Самара" || got["Q2"].Kop != 255951 {
		t.Errorf("samara area: %+v %+v", got["Q1"], got["Q2"])
	}
	if got["Q3"].Basis != BasisCountry || got["Q3"].Kop != 269161 {
		t.Errorf("far city: %+v", got["Q3"])
	}
}
