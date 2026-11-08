package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/auth"
	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/database"
	"github.com/SergeyLazarevv/Klubok/backend/internal/session"
)

// fakeProvider — провайдер входа без сети: «страница провайдера» сразу возвращает на callback с кодом,
// код — это id пользователя у провайдера.
type fakeProvider struct{}

func (fakeProvider) Name() string { return "yandex" }
func (fakeProvider) AuthURL(state, _, redirectURI string) string {
	return redirectURI + "?" + url.Values{"state": {state}, "code": {"fake"}}.Encode()
}
func (fakeProvider) Exchange(_ context.Context, _, verifier, _ string, cb url.Values) (auth.Profile, error) {
	if verifier == "" {
		return auth.Profile{}, fmt.Errorf("no verifier")
	}
	return auth.Profile{Subject: subject, Name: "Тест Тестов"}, nil
}

var subject string

// TestLoginKeepsFavorites — сохранённое до входа остаётся у пользователя, видно на другом устройстве,
// дубли не плодятся, после выхода браузер начинает с чистой сессии. Нужна БД с миграциями:
// KLUBOK_TEST_DATABASE_URL (например, база из make up).
func TestLoginKeepsFavorites(t *testing.T) {
	dsn := os.Getenv("KLUBOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KLUBOK_TEST_DATABASE_URL не задан")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Skip("БД недоступна:", err)
	}
	defer pool.Close()
	subject = "test-" + uuid.NewString()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM sessions WHERE user_id IN (SELECT user_id FROM user_identities WHERE subject = $1)`, subject)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN (SELECT user_id FROM user_identities WHERE subject = $1)`, subject)
	})

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Обратный редирект провайдера ведёт на этот же сервер, а его адрес известен только после запуска
	var h http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { h.ServeHTTP(w, r) }))
	defer srv.Close()
	h, err = NewRouter(Deps{
		Log: log, Pool: pool,
		Sessions:  session.NewManager(pool, log),
		Favorites: favorites.NewService(pool),
		Auth:      auth.NewService(pool, srv.URL, fakeProvider{}),
	})
	if err != nil {
		t.Fatal(err)
	}

	browser := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar}
	}
	save := func(c *http.Client, to string) string {
		body := fmt.Sprintf(`{"title":"t","params":{"from":"%s","to":"%s","date_from":"2030-01-01","date_to":"2030-01-05"},
			"pick":{"kind":"drive"},"snapshot":{"total":{"amount":100,"currency":"RUB"},"per_person":{"amount":100,"currency":"RUB"}}}`,
			"00000000-0000-0000-0000-000000000001", to)
		resp, err := c.Post(srv.URL+"/api/v1/favorites", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 201 && resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("save: %d %s", resp.StatusCode, b)
		}
		var f struct{ ID string }
		_ = json.NewDecoder(resp.Body).Decode(&f)
		return f.ID
	}
	list := func(c *http.Client) []string {
		resp, err := c.Get(srv.URL + "/api/v1/favorites")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out struct{ Items []struct{ ID string } }
		_ = json.NewDecoder(resp.Body).Decode(&out)
		ids := make([]string, len(out.Items))
		for i, f := range out.Items {
			ids[i] = f.ID
		}
		return ids
	}
	login := func(c *http.Client) {
		resp, err := c.Get(srv.URL + "/api/v1/auth/yandex/start?return=/favorites")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.Request.URL.Path != "/favorites" || resp.Request.URL.Query().Get("login") != "" {
			t.Fatalf("после входа: %s", resp.Request.URL)
		}
	}
	me := func(c *http.Client) bool {
		resp, err := c.Get(srv.URL + "/api/v1/me")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m struct{ User *struct{ Name string } }
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return m.User != nil
	}

	// Телефон: сохранил анонимно и вошёл — маршрут остался
	phone := browser()
	a := save(phone, "00000000-0000-0000-0000-00000000000a")
	login(phone)
	if !me(phone) {
		t.Fatal("после входа /me без пользователя")
	}
	if got := list(phone); len(got) != 1 || got[0] != a {
		t.Fatalf("телефон после входа: %v, ждали [%s]", got, a)
	}

	// Ноутбук: анонимно сохранил тот же маршрут и новый, вошёл — видно оба, копии нет
	laptop := browser()
	save(laptop, "00000000-0000-0000-0000-00000000000a")
	b := save(laptop, "00000000-0000-0000-0000-00000000000b")
	login(laptop)
	if got := list(laptop); len(got) != 2 || !(got[0] == b && got[1] == a) {
		t.Fatalf("ноутбук после входа: %v, ждали [%s %s]", got, b, a)
	}
	// Вошедший сохраняет то, что уже есть с другого устройства, — та же запись
	if id := save(laptop, "00000000-0000-0000-0000-00000000000a"); id != a {
		t.Fatalf("повторное сохранение: %s, ждали %s", id, a)
	}
	if got := list(phone); len(got) != 2 {
		t.Fatalf("телефон видит маршруты ноутбука: %v", got)
	}

	// Выход: браузер чистый, при новом входе всё на месте
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/logout", nil)
	if resp, err := phone.Do(req); err != nil || resp.StatusCode != 204 {
		t.Fatalf("logout: %v %v", resp, err)
	}
	if me(phone) || len(list(phone)) != 0 {
		t.Fatal("после выхода видны данные пользователя")
	}
	login(phone)
	if got := list(phone); len(got) != 2 {
		t.Fatalf("после повторного входа: %v", got)
	}
}
