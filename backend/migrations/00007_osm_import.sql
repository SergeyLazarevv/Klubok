-- +goose Up

-- Импорт выгрузок OpenStreetMap (ADR-0014): слои карты, источник, хеш для обновления только изменившегося.
ALTER TABLE pois DROP CONSTRAINT pois_category_check;
ALTER TABLE pois
    ADD COLUMN layer       text,             -- слой карты: fuel, food, attraction, nature, parking…
    ADD COLUMN tags        jsonb,            -- нужные слоям теги OSM: виды топлива, места на парковке…
    ADD COLUMN source      text NOT NULL DEFAULT 'overpass' CHECK (source IN ('overpass', 'extract')),
    ADD COLUMN osm_region  text,             -- выгрузка Geofabrik, из которой пришло место
    ADD COLUMN osm_hash    text;             -- хеш значимых полей: не изменился — строку не трогаем

-- category остаётся грубой группой для текущего API: fuel | food | attraction; места новых слоёв — other
ALTER TABLE pois ADD CONSTRAINT pois_category_check CHECK (category IN ('fuel', 'food', 'attraction', 'other'));
UPDATE pois SET layer = category;
ALTER TABLE pois ALTER COLUMN layer SET NOT NULL;

CREATE INDEX pois_region_idx ON pois (osm_region) WHERE source = 'extract';
CREATE INDEX pois_enrich_idx ON pois (details_at NULLS FIRST)
    WHERE wikipedia IS NOT NULL OR wikidata IS NOT NULL OR commons_file IS NOT NULL;

-- Границы выгрузок: внутри них места берём из своей базы, а не из Overpass.
CREATE TABLE osm_regions (
    region       text PRIMARY KEY,           -- asia/kazakhstan
    area         geography(MultiPolygon, 4326) NOT NULL,
    sequence     integer NOT NULL,           -- номер последнего применённого дифа Geofabrik
    imported_at  timestamptz NOT NULL
);
CREATE INDEX osm_regions_area_idx ON osm_regions USING gist (area);

-- Журнал запусков импорта.
CREATE TABLE osm_imports (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    region       text NOT NULL,
    sequence     integer NOT NULL,
    mode         text NOT NULL CHECK (mode IN ('full', 'diffs', 'unchanged')),
    features     integer NOT NULL,           -- мест после фильтра
    created      integer NOT NULL,
    updated      integer NOT NULL,
    deleted      integer NOT NULL,
    duration_ms  integer NOT NULL,
    finished_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE osm_imports;
DROP TABLE osm_regions;
DROP INDEX pois_enrich_idx;
DROP INDEX pois_region_idx;
DELETE FROM pois WHERE category = 'other';
ALTER TABLE pois DROP CONSTRAINT pois_category_check;
ALTER TABLE pois ADD CONSTRAINT pois_category_check CHECK (category IN ('fuel', 'food', 'attraction'));
ALTER TABLE pois DROP COLUMN osm_hash, DROP COLUMN osm_region, DROP COLUMN source, DROP COLUMN tags, DROP COLUMN layer;
