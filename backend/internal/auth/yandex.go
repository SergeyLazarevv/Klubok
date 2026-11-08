package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"
)

// Yandex — Яндекс ID. Приложение регистрируется на oauth.yandex.ru (платформа «Веб-сервисы», доступы
// login:info, login:email, login:avatar), Redirect URI — <KLUBOK_PUBLIC_URL>/api/v1/auth/yandex/callback.
type Yandex struct {
	clientID, secret string
	http             *http.Client
}

func NewYandex(clientID, secret string) *Yandex {
	return &Yandex{clientID: clientID, secret: secret, http: &http.Client{Timeout: 10 * time.Second}}
}

func (y *Yandex) Name() string { return "yandex" }

func (y *Yandex) AuthURL(state, challenge, redirectURI string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {y.clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return "https://oauth.yandex.ru/authorize?" + q.Encode()
}

func (y *Yandex) Exchange(ctx context.Context, code, verifier, _ string, _ url.Values) (Profile, error) {
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	err := postForm(ctx, y.http, "https://oauth.yandex.ru/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {y.clientID},
		"client_secret": {y.secret},
		"code_verifier": {verifier},
	}, &tok)
	if err != nil {
		return Profile{}, err
	}
	if tok.AccessToken == "" {
		return Profile{}, errors.New("yandex: empty access token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://login.yandex.ru/info?format=json", nil)
	if err != nil {
		return Profile{}, err
	}
	req.Header.Set("Authorization", "OAuth "+tok.AccessToken)
	var info struct {
		ID            string `json:"id"`
		Login         string `json:"login"`
		DisplayName   string `json:"display_name"`
		FirstName     string `json:"first_name"`
		LastName      string `json:"last_name"`
		DefaultEmail  string `json:"default_email"`
		AvatarID      string `json:"default_avatar_id"`
		IsAvatarEmpty bool   `json:"is_avatar_empty"`
	}
	if err := doJSON(y.http, req, &info); err != nil {
		return Profile{}, err
	}
	if info.ID == "" {
		return Profile{}, errors.New("yandex: empty user id")
	}
	p := Profile{Subject: info.ID, Email: info.DefaultEmail, Name: fullName(info.FirstName, info.LastName)}
	if p.Name == "" {
		p.Name = fullName(info.DisplayName, info.Login)
	}
	if info.AvatarID != "" && !info.IsAvatarEmpty {
		p.AvatarURL = "https://avatars.yandex.net/get-yapic/" + info.AvatarID + "/islands-200"
	}
	return p, nil
}
