// Package favorites — избранные маршруты: каркас поиска + выбранный вариант + цена на момент сохранения.
// Цена пересчитывается при открытии повторением того же поиска (ADR-0018).
package favorites

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites/store"
)

var (
	ErrNotFound  = errors.New("favorite not found")
	ErrDuplicate = errors.New("the same trip is already in favorites")
)

// Favorite — запись избранного; params, pick и snapshot хранятся как JSON в формате API.
type Favorite struct {
	ID        uuid.UUID
	Title     string
	Params    json.RawMessage
	Pick      json.RawMessage
	Snapshot  json.RawMessage
	CreatedAt time.Time
}

type Service struct {
	q *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: store.New(pool)}
}

// Add сохраняет вариант. created=false — такой уже был в избранном этой сессии.
func (s *Service) Add(ctx context.Context, sessionID uuid.UUID, f Favorite) (Favorite, bool, error) {
	fp, err := Fingerprint(f.Params, f.Pick)
	if err != nil {
		return Favorite{}, false, err
	}
	// Этот вариант удаляли — возвращаем ту же запись вместе с путевым листом («Отменить», повторная ☆)
	old, err := s.q.RestoreFavorite(ctx, store.RestoreFavoriteParams{
		Title: f.Title, Snapshot: f.Snapshot, SessionID: sessionID, Fingerprint: fp,
	})
	if err == nil {
		return Favorite{old.ID, old.Title, old.Params, old.Pick, old.Snapshot, old.CreatedAt.Time}, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Favorite{}, false, fmt.Errorf("restore favorite: %w", err)
	}
	r, err := s.q.InsertFavorite(ctx, store.InsertFavoriteParams{
		ID: uuid.Must(uuid.NewV7()), SessionID: sessionID, Kind: "trip", Title: f.Title,
		Params: f.Params, Pick: f.Pick, Snapshot: f.Snapshot, Fingerprint: fp,
	})
	if err != nil {
		return Favorite{}, false, fmt.Errorf("insert favorite: %w", err)
	}
	return Favorite{r.ID, r.Title, r.Params, r.Pick, r.Snapshot, r.CreatedAt.Time}, r.Created, nil
}

func (s *Service) List(ctx context.Context, sessionID uuid.UUID) ([]Favorite, error) {
	rows, err := s.q.ListFavorites(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	out := make([]Favorite, len(rows))
	for i, r := range rows {
		out[i] = Favorite{r.ID, r.Title, r.Params, r.Pick, r.Snapshot, r.CreatedAt.Time}
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, sessionID, id uuid.UUID) (Favorite, error) {
	r, err := s.q.GetFavorite(ctx, store.GetFavoriteParams{ID: id, SessionID: sessionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Favorite{}, ErrNotFound
	}
	if err != nil {
		return Favorite{}, fmt.Errorf("get favorite: %w", err)
	}
	return Favorite{r.ID, r.Title, r.Params, r.Pick, r.Snapshot, r.CreatedAt.Time}, nil
}

// Update меняет параметры поездки и снимок цены (остановки по пути); ErrDuplicate — такая поездка
// с теми же параметрами уже есть в избранном этой сессии.
func (s *Service) Update(ctx context.Context, sessionID, id uuid.UUID, params, snapshot json.RawMessage) (Favorite, error) {
	f, err := s.Get(ctx, sessionID, id)
	if err != nil {
		return Favorite{}, err
	}
	fp, err := Fingerprint(params, f.Pick)
	if err != nil {
		return Favorite{}, err
	}
	r, err := s.q.UpdateFavorite(ctx, store.UpdateFavoriteParams{Params: params, Snapshot: snapshot, Fingerprint: fp, ID: id, SessionID: sessionID})
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return Favorite{}, ErrDuplicate
	case errors.Is(err, pgx.ErrNoRows):
		return Favorite{}, ErrNotFound
	case err != nil:
		return Favorite{}, fmt.Errorf("update favorite: %w", err)
	}
	return Favorite{r.ID, r.Title, r.Params, r.Pick, r.Snapshot, r.CreatedAt.Time}, nil
}

// Delete убирает из избранного (мягко: запись остаётся с deleted_at).
func (s *Service) Delete(ctx context.Context, sessionID, id uuid.UUID) error {
	n, err := s.q.DeleteFavorite(ctx, store.DeleteFavoriteParams{ID: id, SessionID: sessionID})
	if err != nil {
		return fmt.Errorf("delete favorite: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// fingerprintParams — по каким параметрам вариант считается «тем же самым». Настройки машины не входят:
// поменял расход — это всё ещё тот же маршрут. Та же строка считается во фронтенде (lib/favorites.ts).
type fingerprintParams struct {
	From      string   `json:"from"`
	To        string   `json:"to"`
	DateFrom  string   `json:"date_from"`
	DateTo    string   `json:"date_to"`
	MinDays   *int     `json:"min_days"`
	MaxDays   *int     `json:"max_days"`
	Travelers *int     `json:"travelers"`
	UseCar    *bool    `json:"use_car"`
	Weekends  bool     `json:"weekends"`
	Via       []string `json:"via"`
}

type fingerprintPick struct {
	Kind       string `json:"kind"`
	HubIATA    string `json:"hub_iata"`
	DepartDate string `json:"depart_date"`
	ReturnDate string `json:"return_date"`
}

// Key — ключ варианта: «откуда|куда|дата от|дата до|дни от|до|люди|машина|вид|аэропорт|туда|обратно».
func Key(params, pick json.RawMessage) (string, error) {
	var p fingerprintParams
	var k fingerprintPick
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("params: %w", err)
	}
	if err := json.Unmarshal(pick, &k); err != nil {
		return "", fmt.Errorf("pick: %w", err)
	}
	parts := []string{
		p.From, p.To, p.DateFrom, p.DateTo, itoa(p.MinDays, 3), itoa(p.MaxDays, 14), itoa(p.Travelers, 1), btoa(p.UseCar),
		k.Kind, k.HubIATA, k.DepartDate, k.ReturnDate,
	}
	// Только выходные — другая выдача; в конце и только когда включено, чтобы ключи старых записей не поменялись
	if p.Weekends {
		parts = append(parts, "weekends")
	}
	// Остановки по пути — тоже другой маршрут; так же только когда есть
	if len(p.Via) > 0 {
		parts = append(parts, "via="+strings.Join(p.Via, ","))
	}
	return strings.Join(parts, "|"), nil
}

// Fingerprint — хеш ключа: по нему не добавляем один и тот же вариант дважды.
func Fingerprint(params, pick json.RawMessage) (string, error) {
	key, err := Key(params, pick)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:]), nil
}

func itoa(v *int, def int) string {
	if v == nil {
		return fmt.Sprint(def)
	}
	return fmt.Sprint(*v)
}

func btoa(v *bool) string {
	if v != nil && *v {
		return "car"
	}
	return "nocar"
}
