package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/auth/store"
)

const (
	flowCookie = "klubok_oauth"
	flowPath   = "/api/v1/auth"
	flowTTL    = 10 * time.Minute
)

var (
	ErrUnknownProvider = errors.New("auth: unknown or disabled provider")
	ErrFlow            = errors.New("auth: no login in progress or state mismatch")
	ErrNotFound        = errors.New("auth: user not found")
)

// User — пользователь для шапки и личного кабинета.
type User struct {
	ID        uuid.UUID
	Name      string
	AvatarURL string
	Email     string
	Providers []string
}

type Service struct {
	pool      *pgxpool.Pool
	q         *store.Queries
	publicURL string // откуда открыт сайт — из него собирается адрес обратного редиректа
	providers map[string]Provider
	order     []string
}

// NewService — провайдеры без настроек (nil) не включаются: кнопки входа через них не будет.
func NewService(pool *pgxpool.Pool, publicURL string, providers ...Provider) *Service {
	s := &Service{pool: pool, q: store.New(pool), publicURL: strings.TrimRight(publicURL, "/"), providers: map[string]Provider{}}
	for _, p := range providers {
		s.providers[p.Name()] = p
		s.order = append(s.order, p.Name())
	}
	return s
}

// Providers — включённые способы входа, в порядке кнопок.
func (s *Service) Providers() []string { return s.order }

func (s *Service) redirectURI(provider string) string {
	return s.publicURL + flowPath + "/" + provider + "/callback"
}

// flow — незаконченный вход: живёт в cookie браузера, который его начал, 10 минут. Сверка state с cookie
// не даёт подсунуть чужой код (вход в чужой аккаунт по ссылке), verifier — секрет PKCE.
type flow struct {
	Provider string `json:"p"`
	State    string `json:"s"`
	Verifier string `json:"v"`
	Return   string `json:"r"`
}

// Start начинает вход: запоминает его в cookie и возвращает адрес страницы провайдера.
// returnTo — куда вернуть после входа (путь на нашем сайте).
func (s *Service) Start(w http.ResponseWriter, r *http.Request, provider, returnTo string) (string, error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", ErrUnknownProvider
	}
	f := flow{Provider: provider, State: randomToken(), Verifier: randomToken(), Return: SafeReturn(returnTo)}
	raw, _ := json.Marshal(f)
	http.SetCookie(w, &http.Cookie{
		Name: flowCookie, Value: base64.RawURLEncoding.EncodeToString(raw), Path: flowPath,
		MaxAge: int(flowTTL.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure(r),
	})
	return p.AuthURL(f.State, challenge(f.Verifier), s.redirectURI(provider)), nil
}

// ReturnPath — куда вернуть пользователя по незаконченному входу (для ошибок и отмены); нет входа — на главную.
func ReturnPath(r *http.Request) string {
	if f, err := readFlow(r); err == nil {
		return f.Return
	}
	return "/"
}

// Finish завершает вход по обратному редиректу провайдера: сверяет state, меняет код на профиль.
// Cookie входа стирается в любом случае — повторить тот же редирект нельзя.
func (s *Service) Finish(w http.ResponseWriter, r *http.Request, provider string) (Profile, string, error) {
	http.SetCookie(w, &http.Cookie{Name: flowCookie, Path: flowPath, MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure(r)})
	p, ok := s.providers[provider]
	if !ok {
		return Profile{}, "/", ErrUnknownProvider
	}
	f, err := readFlow(r)
	q := r.URL.Query()
	if err != nil || f.Provider != provider || f.State == "" || q.Get("state") != f.State {
		return Profile{}, "/", ErrFlow
	}
	code := q.Get("code")
	if code == "" {
		return Profile{}, f.Return, fmt.Errorf("%s: no code in callback (error=%q)", provider, q.Get("error"))
	}
	prof, err := p.Exchange(r.Context(), code, f.Verifier, s.redirectURI(provider), q)
	if err != nil {
		return Profile{}, f.Return, fmt.Errorf("%s exchange: %w", provider, err)
	}
	return prof, f.Return, nil
}

func readFlow(r *http.Request) (flow, error) {
	c, err := r.Cookie(flowCookie)
	if err != nil {
		return flow{}, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return flow{}, err
	}
	var f flow
	if err := json.Unmarshal(raw, &f); err != nil {
		return flow{}, err
	}
	f.Return = SafeReturn(f.Return)
	return f, nil
}

// SafeReturn пропускает только путь на нашем сайте: «//evil.ru» и полные адреса превращаются в «/».
func SafeReturn(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.HasPrefix(p, "/api/") {
		return "/"
	}
	return p
}

// SignIn находит пользователя по аккаунту провайдера или заводит нового.
func (s *Service) SignIn(ctx context.Context, provider string, p Profile) (uuid.UUID, error) {
	for attempt := 0; ; attempt++ {
		id, err := s.signIn(ctx, provider, p)
		// Два первых входа одновременно (двойной клик) — второй упрётся в ключ и найдёт уже заведённого
		var pgErr *pgconn.PgError
		if attempt == 0 && errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue
		}
		return id, err
	}
}

func (s *Service) signIn(ctx context.Context, provider string, p Profile) (uuid.UUID, error) {
	var userID uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		id, err := q.FindIdentity(ctx, store.FindIdentityParams{Provider: provider, Subject: p.Subject})
		if err == nil {
			userID = id
			return q.TouchUserLogin(ctx, store.TouchUserLoginParams{ID: id, AvatarUrl: text(p.AvatarURL), Email: text(p.Email)})
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		userID = uuid.Must(uuid.NewV7())
		if err := q.CreateUser(ctx, store.CreateUserParams{ID: userID, Name: p.Name, AvatarUrl: text(p.AvatarURL), Email: text(p.Email)}); err != nil {
			return err
		}
		return q.CreateIdentity(ctx, store.CreateIdentityParams{Provider: provider, Subject: p.Subject, UserID: userID, Email: text(p.Email)})
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("sign in: %w", err)
	}
	return userID, nil
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (User, error) {
	r, err := s.q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return User{ID: r.ID, Name: r.Name, AvatarURL: r.AvatarUrl.String, Email: r.Email.String, Providers: r.Providers}, nil
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func secure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
