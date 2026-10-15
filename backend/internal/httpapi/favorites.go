package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/session"
)

func (h *handlers) ListFavorites(ctx context.Context, _ apigen.ListFavoritesRequestObject) (apigen.ListFavoritesResponseObject, error) {
	sid, ok := session.FromContext(ctx)
	if !ok {
		return apigen.ListFavorites200JSONResponse{Items: []apigen.Favorite{}}, nil
	}
	list, err := h.favorites.List(ctx, sid)
	if err != nil {
		return nil, err
	}
	items := make([]apigen.Favorite, len(list))
	for i, f := range list {
		if items[i], err = toAPIFavorite(f); err != nil {
			return nil, err
		}
	}
	return apigen.ListFavorites200JSONResponse{Items: items}, nil
}

func (h *handlers) AddFavorite(ctx context.Context, req apigen.AddFavoriteRequestObject) (apigen.AddFavoriteResponseObject, error) {
	sid, ok := session.FromContext(ctx)
	if !ok {
		return nil, errors.New("favorites: no session (middleware must create it)")
	}
	b := req.Body
	if b.Pick.Kind == apigen.Flight && (b.Pick.HubIata == nil || b.Pick.DepartDate == nil || b.Pick.ReturnDate == nil) {
		return apigen.AddFavorite422ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: apigen.ProblemApplicationProblemPlusJSONResponse(
			fieldProblem("pick", "для перелёта нужны аэропорт и даты"))}, nil
	}
	f := favorites.Favorite{Title: b.Title}
	var err error
	if f.Params, err = json.Marshal(b.Params); err != nil {
		return nil, err
	}
	if f.Pick, err = json.Marshal(b.Pick); err != nil {
		return nil, err
	}
	if f.Snapshot, err = json.Marshal(b.Snapshot); err != nil {
		return nil, err
	}
	saved, created, err := h.favorites.Add(ctx, sid, f)
	if err != nil {
		return nil, err
	}
	out, err := toAPIFavorite(saved)
	if err != nil {
		return nil, err
	}
	if created {
		return apigen.AddFavorite201JSONResponse(out), nil
	}
	return apigen.AddFavorite200JSONResponse(out), nil
}

func (h *handlers) DeleteFavorite(ctx context.Context, req apigen.DeleteFavoriteRequestObject) (apigen.DeleteFavoriteResponseObject, error) {
	sid, _ := session.FromContext(ctx)
	err := h.favorites.Delete(ctx, sid, req.Id)
	if errors.Is(err, favorites.ErrNotFound) {
		return apigen.DeleteFavorite404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("favorite")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.DeleteFavorite204Response{}, nil
}

// GetFavoriteCurrent повторяет сохранённый поиск с сохранёнными параметрами и ищет тот же вариант.
func (h *handlers) GetFavoriteCurrent(ctx context.Context, req apigen.GetFavoriteCurrentRequestObject) (apigen.GetFavoriteCurrentResponseObject, error) {
	sid, _ := session.FromContext(ctx)
	fav, err := h.favorites.Get(ctx, sid, req.Id)
	if errors.Is(err, favorites.ErrNotFound) {
		return apigen.GetFavoriteCurrent404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("favorite")}, nil
	}
	if err != nil {
		return nil, err
	}
	var (
		params   apigen.SearchTripsParams
		pick     apigen.FavoritePick
		snapshot apigen.FavoriteSnapshot
	)
	// params хранится в формате TripSearchParams — у него те же JSON-поля, что у запроса GET /trips
	for _, x := range []struct {
		raw []byte
		dst any
	}{{fav.Params, &params}, {fav.Pick, &pick}, {fav.Snapshot, &snapshot}} {
		if err := json.Unmarshal(x.raw, x.dst); err != nil {
			return nil, fmt.Errorf("favorite %s: %w", fav.ID, err)
		}
	}

	if datePassed(params.DateTo, time.Now()) {
		return apigen.GetFavoriteCurrent200JSONResponse{Status: apigen.Past}, nil
	}
	resp, err := h.SearchTrips(ctx, apigen.SearchTripsRequestObject{Params: params})
	if err != nil {
		return nil, err
	}
	res, ok := resp.(apigen.SearchTrips200JSONResponse)
	if !ok {
		return apigen.GetFavoriteCurrent200JSONResponse{Status: apigen.Unavailable}, nil
	}
	trips := apigen.TripsResponse(res)
	cur := matchFavorite(trips, pick)
	cur.Result = &trips
	if cur.Total != nil {
		diff := rub(cur.Total.Amount - snapshot.Total.Amount)
		cur.Diff = &diff
	}
	return apigen.GetFavoriteCurrent200JSONResponse(cur), nil
}

// matchFavorite ищет в свежей выдаче сохранённый вариант; нет его — самый дешёвый из выдачи.
func matchFavorite(res apigen.TripsResponse, pick apigen.FavoritePick) apigen.FavoriteCurrent {
	if pick.Kind == apigen.Drive && res.Drive != nil {
		t := res.Drive.Total
		return apigen.FavoriteCurrent{Status: apigen.Found, Drive: res.Drive, Total: &t}
	}
	if pick.Kind == apigen.Flight && pick.HubIata != nil && pick.DepartDate != nil && pick.ReturnDate != nil {
		for i, o := range res.Options {
			if o.Hub.Iata == *pick.HubIata && localDate(o.DepartAt) == pick.DepartDate.String() &&
				localDate(o.ReturnAt) == pick.ReturnDate.String() {
				t := o.Total
				return apigen.FavoriteCurrent{Status: apigen.Found, Option: &res.Options[i], Total: &t}
			}
		}
	}
	// Сохранённого варианта нет: лучший из той же выдачи — поездка на машине или самый дешёвый перелёт
	var best apigen.FavoriteCurrent
	if len(res.Options) > 0 {
		t := res.Options[0].Total // варианты отсортированы по полной стоимости
		best = apigen.FavoriteCurrent{Status: apigen.Replaced, Option: &res.Options[0], Total: &t}
	}
	if res.Drive != nil && (best.Total == nil || res.Drive.Total.Amount < best.Total.Amount) {
		t := res.Drive.Total
		best = apigen.FavoriteCurrent{Status: apigen.Replaced, Drive: res.Drive, Total: &t}
	}
	if best.Total == nil {
		return apigen.FavoriteCurrent{Status: apigen.Empty}
	}
	return best
}

// localDate — дата по местному времени аэропорта (время в ответе — с его смещением).
func localDate(t time.Time) string { return t.Format("2006-01-02") }

func datePassed(dateTo openapi_types.Date, now time.Time) bool {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return dateTo.Time.Before(today)
}

func toAPIFavorite(f favorites.Favorite) (apigen.Favorite, error) {
	out := apigen.Favorite{Id: f.ID, Title: f.Title, CreatedAt: f.CreatedAt}
	for _, x := range []struct {
		raw []byte
		dst any
	}{{f.Params, &out.Params}, {f.Pick, &out.Pick}, {f.Snapshot, &out.Snapshot}} {
		if err := json.Unmarshal(x.raw, x.dst); err != nil {
			return out, fmt.Errorf("favorite %s: %w", f.ID, err)
		}
	}
	return out, nil
}
