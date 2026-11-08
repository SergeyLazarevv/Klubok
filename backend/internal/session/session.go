// Package session — сессии по cookie klubok_sid. До входа сессия анонимная, и данные пользователя (избранное)
// привязаны к ней. При входе выдаётся новая сессия с user_id, а анонимная переходит пользователю (ADR-0026):
// всё, что сохранили до входа, остаётся с ним.
package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/session/store"
)

const (
	CookieName = "klubok_sid"
	cookieTTL  = 400 * 24 * time.Hour // максимум, который хранят браузеры; продлевается при каждом визите
)

type ctxKey struct{}

type current struct {
	id     uuid.UUID
	userID uuid.UUID // uuid.Nil — сессия анонимная
}

// FromContext — id сессии текущего запроса.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	c, ok := ctx.Value(ctxKey{}).(current)
	return c.id, ok
}

// UserFromContext — пользователь текущего запроса, если он вошёл.
func UserFromContext(ctx context.Context) (uuid.UUID, bool) {
	c, ok := ctx.Value(ctxKey{}).(current)
	return c.userID, ok && c.userID != uuid.Nil
}

type Manager struct {
	pool *pgxpool.Pool
	q    *store.Queries
	log  *slog.Logger
}

func NewManager(pool *pgxpool.Pool, log *slog.Logger) *Manager {
	return &Manager{pool: pool, q: store.New(pool), log: log}
}

// Middleware кладёт сессию в контекст. Новую сессию создаёт, только если create(r) — то есть когда
// пользователь что-то сохраняет: просмотры, боты и разовые заходы не плодят записи в БД.
func (m *Manager) Middleware(create func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			cur, ok := m.existing(r)
			if !ok && create(r) {
				cur = current{id: uuid.Must(uuid.NewV7())}
				if err := m.q.CreateSession(ctx, store.CreateSessionParams{ID: cur.id}); err != nil {
					m.log.ErrorContext(ctx, "create session", slog.Any("err", err))
					http.Error(w, "session unavailable", http.StatusServiceUnavailable)
					return
				}
				ok = true
			}
			if ok {
				setCookie(w, r, cur.id.String(), int(cookieTTL.Seconds()))
				ctx = context.WithValue(ctx, ctxKey{}, cur)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Login открывает новую сессию пользователя и ставит её в cookie. Прежняя сессия этого браузера закрывается;
// если она была анонимной, то переходит пользователю — adopted возвращает её id (uuid.Nil — такой не было),
// чтобы избранное могло убрать дубли. Новый id, а не та же сессия с user_id — чтобы id, известный до входа
// (например, подсунутый чужой cookie), не стал ключом от аккаунта.
func (m *Manager) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) (adopted uuid.UUID, err error) {
	id := uuid.Must(uuid.NewV7())
	prev, hadPrev := m.existing(r)
	err = pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
		q := m.q.WithTx(tx)
		if err := q.CreateSession(ctx, store.CreateSessionParams{ID: id, UserID: pgtype.UUID{Bytes: userID, Valid: true}}); err != nil {
			return err
		}
		if !hadPrev {
			return nil
		}
		if prev.userID == uuid.Nil {
			n, err := q.AdoptSession(ctx, store.AdoptSessionParams{UserID: pgtype.UUID{Bytes: userID, Valid: true}, ID: prev.id})
			if err != nil {
				return err
			}
			if n > 0 {
				adopted = prev.id
			}
			return nil
		}
		return q.EndSession(ctx, prev.id)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("login session: %w", err)
	}
	setCookie(w, r, id.String(), int(cookieTTL.Seconds()))
	return adopted, nil
}

// Logout закрывает сессию и стирает cookie. Данные остаются за пользователем и вернутся при следующем входе;
// после выхода браузер начинает с чистой анонимной сессии.
func (m *Manager) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if cur, ok := m.existing(r); ok {
		if err := m.q.EndSession(ctx, cur.id); err != nil {
			return fmt.Errorf("logout: %w", err)
		}
	}
	setCookie(w, r, "", -1)
	return nil
}

// existing — открытая сессия из cookie (неизвестный или закрытый id — как будто cookie нет).
func (m *Manager) existing(r *http.Request) (current, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return current{}, false
	}
	id, err := uuid.Parse(c.Value)
	if err != nil {
		return current{}, false
	}
	userID, err := m.q.TouchSession(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return current{}, false
	}
	if err != nil {
		m.log.WarnContext(r.Context(), "touch session", slog.Any("err", err))
		return current{}, false
	}
	cur := current{id: id}
	if userID.Valid {
		cur.userID = userID.Bytes
	}
	return cur, true
}

func setCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: value, Path: "/",
		MaxAge: maxAge, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: Secure(r),
	})
}

// Secure — запрос пришёл по HTTPS (напрямую или через прокси): только тогда cookie помечаем Secure.
func Secure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
