-- +goose Up

-- Подробности о месте. Поля OSM приходят вместе с местом; описание и фото — из Wikidata и Википедии
-- при первом открытии карточки (details_at — когда загружали; NULL — ещё нет).
ALTER TABLE pois
    ADD COLUMN wikidata      text,   -- Q123
    ADD COLUMN commons_file  text,   -- файл на Wikimedia Commons из тегов OSM
    ADD COLUMN cuisine       text,   -- pizza;burger
    ADD COLUMN phone         text,
    ADD COLUMN fee           text,   -- charge, если указан, иначе fee: yes | no
    ADD COLUMN description   text,
    ADD COLUMN photo_url     text,   -- уменьшенная копия на upload.wikimedia.org
    ADD COLUMN photo_page    text,   -- страница файла: автор и лицензия
    ADD COLUMN photo_credit  text,   -- «Автор · CC BY-SA 4.0»
    ADD COLUMN details_at    timestamptz;

-- Загруженные области перечитываем: в них не было новых полей OSM
DELETE FROM poi_scans;

-- +goose Down
ALTER TABLE pois
    DROP COLUMN wikidata, DROP COLUMN commons_file, DROP COLUMN cuisine, DROP COLUMN phone, DROP COLUMN fee,
    DROP COLUMN description, DROP COLUMN photo_url, DROP COLUMN photo_page, DROP COLUMN photo_credit,
    DROP COLUMN details_at;
