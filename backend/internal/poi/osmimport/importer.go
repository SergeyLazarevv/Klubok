// Package osmimport — импорт мест из выгрузок OpenStreetMap (Geofabrik) в таблицу pois (ADR-0014).
//
// Для каждого региона: скачать недостающие дневные дифы (или выгрузку целиком) → наложить на локальный
// .osm.pbf → отфильтровать нужные теги → загрузить во временную таблицу → слить в pois, трогая только
// изменившиеся строки, и удалить пропавшие. Запуск без новых дифов ничего не меняет.
package osmimport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/poi"
)

const userAgent = "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)"

// DefaultRegions — Россия, Казахстан и популярные направления (ADR-0014, раздел 2).
var DefaultRegions = []string{
	"russia", "asia/kazakhstan", "europe/belarus", "europe/georgia", "asia/armenia", "asia/azerbaijan",
	"asia/uzbekistan", "asia/kyrgyzstan", "europe/turkey", "asia/gcc-states", "africa/egypt",
	"asia/thailand", "asia/vietnam",
}

// Банкоматы и обмен валют нужны за границей; по России их сотни тысяч — не храним.
var homeRegions = map[string]bool{"russia": true}

type Config struct {
	Dir     string // где хранить выгрузки
	Regions []string
	MaxLag  int // отстали больше чем на столько дифов — скачать выгрузку целиком
}

type Options struct {
	Regions []string // пусто — все из конфигурации
	Full    bool     // скачать выгрузки заново, не применяя дифы
}

// Result — итог по региону.
type Result struct {
	Region                    string
	Sequence                  int
	Mode                      string // full | diffs | unchanged
	Features                  int
	Created, Updated, Deleted int
	Took                      time.Duration
}

type Importer struct {
	cfg    Config
	pool   *pgxpool.Pool
	fabrik *Geofabrik
	osmium *Osmium
	log    *slog.Logger
}

func New(cfg Config, pool *pgxpool.Pool, fabrik *Geofabrik, osmium *Osmium, log *slog.Logger) *Importer {
	if cfg.MaxLag <= 0 {
		cfg.MaxLag = 30
	}
	return &Importer{cfg: cfg, pool: pool, fabrik: fabrik, osmium: osmium, log: log}
}

// ErrLocked — импорт уже идёт в другом процессе.
var ErrLocked = errors.New("osm-update уже запущен в другом процессе")

const lockKey = 0x6b6c756f736d // «kluosm»

// Run обновляет регионы по очереди. Ошибка одного региона не останавливает остальные, но возвращается в конце.
func (im *Importer) Run(ctx context.Context, opts Options) ([]Result, error) {
	regions := opts.Regions
	if len(regions) == 0 {
		regions = im.cfg.Regions
	}
	if err := im.osmium.Check(ctx); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(im.cfg.Dir, 0o755); err != nil {
		return nil, err
	}

	conn, err := im.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&locked); err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrLocked
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, lockKey) //nolint:errcheck

	airports, err := loadAirports(ctx, im.pool)
	if err != nil {
		return nil, fmt.Errorf("load airports: %w", err)
	}

	var (
		results []Result
		failed  []string
	)
	for _, region := range regions {
		res, err := im.region(ctx, conn.Conn(), region, airports, opts.Full)
		if err != nil {
			if ctx.Err() != nil {
				return results, ctx.Err()
			}
			im.log.ErrorContext(ctx, "osm region failed", slog.String("region", region), slog.Any("err", err))
			failed = append(failed, region)
			continue
		}
		im.log.InfoContext(ctx, "osm region done", slog.String("region", res.Region), slog.String("mode", res.Mode),
			slog.Int("sequence", res.Sequence), slog.Int("features", res.Features), slog.Int("created", res.Created),
			slog.Int("updated", res.Updated), slog.Int("deleted", res.Deleted), slog.Duration("took", res.Took))
		results = append(results, res)
	}
	if len(failed) > 0 {
		return results, fmt.Errorf("не удалось обновить регионы: %s", strings.Join(failed, ", "))
	}
	return results, nil
}

func (im *Importer) region(ctx context.Context, conn *pgx.Conn, region string, airports airportIndex, full bool) (Result, error) {
	start := time.Now()
	res := Result{Region: region}
	key := strings.ReplaceAll(region, "/", "-")
	base := filepath.Join(im.cfg.Dir, key+".osm.pbf")

	remote, err := im.fabrik.RemoteSequence(ctx, region)
	if err != nil {
		return res, err
	}
	local := -1
	if _, err := os.Stat(base); err == nil && !full {
		if local, err = im.osmium.Sequence(ctx, base); err != nil {
			im.log.WarnContext(ctx, "osm: не прочитан номер дифа, скачиваем заново", slog.String("region", region), slog.Any("err", err))
			local = -1
		}
	}

	switch {
	case local < 0 || remote-local > im.cfg.MaxLag:
		im.log.InfoContext(ctx, "osm: скачиваем выгрузку целиком", slog.String("region", region))
		if err := im.fabrik.DownloadExtract(ctx, region, base); err != nil {
			return res, err
		}
		if local, err = im.osmium.Sequence(ctx, base); err != nil {
			return res, err
		}
		res.Mode = "full"
	case remote > local:
		im.log.InfoContext(ctx, "osm: применяем дифы", slog.String("region", region),
			slog.Int("from", local+1), slog.Int("to", remote))
		if err := im.applyDiffs(ctx, region, key, base, local, remote); err != nil {
			return res, err
		}
		local = remote
		res.Mode = "diffs"
	default:
		res.Mode = "unchanged"
	}
	res.Sequence = local

	// Файл не менялся и уже загружен в базу — делать нечего
	if res.Mode == "unchanged" {
		var loaded int
		err := conn.QueryRow(ctx, `SELECT sequence FROM osm_regions WHERE region = $1`, region).Scan(&loaded)
		if err == nil && loaded == local {
			res.Took = time.Since(start)
			return res, im.logRun(ctx, conn, res)
		}
	}

	filtered := filepath.Join(im.cfg.Dir, key+".filtered.osm.pbf")
	exported := filepath.Join(im.cfg.Dir, key+".geojsonseq")
	defer os.Remove(filtered)
	defer os.Remove(exported)
	if err := im.osmium.TagsFilter(ctx, base, filtered); err != nil {
		return res, err
	}
	if err := im.osmium.Export(ctx, filtered, exported); err != nil {
		return res, err
	}

	if res.Features, err = im.stage(ctx, conn, region, exported, airports); err != nil {
		return res, err
	}
	poly, err := im.fabrik.Poly(ctx, region)
	if err != nil {
		return res, err
	}
	if res.Created, res.Updated, res.Deleted, err = im.merge(ctx, conn, region, local, poly); err != nil {
		return res, err
	}
	res.Took = time.Since(start)
	return res, im.logRun(ctx, conn, res)
}

// applyDiffs скачивает дифы local+1..remote и накладывает их на выгрузку.
func (im *Importer) applyDiffs(ctx context.Context, region, key, base string, local, remote int) error {
	dir := filepath.Join(im.cfg.Dir, key+"-diffs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var diffs []string
	for seq := local + 1; seq <= remote; seq++ {
		f := filepath.Join(dir, fmt.Sprintf("%09d.osc.gz", seq))
		if err := im.fabrik.DownloadDiff(ctx, region, seq, f); err != nil {
			return err
		}
		diffs = append(diffs, f)
	}
	next := strings.TrimSuffix(base, ".osm.pbf") + ".next.osm.pbf" // osmium узнаёт формат по расширению
	if err := im.osmium.ApplyChanges(ctx, base, diffs, next, remote); err != nil {
		os.Remove(next)
		return err
	}
	return os.Rename(next, base)
}

var stagingColumns = []string{
	"osm_id", "layer", "category", "kind", "name", "brand", "lat", "lon", "opening_hours", "website",
	"wikipedia", "wikidata", "commons_file", "cuisine", "phone", "fee", "tags", "osm_hash",
}

// stage загружает отфильтрованные места региона во временную таблицу.
func (im *Importer) stage(ctx context.Context, conn *pgx.Conn, region, exported string, airports airportIndex) (int, error) {
	_, err := conn.Exec(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS osm_staging (
			osm_id text, layer text, category text, kind text, name text, brand text, lat float8, lon float8,
			opening_hours text, website text, wikipedia text, wikidata text, commons_file text, cuisine text,
			phone text, fee text, tags jsonb, osm_hash text
		);
		TRUNCATE osm_staging;`)
	if err != nil {
		return 0, err
	}
	f, err := os.Open(exported)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	keep := func(r row) bool {
		switch {
		case r.Layer == poi.LayerParking:
			return airports.near(r.POI.Lat, r.POI.Lon)
		case homeRegions[region] && (r.POI.Kind == "atm" || r.POI.Kind == "bureau_de_change"):
			return false
		}
		return true
	}

	// Чтение файла и COPY идут потоком: в памяти не держим весь регион
	rows := make(chan row, 1024)
	errc := make(chan error, 1)
	go func() {
		defer close(rows)
		_, err := readFeatures(f, keep, func(r row) error {
			select {
			case rows <- r:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		errc <- err
	}()
	n, err := conn.CopyFrom(ctx, pgx.Identifier{"osm_staging"}, stagingColumns, pgx.CopyFromFunc(func() ([]any, error) {
		r, ok := <-rows
		if !ok {
			return nil, nil
		}
		p := r.POI
		var tags any
		if r.Tags != nil {
			tags = r.Tags
		}
		return []any{p.OSMID, string(r.Layer), string(p.Category), p.Kind, p.Name, p.Brand, p.Lat, p.Lon,
			p.OpeningHours, p.Website, p.Wikipedia, p.Wikidata, p.CommonsFile, p.Cuisine, p.Phone, p.Fee,
			tags, r.Hash}, nil
	}))
	if err != nil {
		for range rows { // дать горутине чтения завершиться
		}
		return 0, fmt.Errorf("copy staging: %w", err)
	}
	if err := <-errc; err != nil {
		return 0, fmt.Errorf("read %s: %w", exported, err)
	}
	return int(n), nil
}

// mergeSQL — новые места вставляются, изменившиеся (по хешу) обновляются, остальные не трогаются.
// Поменялись ссылки на Википедию или фото — описание и фото загрузятся заново (details_at = NULL).
const mergeSQL = `
WITH src AS (
    -- замкнутая линия выгружается и линией, и площадью — берём одну и ту же копию при каждом запуске
    SELECT DISTINCT ON (osm_id) * FROM osm_staging ORDER BY osm_id, osm_hash
), up AS (
    INSERT INTO pois AS p (osm_id, category, layer, kind, name, brand, location, opening_hours, website,
                           wikipedia, wikidata, commons_file, cuisine, phone, fee, tags,
                           source, osm_region, osm_hash, updated_at)
    SELECT osm_id, category, layer, kind, NULLIF(name, ''), NULLIF(brand, ''),
           ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography,
           NULLIF(opening_hours, ''), NULLIF(website, ''), NULLIF(wikipedia, ''), NULLIF(wikidata, ''),
           NULLIF(commons_file, ''), NULLIF(cuisine, ''), NULLIF(phone, ''), NULLIF(fee, ''), tags,
           'extract', $1, osm_hash, now()
    FROM src
    ON CONFLICT (osm_id) DO UPDATE SET
        category      = EXCLUDED.category,
        layer         = EXCLUDED.layer,
        kind          = EXCLUDED.kind,
        name          = EXCLUDED.name,
        brand         = EXCLUDED.brand,
        location      = EXCLUDED.location,
        opening_hours = EXCLUDED.opening_hours,
        website       = EXCLUDED.website,
        wikipedia     = EXCLUDED.wikipedia,
        -- элемент Wikidata мог найти poi-enrich по статье — не затираем его пустым значением
        wikidata      = COALESCE(EXCLUDED.wikidata, p.wikidata),
        commons_file  = EXCLUDED.commons_file,
        cuisine       = EXCLUDED.cuisine,
        phone         = EXCLUDED.phone,
        fee           = EXCLUDED.fee,
        tags          = EXCLUDED.tags,
        details_at    = CASE
            WHEN p.wikipedia IS DISTINCT FROM EXCLUDED.wikipedia
              OR p.commons_file IS DISTINCT FROM EXCLUDED.commons_file
              OR (EXCLUDED.wikidata IS NOT NULL AND EXCLUDED.wikidata IS DISTINCT FROM p.wikidata)
            THEN NULL ELSE p.details_at END,
        source        = 'extract',
        osm_region    = EXCLUDED.osm_region,
        osm_hash      = EXCLUDED.osm_hash,
        updated_at    = now()
    WHERE p.osm_hash IS DISTINCT FROM EXCLUDED.osm_hash OR p.source <> 'extract'
    RETURNING (xmax = 0) AS inserted
)
SELECT count(*) FILTER (WHERE inserted), count(*) FILTER (WHERE NOT inserted) FROM up`

// deleteSQL — места региона, которых больше нет в выгрузке: объект удалён или потерял нужные теги.
const deleteSQL = `
DELETE FROM pois p
WHERE p.source = 'extract' AND p.osm_region = $1
  AND NOT EXISTS (SELECT 1 FROM osm_staging s WHERE s.osm_id = p.osm_id)`

const regionSQL = `
INSERT INTO osm_regions (region, area, sequence, imported_at)
VALUES ($1, ST_GeogFromText($2), $3, now())
ON CONFLICT (region) DO UPDATE SET area = EXCLUDED.area, sequence = EXCLUDED.sequence, imported_at = now()`

func (im *Importer) merge(ctx context.Context, conn *pgx.Conn, region string, seq int, poly string) (created, updated, deleted int, err error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `CREATE INDEX IF NOT EXISTS osm_staging_id_idx ON osm_staging (osm_id); ANALYZE osm_staging`); err != nil {
		return 0, 0, 0, err
	}
	if err := tx.QueryRow(ctx, mergeSQL, region).Scan(&created, &updated); err != nil {
		return 0, 0, 0, fmt.Errorf("merge: %w", err)
	}
	tag, err := tx.Exec(ctx, deleteSQL, region)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("delete vanished: %w", err)
	}
	if _, err := tx.Exec(ctx, regionSQL, region, poly, seq); err != nil {
		return 0, 0, 0, fmt.Errorf("save region: %w", err)
	}
	return created, updated, int(tag.RowsAffected()), tx.Commit(ctx)
}

func (im *Importer) logRun(ctx context.Context, conn *pgx.Conn, r Result) error {
	_, err := conn.Exec(ctx, `
		INSERT INTO osm_imports (region, sequence, mode, features, created, updated, deleted, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		r.Region, r.Sequence, r.Mode, r.Features, r.Created, r.Updated, r.Deleted, r.Took.Milliseconds())
	return err
}
