package transfer

import (
	"fmt"
	"net/url"
	"strings"
)

type routeLinks struct {
	yandexGo, yandexMapsCar, yandexMapsTransit, yandexMapsWalk, twoGISTransit Link
}

// twoGISDomains — у 2ГИС свой домен в некоторых странах; остальные открываются на 2gis.ru.
var twoGISDomains = map[string]string{"KZ": "2gis.kz", "KG": "2gis.kg", "UZ": "2gis.uz", "AE": "2gis.ae", "AM": "2gis.am", "AZ": "2gis.az", "GE": "2gis.ge", "CY": "2gis.com.cy"}

// linksFor — ссылки, открывающие маршрут в приложениях. Проверены вручную 2026-10-14: все отвечают переходом на маршрут.
func linksFor(from, to Point, country string) routeLinks {
	ya := func(rtt string) string {
		return fmt.Sprintf("https://yandex.ru/maps/?rtext=%s~%s&rtt=%s", latLon(from), latLon(to), rtt)
	}
	domain := twoGISDomains[strings.ToUpper(country)]
	if domain == "" {
		domain = "2gis.ru"
	}
	go_ := url.Values{
		"start-lat": {f(from.Lat)}, "start-lon": {f(from.Lon)},
		"end-lat": {f(to.Lat)}, "end-lon": {f(to.Lon)},
		"level": {"econom"},
		// идентификатор из документации виджета Яндекс Go; партнёрский ref добавим вместе с ключом API такси
		"appmetrica_tracking_id": {"1178268795219780156"},
	}
	return routeLinks{
		yandexGo:          Link{"Вызвать в Яндекс Go", "https://3.redirect.appmetrica.yandex.com/route?" + go_.Encode()},
		yandexMapsCar:     Link{"Маршрут в Яндекс Картах", ya("auto")},
		yandexMapsTransit: Link{"Яндекс Карты", ya("mt")},
		yandexMapsWalk:    Link{"Маршрут в Яндекс Картах", ya("pd")},
		twoGISTransit: Link{"2ГИС", fmt.Sprintf("https://%s/directions/points/%s%%7C%s",
			domain, url.PathEscape(lonLat(from)), url.PathEscape(lonLat(to)))},
	}
}

func latLon(p Point) string { return f(p.Lat) + "," + f(p.Lon) }
func lonLat(p Point) string { return f(p.Lon) + "," + f(p.Lat) }
func f(v float64) string    { return fmt.Sprintf("%.6f", v) }
