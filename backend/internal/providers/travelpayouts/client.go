// Package travelpayouts — источник цен Aviasales Data API (кэш поисков пользователей Aviasales).
// Документация: https://support.travelpayouts.com/hc/ru/articles/203956163
package travelpayouts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
)

const (
	source      = "travelpayouts"
	apiURL      = "https://api.travelpayouts.com/aviasales/v3/prices_for_dates"
	bookingBase = "https://www.aviasales.ru"
	cacheTTL    = 30 * time.Minute
	maxResults  = 1000
)

var ErrNoToken = errors.New("travelpayouts: token is not configured")

type Client struct {
	token  string
	marker string
	http   *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	quotes []fares.Quote
	until  time.Time
}

func New(token, marker string) *Client {
	return &Client{
		token:  token,
		marker: marker,
		http:   &http.Client{Timeout: 15 * time.Second},
		cache:  make(map[string]cached),
	}
}

type response struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Data    []struct {
		OriginAirport      string `json:"origin_airport"`
		DestinationAirport string `json:"destination_airport"`
		DepartureAt        string `json:"departure_at"`
		ReturnAt           string `json:"return_at"`
		Price              int64  `json:"price"`
		Airline            string `json:"airline"`
		Transfers          int    `json:"transfers"`
		ReturnTransfers    int    `json:"return_transfers"`
		DurationTo         int    `json:"duration_to"`
		DurationBack       int    `json:"duration_back"`
		Link               string `json:"link"`
	} `json:"data"`
}

// RoundTrips возвращает самые дешёвые перелёты туда-обратно с вылетом в заданном месяце.
func (c *Client) RoundTrips(ctx context.Context, q fares.RoundTripQuery) ([]fares.Quote, error) {
	if c.token == "" {
		return nil, ErrNoToken
	}
	params := url.Values{
		"origin":       {q.Origin},
		"destination":  {q.Destination},
		"departure_at": {q.Month.Format("2006-01")},
		"one_way":      {"false"},
		"currency":     {"rub"},
		"sorting":      {"price"},
		"limit":        {fmt.Sprint(maxResults)},
	}
	key := params.Encode()
	if quotes, ok := c.fromCache(key); ok {
		return quotes, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"?"+key, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Access-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("travelpayouts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("travelpayouts: status %d: %s", resp.StatusCode, body)
	}
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("travelpayouts: decode: %w", err)
	}
	if !r.Success {
		return nil, fmt.Errorf("travelpayouts: %s", r.Error)
	}

	quotes := make([]fares.Quote, 0, len(r.Data))
	for _, d := range r.Data {
		dep, err1 := time.Parse(time.RFC3339, d.DepartureAt)
		ret, err2 := time.Parse(time.RFC3339, d.ReturnAt)
		if err1 != nil || err2 != nil {
			continue // без дат вариант бесполезен
		}
		quotes = append(quotes, fares.Quote{
			Source:          source,
			Origin:          d.OriginAirport,
			Destination:     d.DestinationAirport,
			DepartAt:        dep,
			ReturnAt:        ret,
			Price:           d.Price * 100,
			Transfers:       d.Transfers,
			ReturnTransfers: d.ReturnTransfers,
			Airline:         d.Airline,
			DurationTo:      time.Duration(d.DurationTo) * time.Minute,
			DurationBack:    time.Duration(d.DurationBack) * time.Minute,
			FoundOn:         foundOn(d.Link),
			BookingURL:      c.bookingURL(d.Link),
		})
	}
	c.toCache(key, quotes)
	return quotes, nil
}

// bookingURL — ссылка на выдачу Aviasales с партнёрским маркером.
func (c *Client) bookingURL(link string) string {
	if link == "" {
		return ""
	}
	u := bookingBase + link
	if c.marker != "" {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "marker=" + url.QueryEscape(c.marker)
	}
	return u
}

// foundOn достаёт дату находки цены из параметра search_date=ddmmyyyy в ссылке.
func foundOn(link string) time.Time {
	u, err := url.Parse(link)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse("02012006", u.Query().Get("search_date"))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (c *Client) fromCache(key string) ([]fares.Quote, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[key]
	if !ok || time.Now().After(e.until) {
		return nil, false
	}
	return e.quotes, true
}

func (c *Client) toCache(key string, quotes []fares.Quote) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, e := range c.cache {
		if now.After(e.until) {
			delete(c.cache, k)
		}
	}
	c.cache[key] = cached{quotes: quotes, until: now.Add(cacheTTL)}
}
