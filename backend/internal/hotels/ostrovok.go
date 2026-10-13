// Package hotels — где жить на месте: ссылки на поиск отелей с городом и датами поездки.
package hotels

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

// PartnerLinker превращает ссылку на сайт бренда в партнёрскую (Travelpayouts).
type PartnerLinker interface {
	PartnerLink(ctx context.Context, link string) (string, error)
}

// Ostrovok строит ссылку на поиск Островка: /hotel/{страна}/{город}/?dates=ДД.ММ.ГГГГ-ДД.ММ.ГГГГ&guests=N.
// Адрес города Островок называет по-английски (kazakhstan/almaty), но не всегда предсказуемо,
// поэтому кандидата проверяем запросом и запоминаем результат.
type Ostrovok struct {
	base   string
	client *http.Client
	linker PartnerLinker // nil — ссылка без партнёрской метки
	log    *slog.Logger

	mu     sync.Mutex
	cities map[string]string // страна/город → путь, "" — города нет на Островке
	links  map[string]string // ссылка → партнёрская
}

func NewOstrovok(linker PartnerLinker, log *slog.Logger) *Ostrovok {
	return &Ostrovok{
		base: "https://ostrovok.ru",
		client: &http.Client{
			// Островок отдаёт страницу города медленно и непредсказуемо (от 1 до 20+ с), ждём недолго
			Timeout: 4 * time.Second,
			// Островок отвечает 302 на страницу с найденным регионом — этого достаточно, дальше не идём
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		linker: linker,
		log:    log,
		cities: make(map[string]string),
		links:  make(map[string]string),
	}
}

// Link — ссылка на поиск отелей в городе на даты поездки. Город не нашёлся — страница страны,
// страна неизвестна — главная Островка. partner — ссылка с партнёрской меткой.
func (o *Ostrovok) Link(ctx context.Context, city geo.Place, checkIn, checkOut time.Time, guests int) (link string, partner bool) {
	path := o.cityPath(ctx, city)
	link = fmt.Sprintf("%s%s?dates=%s-%s&guests=%d", o.base, path,
		checkIn.Format("02.01.2006"), checkOut.Format("02.01.2006"), max(guests, 1))
	if o.linker == nil {
		return link, false
	}
	o.mu.Lock()
	cached, ok := o.links[link]
	o.mu.Unlock()
	if ok {
		return cached, true
	}
	pl, err := o.linker.PartnerLink(ctx, link)
	if err != nil {
		o.log.WarnContext(ctx, "ostrovok partner link", slog.Any("err", err))
		return link, false
	}
	o.mu.Lock()
	o.links[link] = pl
	o.mu.Unlock()
	return pl, true
}

func (o *Ostrovok) cityPath(ctx context.Context, city geo.Place) string {
	country, ok := countrySlugs[city.CountryCode]
	if !ok {
		return "/"
	}
	countryPath := "/hotel/" + country + "/"
	key := country + "/" + city.NameEn
	o.mu.Lock()
	path, known := o.cities[key]
	o.mu.Unlock()
	if !known {
		var sure bool
		path, sure = o.findCity(ctx, countryPath, city.NameEn)
		if sure { // догадку без ответа Островка не запоминаем — проверим в следующий раз
			o.mu.Lock()
			o.cities[key] = path
			o.mu.Unlock()
		}
	}
	if path == "" {
		return countryPath
	}
	return path
}

// findCity проверяет варианты адреса города. sure = false — Островок не ответил, path — самый вероятный вариант.
func (o *Ostrovok) findCity(ctx context.Context, countryPath, nameEn string) (path string, sure bool) {
	slugs := citySlugs(nameEn)
	for _, slug := range slugs {
		candidate := countryPath + slug + "/"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base+candidate, nil)
		if err != nil {
			return "", false
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Klubok/0.1; +https://github.com/SergeyLazarevv/Klubok)")
		resp, err := o.client.Do(req)
		if err != nil {
			if len(slugs) == 0 {
				return "", false
			}
			return countryPath + slugs[0] + "/", false
		}
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK, http.StatusMovedPermanently, http.StatusFound, http.StatusPermanentRedirect:
			return candidate, true
		}
	}
	return "", true // ни один вариант не подошёл — города на Островке нет
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

var nyEnding = regexp.MustCompile(`ny(_|$)`)

// citySlugs — варианты адреса города на Островке: «Naberezhnye Chelny» → naberezhnye_chelny;
// «Saint Petersburg» → st._petersburg (так у Островка); «Nizhny Novgorod» → nizhniy_novgorod (их транслитерация).
func citySlugs(nameEn string) []string {
	base := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(nameEn), "_"), "_")
	if base == "" {
		return nil
	}
	slugs := []string{base}
	for _, prefix := range []string{"st_", "saint_"} {
		if rest, ok := strings.CutPrefix(base, prefix); ok {
			slugs = append(slugs, "st._"+rest)
		}
	}
	if v := nyEnding.ReplaceAllString(base, "niy$1"); v != base {
		slugs = append(slugs, v)
	}
	return slugs
}

// Страны в адресах Островка — английские названия через подчёркивание (проверено на популярных направлениях).
var countrySlugs = map[string]string{
	"RU": "russia", "KZ": "kazakhstan", "BY": "belarus", "UZ": "uzbekistan", "KG": "kyrgyzstan",
	"TJ": "tajikistan", "AM": "armenia", "GE": "georgia", "AZ": "azerbaijan", "TR": "turkey",
	"AE": "united_arab_emirates", "EG": "egypt", "TH": "thailand", "VN": "vietnam", "CN": "china",
	"IN": "india", "ID": "indonesia", "LK": "sri_lanka", "MV": "maldives", "CY": "cyprus",
	"GR": "greece", "IT": "italy", "ES": "spain", "FR": "france", "DE": "germany", "ME": "montenegro",
	"RS": "serbia", "IL": "israel", "QA": "qatar", "OM": "oman", "JO": "jordan", "MA": "morocco",
	"TN": "tunisia", "CU": "cuba", "MN": "mongolia", "KR": "south_korea", "JP": "japan",
}
