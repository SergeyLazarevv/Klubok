package guides

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sample = `[[Файл:Uralsk view.jpg|thumb|Вид на город]]
'''Уральск''' — город на северо-западе [[Казахстан]]а, на реке [[Урал (река)|Урал]].<ref>Источник</ref>

== Понять ==
Основан в 1613 году как казачий городок. {{нп|Яицкий городок}} Сейчас — областной центр.

* пункт списка, не обзор

== Достопримечательности ==
{{listing|type=see| lat=51.183365 | long=51.378744 | wdid=Q1365936
| name= Михайло-Архангельский собор | alt= | image=Cathedral of the Archangel Uralsk.jpg
| url=http://example.kz/ | content=Старейший храм [[Уральск]]а, {{дата|1751}} года.
}}
{{listing| name=Музей без координат | content=Описание }}

== Еда ==
{{listing | name=Кафе «Ромашка» | lat=abc | long=51.3 | price=от 500 тг | url=not-a-url }}

== Как добраться ==
{{go| name=Аэропорт Уральск | lat=51.150833 | long=51.543056 }}
{{Сноска|не место}}
`

func TestParse(t *testing.T) {
	a := Parse(sample)
	if !strings.HasPrefix(a.Overview, "Уральск — город на северо-западе Казахстана, на реке Урал.") {
		t.Errorf("overview = %q", a.Overview)
	}
	if !strings.Contains(a.Overview, "Основан в 1613 году") || strings.Contains(a.Overview, "пункт списка") || strings.Contains(a.Overview, "Источник") {
		t.Errorf("overview must include «Понять» without lists and refs: %q", a.Overview)
	}
	if len(a.Listings) != 4 {
		t.Fatalf("listings = %d: %+v", len(a.Listings), a.Listings)
	}
	cathedral := a.Listings[0]
	if cathedral.Type != "see" || cathedral.Name != "Михайло-Архангельский собор" || cathedral.Wikidata != "Q1365936" ||
		!cathedral.HasLocation || cathedral.Lat != 51.183365 || cathedral.ImageFile != "Cathedral of the Archangel Uralsk.jpg" ||
		cathedral.URL != "http://example.kz/" || cathedral.Description != "Старейший храм Уральска, года." {
		t.Errorf("cathedral = %+v", cathedral)
	}
	museum := a.Listings[1]
	if museum.Type != "see" || museum.HasLocation {
		t.Errorf("type must come from the section, no coordinates: %+v", museum)
	}
	cafe := a.Listings[2]
	if cafe.Type != "eat" || cafe.HasLocation || cafe.URL != "" || cafe.Price != "от 500 тг" {
		t.Errorf("cafe = %+v", cafe)
	}
	if a.Listings[3].Type != "go" || a.Listings[3].Name != "Аэропорт Уральск" {
		t.Errorf("go listing = %+v", a.Listings[3])
	}
}

func TestClip(t *testing.T) {
	long := strings.Repeat("Предложение номер один. ", 60)
	got := clip(long, 100)
	if !strings.HasSuffix(got, ".") || len([]rune(got)) > 100 {
		t.Errorf("clip = %q", got)
	}
}

func TestFindPageAndPhotos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/w" && q.Get("titles") == "Троицк":
			// неоднозначная страница — пропускаем
			_, _ = w.Write([]byte(`{"query":{"pages":[{"title":"Троицк","revisions":[{"revid":1}],"pageprops":{"disambiguation":""}}]}}`))
		case r.URL.Path == "/w" && q.Get("titles") == "Троицк (город)":
			// статья о другом Троицке, далеко от нужного — тоже пропускаем
			_, _ = w.Write([]byte(`{"query":{"pages":[{"title":"Троицк (город)","revisions":[{"revid":2}],"coordinates":[{"lat":55.48,"lon":37.3}]}]}}`))
		case r.URL.Path == "/w" && q.Get("titles") == "Уральск":
			_, _ = w.Write([]byte(`{"query":{"pages":[{"title":"Уральск","revisions":[{"revid":42}],"coordinates":[{"lat":51.23,"lon":51.37}]}]}}`))
		case r.URL.Path == "/commons":
			if !strings.Contains(q.Get("titles"), "File:a_b.jpg") {
				t.Errorf("titles = %q", q.Get("titles"))
			}
			_, _ = w.Write([]byte(`{"query":{"normalized":[{"from":"File:a_b.jpg","to":"File:A b.jpg"}],
				"pages":[{"title":"File:A b.jpg","imageinfo":[{"thumburl":"https://upload/a.jpg","descriptionurl":"https://commons/a",
				"extmetadata":{"Artist":{"value":"<a href=\"x\">Ivan</a>"},"LicenseShortName":{"value":"CC BY-SA 4.0"}}}]},
				{"title":"File:Missing.jpg","missing":true}]}}`))
		default:
			_, _ = w.Write([]byte(`{"query":{"pages":[{"title":"x","missing":true}]}}`))
		}
	}))
	defer srv.Close()
	w := NewWikivoyage()
	w.api, w.commonsAPI, w.pageBase = srv.URL+"/w", srv.URL+"/commons", "https://ru.wikivoyage.org/wiki/"

	page, err := w.FindPage(context.Background(), "Уральск", 51.2, 51.4)
	if err != nil || page == nil || page.Revision != 42 || page.URL != "https://ru.wikivoyage.org/wiki/%D0%A3%D1%80%D0%B0%D0%BB%D1%8C%D1%81%D0%BA" {
		t.Errorf("page = %+v, err = %v", page, err)
	}
	page, err = w.FindPage(context.Background(), "Троицк", 54.08, 61.56) // челябинский Троицк
	if err != nil || page != nil {
		t.Errorf("disambiguation and a far namesake must be skipped: page = %+v, err = %v", page, err)
	}

	photos, err := w.Photos(context.Background(), []string{"a_b.jpg", "Missing.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if p := photos["a_b.jpg"]; p.URL != "https://upload/a.jpg" || p.Credit != "Ivan · CC BY-SA 4.0" {
		t.Errorf("photo = %+v", p)
	}
	if _, ok := photos["Missing.jpg"]; ok {
		t.Error("missing file must not be returned")
	}
}
