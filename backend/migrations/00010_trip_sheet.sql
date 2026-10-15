-- +goose Up

-- Путевой лист (ADR-0019): пункты, которые пользователь добавил к сохранённой поездке, — выбранное жильё,
-- аренда машины, «заехать на озеро», дела перед поездкой. Привязаны к этапу поездки, а не ко времени:
-- у поездки на машине точных дат нет.
CREATE TABLE trip_items (
    id           uuid PRIMARY KEY,
    favorite_id  uuid NOT NULL REFERENCES favorites (id) ON DELETE CASCADE,
    stage        text NOT NULL CHECK (stage IN ('prep', 'road', 'flight', 'stay', 'place', 'return', 'home')),
    kind         text NOT NULL CHECK (kind IN ('note', 'link', 'place')),
    title        text NOT NULL,
    url          text,                          -- объявление, отель, аренда — страница на чужом сайте
    price        bigint,                        -- в копейках, вводит пользователь; идёт в общий бюджет
    done         boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX trip_items_favorite_idx ON trip_items (favorite_id, created_at);

-- +goose Down
DROP TABLE trip_items;
