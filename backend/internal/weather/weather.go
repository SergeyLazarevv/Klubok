// Package weather — погода на даты поездки (ADR-0022). Ближайшие 16 дней — прогноз Open-Meteo, дальше —
// «обычно в эти дни»: средние по наблюдениям за 5 прошлых лет. Ответы источника держим в кэше; свежесть
// проверяется при чтении: прогноз старше 6 часов, норма старше 30 дней — запрашиваем заново, а если источник
// недоступен, отдаём устаревшее с пометкой stale.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	ForecastDays = 16
	ClimateYears = 5

	forecastTTL  = 6 * time.Hour
	forecastKeep = 7 * 24 * time.Hour // столько запись живёт в кэше — запас на недоступный источник
	climateTTL   = 30 * 24 * time.Hour
	climateKeep  = 180 * 24 * time.Hour

	climateWindow = 3   // норма на день — по ±3 дням вокруг него: ~35 наблюдений за 5 лет вместо 5
	wetMM         = 1.0 // сутки с осадками от 1 мм — «дождливый день»

	SourceForecast = "forecast"
	SourceClimate  = "climate"
)

// Day — погода на сутки. Для прогноза Prob — вероятность осадков; для нормы Wet — доля дождливых дней в эти даты.
type Day struct {
	Date   time.Time
	Source string
	Code   int // код WMO: 0 ясно … 99 гроза
	TMax   float64
	TMin   float64
	Precip float64 // мм за сутки (у нормы — в среднем)
	Prob   *int
	Wet    *float64
}

type Result struct {
	Days  []Day
	Stale bool // источник недоступен — часть данных из кэша старше срока
}

// Source — откуда берём погоду (Open-Meteo; в тестах — подделка).
type Source interface {
	Forecast(ctx context.Context, lat, lon float64) ([]raw, error)
	Archive(ctx context.Context, lat, lon float64, from, to time.Time) ([]raw, error)
}

var ErrUnavailable = errors.New("weather: source unavailable")

type Service struct {
	src   Source
	cache Cache
	log   *slog.Logger
	now   func() time.Time
	group singleflight.Group // один запрос к источнику на точку, сколько бы карточек ни спросили разом
}

func NewService(src Source, cache Cache, log *slog.Logger) *Service {
	return &Service{src: src, cache: cache, log: log, now: time.Now}
}

// Days — погода на каждый день [from, to] в точке. Даты — местные для точки.
func (s *Service) Days(ctx context.Context, lat, lon float64, from, to time.Time) (Result, error) {
	lat, lon = round(lat), round(lon) // сетка источника ~10 км: соседние запросы попадают в один кэш
	var res Result

	forecast := map[string]raw{}
	// Прогноз есть только на ближайшие 16 дней — для дальних дат не запрашиваем
	if !from.After(s.now().AddDate(0, 0, ForecastDays)) {
		days, stale, err := s.forecast(ctx, lat, lon)
		if err != nil {
			s.log.WarnContext(ctx, "weather forecast", slog.Any("err", err))
		}
		res.Stale = res.Stale || stale
		for _, d := range days {
			if d.Code != nil && d.TMax != nil && d.TMin != nil {
				forecast[d.Date.Format(time.DateOnly)] = d
			}
		}
	}

	var norms climate
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if f, ok := forecast[d.Format(time.DateOnly)]; ok {
			day := Day{Date: d, Source: SourceForecast, Code: *f.Code, TMax: *f.TMax, TMin: *f.TMin, Prob: f.Prob}
			if f.Precip != nil {
				day.Precip = *f.Precip
			}
			res.Days = append(res.Days, day)
			continue
		}
		if norms == nil {
			c, stale, err := s.climate(ctx, lat, lon)
			if err != nil {
				if len(res.Days) == 0 {
					return Result{}, err
				}
				s.log.WarnContext(ctx, "weather climate", slog.Any("err", err))
				break // прогноз на первые дни есть — отдаём хотя бы его
			}
			norms, res.Stale = c, res.Stale || stale
		}
		if day, ok := norms.day(d); ok {
			res.Days = append(res.Days, day)
		}
	}
	if len(res.Days) == 0 {
		return Result{}, ErrUnavailable
	}
	return res, nil
}

func (s *Service) forecast(ctx context.Context, lat, lon float64) ([]raw, bool, error) {
	var days []raw
	stale, err := s.load(ctx, fmt.Sprintf("klubok:weather:forecast:v1:%.1f:%.1f", lat, lon), forecastTTL, forecastKeep, &days,
		func(ctx context.Context) (any, error) { return s.src.Forecast(ctx, lat, lon) })
	return days, stale, err
}

func (s *Service) climate(ctx context.Context, lat, lon float64) (climate, bool, error) {
	var c climate
	stale, err := s.load(ctx, fmt.Sprintf("klubok:weather:climate:v1:%.1f:%.1f", lat, lon), climateTTL, climateKeep, &c,
		func(ctx context.Context) (any, error) {
			// Прошлые годы целиком: у архива задержка несколько дней, а окно ±3 дня переходит через Новый год
			year := s.now().Year()
			from := time.Date(year-ClimateYears, 1, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(year-1, 12, 31, 0, 0, 0, 0, time.UTC)
			days, err := s.src.Archive(ctx, lat, lon, from, to)
			if err != nil {
				return nil, err
			}
			return aggregate(days), nil
		})
	return c, stale, err
}

// entry — запись кэша: данные и когда их получили. Срок проверяется при чтении, а не истечением ключа,
// чтобы при недоступном источнике было что отдать.
type entry struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Data      json.RawMessage `json:"data"`
}

// load читает key в dst: свежее из кэша — сразу, иначе из источника с записью в кэш. Источник не ответил,
// а в кэше есть устаревшее — отдаём его (stale = true).
func (s *Service) load(ctx context.Context, key string, ttl, keep time.Duration, dst any,
	fetch func(context.Context) (any, error)) (stale bool, err error) {
	cached, hit := s.read(ctx, key)
	if hit && s.now().Sub(cached.FetchedAt) < ttl {
		return false, json.Unmarshal(cached.Data, dst)
	}
	v, err, _ := s.group.Do(key, func() (any, error) {
		// Запрос к источнику не обрываем, если клиент ушёл: ответ пригодится следующему
		data, err := fetch(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		s.write(ctx, key, entry{FetchedAt: s.now(), Data: raw}, keep)
		return json.RawMessage(raw), nil
	})
	if err != nil {
		if hit {
			s.log.WarnContext(ctx, "weather source failed, serving stale", slog.String("key", key), slog.Any("err", err))
			return true, json.Unmarshal(cached.Data, dst)
		}
		return false, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return false, json.Unmarshal(v.(json.RawMessage), dst)
}

func (s *Service) read(ctx context.Context, key string) (entry, bool) {
	if s.cache == nil {
		return entry{}, false
	}
	data, err := s.cache.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, ErrMiss) {
			s.log.WarnContext(ctx, "weather cache get", slog.Any("err", err))
		}
		return entry{}, false
	}
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return entry{}, false
	}
	return e, true
}

func (s *Service) write(ctx context.Context, key string, e entry, keep time.Duration) {
	if s.cache == nil {
		return
	}
	data, err := json.Marshal(e)
	if err == nil {
		err = s.cache.Set(ctx, key, data, keep)
	}
	if err != nil {
		s.log.WarnContext(ctx, "weather cache set", slog.Any("err", err))
	}
}

func round(v float64) float64 { return math.Round(v*10) / 10 }

// --- Норма «обычно в эти дни» ---

// Категории погоды для нормы: какая чаще бывает в эти дни. Код — типичный WMO для категории.
var categoryCode = [...]int{0, 3, 45, 61, 71, 95} // ясно, облачно, туман, дождь, снег, гроза

func category(code int) int {
	switch {
	case code <= 1:
		return 0
	case code <= 3:
		return 1
	case code == 45 || code == 48:
		return 2
	case code >= 95:
		return 5
	case code >= 71 && code <= 77, code == 85 || code == 86:
		return 4
	default:
		return 3 // морось, дождь, ливни
	}
}

// stat — наблюдения за один календарный день («01-23») по всем годам.
type stat struct {
	N      int                    `json:"n"`
	TMax   float64                `json:"tmax"` // суммы — делим при расчёте нормы
	TMin   float64                `json:"tmin"`
	Precip float64                `json:"p"`
	Wet    int                    `json:"wet"`
	Cats   [len(categoryCode)]int `json:"c"`
}

// climate — наблюдения по календарным дням «ММ-ДД».
type climate map[string]stat

func aggregate(days []raw) climate {
	c := climate{}
	for _, d := range days {
		if d.TMax == nil || d.TMin == nil || d.Code == nil {
			continue
		}
		k := d.Date.Format("01-02")
		s := c[k]
		s.N++
		s.TMax += *d.TMax
		s.TMin += *d.TMin
		if d.Precip != nil {
			s.Precip += *d.Precip
			if *d.Precip >= wetMM {
				s.Wet++
			}
		}
		s.Cats[category(*d.Code)]++
		c[k] = s
	}
	return c
}

// day — норма на дату по окну ±climateWindow дней; 29 февраля — как соседние дни.
func (c climate) day(date time.Time) (Day, bool) {
	var sum stat
	// Високосный 2000-й: в нём есть все календарные дни, включая 29 февраля
	center := time.Date(2000, date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	for off := -climateWindow; off <= climateWindow; off++ {
		s, ok := c[center.AddDate(0, 0, off).Format("01-02")]
		if !ok {
			continue
		}
		sum.N += s.N
		sum.TMax += s.TMax
		sum.TMin += s.TMin
		sum.Precip += s.Precip
		sum.Wet += s.Wet
		for i, n := range s.Cats {
			sum.Cats[i] += n
		}
	}
	if sum.N == 0 {
		return Day{}, false
	}
	n := float64(sum.N)
	wet := float64(sum.Wet) / n
	day := Day{
		Date: date, Source: SourceClimate,
		TMax: round(sum.TMax / n), TMin: round(sum.TMin / n), Precip: round(sum.Precip / n), Wet: &wet,
	}
	best := 0
	for i, k := range sum.Cats {
		if k > sum.Cats[best] {
			best = i
		}
	}
	day.Code = categoryCode[best]
	return day, true
}
