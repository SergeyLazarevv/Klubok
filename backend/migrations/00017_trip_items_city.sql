-- +goose Up
-- Пункт путевого листа в городе-остановке по пути (ADR-0023): жильё и дела в этом городе. city — id города
-- карты в Wikidata (как в параметре via); пусто — место назначения или дорога.
ALTER TABLE trip_items ADD COLUMN city text;

-- +goose Down
ALTER TABLE trip_items DROP COLUMN city;
