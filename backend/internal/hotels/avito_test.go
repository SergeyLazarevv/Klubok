package hotels

import (
	"testing"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

func TestAvitoSlug(t *testing.T) {
	cases := map[string]string{
		"Москва":           "moskva",
		"Казань":           "kazan",
		"Санкт-Петербург":  "sankt-peterburg",
		"Нижний Новгород":  "nizhniy_novgorod",
		"Набережные Челны": "naberezhnye_chelny",
		"Ростов-на-Дону":   "rostov-na-donu",
		"Екатеринбург":     "ekaterinburg",
		"Йошкар-Ола":       "yoshkar-ola",
	}
	for in, want := range cases {
		if got := avitoSlug(in); got != want {
			t.Errorf("avitoSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAvitoLink(t *testing.T) {
	link, ok := AvitoLink(geo.Place{Name: "Казань", CountryCode: "RU"})
	if !ok || link != "https://www.avito.ru/kazan/kvartiry/sdam/posutochno-ASgBAgICAkSSA8gQ8AeSUg" {
		t.Errorf("AvitoLink(Казань) = %q, %v", link, ok)
	}
	if _, ok := AvitoLink(geo.Place{Name: "Алматы", CountryCode: "KZ"}); ok {
		t.Error("AvitoLink для Казахстана должен вернуть false")
	}
}
