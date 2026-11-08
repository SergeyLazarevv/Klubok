package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/auth"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/httpx"
	"github.com/SergeyLazarevv/Klubok/backend/internal/session"
)

func (h *handlers) GetMe(ctx context.Context, _ apigen.GetMeRequestObject) (apigen.GetMeResponseObject, error) {
	out := apigen.Me{Providers: make([]apigen.AuthProvider, 0, len(h.auth.Providers()))}
	for _, p := range h.auth.Providers() {
		out.Providers = append(out.Providers, apigen.AuthProvider(p))
	}
	uid, ok := session.UserFromContext(ctx)
	if !ok {
		return apigen.GetMe200JSONResponse(out), nil
	}
	u, err := h.auth.User(ctx, uid)
	if errors.Is(err, auth.ErrNotFound) {
		return apigen.GetMe200JSONResponse(out), nil
	}
	if err != nil {
		return nil, err
	}
	user := apigen.User{Id: u.ID, Name: u.Name, AvatarUrl: optional(u.AvatarURL), Email: optional(u.Email),
		Providers: make([]apigen.AuthProvider, len(u.Providers))}
	for i, p := range u.Providers {
		user.Providers[i] = apigen.AuthProvider(p)
	}
	out.User = &user
	return apigen.GetMe200JSONResponse(out), nil
}

// authRoutes — сам вход: редиректы на провайдера и обратно, выход. Вне OpenAPI: это переходы браузера,
// а не вызовы API (docs/12-auth.md).
func authRoutes(r chi.Router, d Deps) {
	r.Get(basePath+"/auth/{provider}/start", func(w http.ResponseWriter, r *http.Request) {
		to, err := d.Auth.Start(w, r, chi.URLParam(r, "provider"), r.URL.Query().Get("return"))
		if errors.Is(err, auth.ErrUnknownProvider) {
			httpx.WriteProblem(w, httpx.Problem{Status: http.StatusNotFound, Detail: "login provider not enabled"})
			return
		}
		http.Redirect(w, r, to, http.StatusFound)
	})

	r.Get(basePath+"/auth/{provider}/callback", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		provider := chi.URLParam(r, "provider")
		// Пользователь передумал на странице провайдера — просто возвращаем туда, откуда пришёл
		if r.URL.Query().Get("error") == "access_denied" {
			back := auth.ReturnPath(r)
			_, _, _ = d.Auth.Finish(w, r, provider)
			http.Redirect(w, r, back, http.StatusFound)
			return
		}
		prof, back, err := d.Auth.Finish(w, r, provider)
		if err != nil {
			d.Log.WarnContext(ctx, "login failed", slog.String("provider", provider), slog.Any("err", err))
			http.Redirect(w, r, withQuery(back, "login", "failed"), http.StatusFound)
			return
		}
		uid, err := d.Auth.SignIn(ctx, provider, prof)
		if err != nil {
			d.Log.ErrorContext(ctx, "sign in", slog.String("provider", provider), slog.Any("err", err))
			http.Redirect(w, r, withQuery(back, "login", "failed"), http.StatusFound)
			return
		}
		adopted, err := d.Sessions.Login(ctx, w, r, uid)
		if err != nil {
			d.Log.ErrorContext(ctx, "login session", slog.Any("err", err))
			http.Redirect(w, r, withQuery(back, "login", "failed"), http.StatusFound)
			return
		}
		// Маршруты, сохранённые до входа, уже у пользователя; убираем копии того, что у него и так было
		if adopted != uuid.Nil {
			if n, err := d.Favorites.DropAdoptedDuplicates(ctx, adopted); err != nil {
				d.Log.WarnContext(ctx, "drop adopted duplicates", slog.Any("err", err))
			} else if n > 0 {
				d.Log.InfoContext(ctx, "dropped duplicate favorites after login", slog.Int64("count", n))
			}
		}
		d.Log.InfoContext(ctx, "login", slog.String("provider", provider), slog.String("user", uid.String()),
			slog.Bool("adopted_anonymous", adopted != uuid.Nil))
		http.Redirect(w, r, back, http.StatusFound)
	})

	r.Post(basePath+"/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := d.Sessions.Logout(r.Context(), w, r); err != nil {
			d.Log.ErrorContext(r.Context(), "logout", slog.Any("err", err))
			httpx.WriteProblem(w, httpx.Problem{Status: http.StatusInternalServerError})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// withQuery добавляет параметр к пути возврата (path уже прошёл auth.SafeReturn).
func withQuery(path, key, value string) string {
	u, err := url.Parse(path)
	if err != nil {
		return "/"
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}
