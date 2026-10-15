-- +goose Up

-- Превью ссылки в путевом листе: картинка и короткое описание со страницы объявления (og-теги).
ALTER TABLE trip_items ADD COLUMN image_url text, ADD COLUMN description text;

-- +goose Down
ALTER TABLE trip_items DROP COLUMN image_url, DROP COLUMN description;
