-- name: FindIdentity :one
SELECT user_id FROM user_identities WHERE provider = @provider AND subject = @subject;

-- name: CreateUser :exec
INSERT INTO users (id, name, avatar_url, email) VALUES (@id, @name, sqlc.narg('avatar_url'), sqlc.narg('email'));

-- name: CreateIdentity :exec
INSERT INTO user_identities (provider, subject, user_id, email) VALUES (@provider, @subject, @user_id, sqlc.narg('email'));

-- name: TouchUserLogin :exec
-- Повторный вход: фото и почта из профиля провайдера — свежие (если он их отдал), имя не трогаем —
-- его можно будет поменять в личном кабинете.
UPDATE users SET last_login_at = now(),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    email = COALESCE(email, sqlc.narg('email'))
WHERE id = @id;

-- name: GetUser :one
SELECT u.id, u.name, u.avatar_url, u.email,
       COALESCE((SELECT array_agg(i.provider ORDER BY i.created_at) FROM user_identities i WHERE i.user_id = u.id), '{}')::text[] AS providers
FROM users u WHERE u.id = @id;
