-- +goose Up

-- Ориентировочная цена жилья по городам карты (ADR-0021): средняя цена суток в гостинице с человека.
-- Пишет команда stay-update (раз в месяц), читает GET /stay-prices через кэш Redis.
CREATE TABLE stay_prices (
    city_id     text PRIMARY KEY,      -- Wikidata ID города карты (cities.geojson)
    city_name   text NOT NULL,
    price_kop   bigint NOT NULL,       -- сутки с человека
    category    text NOT NULL,         -- hotel3 — гостиница 3*
    source      text NOT NULL,         -- rosstat
    basis       text NOT NULL,         -- чья цена: город наблюдения («Сызрань») или «Россия»
    period      date NOT NULL,         -- месяц наблюдения
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE stay_prices;
