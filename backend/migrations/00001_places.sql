-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE places (
    id            uuid PRIMARY KEY,
    kind          text NOT NULL CHECK (kind IN ('city', 'airport', 'rail', 'bus')),
    iata          text NOT NULL,
    city_iata     text,
    name_ru       text NOT NULL,
    name_en       text,
    name_to       text,
    country_code  char(2) NOT NULL,
    region_code   text,
    location      geography(Point, 4326) NOT NULL,
    timezone      text NOT NULL,
    is_active     boolean NOT NULL DEFAULT true,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, iata)
);

CREATE INDEX places_location_idx ON places USING gist (location);
CREATE INDEX places_name_ru_idx ON places (lower(name_ru) text_pattern_ops);
CREATE INDEX places_name_en_idx ON places (lower(name_en) text_pattern_ops);
CREATE INDEX places_city_iata_idx ON places (city_iata);

-- +goose Down
DROP TABLE places;
