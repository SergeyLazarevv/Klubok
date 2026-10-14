// Package session — анонимные сессии: пока нет авторизации, данные пользователя (избранное) привязаны
// к сессии из cookie klubok_sid. После входа сессии проставится user_id.
package session

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/session/store"
)

const (
	CookieName = "klubok_sid"
	cookieTTL  = 400 * 24 * time.Hour // максимум, который хранят браузеры; продлевается при каждом визите
)

type ctxKey struct{}

// FromContext — id сессии текущего запроса.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

type Manager struct {
	q   *store.Queries
	log *slog.Logger
}

func NewManager(pool *pgxpool.Pool, log *slog.Logger) *Manager {
	return &Manager{q: store.New(pool), log: log}
}

// Middleware кладёт сессию в контекст. Новую сессию создаёт, только если create(r) — то есть когда
// пользователь что-то сохраняет: просмотры, боты и разовые заходы не плодят записи в БД.
func (m *Manager) Middleware(create func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			id, ok := m.existing(r)
			if !ok && create(r) {
				id = uuid.Must(uuid.NewV7())
				if err := m.q.CreateSession(ctx, id); err != nil {
					m.log.ErrorContext(ctx, "create session", slog.Any("err", err))
					http.Error(w, "session unavailable", http.StatusServiceUnavailable)
					return
				}
				ok = true
			}
			if ok {
				http.SetCookie(w, &http.Cookie{
					Name: CookieName, Value: id.String(), Path: "/",
					MaxAge: int(cookieTTL.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode,
					Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
				})
				ctx = context.WithValue(ctx, ctxKey{}, id)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// existing — сессия из cookie, если она есть в БД (неизвестный id — как будто cookie нет).
func (m *Manager) existing(r *http.Request) (uuid.UUID, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(c.Value)
	if err != nil {
		return uuid.Nil, false
	}
	found, err := m.q.TouchSession(r.Context(), id)
	if err != nil {
		m.log.WarnContext(r.Context(), "touch session", slog.Any("err", err))
		return uuid.Nil, false
	}
	return id, found
}
