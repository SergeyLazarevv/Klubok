package hotels

import (
	"fmt"
	"strings"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

// avitoDaily — раздел «Квартиры посуточно» на Авито; регион — первым сегментом адреса.
const avitoDaily = "https://www.avito.ru/%s/kvartiry/sdam/posutochno-ASgBAgICAkSSA8gQ8AeSUg"

// AvitoLink — ссылка на квартиры посуточно в городе на Авито. Даты и гостей в адресе Авито
// передать нельзя (формат фильтров закрытый), их выбирают на сайте. Авито работает только в России:
// для других стран — false.
//
// Проверить адрес запросом, как у Островка, нельзя — Авито отвечает ботам 429, поэтому город
// переводится в адрес по правилам транслитерации Авито: «Нижний Новгород» → nizhniy_novgorod.
func AvitoLink(city geo.Place) (string, bool) {
	if city.CountryCode != "RU" {
		return "", false
	}
	slug := avitoSlug(city.Name)
	if slug == "" {
		slug = "all" // все регионы
	}
	return fmt.Sprintf(avitoDaily, slug), true
}

// avitoSlug: «Санкт-Петербург» → sankt-peterburg, «Набережные Челны» → naberezhnye_chelny.
func avitoSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r == ' ':
			b.WriteByte('_')
		case r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteString(avitoTranslit[r])
		}
	}
	return b.String()
}

var avitoTranslit = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z",
	'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r",
	'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}
