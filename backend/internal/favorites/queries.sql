-- Владелец пока — сессия. С авторизацией добавится условие OR user_id = @user_id.

-- name: InsertFavorite :one
-- Тот же вариант уже в избранном — вернуть существующую запись.
WITH ins AS (
    INSERT INTO favorites (id, session_id, kind, title, params, pick, snapshot, fingerprint)
    VALUES (@id, @session_id, @kind, @title, @params, @pick, @snapshot, @fingerprint)
    ON CONFLICT (session_id, fingerprint) WHERE deleted_at IS NULL DO NOTHING
    RETURNING id, title, params, pick, snapshot, created_at, true AS created
)
SELECT id, title, params, pick, snapshot, created_at, created FROM ins
UNION ALL
SELECT id, title, params, pick, snapshot, created_at, false FROM favorites
WHERE session_id = @session_id AND fingerprint = @fingerprint AND deleted_at IS NULL
LIMIT 1;

-- name: ListFavorites :many
SELECT id, title, params, pick, snapshot, created_at
FROM favorites
WHERE session_id = @session_id AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetFavorite :one
SELECT id, title, params, pick, snapshot, created_at
FROM favorites
WHERE id = @id AND session_id = @session_id AND deleted_at IS NULL;

-- name: DeleteFavorite :execrows
UPDATE favorites SET deleted_at = now()
WHERE id = @id AND session_id = @session_id AND deleted_at IS NULL;
