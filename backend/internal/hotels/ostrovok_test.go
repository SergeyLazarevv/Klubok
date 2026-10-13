package hotels

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

func TestCitySlugs(t *testing.T) {
	cases := map[string][]string{
		"Almaty":             {"almaty"},
		"Naberezhnye Chelny": {"naberezhnye_chelny", "naberezhnye_chelniy"}, // лишний вариант пробуется, только если первый не подошёл
		"Saint Petersburg":   {"saint_petersburg", "st._petersburg"},
		"Nizhny Novgorod":    {"nizhny_novgorod", "nizhniy_novgorod"},
		"":                   nil,
	}
	for in, want := range cases {
		if got := citySlugs(in); !slices.Equal(got, want) {
			t.Errorf("citySlugs(%q) = %v, want %v", in, got, want)
		}
	}
}

type fakeLinker struct{ calls int }

func (f *fakeLinker) PartnerLink(_ context.Context, link string) (string, error) {
	f.calls++
	return "https://tp.media/r?u=" + link, nil
}

func TestOstrovokLink(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/hotel/kazakhstan/almaty/":
			w.Header().Set("Location", "/hotel/kazakhstan/almaty/?q=354")
			w.WriteHeader(http.StatusFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	linker := &fakeLinker{}
	o := NewOstrovok(linker, slog.New(slog.NewTextHandler(io.Discard, nil)))
	o.base = srv.URL
	in := time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)
	out := in.AddDate(0, 0, 4)

	link, partner := o.Link(context.Background(), geo.Place{CountryCode: "KZ", NameEn: "Almaty"}, in, out, 2)
	if !partner || !strings.HasSuffix(link, srv.URL+"/hotel/kazakhstan/almaty/?dates=02.01.2027-06.01.2027&guests=2") {
		t.Errorf("link = %q, partner = %v", link, partner)
	}
	// Повторно — из кэша: ни проверки города, ни запроса партнёрской ссылки
	_, _ = o.Link(context.Background(), geo.Place{CountryCode: "KZ", NameEn: "Almaty"}, in, out, 2)
	if requests != 1 || linker.calls != 1 {
		t.Errorf("requests = %d, linker calls = %d; want 1 and 1", requests, linker.calls)
	}

	// Города нет на Островке — страница страны
	link, _ = o.Link(context.Background(), geo.Place{CountryCode: "KZ", NameEn: "Nowhere"}, in, out, 1)
	if !strings.Contains(link, "/hotel/kazakhstan/?dates=") {
		t.Errorf("unknown city link = %q, want the country page", link)
	}
	// Островок не ответил — самый вероятный адрес, без запоминания
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slow.Close()
	o.base = slow.URL
	o.client.Timeout = 50 * time.Millisecond
	link, _ = o.Link(context.Background(), geo.Place{CountryCode: "KZ", NameEn: "Shymkent"}, in, out, 1)
	if !strings.Contains(link, "/hotel/kazakhstan/shymkent/?dates=") {
		t.Errorf("timeout link = %q, want the best guess", link)
	}
	if _, cached := o.cities["kazakhstan/Shymkent"]; cached {
		t.Error("a guess without Ostrovok's answer must not be cached")
	}
	o.base = srv.URL

	// Страна неизвестна — главная
	o.linker = nil
	link, partner = o.Link(context.Background(), geo.Place{CountryCode: "ZZ", NameEn: "X"}, in, out, 1)
	if partner || link != srv.URL+"/?dates=02.01.2027-06.01.2027&guests=1" {
		t.Errorf("unknown country link = %q, partner = %v", link, partner)
	}
}
