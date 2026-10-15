package favorites

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites/store"
)

// Item — пункт путевого листа (ADR-0019): заметка, ссылка (объявление, аренда) или место по пути.
// Привязан к этапу поездки; цена — в копейках, её вводит пользователь.
type Item struct {
	ID        uuid.UUID
	Stage     string
	Kind      string
	Title     string
	URL       string
	Price     *int64
	Location  *Point // где место — для карты и дороги через места «по пути»
	ImageURL  string // превью ссылки: картинка и описание со страницы
	Desc      string
	Done      bool
	CreatedAt time.Time
}

type Point struct{ Lat, Lon float64 }

// ItemPatch — что меняем; nil — оставить как есть. ClearPrice сбрасывает цену.
type ItemPatch struct {
	Title      *string
	Done       *bool
	Price      *int64
	ClearPrice bool
	ImageURL   *string // превью, подтянутое позже — для ссылок, добавленных без него
	Desc       *string
}

func (s *Service) Items(ctx context.Context, sessionID, favoriteID uuid.UUID) ([]Item, error) {
	if _, err := s.Get(ctx, sessionID, favoriteID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListTripItems(ctx, store.ListTripItemsParams{FavoriteID: favoriteID, SessionID: sessionID})
	if err != nil {
		return nil, fmt.Errorf("list trip items: %w", err)
	}
	out := make([]Item, len(rows))
	for i, r := range rows {
		out[i] = item(store.InsertTripItemRow(r))
	}
	return out, nil
}

// AddItem добавляет пункт; ErrNotFound — поездки нет или она чужая.
func (s *Service) AddItem(ctx context.Context, sessionID, favoriteID uuid.UUID, it Item) (Item, error) {
	r, err := s.q.InsertTripItem(ctx, store.InsertTripItemParams{
		ID: uuid.Must(uuid.NewV7()), Stage: it.Stage, Kind: it.Kind, Title: it.Title,
		Url: pgtype.Text{String: it.URL, Valid: it.URL != ""}, Price: price(it.Price),
		Lat:         coord(it.Location, func(p *Point) float64 { return p.Lat }),
		Lon:         coord(it.Location, func(p *Point) float64 { return p.Lon }),
		ImageUrl:    pgtype.Text{String: it.ImageURL, Valid: it.ImageURL != ""},
		Description: pgtype.Text{String: it.Desc, Valid: it.Desc != ""},
		FavoriteID:  favoriteID, SessionID: sessionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("insert trip item: %w", err)
	}
	return item(r), nil
}

func (s *Service) UpdateItem(ctx context.Context, sessionID, favoriteID, id uuid.UUID, p ItemPatch) (Item, error) {
	arg := store.UpdateTripItemParams{
		Price: price(p.Price), ClearPrice: p.ClearPrice, ID: id, FavoriteID: favoriteID, SessionID: sessionID,
	}
	if p.Title != nil {
		arg.Title = pgtype.Text{String: *p.Title, Valid: true}
	}
	if p.Done != nil {
		arg.Done = pgtype.Bool{Bool: *p.Done, Valid: true}
	}
	if p.ImageURL != nil {
		arg.ImageUrl = pgtype.Text{String: *p.ImageURL, Valid: true}
	}
	if p.Desc != nil {
		arg.Description = pgtype.Text{String: *p.Desc, Valid: true}
	}
	r, err := s.q.UpdateTripItem(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("update trip item: %w", err)
	}
	return item(store.InsertTripItemRow(r)), nil
}

func (s *Service) DeleteItem(ctx context.Context, sessionID, favoriteID, id uuid.UUID) error {
	n, err := s.q.DeleteTripItem(ctx, store.DeleteTripItemParams{ID: id, FavoriteID: favoriteID, SessionID: sessionID})
	if err != nil {
		return fmt.Errorf("delete trip item: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func item(r store.InsertTripItemRow) Item {
	it := Item{
		ID: r.ID, Stage: r.Stage, Kind: r.Kind, Title: r.Title, URL: r.Url.String, Done: r.Done, CreatedAt: r.CreatedAt.Time,
		ImageURL: r.ImageUrl.String, Desc: r.Description.String,
	}
	if r.Price.Valid {
		it.Price = &r.Price.Int64
	}
	if r.Lat.Valid && r.Lon.Valid {
		it.Location = &Point{r.Lat.Float64, r.Lon.Float64}
	}
	return it
}

func coord(p *Point, get func(*Point) float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: get(p), Valid: true}
}

func price(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}
