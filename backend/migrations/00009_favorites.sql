-- +goose Up

-- Анонимные сессии: до авторизации данные пользователя привязаны к сессии (cookie klubok_sid).
-- После входа user_id проставится сессии и её данным — ничего не потеряется.
CREATE TABLE sessions (
    id            uuid PRIMARY KEY,
    user_id       uuid,                        -- появится с авторизацией (FK на users — вместе с таблицей)
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id) WHERE user_id IS NOT NULL;

-- Избранное: каркас поиска (params), выбранный вариант (pick) и цена на момент сохранения (snapshot).
-- Цена при открытии пересчитывается заново, снимок нужен для «было → стало».
CREATE TABLE favorites (
    id           uuid PRIMARY KEY,
    session_id   uuid NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    user_id      uuid,                         -- проставляется после авторизации
    kind         text NOT NULL CHECK (kind IN ('trip')),
    title        text NOT NULL,
    params       jsonb NOT NULL,               -- параметры поиска в формате запроса GET /trips
    pick         jsonb NOT NULL,               -- какой вариант выбран: аэропорт и даты или «на машине»
    snapshot     jsonb NOT NULL,               -- цена и детали на момент сохранения
    fingerprint  text NOT NULL,                -- от повторного добавления того же варианта
    note         text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);
CREATE UNIQUE INDEX favorites_session_fp_idx ON favorites (session_id, fingerprint) WHERE deleted_at IS NULL;
CREATE INDEX favorites_user_idx ON favorites (user_id) WHERE user_id IS NOT NULL AND deleted_at IS NULL;

-- +goose Down
DROP TABLE favorites;
DROP TABLE sessions;
