# Команды разработки. Go: ~/.local/go/bin должен быть в PATH.
# Секреты (TRAVELPAYOUTS_TOKEN и т.п.) — в .env в корне, он не коммитится.
-include .env
export
COMPOSE := docker compose -f deploy/compose.dev.yml
BACKEND := backend

.PHONY: up down gen build test migrate import-places osm-update poi-enrich api web

up:            ## Поднять PostgreSQL, Redis, Mailpit
	$(COMPOSE) up -d

down:          ## Остановить окружение
	$(COMPOSE) down

gen:           ## Сгенерировать код из OpenAPI и SQL
	cd $(BACKEND) && go generate ./... && go tool sqlc generate

build:
	cd $(BACKEND) && go build -o bin/klubok ./cmd/klubok

test:
	cd $(BACKEND) && go vet ./... && go test ./...

migrate: build
	$(BACKEND)/bin/klubok migrate up

import-places: build
	$(BACKEND)/bin/klubok import-places

osm-update: build   ## Места на карте из выгрузок OSM: make osm-update ARGS="--regions asia/armenia"
	$(BACKEND)/bin/klubok osm-update $(ARGS)

poi-enrich: build   ## Описания и фото мест из Википедии: make poi-enrich ARGS="--limit 500"
	$(BACKEND)/bin/klubok poi-enrich $(ARGS)

api: build     ## Запустить API на 127.0.0.1:8080
	$(BACKEND)/bin/klubok api

web:           ## Запустить фронтенд на 127.0.0.1:5173
	cd frontend && npm run dev
