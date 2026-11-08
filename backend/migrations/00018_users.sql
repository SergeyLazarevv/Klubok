-- +goose Up

-- Пользователи (ADR-0026): вход через Яндекс ID и VK ID, своих паролей нет.
CREATE TABLE users (
    id             uuid PRIMARY KEY,
    name           text NOT NULL,
    avatar_url     text,
    email          text,                      -- из профиля провайдера, если он его отдал; не логин
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_login_at  timestamptz NOT NULL DEFAULT now()
);

-- Способы входа: аккаунт у провайдера → пользователь. Один пользователь может войти и через Яндекс, и через VK
-- (привязка второго способа — позже, из личного кабинета). По email аккаунты не склеиваем: так чужой ящик
-- у другого провайдера не откроет доступ к маршрутам.
CREATE TABLE user_identities (
    provider    text NOT NULL CHECK (provider IN ('yandex', 'vk')),
    subject     text NOT NULL,                -- id пользователя у провайдера
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX user_identities_user_idx ON user_identities (user_id);

-- Сессия — это cookie klubok_sid. Пока user_id пуст, сессия анонимная. При входе выдаётся новая сессия,
-- а старая анонимная получает user_id и закрывается: её маршруты остаются за пользователем.
-- Закрытая сессия (ended_at) по cookie больше не открывается, но её данные принадлежат пользователю.
ALTER TABLE sessions ADD COLUMN ended_at timestamptz;
ALTER TABLE sessions ADD CONSTRAINT sessions_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL;

-- Владелец избранного — сессия, а через неё пользователь; отдельный user_id у записи не нужен.
DROP INDEX favorites_user_idx;
ALTER TABLE favorites DROP COLUMN user_id;

-- +goose StatementBegin
-- Сессии того же владельца: сама сессия и, если она вошла, все сессии её пользователя (другие устройства,
-- анонимные сессии до входа). По ним проверяется доступ к избранному и путевому листу.
CREATE FUNCTION owner_sessions(sid uuid) RETURNS SETOF uuid
LANGUAGE sql STABLE AS $$
    SELECT sid
    UNION
    SELECT o.id FROM sessions s JOIN sessions o ON o.user_id = s.user_id
    WHERE s.id = sid AND s.user_id IS NOT NULL
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION owner_sessions(uuid);
ALTER TABLE favorites ADD COLUMN user_id uuid;
CREATE INDEX favorites_user_idx ON favorites (user_id) WHERE user_id IS NOT NULL AND deleted_at IS NULL;
ALTER TABLE sessions DROP CONSTRAINT sessions_user_fk;
ALTER TABLE sessions DROP COLUMN ended_at;
DROP TABLE user_identities;
DROP TABLE users;
