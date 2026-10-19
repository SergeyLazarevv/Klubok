package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/SergeyLazarevv/Klubok/backend/internal/favorites"
	"github.com/SergeyLazarevv/Klubok/backend/internal/httpapi/apigen"
	"github.com/SergeyLazarevv/Klubok/backend/internal/linkpreview"
	"github.com/SergeyLazarevv/Klubok/backend/internal/session"
)

// Путевой лист — пункты сохранённой поездки (ADR-0019). Владелец — тот же, что у поездки.

func (h *handlers) ListTripItems(ctx context.Context, req apigen.ListTripItemsRequestObject) (apigen.ListTripItemsResponseObject, error) {
	sid, _ := session.FromContext(ctx)
	list, err := h.favorites.Items(ctx, sid, req.Id)
	if errors.Is(err, favorites.ErrNotFound) {
		return apigen.ListTripItems404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("favorite")}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]apigen.TripItem, len(list))
	for i, it := range list {
		items[i] = toAPITripItem(it)
	}
	return apigen.ListTripItems200JSONResponse{Items: items}, nil
}

func (h *handlers) AddTripItem(ctx context.Context, req apigen.AddTripItemRequestObject) (apigen.AddTripItemResponseObject, error) {
	sid, _ := session.FromContext(ctx)
	b := req.Body
	it := favorites.Item{Stage: string(b.Stage), Kind: string(b.Kind), Title: strings.TrimSpace(b.Title)}
	if b.Url != nil {
		it.URL = strings.TrimSpace(*b.Url)
	}
	if b.Price != nil {
		it.Price = &b.Price.Amount
	}
	if b.ImageUrl != nil {
		it.ImageURL = *b.ImageUrl
	}
	if b.Description != nil {
		it.Desc = strings.TrimSpace(*b.Description)
	}
	if b.City != nil {
		it.City = *b.City
	}
	if l := b.Location; l != nil {
		if l.Lat < -90 || l.Lat > 90 || l.Lon < -180 || l.Lon > 180 {
			return apigen.AddTripItem422ApplicationProblemPlusJSONResponse(fieldProblem("location", "координаты вне диапазона")), nil
		}
		it.Location = &favorites.Point{Lat: l.Lat, Lon: l.Lon}
	}
	if p := itemProblem(it.Title, it.Price); p != nil {
		return apigen.AddTripItem422ApplicationProblemPlusJSONResponse(*p), nil
	}
	if it.Kind == string(apigen.KindLink) && it.URL == "" {
		return apigen.AddTripItem422ApplicationProblemPlusJSONResponse(fieldProblem("url", "для ссылки нужен адрес")), nil
	}
	saved, err := h.favorites.AddItem(ctx, sid, req.Id, it)
	if errors.Is(err, favorites.ErrNotFound) {
		return apigen.AddTripItem404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("favorite")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.AddTripItem201JSONResponse(toAPITripItem(saved)), nil
}

func (h *handlers) UpdateTripItem(ctx context.Context, req apigen.UpdateTripItemRequestObject) (apigen.UpdateTripItemResponseObject, error) {
	sid, _ := session.FromContext(ctx)
	b := req.Body
	p := favorites.ItemPatch{Done: b.Done, ClearPrice: b.ClearPrice != nil && *b.ClearPrice, ImageURL: b.ImageUrl, Desc: b.Description}
	if b.Title != nil {
		t := strings.TrimSpace(*b.Title)
		p.Title = &t
	}
	if b.Price != nil {
		p.Price = &b.Price.Amount
	}
	if prob := itemProblem(derefOr(p.Title, "-"), p.Price); prob != nil {
		return apigen.UpdateTripItem422ApplicationProblemPlusJSONResponse(*prob), nil
	}
	saved, err := h.favorites.UpdateItem(ctx, sid, req.Id, req.ItemId, p)
	if errors.Is(err, favorites.ErrNotFound) {
		return apigen.UpdateTripItem404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("trip item")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.UpdateTripItem200JSONResponse(toAPITripItem(saved)), nil
}

func (h *handlers) DeleteTripItem(ctx context.Context, req apigen.DeleteTripItemRequestObject) (apigen.DeleteTripItemResponseObject, error) {
	sid, _ := session.FromContext(ctx)
	err := h.favorites.DeleteItem(ctx, sid, req.Id, req.ItemId)
	if errors.Is(err, favorites.ErrNotFound) {
		return apigen.DeleteTripItem404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("trip item")}, nil
	}
	if err != nil {
		return nil, err
	}
	return apigen.DeleteTripItem204Response{}, nil
}

// itemProblem — проверки, которых нет в схеме: название из одних пробелов, отрицательная цена.
func itemProblem(title string, price *int64) *apigen.Problem {
	if title == "" {
		p := fieldProblem("title", "пустое название")
		return &p
	}
	if price != nil && *price < 0 {
		p := fieldProblem("price", "цена не может быть отрицательной")
		return &p
	}
	return nil
}

func derefOr(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

func toAPITripItem(it favorites.Item) apigen.TripItem {
	out := apigen.TripItem{
		Id: it.ID, Stage: apigen.TripItemStage(it.Stage), Kind: apigen.TripItemKind(it.Kind),
		Title: it.Title, Url: optional(it.URL), Done: it.Done, CreatedAt: it.CreatedAt,
		ImageUrl: optional(it.ImageURL), Description: optional(it.Desc), City: optional(it.City),
	}
	if it.Price != nil {
		out.Price = &apigen.Money{Amount: *it.Price, Currency: apigen.RUB}
	}
	if it.Location != nil {
		out.Location = &apigen.Point{Lat: it.Location.Lat, Lon: it.Location.Lon}
	}
	return out
}

// GetLinkPreview — карточка ссылки (название, картинка, цена за сутки) до того, как пункт добавлен.
func (h *handlers) GetLinkPreview(ctx context.Context, req apigen.GetLinkPreviewRequestObject) (apigen.GetLinkPreviewResponseObject, error) {
	p, err := h.previews.Get(ctx, req.Params.Url)
	if errors.Is(err, linkpreview.ErrNotSupported) {
		return apigen.GetLinkPreview404ApplicationProblemPlusJSONResponse{ProblemApplicationProblemPlusJSONResponse: notFound("preview")}, nil
	}
	if err != nil {
		h.log.WarnContext(ctx, "link preview failed", slog.Any("err", err))
		detail := "сайт не ответил"
		return apigen.GetLinkPreview502ApplicationProblemPlusJSONResponse{Status: http.StatusBadGateway, Title: "Bad Gateway", Detail: &detail}, nil
	}
	out := apigen.LinkPreview{Site: siteName(req.Params.Url), Title: optional(p.Title), Description: optional(p.Description), ImageUrl: optional(p.Image)}
	if p.PerNight > 0 {
		out.PerNight = &apigen.Money{Amount: p.PerNight, Currency: apigen.RUB}
	}
	return apigen.GetLinkPreview200JSONResponse(out), nil
}

var siteNames = map[string]string{
	"sutochno.ru": "Суточно.ру", "ostrovok.ru": "Островок", "101hotels.com": "101 Отель", "tvil.ru": "Твил",
	"tripster.ru": "Трипстер", "sputnik8.com": "Спутник8", "localrent.com": "Localrent",
}

func siteName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	for d, name := range siteNames {
		if host == d || strings.HasSuffix(host, "."+d) {
			return name
		}
	}
	return host
}
