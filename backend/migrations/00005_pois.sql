-- +goose Up

-- Места из OpenStreetMap: заправки, кафе, достопримечательности. Пока наполняются по запросу
-- из Overpass API, позже — импортом выгрузки OSM; схема от источника не зависит.
CREATE TABLE pois (
    osm_id         text PRIMARY KEY,         -- node/123, way/456
    category       text NOT NULL CHECK (category IN ('fuel', 'food', 'attraction')),
    kind           text NOT NULL,            -- значение тега: fuel, cafe, museum, castle…
    name           text,
    brand          text,
    location       geography(Point, 4326) NOT NULL,
    opening_hours  text,
    website        text,
    wikipedia      text,                     -- ru:Название
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX pois_location_idx ON pois USING gist (location);

-- Какие области уже загружены из источника и когда.
CREATE TABLE poi_scans (
    scan_key    text PRIMARY KEY,            -- road:<from>:<to> | place:<id>
    scanned_at  timestamptz NOT NULL
);

-- +goose Down
DROP TABLE poi_scans;
DROP TABLE pois;
