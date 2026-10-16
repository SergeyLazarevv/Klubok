package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// OpenMeteo — источник погоды: прогноз на 16 дней и архив наблюдений (для «обычно в эти дни»).
// Бесплатно и без ключа для некоммерческого использования, нужна ссылка на источник (CC BY 4.0).
type OpenMeteo struct {
	forecastURL string
	archiveURL  string
	http        *http.Client
}

func NewOpenMeteo() *OpenMeteo {
	return &OpenMeteo{
		forecastURL: "https://api.open-meteo.com/v1/forecast",
		archiveURL:  "https://archive-api.open-meteo.com/v1/archive",
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

// raw — сутки из ответа источника; nil — значения нет (у последнего дня прогноза так бывает).
type raw struct {
	Date   time.Time
	Code   *int
	TMax   *float64
	TMin   *float64
	Precip *float64
	Prob   *int
}

type dailyResponse struct {
	Daily struct {
		Time   []string   `json:"time"`
		Code   []*int     `json:"weather_code"`
		TMax   []*float64 `json:"temperature_2m_max"`
		TMin   []*float64 `json:"temperature_2m_min"`
		Precip []*float64 `json:"precipitation_sum"`
		Prob   []*int     `json:"precipitation_probability_max"`
	} `json:"daily"`
}

// Forecast — прогноз по дням, даты — местные для точки.
func (o *OpenMeteo) Forecast(ctx context.Context, lat, lon float64) ([]raw, error) {
	q := o.query(lat, lon)
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max")
	q.Set("forecast_days", strconv.Itoa(ForecastDays))
	return o.get(ctx, o.forecastURL, q)
}

// Archive — наблюдения по дням за период (у архива задержка несколько дней — берём прошлые годы целиком).
func (o *OpenMeteo) Archive(ctx context.Context, lat, lon float64, from, to time.Time) ([]raw, error) {
	q := o.query(lat, lon)
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum")
	q.Set("start_date", from.Format(time.DateOnly))
	q.Set("end_date", to.Format(time.DateOnly))
	return o.get(ctx, o.archiveURL, q)
}

func (o *OpenMeteo) query(lat, lon float64) url.Values {
	return url.Values{
		"latitude":  {strconv.FormatFloat(lat, 'f', 2, 64)},
		"longitude": {strconv.FormatFloat(lon, 'f', 2, 64)},
		"timezone":  {"auto"},
	}
}

func (o *OpenMeteo) get(ctx context.Context, base string, q url.Values) ([]raw, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	res, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo: %s", res.Status)
	}
	var body dailyResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("open-meteo: decode: %w", err)
	}
	d := body.Daily
	at := func(i int, n int) bool { return i < n }
	out := make([]raw, 0, len(d.Time))
	for i, s := range d.Time {
		date, err := time.Parse(time.DateOnly, s)
		if err != nil {
			return nil, fmt.Errorf("open-meteo: date %q: %w", s, err)
		}
		r := raw{Date: date}
		if at(i, len(d.Code)) {
			r.Code = d.Code[i]
		}
		if at(i, len(d.TMax)) {
			r.TMax = d.TMax[i]
		}
		if at(i, len(d.TMin)) {
			r.TMin = d.TMin[i]
		}
		if at(i, len(d.Precip)) {
			r.Precip = d.Precip[i]
		}
		if at(i, len(d.Prob)) {
			r.Prob = d.Prob[i]
		}
		out = append(out, r)
	}
	return out, nil
}
