package hotels

import (
	"net/url"
	"testing"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

func TestSutochnoLink(t *testing.T) {
	in := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)

	link, ok := SutochnoLink(geo.Place{Name: "Казань", CountryCode: "RU"}, in, out, 2)
	if !ok {
		t.Fatal("Russia must be supported")
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "sutochno.ru" || q.Get("term") != "Казань" || q.Get("occupied") != "2026-11-16;2026-11-21" || q.Get("guests_adults") != "2" {
		t.Errorf("link = %s", link)
	}

	if _, ok := SutochnoLink(geo.Place{Name: "Алматы", CountryCode: "KZ"}, in, out, 0); !ok {
		t.Error("Kazakhstan must be supported")
	}
	if _, ok := SutochnoLink(geo.Place{Name: "Рим", CountryCode: "IT"}, in, out, 1); ok {
		t.Error("Italy is not covered by Sutochno")
	}
}
