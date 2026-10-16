-- name: UpsertPlace :exec
INSERT INTO places (id, kind, iata, city_iata, name_ru, name_en, name_to, country_code, location, timezone, is_active, updated_at)
VALUES (
    @id, @kind, @iata, @city_iata, @name_ru, @name_en, @name_to, @country_code,
    ST_SetSRID(ST_MakePoint(@lon::float8, @lat::float8), 4326)::geography,
    @timezone, @is_active, now()
)
ON CONFLICT (kind, iata) DO UPDATE SET
    city_iata    = EXCLUDED.city_iata,
    name_ru      = EXCLUDED.name_ru,
    name_en      = EXCLUDED.name_en,
    name_to      = EXCLUDED.name_to,
    country_code = EXCLUDED.country_code,
    location     = EXCLUDED.location,
    timezone     = EXCLUDED.timezone,
    is_active    = EXCLUDED.is_active,
    updated_at   = now();

-- name: GetPlace :one
SELECT p.id, p.kind, p.iata, p.city_iata, p.name_ru, p.name_en, p.name_to, p.country_code,
       ST_Y(p.location::geometry)::float8 AS lat, ST_X(p.location::geometry)::float8 AS lon, p.timezone,
       c.name_ru AS city_name
FROM places p
LEFT JOIN places c ON c.kind = 'city' AND c.iata = p.city_iata
WHERE p.id = @id AND p.is_active;

-- name: SearchPlaces :many
-- Порядок: точный IATA, точное название, Россия, СНГ, города с аэропортом, короткие названия.
SELECT p.id, p.kind, p.iata, p.city_iata, p.name_ru, p.name_en, p.name_to, p.country_code,
       ST_Y(p.location::geometry)::float8 AS lat, ST_X(p.location::geometry)::float8 AS lon, p.timezone,
       c.name_ru AS city_name
FROM places p
LEFT JOIN places c ON c.kind = 'city' AND c.iata = p.city_iata
WHERE p.is_active
  AND (sqlc.narg('kind')::text IS NULL OR p.kind = sqlc.narg('kind')::text)
  AND (
        p.iata = upper(@query::text)
     OR lower(p.name_ru) LIKE lower(@query::text) || '%'
     OR lower(p.name_en) LIKE lower(@query::text) || '%'
  )
ORDER BY (p.iata = upper(@query::text)) DESC,
         (lower(p.name_ru) = lower(@query::text) OR lower(p.name_en) = lower(@query::text)) DESC,
         CASE
             WHEN p.country_code = 'RU' THEN 0
             WHEN p.country_code IN ('KZ', 'BY', 'UZ', 'KG', 'TJ', 'AM', 'AZ', 'GE', 'MD', 'TM') THEN 1
             ELSE 2
         END,
         (p.kind = 'airport' OR EXISTS (
             SELECT 1 FROM places a WHERE a.kind = 'airport' AND a.is_active AND a.city_iata = p.iata
         )) DESC,
         (p.kind = 'city') DESC,
         length(p.name_ru),
         p.name_ru
LIMIT @lim;

-- name: ListAirportsWithin :many
-- Аэропорты в радиусе от точки, по возрастанию расстояния по прямой.
SELECT p.id, p.kind, p.iata, p.city_iata, p.name_ru, p.name_en, p.name_to, p.country_code,
       ST_Y(p.location::geometry)::float8 AS lat, ST_X(p.location::geometry)::float8 AS lon, p.timezone,
       c.name_ru AS city_name,
       (ST_Distance(p.location, o.location) / 1000)::float8 AS distance_km
FROM places p
JOIN places o ON o.id = @origin_id
LEFT JOIN places c ON c.kind = 'city' AND c.iata = p.city_iata
WHERE true
  AND p.kind = 'airport'
  AND p.is_active
  AND ST_DWithin(p.location, o.location, @radius_m::float8)
ORDER BY distance_km;

-- name: GetCityByIATA :one
SELECT p.id, p.kind, p.iata, p.city_iata, p.name_ru, p.name_en, p.name_to, p.country_code,
       ST_Y(p.location::geometry)::float8 AS lat, ST_X(p.location::geometry)::float8 AS lon, p.timezone,
       c.name_ru AS city_name
FROM places p
LEFT JOIN places c ON c.id = p.id -- та же точка: так city_name того же типа, что в остальных запросах
WHERE p.kind = 'city' AND p.iata = @iata AND p.is_active;

-- name: ListAirportsByIATA :many
SELECT p.id, p.kind, p.iata, p.city_iata, p.name_ru, p.name_en, p.name_to, p.country_code,
       ST_Y(p.location::geometry)::float8 AS lat, ST_X(p.location::geometry)::float8 AS lon, p.timezone,
       c.name_ru AS city_name
FROM places p
LEFT JOIN places c ON c.kind = 'city' AND c.iata = p.city_iata
WHERE p.kind = 'airport' AND p.iata = ANY(@codes::text[]) AND p.is_active;

-- name: ListCitiesByIATA :many
SELECT p.id, p.kind, p.iata, p.city_iata, p.name_ru, p.name_en, p.name_to, p.country_code,
       ST_Y(p.location::geometry)::float8 AS lat, ST_X(p.location::geometry)::float8 AS lon, p.timezone,
       NULL::text AS city_name
FROM places p
WHERE p.kind = 'city' AND p.iata = ANY(@codes::text[]) AND p.is_active;

-- name: ListCitiesWithAirports :many
-- Города страны, из которых летают: есть хотя бы один действующий аэропорт.
SELECT DISTINCT c.iata FROM places c
JOIN places a ON a.kind = 'airport' AND a.city_iata = c.iata AND a.is_active
WHERE c.kind = 'city' AND c.is_active AND c.country_code = @country_code
ORDER BY c.iata;
