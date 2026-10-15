-- +goose Up

-- Где находится место из путевого листа: отмечаем на карте, через места «по пути» прокладываем дорогу.
ALTER TABLE trip_items ADD COLUMN lat double precision, ADD COLUMN lon double precision,
    ADD CONSTRAINT trip_items_location_check CHECK ((lat IS NULL) = (lon IS NULL)
        AND (lat IS NULL OR (lat BETWEEN -90 AND 90 AND lon BETWEEN -180 AND 180)));

-- +goose Down
ALTER TABLE trip_items DROP CONSTRAINT trip_items_location_check, DROP COLUMN lat, DROP COLUMN lon;
