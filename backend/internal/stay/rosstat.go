package stay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Rosstat — средние потребительские цены Росстата из ЕМИСС (fedstat.ru), показатель 31448
// «Средние потребительские цены (тарифы) на товары и услуги»: помесячно по городам наблюдения и регионам.
// Отдельного API нет — берём то же, что загружает таблица на сайте: справочники фильтров со страницы
// показателя и данные POST-запросом /indicator/dataGrid.do. Данные открытые, лицензия CC BY 3.0.
type Rosstat struct {
	baseURL string
	http    *http.Client
}

func NewRosstat(baseURL string) *Rosstat {
	return &Rosstat{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 3 * time.Minute}}
}

const (
	indicator = "31448"
	// Позиции «Виды товаров и услуг» (фильтр 58273)
	itemHotel3 = "1836584" // Проживание в гостинице 3*, сутки с человека
	itemHotel2 = "1836583" // Проживание в гостинице 2*, сутки с человека

	filterYear   = "3"
	filterItem   = "58273"
	filterOKATO  = "57831"
	filterUnit   = "30611"
	filterPeriod = "33560"
)

// Level — уровень территории в справочнике ОКАТО.
type Level int

const (
	LevelCountry Level = iota
	LevelDistrict
	LevelRegion
	LevelCity
)

// Territory — цены на проживание по территории за последний месяц, где они есть.
type Territory struct {
	Name   string
	Region string // для города — регион, в котором он стоит в справочнике
	Level  Level
	Hotel3 Observation
	Hotel2 Observation
}

// Observation — цена суток с человека; Kop == 0 — нет данных.
type Observation struct {
	Kop   int64
	Month time.Time
}

type filterValue struct{ id, title string }

// Fetch — цены за год (и предыдущий, чтобы в январе было с чем работать), последнее значение по каждой территории.
func (r *Rosstat) Fetch(ctx context.Context, now time.Time) ([]Territory, error) {
	filters, err := r.filters(ctx)
	if err != nil {
		return nil, err
	}
	okato, periods := filters[filterOKATO], filters[filterPeriod]
	if len(okato) == 0 || len(periods) == 0 {
		return nil, errors.New("rosstat: на странице показателя нет справочников ОКАТО или периодов — поменялась разметка")
	}
	monthOf := make(map[string]time.Month, len(periods))
	for _, p := range periods {
		if m, ok := monthsRu[p.title]; ok {
			monthOf[p.id] = m
		}
	}

	form := url.Values{}
	for _, id := range []string{"0", filterUnit, filterOKATO, filterItem} {
		form.Add("lineObjectIds", id)
	}
	form.Add("columnObjectIds", filterYear)
	form.Add("columnObjectIds", filterPeriod)
	form.Add("selectedFilterIds", "0_"+indicator)
	form.Add("selectedFilterIds", filterUnit+"_950351") // рубль
	for _, y := range []int{now.Year() - 1, now.Year()} {
		form.Add("selectedFilterIds", fmt.Sprintf("%s_%d", filterYear, y))
	}
	for _, item := range []string{itemHotel3, itemHotel2} {
		form.Add("selectedFilterIds", filterItem+"_"+item)
	}
	for id := range monthOf {
		form.Add("selectedFilterIds", filterPeriod+"_"+id)
	}
	for _, o := range okato {
		form.Add("selectedFilterIds", filterOKATO+"_"+o.id)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/indicator/dataGrid.do?id="+indicator, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rosstat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rosstat: status %d", resp.StatusCode)
	}
	var data struct {
		Results []map[string]string `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("rosstat: decode: %w", err)
	}
	return collect(data.Results, okato, monthOf), nil
}

// dataKey — колонка значения: dim<год>_<id периода>_d…_i…
var dataKey = regexp.MustCompile(`^dim(\d{4})_(\d+)_d\d*_i\d*$`)

func collect(rows []map[string]string, okato []filterValue, monthOf map[string]time.Month) []Territory {
	byName := make(map[string]*Territory)
	for _, row := range rows {
		name := row["dim"+filterOKATO]
		item := row["dim"+filterItem]
		var latest Observation
		for k, v := range row {
			m := dataKey.FindStringSubmatch(k)
			if m == nil {
				continue
			}
			month, ok := monthOf[m[2]]
			year, _ := strconv.Atoi(m[1])
			price, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(v, " ", ""), ",", "."), 64)
			if !ok || err != nil || price <= 0 {
				continue
			}
			at := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
			if at.After(latest.Month) {
				latest = Observation{Kop: int64(price*100 + 0.5), Month: at}
			}
		}
		t := byName[name]
		if t == nil {
			t = &Territory{Name: name}
			byName[name] = t
		}
		switch {
		case strings.Contains(item, "3*"):
			t.Hotel3 = latest
		case strings.Contains(item, "2*"):
			t.Hotel2 = latest
		}
	}
	// Порядок справочника ОКАТО — иерархия: округ, регион, его города
	var out []Territory
	region := ""
	for _, o := range okato {
		level := levelOf(o.title)
		if level == LevelRegion {
			region = o.title
		}
		t, ok := byName[o.title]
		if !ok {
			continue
		}
		t.Level = level
		if level == LevelCity {
			t.Region = region
		}
		out = append(out, *t)
	}
	return out
}

func levelOf(title string) Level {
	l := strings.ToLower(title)
	switch {
	case strings.HasPrefix(l, "российская федерация"):
		return LevelCountry
	case strings.Contains(l, "федеральный округ"):
		return LevelDistrict
	case strings.Contains(l, "федерального значения"):
		return LevelCity // Москва, Петербург, Севастополь — и регион, и город
	case strings.Contains(l, "область"), strings.Contains(l, "край"), strings.Contains(l, "республика"),
		strings.Contains(l, "автономный округ"), strings.Contains(l, "автономная"), strings.Contains(l, "округ"):
		return LevelRegion
	}
	return LevelCity
}

// filters — справочники фильтров показателя со страницы ЕМИСС: id владельца → значения в порядке справочника.
func (r *Rosstat) filters(ctx context.Context) (map[string][]filterValue, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/indicator/"+indicator, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rosstat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rosstat: page status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	return parseFilters(string(body)), nil
}

var (
	unicodeEscape = regexp.MustCompile(`\\u([0-9A-Fa-f]{4})`)
	// Заголовок фильтра: «<id>: {\n title: '…'» с отступом 28 пробелов; значения — глубже
	filterHead = regexp.MustCompile(`\n {28}(\d+): \{\s*title: '([^']*)'`)
	valueItem  = regexp.MustCompile(`(\d+): \{\s*title: '([^']*)'`)
)

func parseFilters(page string) map[string][]filterValue {
	start := strings.Index(page, "new FGrid(")
	if start < 0 {
		return nil
	}
	text := unicodeEscape.ReplaceAllStringFunc(page[start:], func(s string) string {
		n, _ := strconv.ParseUint(s[2:], 16, 32)
		return string(rune(n))
	})
	heads := filterHead.FindAllStringSubmatchIndex(text, -1)
	out := make(map[string][]filterValue, len(heads))
	for i, h := range heads {
		end := len(text)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		owner := text[h[2]:h[3]]
		for _, v := range valueItem.FindAllStringSubmatch(text[h[1]:end], -1) {
			out[owner] = append(out[owner], filterValue{id: v[1], title: v[2]})
		}
	}
	return out
}

var monthsRu = map[string]time.Month{
	"январь": time.January, "февраль": time.February, "март": time.March, "апрель": time.April,
	"май": time.May, "июнь": time.June, "июль": time.July, "август": time.August,
	"сентябрь": time.September, "октябрь": time.October, "ноябрь": time.November, "декабрь": time.December,
}
