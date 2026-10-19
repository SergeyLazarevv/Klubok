-- +goose Up
-- Города без аэропорта (города карты, cities.geojson) — направления «только на машине»: в iata — id Wikidata
-- (Q…), чтобы ключ (kind, iata) оставался уникальным. ADR-0023.
ALTER TABLE places DROP CONSTRAINT places_kind_check;
ALTER TABLE places ADD CONSTRAINT places_kind_check CHECK (kind IN ('city', 'airport', 'rail', 'bus', 'town'));

-- +goose Down
DELETE FROM places WHERE kind = 'town';
ALTER TABLE places DROP CONSTRAINT places_kind_check;
ALTER TABLE places ADD CONSTRAINT places_kind_check CHECK (kind IN ('city', 'airport', 'rail', 'bus'));
