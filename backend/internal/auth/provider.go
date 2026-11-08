// Package auth — вход через Яндекс ID и VK ID (ADR-0026): OAuth 2.0 Authorization Code + PKCE и учёт
// пользователей. Сессию после входа выдаёт пакет session, своих паролей и токенов здесь нет.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Profile — то, что провайдер рассказал о пользователе. Subject — постоянный id у провайдера.
type Profile struct {
	Subject   string
	Name      string
	Email     string // пусто — провайдер не отдал
	AvatarURL string
}

// Provider — способ входа. AuthURL ведёт на страницу провайдера, Exchange меняет код из обратного
// редиректа на профиль; callback — все параметры обратного редиректа (VK присылает в нём device_id).
type Provider interface {
	Name() string
	AuthURL(state, challenge, redirectURI string) string
	Exchange(ctx context.Context, code, verifier, redirectURI string, callback url.Values) (Profile, error)
}

// randomToken — случайная строка для state и code_verifier (43 символа base64url — минимум для PKCE).
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// challenge — code_challenge метода S256.
func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// postForm шлёт форму и разбирает JSON-ответ; ошибку OAuth (error, error_description) превращает в error.
func postForm(ctx context.Context, c *http.Client, endpoint string, form url.Values, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doJSON(c, req, dst)
}

func doJSON(c *http.Client, req *http.Request, dst any) error {
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var oauthErr struct {
		Error       any    `json:"error"` // у VK бывает объектом
		Description string `json:"error_description"`
	}
	if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Error != nil {
		return fmt.Errorf("%s %s: %v %s", req.Method, req.URL.Host+req.URL.Path, oauthErr.Error, oauthErr.Description)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: status %d", req.Method, req.URL.Host+req.URL.Path, resp.StatusCode)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("%s %s: decode: %w", req.Method, req.URL.Host+req.URL.Path, err)
	}
	return nil
}

// fullName — «Имя Фамилия» без лишних пробелов, если одной из частей нет.
func fullName(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}
