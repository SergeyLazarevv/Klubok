package linkpreview

import (
	"net/url"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	page := `<html><head><title>Запасной</title>
<meta property="og:title" content="1-комнатная квартира посуточно, Казань, Портовая улица, 37Вк1" />
<meta property="og:description" content="От 3250 рублей за сутки — Wi-Fi, кондиционер &amp; холодильник" />
<meta property="og:image" content="/doc/files/objects/1.jpg" />
</head><body><meta property="og:title" content="не отсюда"></body></html>`
	base, _ := url.Parse("https://kazan.sutochno.ru/1331609")
	p := parse(strings.NewReader(page), base)
	if p.Title != "1-комнатная квартира посуточно, Казань, Портовая улица, 37Вк1" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Image != "https://kazan.sutochno.ru/doc/files/objects/1.jpg" {
		t.Errorf("image = %q", p.Image)
	}
	if p.PerNight != 325000 || !strings.Contains(p.Description, "кондиционер & холодильник") {
		t.Errorf("preview = %+v", p)
	}
}

func TestPerNight(t *testing.T) {
	for s, want := range map[string]int64{
		"Цена 5850 RUB за сутки. 26 отзывов":    585000,
		"от 12 500 ₽ за ночь":                   1250000,
		"Бронирование отеля, рейтинг 8,6":       0,
		"от 3 номеров, 2000 рублей за парковку": 0,
	} {
		if got := perNight(s); got != want {
			t.Errorf("perNight(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestAllowed(t *testing.T) {
	for host, want := range map[string]bool{
		"kazan.sutochno.ru": true, "ostrovok.ru": true, "www.avito.ru": false,
		"sutochno.ru.evil.com": false, "evilsutochno.ru": false, "127.0.0.1": false,
	} {
		if got := Allowed(host); got != want {
			t.Errorf("Allowed(%q) = %v", host, got)
		}
	}
}

func TestCleanTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Отель Биляр Палас 4* в Казани 178 отзывов об отеле, цены и фото номеров - забронировать гостиницу онлайн": "Отель Биляр Палас 4* в Казани",
		"1-комнатная квартира посуточно, Казань, Портовая улица, 37Вк1, объявление 1331609":                        "1-комнатная квартира посуточно, Казань, Портовая улица, 37Вк1",
		"Квартира — Сочи": "Квартира — Сочи",
		"Дом у моря":      "Дом у моря",
	} {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
