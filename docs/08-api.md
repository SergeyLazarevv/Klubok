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

## 8.6. v2+

| Метод | Путь | Описание |
|---|---|---|
| GET | `/deals` | Горячие направления для пользователя |
| GET | `/destinations/{id}/calendar` | Календарь цен |
| CRUD | `/me/favorites` | Избранные направления |
| CRUD | `/me/alerts` | Уведомления о ценах |
