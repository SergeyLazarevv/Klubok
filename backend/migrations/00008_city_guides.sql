-- +goose Up

-- Путеводители по городам из внешних источников (сейчас — Викигид). Хранятся отдельно от pois (OSM):
-- у каждого источника своя модель, лицензия и порядок обновления; на странице источник подписывается.
CREATE TABLE guides (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    place_id     uuid NOT NULL REFERENCES places (id) ON DELETE CASCADE,
    source       text NOT NULL CHECK (source IN ('wikivoyage')),
    lang         text NOT NULL,          -- язык раздела: ru
    page_title   text,                   -- NULL — статьи о городе нет (запоминаем, чтобы не искать каждый раз)
    page_url     text,
    revision_id  bigint,                 -- ревизия статьи: та же — статью заново не разбираем
    overview     text,                   -- начало раздела «Понять»
    license      text NOT NULL,          -- CC BY-SA 4.0
    fetched_at   timestamptz NOT NULL,   -- когда проверяли статью
    UNIQUE (place_id, source, lang)
);

-- Места из статьи путеводителя: отобраны авторами путеводителя, в отличие от всех объектов OSM.
CREATE TABLE guide_listings (
    guide_id      bigint NOT NULL REFERENCES guides (id) ON DELETE CASCADE,
    position      int NOT NULL,           -- порядок в статье
    type          text NOT NULL CHECK (type IN ('see', 'do', 'eat', 'drink', 'sleep', 'buy', 'go', 'other')),
    name          text NOT NULL,
    alt           text,
    description   text,
    address       text,
    url           text,
    hours         text,
    price         text,
    location      geography(Point, 4326), -- NULL — координаты в статье не указаны
    wikidata      text,
    image_file    text,                   -- файл на Wikimedia Commons
    photo_url     text,                   -- уменьшенная копия
    photo_page    text,
    photo_credit  text,
    PRIMARY KEY (guide_id, position)
);

-- +goose Down
DROP TABLE guide_listings;
DROP TABLE guides;
