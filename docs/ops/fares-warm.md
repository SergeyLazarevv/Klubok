# Ежедневный прогрев цен «Куда угодно»

Решение — [ADR-0020](../adr/0020-explore-anywhere.md).

Экран «Куда угодно» показывает цены во все города сразу. Чтобы не ждать источник, цены «во все города»
по каждому городу вылета и месяцу хранятся снимком в таблице `fare_snapshots` и обновляются раз в сутки.

## Запуск

```bash
make build
backend/bin/klubok fares-warm                       # все города России с аэропортом, 4 месяца
backend/bin/klubok fares-warm --origins KUF,ULV     # только эти города вылета
backend/bin/klubok fares-warm --months 2 --pause 1s
```

Нужен `TRAVELPAYOUTS_TOKEN`. Полный прогон — около 1000 запросов, ~20 минут. Котировки заодно пишутся
в историю `price_quotes` (для будущей «дешевле обычного»).

## Крон

```
15 4 * * *  /opt/klubok/deploy/cron/fares-daily.sh
```

Лог — `data/logs/fares-ГГГГ-ММ-ДД.log`, хранится 90 дней. Код возврата не нулевой, если не удался
ни один запрос (токен, сеть). Если прогрев не прошёл, `/explore` сам запросит источник для снимков
старше 36 часов — медленнее, но работает.

# Цены на жильё по городам карты

Решение — [ADR-0021](../adr/0021-stay-prices.md).

```bash
backend/bin/klubok stay-update     # Росстат → stay_prices, затем сброс кэша Redis
```

Нужны доступ к fedstat.ru, Redis (`KLUBOK_REDIS_URL`) и справочник городов карты (`KLUBOK_CITIES_FILE`,
по умолчанию `frontend/public/data/cities.geojson`). Около 5 секунд. Если ЕМИСС не ответил или вернул пусто,
старые данные не трогаются.

```
10 5 20 * *  /opt/klubok/deploy/cron/stay-monthly.sh
```
