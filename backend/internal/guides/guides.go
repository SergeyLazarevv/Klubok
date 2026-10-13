// Package guides — путеводители по городам из внешних источников (Викигид): обзор города и места,
// отобранные авторами путеводителя. Хранится отдельно от мест OSM (package poi).
package guides

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/guides/store"
)

const (
	sourceWikivoyage  = "wikivoyage"
	langRu            = "ru"
	licenseWikivoyage = "CC BY-SA 4.0"
	// refreshAfter — как часто проверять статью; если ревизия не менялась, статью заново не разбираем.
	refreshAfter = 30 * 24 * time.Hour
)

var (
	// ErrNotFound — статьи о городе в путеводителе нет.
	ErrNotFound = errors.New("guide not found")
	// ErrSource — путеводитель не ответил, а сохранённой статьи нет.
	ErrSource = errors.New("guide source unavailable")
)

// Source — путеводитель.
type Source interface {
	FindPage(ctx context.Context, name string, lat, lon float64) (*Page, error)
	Wikitext(ctx context.Context, title string) (string, error)
	Photos(ctx context.Context, files []string) (map[string]Photo, error)
}

// Guide — путеводитель по городу из одного источника.
type Guide struct {
	Source   string
	Title    string
	URL      string
	Overview string
	License  string
	Listings []GuideListing
}

type GuideListing struct {
	Listing
	Photo *Photo
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	src  Source
	log  *slog.Logger
}

func NewService(pool *pgxpool.Pool, src Source, log *slog.Logger) *Service {
	return &Service{pool: pool, q: store.New(pool), src: src, log: log}
}

// Get возвращает путеводитель по городу. Статья загружается при первом запросе и раз в refreshAfter
// проверяется: изменилась ревизия — разбирается заново, нет — остаётся как есть.
func (s *Service) Get(ctx context.Context, city geo.Place) (Guide, error) {
	key := store.GetGuideParams{PlaceID: city.ID, Source: sourceWikivoyage, Lang: langRu}
	row, err := s.q.GetGuide(ctx, key)
	stored := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Guide{}, fmt.Errorf("get guide: %w", err)
	}
	if stored && time.Since(row.FetchedAt.Time) < refreshAfter {
		return s.load(ctx, row)
	}

	page, err := s.src.FindPage(ctx, city.Name, city.Lat, city.Lon)
	if err != nil {
		if stored { // путеводитель недоступен — отдаём то, что есть
			s.log.WarnContext(ctx, "wikivoyage unavailable, serving stored guide", slog.Any("err", err))
			return s.load(ctx, row)
		}
		return Guide{}, fmt.Errorf("%w: %w", ErrSource, err)
	}
	if page == nil {
		// Статьи нет — запоминаем, чтобы не искать при каждом запросе
		if err := s.replace(ctx, key, nil, Article{}, nil); err != nil {
			return Guide{}, err
		}
		return Guide{}, ErrNotFound
	}
	if stored && row.PageTitle.String == page.Title && row.RevisionID.Int64 == page.Revision {
		if err := s.q.TouchGuide(ctx, row.ID); err != nil {
			return Guide{}, fmt.Errorf("touch guide: %w", err)
		}
		return s.load(ctx, row)
	}

	text, err := s.src.Wikitext(ctx, page.Title)
	if err != nil {
		if stored {
			return s.load(ctx, row)
		}
		return Guide{}, fmt.Errorf("%w: %w", ErrSource, err)
	}
	article := Parse(text)
	var files []string
	for _, l := range article.Listings {
		if l.ImageFile != "" {
			files = append(files, l.ImageFile)
		}
	}
	photos, err := s.src.Photos(ctx, files)
	if err != nil { // без фото путеводитель всё равно полезен
		s.log.WarnContext(ctx, "guide photos", slog.Any("err", err))
	}
	if err := s.replace(ctx, key, page, article, photos); err != nil {
		return Guide{}, err
	}
	s.log.InfoContext(ctx, "guide updated", slog.String("page", page.Title), slog.Int64("revision", page.Revision),
		slog.Int("listings", len(article.Listings)), slog.Int("photos", len(photos)))
	row, err = s.q.GetGuide(ctx, key)
	if err != nil {
		return Guide{}, fmt.Errorf("get guide: %w", err)
	}
	return s.load(ctx, row)
}

// replace сохраняет статью целиком в одной транзакции: путеводитель и его места заменяются вместе.
func (s *Service) replace(ctx context.Context, key store.GetGuideParams, page *Page, a Article, photos map[string]Photo) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit — no-op
	q := s.q.WithTx(tx)

	params := store.UpsertGuideParams{PlaceID: key.PlaceID, Source: key.Source, Lang: key.Lang, License: licenseWikivoyage}
	if page != nil {
		params.PageTitle = text(page.Title)
		params.PageUrl = text(page.URL)
		params.RevisionID = pgtype.Int8{Int64: page.Revision, Valid: true}
		params.Overview = text(a.Overview)
	}
	id, err := q.UpsertGuide(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert guide: %w", err)
	}
	if err := q.DeleteGuideListings(ctx, id); err != nil {
		return fmt.Errorf("delete guide listings: %w", err)
	}
	if len(a.Listings) > 0 {
		rows := make([]store.InsertGuideListingsParams, len(a.Listings))
		for i, l := range a.Listings {
			r := store.InsertGuideListingsParams{
				GuideID: id, Position: int32(i), Type: l.Type, Name: l.Name,
				Alt: text(l.Alt), Description: text(l.Description), Address: text(l.Address), Url: text(l.URL),
				Hours: text(l.Hours), Price: text(l.Price), Wikidata: text(l.Wikidata), ImageFile: text(l.ImageFile),
			}
			if l.HasLocation {
				r.Lat = pgtype.Float8{Float64: l.Lat, Valid: true}
				r.Lon = pgtype.Float8{Float64: l.Lon, Valid: true}
			}
			if p, ok := photos[l.ImageFile]; ok {
				r.PhotoUrl, r.PhotoPage, r.PhotoCredit = text(p.URL), text(p.Page), text(p.Credit)
			}
			rows[i] = r
		}
		var firstErr error
		q.InsertGuideListings(ctx, rows).Exec(func(_ int, err error) {
			if err != nil && firstErr == nil {
				firstErr = err
			}
		})
		if firstErr != nil {
			return fmt.Errorf("insert guide listings: %w", firstErr)
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) load(ctx context.Context, row store.GetGuideRow) (Guide, error) {
	if !row.PageTitle.Valid {
		return Guide{}, ErrNotFound
	}
	rows, err := s.q.ListGuideListings(ctx, row.ID)
	if err != nil {
		return Guide{}, fmt.Errorf("list guide listings: %w", err)
	}
	g := Guide{
		Source: sourceWikivoyage, Title: row.PageTitle.String, URL: row.PageUrl.String,
		Overview: row.Overview.String, License: row.License, Listings: make([]GuideListing, len(rows)),
	}
	for i, r := range rows {
		l := GuideListing{Listing: Listing{
			Type: r.Type, Name: r.Name, Alt: r.Alt.String, Description: r.Description.String, Address: r.Address.String,
			URL: r.Url.String, Hours: r.Hours.String, Price: r.Price.String, Lat: r.Lat, Lon: r.Lon,
			HasLocation: r.HasLocation, Wikidata: r.Wikidata.String,
		}}
		if r.PhotoUrl.Valid {
			l.Photo = &Photo{URL: r.PhotoUrl.String, Page: r.PhotoPage.String, Credit: r.PhotoCredit.String}
		}
		g.Listings[i] = l
	}
	return g, nil
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
