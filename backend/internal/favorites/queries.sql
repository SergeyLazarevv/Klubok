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

-- name: UpdateFavorite :one
-- Поменялись параметры поездки (остановки по пути) — новый отпечаток и снимок цены.
UPDATE favorites SET params = @params, snapshot = @snapshot, fingerprint = @fingerprint
WHERE id = @id AND session_id = @session_id AND deleted_at IS NULL
RETURNING id, title, params, pick, snapshot, created_at;

-- name: DeleteFavorite :execrows
UPDATE favorites SET deleted_at = now()
WHERE id = @id AND session_id = @session_id AND deleted_at IS NULL;

-- name: RestoreFavorite :one
-- Вариант удаляли, а теперь сохраняют снова — возвращаем ту же запись, чтобы не потерять путевой лист.
UPDATE favorites SET deleted_at = NULL, title = @title, snapshot = @snapshot, created_at = now()
WHERE id = (
    SELECT f.id FROM favorites f
    WHERE f.session_id = @session_id AND f.fingerprint = @fingerprint AND f.deleted_at IS NOT NULL
    ORDER BY f.deleted_at DESC LIMIT 1
)
AND NOT EXISTS (
    SELECT 1 FROM favorites a WHERE a.session_id = @session_id AND a.fingerprint = @fingerprint AND a.deleted_at IS NULL
)
RETURNING id, title, params, pick, snapshot, created_at;

-- Путевой лист. Владелец пункта — владелец сохранённой поездки, поэтому везде проверяем сессию через favorites.

-- name: ListTripItems :many
SELECT i.id, i.stage, i.kind, i.title, i.url, i.price, i.done, i.created_at, i.lat, i.lon, i.image_url, i.description, i.city
FROM trip_items i
JOIN favorites f ON f.id = i.favorite_id
WHERE i.favorite_id = @favorite_id AND f.session_id = @session_id AND f.deleted_at IS NULL
ORDER BY i.created_at;

-- name: InsertTripItem :one
INSERT INTO trip_items (id, favorite_id, stage, kind, title, url, price, lat, lon, image_url, description, city)
SELECT @id, f.id, @stage, @kind, @title, sqlc.narg('url'), sqlc.narg('price'), sqlc.narg('lat'), sqlc.narg('lon'),
       sqlc.narg('image_url'), sqlc.narg('description'), sqlc.narg('city')
FROM favorites f
WHERE f.id = @favorite_id AND f.session_id = @session_id AND f.deleted_at IS NULL
RETURNING id, stage, kind, title, url, price, done, created_at, lat, lon, image_url, description, city;

-- name: UpdateTripItem :one
-- Пустое поле — не менять; цену можно сбросить флагом clear_price.
UPDATE trip_items i SET
    title = COALESCE(sqlc.narg('title'), i.title),
    done  = COALESCE(sqlc.narg('done'), i.done),
    image_url   = COALESCE(sqlc.narg('image_url'), i.image_url),
    description = COALESCE(sqlc.narg('description'), i.description),
    price = CASE WHEN @clear_price::boolean THEN NULL ELSE COALESCE(sqlc.narg('price'), i.price) END
FROM favorites f
WHERE i.id = @id AND i.favorite_id = @favorite_id
  AND f.id = i.favorite_id AND f.session_id = @session_id AND f.deleted_at IS NULL
RETURNING i.id, i.stage, i.kind, i.title, i.url, i.price, i.done, i.created_at, i.lat, i.lon, i.image_url, i.description, i.city;

-- name: DeleteTripItem :execrows
DELETE FROM trip_items i
USING favorites f
WHERE i.id = @id AND i.favorite_id = @favorite_id
  AND f.id = i.favorite_id AND f.session_id = @session_id AND f.deleted_at IS NULL;
