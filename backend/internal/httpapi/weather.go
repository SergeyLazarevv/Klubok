package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/weather"
)

const (
	weatherMaxDays = 62
	// Ссылка на источник обязательна по лицензии Open-Meteo (CC BY 4.0)
	weatherAttribution = "Open-Meteo.com"
)

func (h *handlers) GetWeather(ctx context.Context, req apigen.GetWeatherRequestObject) (apigen.GetWeatherResponseObject, error) {
	p := req.Params
	from, to := p.DateFrom.Time, p.DateTo.Time
	// Сутки запаса: у точки свой часовой пояс, у неё «сегодня» может быть вчерашним по UTC
	today := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)
	var problem *apigen.Problem
	switch {
	case from.Before(today):
		pr := fieldProblem("date_from", "погода — только на будущие даты")
		problem = &pr
	case from.After(today.AddDate(1, 0, 0)):
		pr := fieldProblem("date_from", "не дальше чем через год")
		problem = &pr
	case to.Before(from):
		pr := fieldProblem("date_to", "не может быть раньше date_from")
		problem = &pr
	case to.After(from.AddDate(0, 0, weatherMaxDays-1)):
		pr := fieldProblem("date_to", "диапазон — не больше 62 дней")
		problem = &pr
	}
	if problem != nil {
		return apigen.GetWeather422ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: apigen.ProblemApplicationProblemPlusJSONResponse(*problem)}, nil
	}

	res, err := h.weather.Days(ctx, p.Lat, p.Lon, from, to)
	if errors.Is(err, weather.ErrUnavailable) {
		detail := "источник погоды недоступен, попробуйте позже"
		return apigen.GetWeather502ApplicationProblemPlusJSONResponse(apigen.Problem{
			Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail,
		}), nil
	}
	if err != nil {
		return nil, err
	}

	resp := apigen.GetWeather200JSONResponse{Stale: res.Stale, Attribution: weatherAttribution, Days: make([]apigen.WeatherDay, len(res.Days))}
	for i, d := range res.Days {
		resp.Days[i] = apigen.WeatherDay{
			Date:              openapi_types.Date{Time: d.Date},
			Source:            apigen.WeatherDaySource(d.Source),
			Code:              d.Code,
			TMax:              d.TMax,
			TMin:              d.TMin,
			PrecipMm:          d.Precip,
			PrecipProbability: d.Prob,
			WetShare:          d.Wet,
		}
	}
	return resp, nil
}
