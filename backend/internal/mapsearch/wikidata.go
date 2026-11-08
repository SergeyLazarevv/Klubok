package mapsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/transfer"
)

// Wikidata — поиск известных мест: музеи, горы, озёра, города. В отличие от Photon ранжирует по
// известности: «Эрмитаж» — это музей в Петербурге, а не кафе на соседней улице.
type Wikidata struct {
	api    string
	client *http.Client
}

func NewWikidata() *Wikidata {
	return &Wikidata{api: "https://www.wikidata.org/w/api.php", client: &http.Client{Timeout: 5 * time.Second}}
}

// Find — элементы с координатами, лучшие первыми.
func (w *Wikidata) Find(ctx context.Context, q string, limit int) ([]transfer.Found, error) {
	var found struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"search"`
	}
	if err := w.get(ctx, url.Values{
		"action": {"wbsearchentities"}, "search": {q}, "language": {"ru"}, "uselang": {"ru"},
		"type": {"item"}, "limit": {fmt.Sprint(limit)},
	}, &found); err != nil {
		return nil, err
	}
	if len(found.Search) == 0 {
		return nil, nil
	}
	ids := make([]string, len(found.Search))
	for i, s := range found.Search {
		ids[i] = s.ID
	}

	// Координаты (P625) — одним запросом на все элементы
	var coords struct {
		Query struct {
			Pages []struct {
				Title       string `json:"title"`
				Coordinates []struct {
					Lat   float64 `json:"lat"`
					Lon   float64 `json:"lon"`
					Globe string  `json:"globe"`
				} `json:"coordinates"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := w.get(ctx, url.Values{
		"action": {"query"}, "prop": {"coordinates"}, "titles": {strings.Join(ids, "|")}, "formatversion": {"2"},
	}, &coords); err != nil {
		return nil, err
	}
	at := map[string]transfer.Point{}
	for _, p := range coords.Query.Pages {
		if len(p.Coordinates) > 0 && p.Coordinates[0].Globe == "earth" {
			at[p.Title] = transfer.Point{Lat: p.Coordinates[0].Lat, Lon: p.Coordinates[0].Lon}
		}
	}

	// Без координат — не место (фамилия, понятие, статья): на карте их не показать
	out := []transfer.Found{}
	for _, s := range found.Search {
		pt, ok := at[s.ID]
		if !ok || s.Label == "" {
			continue
		}
		out = append(out, transfer.Found{Name: s.Label, Subtitle: s.Description, Kind: kindFromDescription(s.Description), Point: pt})
	}
	return out, nil
}

func (w *Wikidata) get(ctx context.Context, params url.Values, dst any) error {
	params.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.api+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("wikidata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wikidata: status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("wikidata: decode: %w", err)
	}
	return nil
}

// Вид места по описанию Wikidata: «город в России», «село в Карелии», «озеро в Сибири».
// Русского описания нет — Wikidata отдаёт английское.
var descriptionKinds = []struct {
	kind  string
	words []string
}{
	{"city", []string{"город", "столица", "мегаполис", "административный центр", "city", "town", "capital"}},
	{"village", []string{"село", "деревня", "посёлок", "поселок", "станица", "хутор", "аул", "пгт", "village", "hamlet"}},
	{"nature", []string{
		"озеро", "река", "гора", "вулкан", "остров", "архипелаг", "залив", "бухта", "водопад", "пещера",
		"национальный парк", "заповедник", "природный парк", "хребет", "горный", "пик", "перевал", "пролив",
		"море", "полуостров", "ущелье", "каньон", "водохранилище", "ледник", "мыс", "коса",
		"river", "lake", "mountain", "volcano", "island", "bay", "waterfall", "cave", "national park", "peak",
	}},
}

func kindFromDescription(d string) string {
	d = strings.ToLower(d)
	for _, k := range descriptionKinds {
		for _, w := range k.words {
			if strings.HasPrefix(d, w) {
				return k.kind
			}
		}
	}
	return "sight"
}
