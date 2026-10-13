-- name: GetGuide :one
SELECT id, page_title, page_url, revision_id, overview, license, fetched_at
FROM guides
WHERE place_id = @place_id AND source = @source AND lang = @lang;

-- name: UpsertGuide :one
INSERT INTO guides (place_id, source, lang, page_title, page_url, revision_id, overview, license, fetched_at)
VALUES (@place_id, @source, @lang, @page_title, @page_url, @revision_id, @overview, @license, now())
ON CONFLICT (place_id, source, lang) DO UPDATE SET
    page_title  = EXCLUDED.page_title,
    page_url    = EXCLUDED.page_url,
    revision_id = EXCLUDED.revision_id,
    overview    = EXCLUDED.overview,
    license     = EXCLUDED.license,
    fetched_at  = now()
RETURNING id;

-- name: TouchGuide :exec
UPDATE guides SET fetched_at = now() WHERE id = @id;

-- name: DeleteGuideListings :exec
DELETE FROM guide_listings WHERE guide_id = @guide_id;

-- name: InsertGuideListings :batchexec
-- Без координат ST_MakePoint(NULL, NULL) даёт NULL — место без точки на карте.
INSERT INTO guide_listings (guide_id, position, type, name, alt, description, address, url, hours, price, location,
                            wikidata, image_file, photo_url, photo_page, photo_credit)
VALUES (@guide_id, @position, @type, @name, @alt, @description, @address, @url, @hours, @price,
        ST_SetSRID(ST_MakePoint(sqlc.narg('lon')::float8, sqlc.narg('lat')::float8), 4326)::geography,
        @wikidata, @image_file, @photo_url, @photo_page, @photo_credit);

-- name: ListGuideListings :many
SELECT position, type, name, alt, description, address, url, hours, price,
       COALESCE(ST_Y(location::geometry), 0)::float8 AS lat, COALESCE(ST_X(location::geometry), 0)::float8 AS lon,
       (location IS NOT NULL)::bool AS has_location,
       wikidata, photo_url, photo_page, photo_credit
FROM guide_listings
WHERE guide_id = @guide_id
ORDER BY position;
