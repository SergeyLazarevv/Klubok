-- +goose Up

-- Отдельный этап «Что посмотреть» (see): список мест, которые хочется увидеть на месте. В place остаются
-- услуги на месте — аренда машины, трансфер. Места, добавленные раньше из лент, переезжают в see.
ALTER TABLE trip_items DROP CONSTRAINT trip_items_stage_check;
ALTER TABLE trip_items ADD CONSTRAINT trip_items_stage_check
    CHECK (stage IN ('prep', 'road', 'flight', 'stay', 'see', 'place', 'return', 'home'));
UPDATE trip_items SET stage = 'see' WHERE stage = 'place' AND kind = 'place';

-- +goose Down
UPDATE trip_items SET stage = 'place' WHERE stage = 'see';
ALTER TABLE trip_items DROP CONSTRAINT trip_items_stage_check;
ALTER TABLE trip_items ADD CONSTRAINT trip_items_stage_check
    CHECK (stage IN ('prep', 'road', 'flight', 'stay', 'place', 'return', 'home'));
