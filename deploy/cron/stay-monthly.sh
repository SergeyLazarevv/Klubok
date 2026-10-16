#!/usr/bin/env bash
# Ежемесячное обновление цен на жильё по городам карты (ADR-0021): Росстат публикует цены за месяц
# в середине следующего. Пишет stay_prices и сбрасывает кэш Redis. Пример для crontab (20-го, 05:10):
#   10 5 20 * *  /opt/klubok/deploy/cron/stay-monthly.sh
set -uo pipefail

cd "$(dirname "$0")/../.."
if [ -f .env ]; then
  set -a; . ./.env; set +a
fi

BIN=${KLUBOK_BIN:-backend/bin/klubok}
LOG_DIR=${KLUBOK_LOG_DIR:-data/logs}
mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/stay-$(date +%F).log"

status=0
"$BIN" stay-update >>"$LOG" 2>&1 || status=$?

find "$LOG_DIR" -name 'stay-*.log' -mtime +400 -delete
exit "$status"
