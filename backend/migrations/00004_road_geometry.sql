-- +goose Up

-- Линия маршрута для карты: Encoded Polyline, точность 6 знаков. NULL — ещё не запрашивали.
ALTER TABLE road_distances ADD COLUMN geometry text;

-- +goose Down
ALTER TABLE road_distances DROP COLUMN geometry;
