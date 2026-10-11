// Package config читает настройки приложения из переменных окружения.
package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	LogFormat       string // text | json
	OSRMURL         string

	TravelpayoutsToken  string
	TravelpayoutsMarker string
}

func Load() Config {
	return Config{
		HTTPAddr:        env("KLUBOK_HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:     env("KLUBOK_DATABASE_URL", "postgres://klubok:klubok@127.0.0.1:5432/klubok?sslmode=disable"),
		ShutdownTimeout: 10 * time.Second,
		LogFormat:       env("KLUBOK_LOG_FORMAT", "text"),
		// Публичный демо-сервер OSRM — только для разработки; результаты кэшируются в БД.
		OSRMURL: env("KLUBOK_OSRM_URL", "https://router.project-osrm.org"),

		TravelpayoutsToken:  env("TRAVELPAYOUTS_TOKEN", ""),
		TravelpayoutsMarker: env("TRAVELPAYOUTS_MARKER", ""),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
