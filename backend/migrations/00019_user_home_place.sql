-- +goose Up
ALTER TABLE users
    ADD COLUMN home_place_id uuid REFERENCES places (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE users DROP COLUMN home_place_id;