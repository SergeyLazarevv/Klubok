// Package httpapi собирает HTTP API: роутер, проверку запросов по OpenAPI и обработчики модулей.
package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	oapimw "github.com/oapi-codegen/nethttp-middleware"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/platform/httpx"
	"github.com/SergeyLazarevv/Klubok/backend/internal/search"
)

const basePath = "/api/v1"

type Deps struct {
	Log    *slog.Logger
	Pool   *pgxpool.Pool
	Geo    *geo.Service
	Search *search.Service
}

func NewRouter(d Deps) (http.Handler, error) {
	spec, err := apigen.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load openapi spec: %w", err)
	}
	spec.Servers = nil // префикс снимает валидатор (Prefix ниже)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, httpx.Logger(d.Log), middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Pool.Ping(ctx); err != nil {
			httpx.WriteProblem(w, httpx.Problem{Status: http.StatusServiceUnavailable, Detail: "database unavailable"})
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Group(func(api chi.Router) {
		api.Use(oapimw.OapiRequestValidatorWithOptions(spec, &oapimw.Options{
			Prefix:               basePath,
			DoNotValidateServers: true,
			Options:              openapi3filter.Options{MultiError: true},
			ErrorHandlerWithOpts: validationError,
		}))
		strict := apigen.NewStrictHandlerWithOptions(&handlers{geo: d.Geo, search: d.Search, log: d.Log}, nil, apigen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
				httpx.WriteProblem(w, httpx.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				d.Log.ErrorContext(r.Context(), "handler failed", slog.String("path", r.URL.Path), slog.Any("err", err))
				httpx.WriteProblem(w, httpx.Problem{Status: http.StatusInternalServerError})
			},
		})
		apigen.HandlerWithOptions(strict, apigen.ChiServerOptions{
			BaseURL:          basePath,
			BaseRouter:       api,
			ErrorHandlerFunc: paramError,
		})
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteProblem(w, httpx.Problem{Status: http.StatusNotFound})
	})
	return r, nil
}

// validationError превращает ошибки проверки по OpenAPI в 422 со списком полей.
func validationError(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts oapimw.ErrorHandlerOpts) {
	if opts.MatchedRoute == nil {
		httpx.WriteProblem(w, httpx.Problem{Status: opts.StatusCode})
		return
	}
	var fields []httpx.FieldError
	collectFieldErrors(err, &fields)
	httpx.WriteProblem(w, httpx.Problem{
		Status: http.StatusUnprocessableEntity,
		Title:  "Validation failed",
		Errors: fields,
	})
}

// paramError — параметр прошёл проверку схемы, но не разобрался в Go-тип (например, кривой UUID).
func paramError(w http.ResponseWriter, _ *http.Request, err error) {
	var formatErr *apigen.InvalidParamFormatError
	if errors.As(err, &formatErr) {
		httpx.WriteProblem(w, httpx.Problem{
			Status: http.StatusUnprocessableEntity,
			Title:  "Validation failed",
			Errors: []httpx.FieldError{{Field: formatErr.ParamName, Message: "invalid format"}},
		})
		return
	}
	httpx.WriteProblem(w, httpx.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
}

func collectFieldErrors(err error, out *[]httpx.FieldError) {
	var multi interface{ Unwrap() []error }
	if errors.As(err, &multi) {
		for _, e := range multi.Unwrap() {
			collectFieldErrors(e, out)
		}
		return
	}
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) && reqErr.Parameter != nil {
		msg := reqErr.Reason
		var schemaErr *openapi3.SchemaError
		if errors.As(reqErr.Err, &schemaErr) {
			msg = schemaErr.Reason
		} else if reqErr.Err != nil {
			msg = reqErr.Err.Error()
		}
		*out = append(*out, httpx.FieldError{Field: reqErr.Parameter.Name, Message: msg})
		return
	}
	*out = append(*out, httpx.FieldError{Field: "", Message: err.Error()})
}
