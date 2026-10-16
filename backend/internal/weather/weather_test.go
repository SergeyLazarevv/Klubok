package weather

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type memCache map[string][]byte

func (m memCache) Get(_ context.Context, key string) ([]byte, error) {
	if v, ok := m[key]; ok {
		return v, nil
	}
	return nil, ErrMiss
}

func (m memCache) Set(_ context.Context, key string, v []byte, _ time.Duration) error {
	m[key] = v
	return nil
}

type fakeSource struct {
	forecast, archive int // сколько раз спросили
	fail              bool
	today             time.Time
}

func ptr[T any](v T) *T { return &v }

func (f *fakeSource) Forecast(context.Context, float64, float64) ([]raw, error) {
	f.forecast++
	if f.fail {
		return nil, errors.New("down")
	}
	out := make([]raw, ForecastDays)
	for i := range out {
		out[i] = raw{Date: f.today.AddDate(0, 0, i), Code: ptr(61), TMax: ptr(20.0), TMin: ptr(10.0), Precip: ptr(2.5), Prob: ptr(70)}
	}
	out[ForecastDays-1].Code = nil // у последнего дня источник иногда не знает погоду — берём норму
	return out, nil
}

// Архив: каждый год 1–10 января −5…−1 и снег в чётные дни, остальное время +15…+25 и ясно.
func (f *fakeSource) Archive(_ context.Context, _, _ float64, from, to time.Time) ([]raw, error) {
	f.archive++
	if f.fail {
		return nil, errors.New("down")
	}
	var out []raw
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		r := raw{Date: d, Code: ptr(0), TMax: ptr(25.0), TMin: ptr(15.0), Precip: ptr(0.0)}
		if d.Month() == time.January && d.Day() <= 10 {
			r.TMax, r.TMin = ptr(-1.0), ptr(-5.0)
			if d.Day()%2 == 0 {
				r.Code, r.Precip = ptr(73), ptr(4.0)
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func newTest(today time.Time) (*Service, *fakeSource, *time.Time) {
	src := &fakeSource{today: today}
	now := today.Add(9 * time.Hour)
	s := NewService(src, memCache{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.now = func() time.Time { return now }
	return s, src, &now
}

func date(s string) time.Time { t, _ := time.Parse(time.DateOnly, s); return t }

func TestForecastThenClimate(t *testing.T) {
	s, _, _ := newTest(date("2026-12-20"))
	// 20 дек. + 16 дней прогноза = по 4 янв.; последний день прогноза без кода — норма; дальше — норма
	res, err := s.Days(context.Background(), 41.01, 28.98, date("2027-01-03"), date("2027-01-06"))
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, d := range res.Days {
		got = append(got, d.Date.Format("01-02")+":"+d.Source)
	}
	want := []string{"01-03:forecast", "01-04:climate", "01-05:climate", "01-06:climate"}
	if len(got) != len(want) {
		t.Fatalf("days = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("days = %v, want %v", got, want)
		}
	}
	if f := res.Days[0]; f.TMax != 20 || f.Prob == nil || *f.Prob != 70 || f.Wet != nil {
		t.Errorf("forecast day = %+v", f)
	}
	// 5 янв.: окно 2–8 янв. — всё холодные дни, снег в 2, 4, 6, 8 — 4 из 7
	c := res.Days[2]
	if c.TMax != -1 || c.TMin != -5 || c.Code != 71 || c.Wet == nil || *c.Wet < 0.57 || *c.Wet > 0.58 {
		t.Errorf("climate day = %+v wet %v", c, *c.Wet)
	}
}

func TestCacheAndReadTimeInvalidation(t *testing.T) {
	s, src, now := newTest(date("2026-10-17"))
	ctx := context.Background()
	from, to := date("2026-10-20"), date("2026-10-22")
	for range 3 {
		if _, err := s.Days(ctx, 41.01, 28.98, from, to); err != nil {
			t.Fatal(err)
		}
	}
	// Соседняя точка в пределах сетки 0,1° — тот же кэш
	if _, err := s.Days(ctx, 41.04, 28.97, from, to); err != nil {
		t.Fatal(err)
	}
	if src.forecast != 1 {
		t.Fatalf("forecast requests = %d, want 1 (cached)", src.forecast)
	}
	*now = now.Add(5 * time.Hour)
	s.Days(ctx, 41.01, 28.98, from, to) //nolint:errcheck
	if src.forecast != 1 {
		t.Fatalf("5h later: forecast requests = %d, want 1", src.forecast)
	}
	*now = now.Add(2 * time.Hour)       // 7 ч — старше 6 ч: при чтении запрашиваем заново
	s.Days(ctx, 41.01, 28.98, from, to) //nolint:errcheck
	if src.forecast != 2 {
		t.Fatalf("7h later: forecast requests = %d, want 2", src.forecast)
	}

	// Источник лёг — отдаём устаревшее с пометкой
	src.fail = true
	*now = now.Add(7 * time.Hour)
	res, err := s.Days(ctx, 41.01, 28.98, from, to)
	if err != nil || !res.Stale || len(res.Days) != 3 {
		t.Fatalf("stale fallback: res %+v err %v", res, err)
	}
	// Нет ни источника, ни кэша — ошибка
	if _, err := s.Days(ctx, 55.75, 37.62, from, to); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no cache, source down: err = %v", err)
	}
}

func TestFarDatesSkipForecast(t *testing.T) {
	s, src, _ := newTest(date("2026-10-17"))
	if _, err := s.Days(context.Background(), 41.01, 28.98, date("2027-01-02"), date("2027-01-03")); err != nil {
		t.Fatal(err)
	}
	if src.forecast != 0 || src.archive != 1 {
		t.Fatalf("forecast %d archive %d, want 0 and 1", src.forecast, src.archive)
	}
}

func TestClimateLeapDay(t *testing.T) {
	c := aggregate([]raw{{Date: date("2023-02-28"), Code: ptr(3), TMax: ptr(4.0), TMin: ptr(0.0)}})
	d, ok := c.day(date("2028-02-29"))
	if !ok || d.TMax != 4 || d.Code != 3 {
		t.Fatalf("29 Feb = %+v ok %v", d, ok)
	}
}
