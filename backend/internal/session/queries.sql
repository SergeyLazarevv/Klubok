-- name: CreateSession :exec
INSERT INTO sessions (id, user_id) VALUES (@id, sqlc.narg('user_id'));

-- name: TouchSession :one
-- Открытая сессия — её пользователь (NULL — анонимная); нет строки — сессии нет или она закрыта.
-- Заодно отмечаем визит — не чаще раза в сутки, чтобы не писать в БД на каждый запрос.
WITH touched AS (
    UPDATE sessions t SET last_seen_at = now()
    WHERE t.id = @sid AND t.ended_at IS NULL AND t.last_seen_at < now() - interval '1 day'
    RETURNING t.id
)
SELECT s.user_id FROM sessions s WHERE s.id = @sid AND s.ended_at IS NULL;

-- name: AdoptSession :execrows
-- Вход из анонимной сессии: она переходит пользователю и закрывается — её данные теперь его.
UPDATE sessions SET user_id = @user_id, ended_at = now()
WHERE id = @id AND user_id IS NULL AND ended_at IS NULL;

-- name: EndSession :exec
-- Выход (или вход под другим аккаунтом): cookie больше не действует, данные остаются за пользователем.
UPDATE sessions SET ended_at = now() WHERE id = @id AND ended_at IS NULL;
