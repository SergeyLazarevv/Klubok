// Package poi — места на карте из OpenStreetMap: заправки, кафе, достопримечательности.
package poi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/poi/store"
)

type Category string

const (
	CategoryFuel       Category = "fuel"
	CategoryFood       Category = "food"
	CategoryAttraction Category = "attraction"
)

var AllCategories = []Category{CategoryFuel, CategoryFood, CategoryAttraction}

var (
	// ErrSource — источник мест не ответил; в кэше области ещё нет.
	ErrSource = errors.New("poi source unavailable")
	// ErrNotFound — места нет в базе: его не загружали ни с одной областью.
	ErrNotFound = errors.New("poi not found")
)

type Point struct{ Lat, Lon float64 }

// POI — место на карте.
type POI struct {
	OSMID        string // node/123
	Category     Category
	Kind         string // значение тега: fuel, cafe, museum…
	Name         string
	Brand        string
	Lat, Lon     float64
	OpeningHours string
	Website      string
	Wikipedia    string // ru:Название
	Wikidata     string // Q123
	CommonsFile  string // файл на Wikimedia Commons из тегов OSM, без «File:»
	Cuisine      string // pizza;burger
	Phone        string
	Fee          string // charge, если указан, иначе fee: yes | no

	Details
	DetailsLoaded bool // подробности из Wikidata уже загружали (даже если ничего не нашли)
}

// Details — описание и фото места из Wikidata, Википедии и Wikimedia Commons.
type Details struct {
	Description string
	PhotoURL    string // уменьшенная копия
	PhotoPage   string // страница файла с автором и лицензией
	PhotoCredit string // «Автор · CC BY-SA 4.0»
}

// HasWikiRefs — есть ли по чему искать описание и фото.
func (p POI) HasWikiRefs() bool {
	return p.Wikipedia != "" || p.Wikidata != "" || p.CommonsFile != ""
}

// BBox — прямоугольник в градусах.
type BBox struct{ South, West, North, East float64 }

// Source загружает места в прямоугольниках.
type Source interface {
	Fetch(ctx context.Context, boxes []BBox, categories []Category) ([]POI, error)
}

// Enricher находит описание и фото места; второй результат — элемент Wikidata, если его не было.
type Enricher interface {
	Lookup(ctx context.Context, p POI) (Details, string, error)
}

const (
	scanTTL = 30 * 24 * time.Hour // места меняются медленно
	// Коридор вдоль дороги: заправки и кафе — рядом с трассой, достопримечательности — можно и свернуть.
	roadNarrowM = 2500
	roadWideM   = 5000
	// Длина куска маршрута для запроса к источнику: прямоугольник вокруг всего маршрута
	// захватывает целые города, а по кускам — только окрестности дороги.
	chunkKm = 50
)

type Service struct {
	q    *store.Queries
	src  Source
	wiki Enricher
	log  *slog.Logger
}

func NewService(pool *pgxpool.Pool, src Source, wiki Enricher, log *slog.Logger) *Service {
	return &Service{q: store.New(pool), src: src, wiki: wiki, log: log}
}

// detailsTimeout — сколько ждать Википедию; не успели — карточка покажется без описания и фото.
const detailsTimeout = 10 * time.Second

// Get возвращает место с подробностями. Описание и фото загружаются при первом запросе и сохраняются;
// если Википедия не ответила — место возвращается без них и в следующий раз загрузка повторится.
func (s *Service) Get(ctx context.Context, osmID string) (POI, error) {
	r, err := s.q.GetPOI(ctx, osmID)
	if errors.Is(err, pgx.ErrNoRows) {
		return POI{}, ErrNotFound
	}
	if err != nil {
		return POI{}, fmt.Errorf("get poi: %w", err)
	}
	p := fromRow(store.ListPOIsAroundRow(r))
	if p.DetailsLoaded || !p.HasWikiRefs() {
		return p, nil
	}

	lctx, cancel := context.WithTimeout(ctx, detailsTimeout)
	defer cancel()
	details, qid, err := s.wiki.Lookup(lctx, p)
	if err != nil {
		s.log.WarnContext(ctx, "poi details", slog.String("osm_id", osmID), slog.Any("err", err))
		return p, nil
	}
	err = s.q.SavePOIDetails(ctx, store.SavePOIDetailsParams{
		OsmID: osmID, Wikidata: qid, Description: details.Description,
		PhotoUrl: details.PhotoURL, PhotoPage: details.PhotoPage, PhotoCredit: details.PhotoCredit,
	})
	if err != nil {
		return POI{}, fmt.Errorf("save poi details: %w", err)
	}
	p.Details, p.DetailsLoaded = details, true
	if p.Wikidata == "" {
		p.Wikidata = qid
	}
	return p, nil
}

// AlongLine — места в коридоре вдоль линии маршрута. key — ключ области для кэша.
func (s *Service) AlongLine(ctx context.Context, key string, line []Point) ([]POI, error) {
	if len(line) == 0 {
		return []POI{}, nil
	}
	if err := s.ensureScanned(ctx, key, corridorBoxes(line, roadWideM/1000.0), AllCategories); err != nil {
		return nil, err
	}
	rows, err := s.q.ListPOIsAlongLine(ctx, store.ListPOIsAlongLineParams{
		LineWkt: lineWKT(line), NarrowM: roadNarrowM, WideM: roadWideM,
	})
	if err != nil {
		return nil, fmt.Errorf("list pois along line: %w", err)
	}
	out := make([]POI, len(rows))
	for i, r := range rows {
		out[i] = fromRow(store.ListPOIsAroundRow(r))
	}
	return out, nil
}

// Around — места выбранных категорий в радиусе от точки.
func (s *Service) Around(ctx context.Context, key string, center Point, radiusKm float64, categories []Category) ([]POI, error) {
	if err := s.ensureScanned(ctx, key, []BBox{boxAround([]Point{center}, radiusKm)}, categories); err != nil {
		return nil, err
	}
	cats := make([]string, len(categories))
	for i, c := range categories {
		cats[i] = string(c)
	}
	rows, err := s.q.ListPOIsAround(ctx, store.ListPOIsAroundParams{
		Categories: cats, Lat: center.Lat, Lon: center.Lon, RadiusM: radiusKm * 1000,
	})
	if err != nil {
		return nil, fmt.Errorf("list pois around: %w", err)
	}
	out := make([]POI, len(rows))
	for i, r := range rows {
		out[i] = fromRow(r)
	}
	return out, nil
}

// ensureScanned загружает область из источника, если её ещё не загружали или данные устарели.
func (s *Service) ensureScanned(ctx context.Context, key string, boxes []BBox, categories []Category) error {
	scannedAt, err := s.q.GetScan(ctx, key)
	switch {
	case err == nil && time.Since(scannedAt.Time) < scanTTL:
		return nil
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("get poi scan: %w", err)
	}
	// Область внутри загруженных выгрузок OSM — места уже в базе (osm-update), Overpass не нужен
	covered, err := s.q.BoxesCovered(ctx, boxesWKT(boxes))
	if err != nil {
		return fmt.Errorf("check osm coverage: %w", err)
	}
	if covered {
		return nil
	}

	pois, err := s.src.Fetch(ctx, boxes, categories)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSource, err)
	}
	if err := s.save(ctx, pois); err != nil {
		return err
	}
	if err := s.q.SaveScan(ctx, key); err != nil {
		return fmt.Errorf("save poi scan: %w", err)
	}
	s.log.InfoContext(ctx, "pois scanned", slog.String("key", key), slog.Int("boxes", len(boxes)), slog.Int("pois", len(pois)))
	return nil
}

func (s *Service) save(ctx context.Context, pois []POI) error {
	if len(pois) == 0 {
		return nil
	}
	params := make([]store.UpsertPOIsParams, len(pois))
	for i, p := range pois {
		params[i] = store.UpsertPOIsParams{
			OsmID: p.OSMID, Category: string(p.Category), Kind: p.Kind,
			Name: text(p.Name), Brand: text(p.Brand), Lat: p.Lat, Lon: p.Lon,
			OpeningHours: text(p.OpeningHours), Website: text(p.Website), Wikipedia: text(p.Wikipedia),
			Wikidata: text(p.Wikidata), CommonsFile: text(p.CommonsFile), Cuisine: text(p.Cuisine),
			Phone: text(p.Phone), Fee: text(p.Fee),
		}
	}
	var firstErr error
	s.q.UpsertPOIs(ctx, params).Exec(func(_ int, err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	})
	if firstErr != nil {
		return fmt.Errorf("save pois: %w", firstErr)
	}
	return nil
}

func boxesWKT(boxes []BBox) []string {
	out := make([]string, len(boxes))
	for i, b := range boxes {
		out[i] = fmt.Sprintf("POLYGON((%[1]s %[2]s,%[3]s %[2]s,%[3]s %[4]s,%[1]s %[4]s,%[1]s %[2]s))",
			coord(b.West), coord(b.South), coord(b.East), coord(b.North))
	}
	return out
}

// corridorBoxes режет линию на куски примерно по chunkKm и строит вокруг каждого прямоугольник с отступом padKm.
func corridorBoxes(line []Point, padKm float64) []BBox {
	var boxes []BBox
	chunk := []Point{line[0]}
	km := 0.0
	for i := 1; i < len(line); i++ {
		km += haversineKm(line[i-1], line[i])
		chunk = append(chunk, line[i])
		if km >= chunkKm {
			boxes = append(boxes, boxAround(chunk, padKm))
			chunk = []Point{line[i]}
			km = 0
		}
	}
	if len(chunk) > 1 || len(boxes) == 0 {
		boxes = append(boxes, boxAround(chunk, padKm))
	}
	return boxes
}

func boxAround(points []Point, padKm float64) BBox {
	b := BBox{South: 90, West: 180, North: -90, East: -180}
	for _, p := range points {
		b.South, b.North = min(b.South, p.Lat), max(b.North, p.Lat)
		b.West, b.East = min(b.West, p.Lon), max(b.East, p.Lon)
	}
	dLat := padKm / 111.0
	dLon := padKm / (111.0 * math.Max(math.Cos((b.South+b.North)/2*math.Pi/180), 0.1))
	return BBox{South: b.South - dLat, West: b.West - dLon, North: b.North + dLat, East: b.East + dLon}
}

func lineWKT(line []Point) string {
	if len(line) == 1 {
		line = append(line, line[0])
	}
	coords := make([]string, len(line))
	for i, p := range line {
		coords[i] = strconv.FormatFloat(p.Lon, 'f', 6, 64) + " " + strconv.FormatFloat(p.Lat, 'f', 6, 64)
	}
	return "SRID=4326;LINESTRING(" + strings.Join(coords, ",") + ")"
}

func haversineKm(a, b Point) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (b.Lat - a.Lat) * rad
	dLon := (b.Lon - a.Lon) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

func fromRow(r store.ListPOIsAroundRow) POI {
	return POI{
		OSMID: r.OsmID, Category: Category(r.Category), Kind: r.Kind,
		Name: r.Name.String, Brand: r.Brand.String, Lat: r.Lat, Lon: r.Lon,
		OpeningHours: r.OpeningHours.String, Website: r.Website.String, Wikipedia: r.Wikipedia.String,
		Wikidata: r.Wikidata.String, CommonsFile: r.CommonsFile.String, Cuisine: r.Cuisine.String,
		Phone: r.Phone.String, Fee: r.Fee.String,
		Details: Details{
			Description: r.Description.String,
			PhotoURL:    r.PhotoUrl.String, PhotoPage: r.PhotoPage.String, PhotoCredit: r.PhotoCredit.String,
		},
		DetailsLoaded: r.DetailsAt.Valid,
	}
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
