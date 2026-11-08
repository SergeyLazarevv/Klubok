#!/usr/bin/env bash
# Еженедельное обновление мест на карте (ADR-0014): osm-update, затем poi-enrich.
# Пример для crontab (понедельник, 03:30):
#   30 3 * * 1  /opt/klubok/deploy/cron/osm-weekly.sh
# Подробности — docs/ops/osm-import.md.
set -uo pipefail

cd "$(dirname "$0")/../.."
if [ -f .env ]; then
  set -a; . ./.env; set +a
fi

BIN=${KLUBOK_BIN:-backend/bin/klubok}
LOG_DIR=${KLUBOK_LOG_DIR:-data/logs}
mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/osm-$(date +%F).log"

status=0
# Ошибка одного региона не мешает обновить остальные и загрузить фото — но код возврата сохраняем для мониторинга
"$BIN" osm-update >>"$LOG" 2>&1 || status=$?
"$BIN" poi-enrich --limit "${KLUBOK_ENRICH_LIMIT:-5000}" >>"$LOG" 2>&1 || status=$?

# Старые логи — не дольше 90 дней
find "$LOG_DIR" -name 'osm-*.log' -mtime +90 -delete
exit "$status"
