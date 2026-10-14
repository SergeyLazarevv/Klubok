-- name: CreateSession :exec
INSERT INTO sessions (id) VALUES (@id);

-- name: TouchSession :one
-- Сессия существует? Заодно отмечаем визит — не чаще раза в сутки, чтобы не писать в БД на каждый запрос.
WITH touched AS (
    UPDATE sessions t SET last_seen_at = now()
    WHERE t.id = @sid AND t.last_seen_at < now() - interval '1 day'
    RETURNING t.id
)
SELECT (EXISTS (SELECT 1 FROM sessions s WHERE s.id = @sid) OR EXISTS (SELECT 1 FROM touched))::bool;
