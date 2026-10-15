package hotels

import (
	"net/url"
	"strconv"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

// sutochnoSearch — поиск Суточно.ру: квартиры, дома, гостевые дома и отели.
const sutochnoSearch = "https://sutochno.ru/front/searchapp/search"

// sutochnoCountries — страны, где работает Суточно.ру (разделы сайта: /kazakhstan, /georgia…).
var sutochnoCountries = map[string]bool{
	"RU": true, "AM": true, "AZ": true, "BY": true, "EG": true, "GE": true, "KZ": true,
	"KG": true, "TH": true, "TR": true, "AE": true, "UZ": true,
}

// SutochnoLink — поиск жилья в городе на даты поездки: term — название города (сайт сам находит его),
// occupied — «заезд;выезд» в формате ГГГГ-ММ-ДД (так их читает сайт), guests_adults — число гостей.
// Для стран, где Суточно.ру не работает, — false.
func SutochnoLink(city geo.Place, checkIn, checkOut time.Time, guests int) (string, bool) {
	if !sutochnoCountries[city.CountryCode] {
		return "", false
	}
	q := url.Values{
		"term":          {city.Name},
		"occupied":      {checkIn.Format(time.DateOnly) + ";" + checkOut.Format(time.DateOnly)},
		"guests_adults": {strconv.Itoa(max(guests, 1))},
	}
	return sutochnoSearch + "?" + q.Encode(), true
}
