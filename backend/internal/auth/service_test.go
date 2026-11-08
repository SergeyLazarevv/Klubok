package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSafeReturn(t *testing.T) {
	for in, want := range map[string]string{
		"/favorites?x=1":      "/favorites?x=1",
		"":                    "/",
		"https://evil.ru/":    "/",
		"//evil.ru":           "/",
		"/\\evil.ru":          "/",
		"/api/v1/auth/logout": "/",
	} {
		if got := SafeReturn(in); got != want {
			t.Errorf("SafeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}

type stubProvider struct{}

func (stubProvider) Name() string { return "vk" }
func (stubProvider) AuthURL(state, challenge, redirectURI string) string {
	return "https://id.example/authorize?" + url.Values{"state": {state}, "code_challenge": {challenge}, "redirect_uri": {redirectURI}}.Encode()
}
func (stubProvider) Exchange(context.Context, string, string, string, url.Values) (Profile, error) {
	return Profile{Subject: "1"}, nil
}

// Код от чужого входа (state не тот, что в cookie этого браузера) не принимается.
func TestFinishChecksState(t *testing.T) {
	s := NewService(nil, "http://localhost:5173", stubProvider{})
	w := httptest.NewRecorder()
	to, err := s.Start(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/vk/start", nil), "vk", "/explore")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(to)
	if got := u.Query().Get("redirect_uri"); got != "http://localhost:5173/api/v1/auth/vk/callback" {
		t.Errorf("redirect_uri = %q", got)
	}
	cookie := w.Result().Cookies()[0]
	state := u.Query().Get("state")

	call := func(q string, withCookie bool) (string, error) {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/vk/callback?"+q, nil)
		if withCookie {
			r.AddCookie(cookie)
		}
		_, back, err := s.Finish(httptest.NewRecorder(), r, "vk")
		return back, err
	}
	if _, err := call("code=c&state=other", true); err != ErrFlow {
		t.Errorf("чужой state: %v", err)
	}
	if _, err := call("code=c&state="+state, false); err != ErrFlow {
		t.Errorf("без cookie: %v", err)
	}
	back, err := call("code=c&state="+state, true)
	if err != nil || back != "/explore" {
		t.Errorf("свой вход: %q %v", back, err)
	}
}
