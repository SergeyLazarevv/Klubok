-- +goose Up

-- Снимок цен «во все города» по городу вылета и месяцу (ADR-0020): обновляется ежедневно командой
-- fares-warm, «Куда угодно» читает его вместо живого запроса к источнику.
CREATE TABLE fare_snapshots (
    source       text NOT NULL,
    origin_iata  text NOT NULL,          -- код города вылета
    month        date NOT NULL,          -- 1-е число месяца вылета
    fetched_at   timestamptz NOT NULL DEFAULT now(),
    quotes       jsonb NOT NULL,         -- []fares.Quote
    PRIMARY KEY (source, origin_iata, month)
);

-- +goose Down
DROP TABLE fare_snapshots;
