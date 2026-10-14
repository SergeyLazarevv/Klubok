// Package config читает настройки приложения из переменных окружения.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	LogFormat       string // text | json
	OSRMURL         string
	OverpassURL     string

	// Импорт выгрузок OSM (ADR-0014)
	OSMDir       string // где хранить выгрузки .osm.pbf
	OSMRegions   string // регионы Geofabrik через запятую; пусто — список по умолчанию
	OSMMaxLag    int    // отстали больше чем на столько дневных дифов — скачать выгрузку целиком
	Osmium       string // путь к osmium
	GeofabrikURL string

	// Как добраться внутри города (публичные серверы — только для разработки)
	PhotonURL   string
	OSRMFootURL string

	TravelpayoutsToken  string
	TravelpayoutsMarker string
	TravelpayoutsTRS    string // ID проекта, к которому подключены программы брендов (Островок)
}

func Load() Config {
	return Config{
		HTTPAddr:        env("KLUBOK_HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:     env("KLUBOK_DATABASE_URL", "postgres://klubok:klubok@127.0.0.1:5432/klubok?sslmode=disable"),
		ShutdownTimeout: 10 * time.Second,
		LogFormat:       env("KLUBOK_LOG_FORMAT", "text"),
		// Публичный демо-сервер OSRM — только для разработки; результаты кэшируются в БД.
		OSRMURL: env("KLUBOK_OSRM_URL", "https://router.project-osrm.org"),
		// Публичный Overpass — только для разработки; места кэшируются в БД.
		OverpassURL: env("KLUBOK_OVERPASS_URL", "https://overpass-api.de/api/interpreter"),

		OSMDir:       env("KLUBOK_OSM_DIR", "data/osm"),
		OSMRegions:   env("KLUBOK_OSM_REGIONS", ""),
		OSMMaxLag:    envInt("KLUBOK_OSM_MAX_LAG", 30),
		Osmium:       env("KLUBOK_OSMIUM", "osmium"),
		GeofabrikURL: env("KLUBOK_GEOFABRIK_URL", "https://download.geofabrik.de"),

		PhotonURL:   env("KLUBOK_PHOTON_URL", "https://photon.komoot.io"),
		OSRMFootURL: env("KLUBOK_OSRM_FOOT_URL", "https://routing.openstreetmap.de/routed-foot"),

		TravelpayoutsToken:  env("TRAVELPAYOUTS_TOKEN", ""),
		TravelpayoutsMarker: env("TRAVELPAYOUTS_MARKER", ""),
		TravelpayoutsTRS:    env("TRAVELPAYOUTS_TRS", ""),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}
