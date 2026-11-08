package tours

import (
	"context"
	"fmt"
	"time"
)

// Адреса Level.Travel (проверено 2026-11-08: старые «быстрые ссылки» /landing/search/TR/Any
// перенаправляют на этот формат; поддомены городов вылета больше не работают):
//   - поиск: /search/Moscow-RU-to-Any-TR-departure-24.11.2026-for-3-nights-2-adults-0-kids-1..5-stars-package-type
//   - горящие туры города: /explore/Moscow-Russia/any/hot
const (
	levelTravelSearch = "https://level.travel/search/%s-RU-to-Any-%s-departure-%s-for-%d-nights-%d-adults-0-kids-1..5-stars-package-type"
	levelTravelHot    = "https://level.travel/explore/%s-Russia/any/hot"
)

const (
	hotLeadDays = 3 // горящий тур — вылет через несколько дней
	maxOffers   = 6
	maxAdults   = 4 // больше взрослых в одном номере Level.Travel не ищет
)

// LevelTravelLinks — горящие туры как ссылки на поиск Level.Travel по направлениям сезона, без цен.
type LevelTravelLinks struct {
	Destinations []Destination
}

func (l LevelTravelLinks) Hot(_ context.Context, q Query) ([]Offer, error) {
	date := q.Today.AddDate(0, 0, hotLeadDays)
	adults := min(max(q.Travelers, 1), maxAdults)
	var offers []Offer
	for _, d := range l.Destinations {
		if !d.InSeason(date.Month()) {
			continue
		}
		offers = append(offers, Offer{
			Destination: d,
			Departure:   date,
			Nights:      d.Nights,
			URL:         fmt.Sprintf(levelTravelSearch, q.From.Slug, d.Country, date.Format("02.01.2006"), d.Nights, adults),
		})
		if len(offers) == maxOffers {
			break
		}
	}
	return offers, nil
}

func (LevelTravelLinks) AllURL(from Departure) string {
	return fmt.Sprintf(levelTravelHot, from.Slug)
}

func months(ms ...time.Month) []time.Month { return ms }

// HotDestinations — популярные направления пакетных туров по порядку популярности и их сезон.
var HotDestinations = []Destination{
	{Country: "TR", Name: "Турция", Nights: 7, Months: months(5, 6, 7, 8, 9, 10)},
	{Country: "EG", Name: "Египет", Nights: 7},
	{Country: "AE", Name: "ОАЭ", Nights: 7, Months: months(10, 11, 12, 1, 2, 3, 4)},
	{Country: "TH", Name: "Таиланд", Nights: 10, Months: months(11, 12, 1, 2, 3, 4)},
	{Country: "VN", Name: "Вьетнам", Nights: 10, Months: months(12, 1, 2, 3, 4, 5, 6, 7, 8)},
	{Country: "AB", Name: "Абхазия", Nights: 7, Months: months(6, 7, 8, 9)},
	{Country: "CY", Name: "Кипр", Nights: 7, Months: months(5, 6, 7, 8, 9, 10)},
	{Country: "GR", Name: "Греция", Nights: 7, Months: months(5, 6, 7, 8, 9, 10)},
	{Country: "TN", Name: "Тунис", Nights: 7, Months: months(6, 7, 8, 9)},
	{Country: "ME", Name: "Черногория", Nights: 7, Months: months(6, 7, 8, 9)},
	{Country: "LK", Name: "Шри-Ланка", Nights: 10, Months: months(11, 12, 1, 2, 3, 4)},
	{Country: "MV", Name: "Мальдивы", Nights: 7, Months: months(11, 12, 1, 2, 3, 4)},
	{Country: "IN", Name: "Индия (Гоа)", Nights: 10, Months: months(11, 12, 1, 2, 3)},
	{Country: "CU", Name: "Куба", Nights: 10, Months: months(11, 12, 1, 2, 3, 4)},
}

// Departures — крупные города вылета Level.Travel. Названия в адресах — [сверить] в браузере:
// сайт закрыт от ботов, автоматически проверить их нельзя.
var Departures = []Departure{
	{Slug: "Moscow", Name: "Москва", NameFrom: "из Москвы", Lat: 55.7558, Lon: 37.6173},
	{Slug: "St.Petersburg", Name: "Санкт-Петербург", NameFrom: "из Санкт-Петербурга", Lat: 59.9386, Lon: 30.3141},
	{Slug: "Kazan", Name: "Казань", NameFrom: "из Казани", Lat: 55.7963, Lon: 49.1088},
	{Slug: "Yekaterinburg", Name: "Екатеринбург", NameFrom: "из Екатеринбурга", Lat: 56.8389, Lon: 60.6057},
	{Slug: "Novosibirsk", Name: "Новосибирск", NameFrom: "из Новосибирска", Lat: 55.0302, Lon: 82.9204},
	{Slug: "Samara", Name: "Самара", NameFrom: "из Самары", Lat: 53.1959, Lon: 50.1002},
	{Slug: "Krasnodar", Name: "Краснодар", NameFrom: "из Краснодара", Lat: 45.0355, Lon: 38.9753},
	{Slug: "Ufa", Name: "Уфа", NameFrom: "из Уфы", Lat: 54.7388, Lon: 55.9721},
	{Slug: "Perm", Name: "Пермь", NameFrom: "из Перми", Lat: 58.0105, Lon: 56.2502},
	{Slug: "Rostov-on-Don", Name: "Ростов-на-Дону", NameFrom: "из Ростова-на-Дону", Lat: 47.2357, Lon: 39.7015},
	{Slug: "Nizhny.Novgorod", Name: "Нижний Новгород", NameFrom: "из Нижнего Новгорода", Lat: 56.3269, Lon: 44.0059},
	{Slug: "Chelyabinsk", Name: "Челябинск", NameFrom: "из Челябинска", Lat: 55.1644, Lon: 61.4368},
	{Slug: "Voronezh", Name: "Воронеж", NameFrom: "из Воронежа", Lat: 51.6608, Lon: 39.2003},
	{Slug: "Volgograd", Name: "Волгоград", NameFrom: "из Волгограда", Lat: 48.7080, Lon: 44.5133},
	{Slug: "Saratov", Name: "Саратов", NameFrom: "из Саратова", Lat: 51.5331, Lon: 46.0342},
	{Slug: "Tyumen", Name: "Тюмень", NameFrom: "из Тюмени", Lat: 57.1530, Lon: 65.5343},
	{Slug: "Omsk", Name: "Омск", NameFrom: "из Омска", Lat: 54.9885, Lon: 73.3242},
	{Slug: "Krasnoyarsk", Name: "Красноярск", NameFrom: "из Красноярска", Lat: 56.0153, Lon: 92.8932},
	{Slug: "Irkutsk", Name: "Иркутск", NameFrom: "из Иркутска", Lat: 52.2870, Lon: 104.3050},
	{Slug: "Khabarovsk", Name: "Хабаровск", NameFrom: "из Хабаровска", Lat: 48.4802, Lon: 135.0719},
	{Slug: "Vladivostok", Name: "Владивосток", NameFrom: "из Владивостока", Lat: 43.1155, Lon: 131.8855},
	{Slug: "Mineralnye.Vody", Name: "Минеральные Воды", NameFrom: "из Минеральных Вод", Lat: 44.2087, Lon: 43.1388},
	{Slug: "Kaliningrad", Name: "Калининград", NameFrom: "из Калининграда", Lat: 54.7104, Lon: 20.4522},
	{Slug: "Sochi", Name: "Сочи", NameFrom: "из Сочи", Lat: 43.5855, Lon: 39.7231},
}
