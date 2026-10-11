-- +goose Up

-- Расстояние и время по дороге между точками. Кэш ответов маршрутизатора: почти не меняется.
CREATE TABLE road_distances (
    from_place_id  uuid NOT NULL REFERENCES places (id) ON DELETE CASCADE,
    to_place_id    uuid NOT NULL REFERENCES places (id) ON DELETE CASCADE,
    distance_m     integer NOT NULL,
    duration_s     integer NOT NULL,
    source         text NOT NULL,           -- osrm | estimate
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (from_place_id, to_place_id)
);

-- Цены на топливо. region_code пустой — значение по стране.
CREATE TABLE fuel_prices (
    country_code  char(2) NOT NULL,
    region_code   text NOT NULL DEFAULT '',
    fuel_type     text NOT NULL CHECK (fuel_type IN ('ai92', 'ai95', 'dt', 'lpg')),
    price_kop     bigint NOT NULL CHECK (price_kop > 0),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (country_code, region_code, fuel_type)
);

-- Стоимость парковки у хаба в сутки.
CREATE TABLE hub_costs (
    place_id             uuid PRIMARY KEY REFERENCES places (id) ON DELETE CASCADE,
    parking_per_day_kop  bigint NOT NULL CHECK (parking_per_day_kop >= 0),
    source               text NOT NULL,
    updated_at           timestamptz NOT NULL DEFAULT now()
);

-- Правила въезда на машине: из страны дома в страну хаба.
CREATE TABLE border_rules (
    from_country    char(2) NOT NULL,
    to_country      char(2) NOT NULL,
    insurance_kop   bigint NOT NULL DEFAULT 0,  -- страховка авто на поездку
    crossing_hours  numeric(4, 1) NOT NULL DEFAULT 0, -- ориентировочное время на КПП
    note            text NOT NULL DEFAULT '',
    PRIMARY KEY (from_country, to_country)
);

-- Ориентировочные значения на 2026-10 для старта. Уточняются вручную; пользователь может задать свою цену топлива.
INSERT INTO fuel_prices (country_code, fuel_type, price_kop) VALUES
    ('RU', 'ai92', 5900),
    ('RU', 'ai95', 6500),
    ('RU', 'dt',   7300),
    ('RU', 'lpg',  3200);

INSERT INTO border_rules (from_country, to_country, insurance_kop, crossing_hours, note) VALUES
    ('RU', 'KZ', 150000, 1.5, 'Нужна казахстанская страховка авто (ОСАГО), её оформляют у КПП или онлайн. Время на границе непредсказуемо.');

-- +goose Down
DROP TABLE border_rules;
DROP TABLE hub_costs;
DROP TABLE fuel_prices;
DROP TABLE road_distances;
