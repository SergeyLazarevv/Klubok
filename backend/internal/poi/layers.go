package poi

// Layer — слой карты (ADR-0014, раздел 4). Кафе у трассы — тот же слой food, отбирается по месту на карте.
type Layer string

const (
	LayerFuel       Layer = "fuel"        // заправки
	LayerCharging   Layer = "charging"    // зарядки для электромобилей
	LayerFood       Layer = "food"        // кафе и рестораны
	LayerRest       Layer = "rest"        // площадки отдыха, туалеты
	LayerBorder     Layer = "border"      // пункты пропуска на границе
	LayerCarService Layer = "car_service" // автосервисы и шиномонтаж
	LayerParking    Layer = "parking"     // парковки у аэропортов
	LayerAttraction Layer = "attraction"  // достопримечательности
	LayerNature     Layer = "nature"      // природа и отдых: пляжи, водопады, парки
	LayerUseful     Layer = "useful"      // банкоматы, обмен, аптеки, вокзалы
)

// CategoryOther — места новых слоёв, которых нет в текущем API мест (fuel | food | attraction).
const CategoryOther Category = "other"

// LayerCategory — грубая группа слоя для текущего API.
func LayerCategory(l Layer) Category {
	switch l {
	case LayerFuel:
		return CategoryFuel
	case LayerFood:
		return CategoryFood
	case LayerAttraction, LayerNature:
		return CategoryAttraction
	}
	return CategoryOther
}

// Classification — к какому слою относится объект OSM.
type Classification struct {
	Layer Layer
	Kind  string // значение тега: fuel, cafe, museum, beach…
}

var (
	notableHistoric = set("castle", "fort", "monument", "ruins", "archaeological_site", "manor", "monastery")
	attractionTypes = set("attraction", "museum", "viewpoint", "gallery", "zoo", "theme_park")
	natureNatural   = set("beach", "waterfall", "peak", "cave_entrance", "spring")
	natureLeisure   = set("park", "nature_reserve", "beach_resort")
	// Жильё и служебные точки — не достопримечательности, даже со ссылкой на Википедию.
	notTouristic = set("hotel", "motel", "hostel", "guest_house", "apartment", "chalet", "camp_site",
		"caravan_site", "information", "yes")
)

// Classify определяет слой объекта по тегам. ok=false — объект на карту не попадает.
// Правило ADR-0014: любое место туристической категории с фото или статьёй храним и показываем.
func Classify(tags map[string]string) (c Classification, ok bool) {
	name := tags["name"] != "" || tags["name:ru"] != ""
	wiki := hasWikiTags(tags)

	switch a := tags["amenity"]; a {
	case "fuel":
		return Classification{LayerFuel, a}, true
	case "charging_station":
		return Classification{LayerCharging, a}, true
	case "cafe", "restaurant", "fast_food":
		if name {
			return Classification{LayerFood, a}, true
		}
	case "toilets":
		return Classification{LayerRest, a}, true
	case "parking":
		return Classification{LayerParking, a}, true
	case "atm", "bureau_de_change", "pharmacy":
		return Classification{LayerUseful, a}, true
	case "bus_station":
		if name {
			return Classification{LayerUseful, a}, true
		}
	}
	switch h := tags["highway"]; h {
	case "rest_area", "services":
		return Classification{LayerRest, h}, true
	}
	if tags["barrier"] == "border_control" {
		return Classification{LayerBorder, "border_control"}, true
	}
	switch s := tags["shop"]; s {
	case "car_repair", "tyres":
		return Classification{LayerCarService, s}, true
	}
	if tags["railway"] == "station" && name {
		return Classification{LayerUseful, "station"}, true
	}

	if t := tags["tourism"]; attractionTypes[t] && name {
		return Classification{LayerAttraction, t}, true
	}
	if h := tags["historic"]; h != "" && name && (notableHistoric[h] || wiki) {
		return Classification{LayerAttraction, h}, true
	}
	// Природных объектов тысячи — без фото или статьи они только шумят на карте
	if n := tags["natural"]; natureNatural[n] && wiki {
		return Classification{LayerNature, n}, true
	}
	if l := tags["leisure"]; natureLeisure[l] && wiki {
		return Classification{LayerNature, l}, true
	}
	if tags["tourism"] == "picnic_site" && wiki {
		return Classification{LayerNature, "picnic_site"}, true
	}

	// Любое другое туристическое место с фото или статьёй
	if wiki && (name || tags["wikidata"] != "") {
		if t := tags["tourism"]; t != "" && !notTouristic[t] {
			return Classification{LayerAttraction, t}, true
		}
		if n := tags["natural"]; n != "" {
			return Classification{LayerNature, n}, true
		}
		if l := tags["leisure"]; l != "" {
			return Classification{LayerNature, l}, true
		}
	}
	return Classification{}, false
}

// hasWikiTags — есть ли ссылка на Википедию, Wikidata или фото на Commons.
func hasWikiTags(tags map[string]string) bool {
	return tags["wikipedia"] != "" || tags["wikidata"] != "" ||
		commonsFile(firstTag(tags, "wikimedia_commons", "image")) != ""
}

// FromTags собирает место из тегов OSM — общие поля для Overpass и импорта выгрузок.
func FromTags(osmID string, c Classification, tags map[string]string, lat, lon float64) POI {
	name := tags["name:ru"]
	if name == "" {
		name = tags["name"]
	}
	return POI{
		OSMID: osmID, Category: LayerCategory(c.Layer), Kind: c.Kind, Name: name, Brand: tags["brand"],
		Lat: lat, Lon: lon,
		OpeningHours: tags["opening_hours"], Website: firstTag(tags, "website", "contact:website"),
		Wikipedia: tags["wikipedia"], Wikidata: tags["wikidata"],
		CommonsFile: commonsFile(firstTag(tags, "wikimedia_commons", "image")),
		Cuisine:     tags["cuisine"],
		Phone:       firstTag(tags, "phone", "contact:phone"),
		Fee:         firstTag(tags, "charge", "fee"),
	}
}

// layerTagKeys — теги, которые нужны слоям сверх общих полей: топливо для фильтра по профилю, места на парковке…
var layerTagKeys = map[Layer][]string{
	LayerFuel:     {"fuel:octane_92", "fuel:octane_95", "fuel:octane_98", "fuel:diesel", "fuel:lpg", "fuel:cng"},
	LayerCharging: {"socket:type2", "socket:type2_combo", "socket:chademo", "socket:tesla_supercharger", "capacity", "fee"},
	LayerParking:  {"capacity", "fee", "parking", "access"},
	LayerRest:     {"fee", "access", "toilets"},
	LayerUseful:   {"operator", "currency:RUB", "currency:USD", "dispensing"},
}

// LayerTags — значения тегов слоя, nil — нет ни одного.
func LayerTags(l Layer, tags map[string]string) map[string]string {
	keys := layerTagKeys[l]
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]string)
	for _, k := range keys {
		if v := tags[k]; v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func set(values ...string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}
