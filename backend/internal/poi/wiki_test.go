package poi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWikiLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.URL.Path == "/en" && q.Get("prop") == "pageprops":
			if q.Get("titles") != "Ascension Cathedral" {
				t.Errorf("pageprops titles = %q", q.Get("titles"))
			}
			_, _ = w.Write([]byte(`{"query":{"pages":{"1":{"pageprops":{"wikibase_item":"Q42"}}}}}`))
		case r.URL.Path == "/wikidata":
			_, _ = w.Write([]byte(`{"entities":{"Q42":{
				"claims":{"P18":[{"mainsnak":{"datavalue":{"value":"Cathedral.jpg"}}}]},
				"sitelinks":{"ruwiki":{"title":"Вознесенский собор"}},
				"descriptions":{"ru":{"value":"храм в Алматы"}}}}}`))
		case r.URL.Path == "/ru" && q.Get("prop") == "extracts":
			if q.Get("titles") != "Вознесенский собор" {
				t.Errorf("extract titles = %q, want the Russian article", q.Get("titles"))
			}
			_, _ = w.Write([]byte(`{"query":{"pages":{"2":{"extract":"Вознесенский собор (каз. Вознесение соборы)  — православный\nхрам."}}}}`))
		case r.URL.Path == "/commons":
			if q.Get("titles") != "File:Cathedral.jpg" {
				t.Errorf("commons titles = %q", q.Get("titles"))
			}
			_, _ = w.Write([]byte(`{"query":{"pages":{"-1":{"imageinfo":[{
				"thumburl":"https://upload.wikimedia.org/x/640px-Cathedral.jpg",
				"descriptionurl":"https://commons.wikimedia.org/wiki/File:Cathedral.jpg",
				"extmetadata":{"Artist":{"value":"<a href=\"//commons.wikimedia.org/wiki/User:Ivan\">Ivan &amp; Co</a>"},
				               "LicenseShortName":{"value":"CC BY-SA 4.0"}}}]}}}}`))
		default:
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	w := NewWiki()
	w.wikidataAPI = srv.URL + "/wikidata"
	w.commonsAPI = srv.URL + "/commons"
	w.wikipediaAPI = func(lang string) string { return srv.URL + "/" + lang }

	d, qid, err := w.Lookup(context.Background(), POI{Wikipedia: "en:Ascension Cathedral"})
	if err != nil {
		t.Fatal(err)
	}
	if qid != "Q42" {
		t.Errorf("qid = %q", qid)
	}
	if d.Description != "Вознесенский собор — православный храм." {
		t.Errorf("description = %q", d.Description)
	}
	if d.PhotoURL != "https://upload.wikimedia.org/x/640px-Cathedral.jpg" || d.PhotoCredit != "Ivan & Co · CC BY-SA 4.0" {
		t.Errorf("photo = %q, credit = %q", d.PhotoURL, d.PhotoCredit)
	}
}

func TestCommonsFile(t *testing.T) {
	cases := map[string]string{
		"File:Samara Drama Theatre.jpg":                                           "Samara Drama Theatre.jpg",
		"https://commons.wikimedia.org/wiki/File:Samara_Drama_Theatre%282%29.jpg": "Samara Drama Theatre(2).jpg",
		"https://example.com/photo.jpg":                                           "",
		"":                                                                        "",
	}
	for in, want := range cases {
		if got := commonsFile(in); got != want {
			t.Errorf("commonsFile(%q) = %q, want %q", in, got, want)
		}
	}
}
