# Гайд: подключение Travelpayouts

Цель: получить **API-токен** и **marker** (ID партнёра) и вручную проверить, какие цены отдаёт API по нашим тестовым маршрутам.

> Интерфейс кабинета Travelpayouts периодически меняется: названия пунктов меню могут отличаться от описанных. Ориентируйтесь на смысл шага.

## Шаг 1. Регистрация

1. Откройте [travelpayouts.com](https://www.travelpayouts.com/) → «Регистрация».
2. Зарегистрируйтесь по email (подтвердите его по письму).
3. При регистрации попросят описать **проект / источник трафика**:
   - тип: сайт;
   - адрес: `https://github.com/SergeyLazarevv/Klubok` (позже заменим на домен сервиса);
   - тематика: путешествия, планирование поездок;
   - аудитория: Россия.

## Шаг 2. Подключение программы Aviasales

1. В кабинете откройте раздел **«Программы»** (Brands / Programs).
2. Найдите **Aviasales** → «Подключить» / «Join».
3. Подключение к Aviasales обычно одобряется сразу.

> Здесь же можно посмотреть программы для поездов и автобусов — пригодится на этапе 3. Запишите, какие есть.

## Шаг 3. Токен и marker

1. Откройте **профиль** (иконка аккаунта) → раздел **API token** / «Инструменты → API».
2. Скопируйте:
   - **API token** — строка из 32 символов;
   - **marker** (Partner ID) — число, обычно видно рядом с токеном или в профиле.
3. **Не публикуйте токен**: не вставляйте его в код, issues и чаты. Если токен утёк — перевыпустите его в кабинете.

## Шаг 4. Сохраните секреты локально

В корне проекта создайте файл `.env` (он в `.gitignore` и не попадёт в репозиторий):

```env
TRAVELPAYOUTS_TOKEN=ваш_токен
TRAVELPAYOUTS_MARKER=ваш_marker
```

## Шаг 5. Проверка API вручную

Коды: Самара — `KUF`, Уральск — `URA`, Алматы — `ALA` (город `ALA`).

```bash
set -a; source .env; set +a

# 1. Самара ⇄ Алматы, туда-обратно, ноябрь
curl -s --compressed -H "X-Access-Token: $TRAVELPAYOUTS_TOKEN" \
  "https://api.travelpayouts.com/aviasales/v3/prices_for_dates?origin=KUF&destination=ALA&departure_at=2026-11&return_at=2026-11&one_way=false&currency=rub&sorting=price&limit=10" \
  | python3 -m json.tool

# 2. Уральск ⇄ Алматы, то же самое
curl -s --compressed -H "X-Access-Token: $TRAVELPAYOUTS_TOKEN" \
  "https://api.travelpayouts.com/aviasales/v3/prices_for_dates?origin=URA&destination=ALA&departure_at=2026-11&return_at=2026-11&one_way=false&currency=rub&sorting=price&limit=10" \
  | python3 -m json.tool

# 3. Популярные направления из Самары
curl -s --compressed -H "X-Access-Token: $TRAVELPAYOUTS_TOKEN" \
  "https://api.travelpayouts.com/v1/city-directions?origin=KUF&currency=rub" \
  | python3 -m json.tool
```

Если какой-то эндпоинт вернул ошибку `404` или неожиданный формат — сверьтесь с актуальной документацией в разделе API кабинета Travelpayouts.

## Шаг 6. Что записать по итогам проверки

Результаты занесите в [07-integrations.md](../07-integrations.md) (раздел 7.1) или пришлите в чат:

- [ ] Пришли ли цены по каждому из трёх запросов? Сколько вариантов?
- [ ] Какие поля есть в ответе (дата вылета/возврата, пересадки, авиакомпания, `found_at`, ссылка)?
- [ ] Насколько свежие цены (`found_at`)?
- [ ] Совпадает ли цена с тем, что видно на aviasales.ru по той же ссылке?
- [ ] Есть ли данные по точным датам (`departure_at=2026-11-14`), а не только по месяцу?

Эти ответы определят, хватит ли Data API для MVP или нужен дополнительный источник.
