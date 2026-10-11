-- +goose Up

-- История котировок: из неё считаются базовые цены и горячие предложения (этап 2).
CREATE TABLE price_quotes (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source            text NOT NULL,
    origin_iata       text NOT NULL,      -- аэропорт вылета
    destination_iata  text NOT NULL,      -- аэропорт прилёта
    depart_at         timestamptz NOT NULL,
    return_at         timestamptz,        -- NULL — в одну сторону
    price_kop         bigint NOT NULL,    -- за одного взрослого
    transfers         smallint NOT NULL,
    return_transfers  smallint,
    airline           text NOT NULL,
    found_on          date NOT NULL,      -- когда источник нашёл цену
    fetched_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, origin_iata, destination_iata, depart_at, return_at, price_kop, airline, found_on)
);

CREATE INDEX price_quotes_route_idx ON price_quotes (origin_iata, destination_iata, depart_at);

-- +goose Down
DROP TABLE price_quotes;
