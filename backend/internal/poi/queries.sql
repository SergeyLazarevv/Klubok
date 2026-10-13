-- name: GetScan :one
SELECT scanned_at FROM poi_scans WHERE scan_key = @scan_key;

-- name: SaveScan :exec
INSERT INTO poi_scans (scan_key, scanned_at) VALUES (@scan_key, now())
ON CONFLICT (scan_key) DO UPDATE SET scanned_at = now();

-- name: UpsertPOIs :batchexec
-- Места из Overpass. Если поменялись ссылки на Википедию или фото — подробности загрузим заново.
-- Места из выгрузок OSM (source = extract) не трогаем: их обновляет osm-update.
INSERT INTO pois (osm_id, category, layer, kind, name, brand, location, opening_hours, website, wikipedia,
                  wikidata, commons_file, cuisine, phone, fee, updated_at)
VALUES (
    @osm_id, @category, @category, @kind, @name, @brand,
    ST_SetSRID(ST_MakePoint(@lon::float8, @lat::float8), 4326)::geography,
    @opening_hours, @website, @wikipedia, @wikidata, @commons_file, @cuisine, @phone, @fee, now()
)
ON CONFLICT (osm_id) DO UPDATE SET
    category      = EXCLUDED.category,
    kind          = EXCLUDED.kind,
    name          = EXCLUDED.name,
    brand         = EXCLUDED.brand,
    location      = EXCLUDED.location,
    opening_hours = EXCLUDED.opening_hours,
    website       = EXCLUDED.website,
    wikipedia     = EXCLUDED.wikipedia,
    wikidata      = EXCLUDED.wikidata,
    commons_file  = EXCLUDED.commons_file,
    cuisine       = EXCLUDED.cuisine,
    phone         = EXCLUDED.phone,
    fee           = EXCLUDED.fee,
    details_at    = CASE
        WHEN (pois.wikipedia, pois.wikidata, pois.commons_file)
             IS DISTINCT FROM (EXCLUDED.wikipedia, EXCLUDED.wikidata, EXCLUDED.commons_file)
        THEN NULL ELSE pois.details_at END,
    updated_at    = now()
WHERE pois.source = 'overpass';

-- name: ListPOIsAlongLine :many
-- Заправки и кафе — в узком коридоре вдоль дороги, достопримечательности — в широком.
SELECT osm_id, category, kind, name, brand,
       ST_Y(location::geometry)::float8 AS lat, ST_X(location::geometry)::float8 AS lon,
       opening_hours, website, wikipedia, wikidata, commons_file, cuisine, phone, fee,
       description, photo_url, photo_page, photo_credit, details_at
FROM pois
WHERE category IN ('fuel', 'food', 'attraction')
  AND ST_DWithin(location, ST_GeogFromText(@line_wkt::text),
                 CASE WHEN category = 'attraction' THEN @wide_m::float8 ELSE @narrow_m::float8 END)
ORDER BY osm_id;

-- name: ListPOIsAround :many
SELECT osm_id, category, kind, name, brand,
       ST_Y(location::geometry)::float8 AS lat, ST_X(location::geometry)::float8 AS lon,
       opening_hours, website, wikipedia, wikidata, commons_file, cuisine, phone, fee,
       description, photo_url, photo_page, photo_credit, details_at
FROM pois
WHERE category = ANY(@categories::text[])
  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint(@lon::float8, @lat::float8), 4326)::geography, @radius_m::float8)
ORDER BY osm_id;

-- name: GetPOI :one
SELECT osm_id, category, kind, name, brand,
       ST_Y(location::geometry)::float8 AS lat, ST_X(location::geometry)::float8 AS lon,
       opening_hours, website, wikipedia, wikidata, commons_file, cuisine, phone, fee,
       description, photo_url, photo_page, photo_credit, details_at
FROM pois
WHERE osm_id = @osm_id;

-- name: SavePOIDetails :exec
UPDATE pois SET
    wikidata     = COALESCE(NULLIF(@wikidata::text, ''), wikidata),
    description  = NULLIF(@description::text, ''),
    photo_url    = NULLIF(@photo_url::text, ''),
    photo_page   = NULLIF(@photo_page::text, ''),
    photo_credit = NULLIF(@photo_credit::text, ''),
    details_at   = now()
WHERE osm_id = @osm_id;

-- name: BoxesCovered :one
-- Все ли прямоугольники целиком внутри загруженных выгрузок OSM: тогда Overpass не нужен.
SELECT coalesce(bool_and(EXISTS (
    SELECT 1 FROM osm_regions r WHERE ST_Covers(r.area::geometry, ST_GeomFromText(b, 4326))
)), false)::bool
FROM unnest(@boxes_wkt::text[]) AS b;

-- name: ListPOIsToEnrich :many
-- Места со ссылками на Википедию, Wikidata или фото, у которых описания ещё нет или оно старое.
SELECT osm_id, category, kind, name, brand,
       ST_Y(location::geometry)::float8 AS lat, ST_X(location::geometry)::float8 AS lon,
       opening_hours, website, wikipedia, wikidata, commons_file, cuisine, phone, fee,
       description, photo_url, photo_page, photo_credit, details_at
FROM pois
WHERE (wikipedia IS NOT NULL OR wikidata IS NOT NULL OR commons_file IS NOT NULL)
  AND (details_at IS NULL OR details_at < now() - make_interval(days => @max_age_days::int))
-- сначала то, ради чего открывают карточки: достопримечательности и природа
ORDER BY details_at NULLS FIRST, (layer IN ('attraction', 'nature')) DESC, osm_id
LIMIT @lim;
