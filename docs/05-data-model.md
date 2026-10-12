# 5. Модель данных

СУБД: PostgreSQL + PostGIS. Идентификаторы — UUID v7 (сортируемые по времени). Деньги — целые копейки (`bigint`), валюта — RUB. Время — `timestamptz` в UTC.

## 5.1. Схема (MVP)

```mermaid
erDiagram
    users ||--o{ user_identities : has
    users ||--o{ refresh_tokens : has
    users ||--|| profiles : has
    profiles }o--|| places : "home"
    places ||--o{ road_distances : from
    users ||--o{ searches : runs

    users {
        uuid id PK
        text email UK "nullable при входе через VK"
        text password_hash "nullable"
        timestamptz email_verified_at
        timestamptz created_at
        timestamptz deleted_at
    }
    user_identities {
        uuid id PK
        uuid user_id FK
        text provider "vk"
        text provider_user_id
    }
    refresh_tokens {
        uuid id PK
        uuid user_id FK
        bytea token_hash
        text user_agent
        timestamptz expires_at
        timestamptz revoked_at
    }
    profiles {
        uuid user_id PK
        uuid home_place_id FK
        smallint travelers
        boolean has_car
        text fuel_type
        numeric consumption_l100
        bigint fuel_price_override_kop
        int search_radius_km
        numeric max_drive_hours
        boolean has_foreign_passport
        boolean allow_border_by_car
        text baggage
    }
    places {
        uuid id PK
        text kind "city|airport|rail|bus"
        text iata
        text city_iata
        text name_ru
        text name_en
        char country_code
        text region_code
        geography location
        text timezone
        boolean is_active
    }
    road_distances {
        uuid from_place_id FK
        uuid to_place_id FK
        int distance_m
        int duration_s
        text source "osrm"
        text geometry "nullable, polyline6 для карты"
        timestamptz updated_at
    }
    pois {
        text osm_id PK "node/123"
        text category "fuel | food | attraction"
        text kind
        text name
        geography location
        text wikidata
        text description "из Википедии, при первом открытии"
        text photo_url "Commons, 640 px"
        timestamptz details_at
    }
    poi_scans {
        text scan_key PK "road:from:to | place:id"
        timestamptz scanned_at
    }
    searches {
        uuid id PK
        uuid user_id FK "nullable"
        jsonb params
        jsonb result_summary
        timestamptz created_at
    }
```

## 5.2. Справочные таблицы

**`fuel_prices`** — цены на топливо.

| Поле | Тип | Примечание |
|---|---|---|
| country_code | char(2) | Страна |
| region_code | text | Код региона (ISO 3166-2, например `RU-SAM`); пустая строка — цена по стране |
| fuel_type | text | `ai92`, `ai95`, `dt`, `lpg` |
| price_kop | bigint | |
| updated_at | timestamptz | |

**`hub_costs`** — расходы на хабе.

| Поле | Тип | Примечание |
|---|---|---|
| place_id | uuid | Аэропорт / вокзал |
| parking_per_day_kop | bigint | Стоимость парковки в сутки |
| source | text | Ручной ввод / источник |
| updated_at | timestamptz | |

**`border_rules`** — правила пересечения границы на машине: `from_country`, `to_country`, страховка (`insurance_kop` за поездку), `crossing_hours` (ориентировочное время на КПП), `note` (текст предупреждения).

## 5.3. Цены (v2)

**`price_quotes`** — история котировок для горячих предложений.

| Поле | Тип |
|---|---|
| origin_iata, destination_iata | text |
| depart_date, return_date | date |
| price_kop | bigint |
| transfers | smallint |
| airline | text |
| source | text |
| found_at | timestamptz (момент, когда источник нашёл цену) |
| fetched_at | timestamptz (момент, когда мы её получили) |

Индекс: `(origin_iata, destination_iata, fetched_at)`. Хранение — 12 месяцев, далее агрегаты по неделям.

## 5.4. Кэш (Redis)

| Ключ | TTL | Содержимое |
|---|---|---|
| `fare:{provider}:{origin}:{dest}:{dep}:{ret}` | 1 ч | Ответ источника |
| `rl:{ip}` / `rl:user:{id}` | 1 мин | Ограничение частоты запросов |

Расстояния по дороге кэшируются в PostgreSQL (`road_distances`) бессрочно: они почти не меняются, а набор пар «дом → хаб» ограничен.

## 5.5. Загрузка справочника точек

- Города и аэропорты: открытые данные Travelpayouts (`cities.json`, `airports.json`) — совпадают по кодам с ценами `[сверить]`; координаты при необходимости дополняются из OurAirports.
- Ж/д вокзалы и автостанции (v3): `[открыто]`, зависит от источника цен на поезда/автобусы.
- Регион (`region_code`) нужен для цены топлива; определяется по координатам.
