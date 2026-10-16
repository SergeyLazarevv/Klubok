# 8. API

REST, JSON, префикс `/api/v1`. Контракт — `api/openapi.yaml` (spec-first: сначала спецификация, потом код; сервер и клиент генерируются). Этот документ — обзор; источник истины — спецификация.

## 8.1. Общие правила

- Аутентификация: `Authorization: Bearer <access JWT>` (15 мин). Refresh-токен: в веб — `HttpOnly; Secure; SameSite=Strict` cookie; в мобильном приложении — в теле ответа.
- Ошибки — формат RFC 9457 (`application/problem+json`):
  ```json
  { "type": "about:blank", "title": "Validation failed", "status": 422,
    "errors": [{ "field": "travelers", "message": "must be between 1 and 9" }] }
  ```
- Деньги — целые копейки и валюта: `{ "amount": 2687400, "currency": "RUB" }`.
- Даты — `YYYY-MM-DD`, время — ISO 8601 в UTC.
- Пагинация — курсорная: `?cursor=...&limit=...`.

## 8.2. Аутентификация — MVP

| Метод | Путь | Описание |
|---|---|---|
| POST | `/auth/register` | Регистрация по email и паролю |
| POST | `/auth/verify-email` | Подтверждение email по токену из письма |
| POST | `/auth/login` | Вход → access + refresh |
| POST | `/auth/refresh` | Обновление пары токенов (ротация refresh) |
| POST | `/auth/logout` | Выход (отзыв текущего refresh) |
| POST | `/auth/logout-all` | Выход со всех устройств |
| POST | `/auth/password/forgot` | Письмо для сброса пароля |
| POST | `/auth/password/reset` | Новый пароль по токену |
| GET | `/auth/vk/start` | Редирект на VK ID |
| GET | `/auth/vk/callback` | Возврат от VK ID |

## 8.3. Пользователь и профиль — MVP

| Метод | Путь | Описание |
|---|---|---|
| GET | `/me` | Текущий пользователь |
| DELETE | `/me` | Удаление аккаунта и данных |
| GET | `/me/profile` | Настройки |
| PUT | `/me/profile` | Обновить настройки |

## 8.4. Справочник — MVP

| Метод | Путь | Описание |
|---|---|---|
| GET | `/places?q=сам&kind=city` | Автодополнение |
| GET | `/places/{id}` | Точка |

## 8.4a. Машина — MVP

`GET /car-hubs?from={city}&radius_km&max_drive_hours&days&travelers&consumption_l100&fuel_type&fuel_price_rub&allow_border`

Аэропорты, до которых пользователь готов доехать, с дорогой (км, время, с учётом КПП) и расходами на машину туда и обратно: топливо, парковка, страховка, итого и на человека. Параметры профиля передаются явно — эндпоинт работает и для гостя.

## 8.5. Поиск — MVP

**Реализовано:** `GET /trips?from&to&date_from=YYYY-MM-DD&date_to=YYYY-MM-DD&min_days&max_days&weekends&travelers&use_car&<параметры машины>` (начало диапазона — не дальше 3 месяцев вперёд, диапазон — до 60 дней, вылет и возврат внутри него; с машиной в ответе ещё `drive` — поездка целиком на машине, `drive_skipped`, `hidden_flights`, см. 6.1a) — без сохранения поиска; ссылку на результат даёт URL фронтенда (`/?from&to&date_from&date_to&min&max`). Ответ: лучший вариант через каждый аэропорт (билеты на всех + машина, аэропорт прилёта `arrival`), `home_best`, `savings_vs_home`, `no_data`, `partial`, предупреждения. Контракт — `api/openapi.yaml`.

**Реализовано:** `GET /explore?from&date_from&date_to&min_days&max_days&weekends&travelers&use_car&<параметры машины>` — «Куда угодно» ([ADR-0020](adr/0020-explore-anywhere.md)): по каждому городу, куда у источника есть цены, лучший вариант по полной стоимости (`best`, с вылетом из соседних аэропортов при `use_car`) и лучший вылет из своего города (`home_best`); `weekends` — вылет чт–сб, возвращение вс–пн. Ограничения дат — как у `/trips`. Поездки целиком на машине здесь не считаются — их оценивает фронтенд.

**Реализовано:** `GET /routes/drive?from_lat&from_lon&to_lat&to_lon` — линия маршрута на машине между точками по координатам (города карты «Куда угодно» не из справочника), кэш Redis на 30 дней; `GET /stay-prices` — цена жилья по городам карты ([ADR-0021](adr/0021-stay-prices.md)); `GET /weather?lat&lon&date_from&date_to` — погода по дням: ближайшие 16 дней — прогноз, дальше — «обычно в эти дни» по 5 прошлым годам ([ADR-0022](adr/0022-weather.md)).

**Реализовано:** `GET /routes/road?from&to` — дорога между двумя точками справочника для мини-карты: `road` (км, минуты, `approximate`) и `polyline` (Encoded Polyline, точность 6).

**Реализовано:** `GET /routes/road/pois?from&to` — заправки и кафе в 2,5 км от дороги, достопримечательности в 5 км; `GET /places/{id}/pois` — достопримечательности в 15 км от точки. Источник — OpenStreetMap, первый запрос по области — несколько секунд; `502`, если источник не ответил. `GET /places/{id}/guide` — путеводитель Викигида по городу: обзор и места (тип, описание, координаты, фото), с источником и лицензией. `GET /pois/{type}/{id}` — место с описанием и фото из Википедии / Wikimedia Commons (загружаются при первом запросе и сохраняются; признак `details_pending` в списках).

Ниже — исходный набросок с сохранением поиска (`POST /searches`), понадобится для истории поисков в личном кабинете.

`POST /searches`

```json
{
  "destination_place_id": "…",
  "depart_date": "2026-11-14",
  "return_date": "2026-11-21",
  "travelers": 2,
  "use_car": true
}
```

Ответ `200`:

```json
{
  "id": "…",
  "options": [
    {
      "hub": { "iata": "URA", "name": "Уральск" },
      "total": { "amount": 2687400, "currency": "RUB" },
      "per_person": { "amount": 1343700, "currency": "RUB" },
      "breakdown": {
        "fuel": 327400, "parking": 210000, "border": 150000,
        "tolls": 0, "tickets": 2000000, "baggage": 0
      },
      "legs": [
        { "mode": "car", "from": "Самара", "to": "Уральск", "distance_km": 330, "duration_min": 240 },
        { "mode": "flight", "from": "URA", "to": "ALA", "depart_at": "…", "transfers": 0, "airline": "…" },
        { "mode": "flight", "from": "ALA", "to": "URA", "depart_at": "…", "transfers": 0, "airline": "…" },
        { "mode": "car", "from": "Уральск", "to": "Самара", "distance_km": 330, "duration_min": 240 }
      ],
      "savings_vs_home": { "amount": 5312600, "currency": "RUB" },
      "warnings": ["border_crossing_by_car"],
      "price_found_at": "2026-10-11T08:00:00Z",
      "booking_url": "https://…?marker=…"
    }
  ],
  "partial": false
}
```

`partial: true` — часть источников не ответила за таймаут.

| Метод | Путь | Описание |
|---|---|---|
| GET | `/searches/{id}` | Повторно открыть результат (хранится 24 ч) |
| GET | `/me/searches` | История поисков пользователя |

## 8.5a. Избранное и путевой лист — MVP

Владелец — анонимная сессия (cookie `klubok_sid`), см. [ADR-0018](adr/0018-favorites.md) и [ADR-0019](adr/0019-trip-sheet.md).

| Метод | Путь | Описание |
|---|---|---|
| GET, POST | `/favorites` | Список и сохранение поездки (тот же вариант — та же запись; удалённая возвращается вместе с путевым листом) |
| DELETE | `/favorites/{id}` | Убрать из избранного (мягко) |
| GET | `/favorites/{id}/current` | Актуальная цена сохранённого варианта |
| GET, POST | `/favorites/{id}/items` | Путевой лист: пункты по этапам поездки, добавление пункта |
| GET | `/routes/road?from&to&via=lat,lon…` | Дорога для карты; `via` — места из путевого листа, через которые её проложить (до 10) |
| GET | `/link-preview?url=` | Превью ссылки: название, картинка, цена за сутки — только сайты из белого списка |
| PATCH, DELETE | `/favorites/{id}/items/{item_id}` | Отметить сделанным, переименовать, поменять цену (`clear_price` — убрать); удалить |

## 8.6. v2+

| Метод | Путь | Описание |
|---|---|---|
| GET | `/deals` | Горячие направления для пользователя |
| GET | `/destinations/{id}/calendar` | Календарь цен |
| CRUD | `/me/favorites` | Избранные направления |
| CRUD | `/me/alerts` | Уведомления о ценах |
