# ADR-0011. Деплой: Docker Compose + Caddy на одном сервере

**Статус:** Предложено · 2026-10-12

## Контекст
MVP, один пользователь, бюджет минимальный. Нужен HTTPS, раздача PWA, API, воркер, PostgreSQL, Redis.

## Решение
Одна облачная ВМ Selectel, Docker Compose. Caddy вместо Nginx: автоматически получает и продлевает сертификаты Let's Encrypt, конфиг в несколько строк. Деплой — GitHub Actions: сборка образов → GitHub Container Registry → `docker compose pull && up -d` на сервере по SSH.

## Последствия
- Минимальная стоимость и простота; нет отказоустойчивости (приемлемо для MVP).
- Бэкапы PostgreSQL — cron с `pg_dump` в объектное хранилище Selectel.
- При росте: управляемые PostgreSQL/Redis Selectel, затем несколько инстансов api за балансировщиком.
