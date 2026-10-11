// Package httpx — общие HTTP-утилиты: ошибки в формате RFC 9457, логирование запросов.
package httpx

import (
	"encoding/json"
	"net/http"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem — тело ошибки application/problem+json.
type Problem struct {
	Type   string       `json:"type,omitempty"`
	Title  string       `json:"title"`
	Status int          `json:"status"`
	Detail string       `json:"detail,omitempty"`
	Errors []FieldError `json:"errors,omitempty"`
}

func WriteProblem(w http.ResponseWriter, p Problem) {
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = http.StatusText(p.Status)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
