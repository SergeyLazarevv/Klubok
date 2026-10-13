package poi

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		tags  map[string]string
		layer Layer
		ok    bool
	}{
		{map[string]string{"amenity": "fuel"}, LayerFuel, true},
		{map[string]string{"amenity": "cafe"}, "", false}, // кафе без названия
		{map[string]string{"amenity": "cafe", "name": "Кофейня"}, LayerFood, true},
		{map[string]string{"barrier": "border_control"}, LayerBorder, true},
		{map[string]string{"historic": "memorial", "name": "Памятник"}, "", false}, // массовые мемориалы — шум
		{map[string]string{"historic": "memorial", "name": "Мамаев курган", "wikidata": "Q1"}, LayerAttraction, true},
		{map[string]string{"natural": "beach", "name": "Пляж"}, "", false}, // природа — только с фото или статьёй
		{map[string]string{"natural": "beach", "name": "Пляж", "wikipedia": "ru:Пляж"}, LayerNature, true},
		{map[string]string{"natural": "water", "name": "Озеро", "wikidata": "Q3"}, LayerNature, true}, // любое с фото
		{map[string]string{"tourism": "hotel", "name": "Отель", "wikidata": "Q4"}, "", false},
		{map[string]string{"highway": "primary", "wikidata": "Q5"}, "", false},
		{map[string]string{"image": "https://example.com/x.jpg", "natural": "peak", "name": "Пик"}, "", false}, // фото не с Commons
	}
	for _, c := range cases {
		got, ok := Classify(c.tags)
		if ok != c.ok || got.Layer != c.layer {
			t.Errorf("Classify(%v) = %v, %v; want %v, %v", c.tags, got.Layer, ok, c.layer, c.ok)
		}
	}
}
