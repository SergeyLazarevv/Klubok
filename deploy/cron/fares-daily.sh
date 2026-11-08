#!/usr/bin/env bash
# Ежедневный прогрев цен «во все города» для «Куда угодно» (ADR-0020): снимки в fare_snapshots,
# котировки — в историю price_quotes. Пример для crontab (каждый день, 04:15):
#   15 4 * * *  /opt/klubok/deploy/cron/fares-daily.sh
set -uo pipefail

cd "$(dirname "$0")/../.."
if [ -f .env ]; then
  set -a; . ./.env; set +a
fi

BIN=${KLUBOK_BIN:-backend/bin/klubok}
LOG_DIR=${KLUBOK_LOG_DIR:-data/logs}
mkdir -p "$LOG_DIR"
LOG="$LOG_DIR/fares-$(date +%F).log"

status=0
"$BIN" fares-warm >>"$LOG" 2>&1 || status=$?

# Старые логи — не дольше 90 дней
find "$LOG_DIR" -name 'fares-*.log' -mtime +90 -delete
exit "$status"
