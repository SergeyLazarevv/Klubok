package travelpayouts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Партнёрские ссылки: обычная ссылка на сайт бренда превращается в ссылку с меткой партнёра.
// Документация: https://support.travelpayouts.com/hc/en-us/articles/25289759198226
const linksURL = "https://api.travelpayouts.com/links/v1/create"

// ErrNoProject — не задан ID проекта (trs), к которому подключена программа бренда.
var ErrNoProject = errors.New("travelpayouts: project id (trs) is not configured")

// Linker делает партнёрские ссылки для проекта trs.
type Linker struct {
	c       *Client
	project string
	url     string
}

func (c *Client) Linker(project string) *Linker {
	return &Linker{c: c, project: project, url: linksURL}
}

type linksRequest struct {
	TRS     string `json:"trs"`
	Marker  string `json:"marker"`
	Shorten bool   `json:"shorten"`
	Links   []struct {
		URL string `json:"url"`
	} `json:"links"`
}

type linksResponse struct {
	Code   string `json:"code"`
	Result struct {
		Links []struct {
			URL        string `json:"url"`
			Code       string `json:"code"`
			PartnerURL string `json:"partner_url"`
			Message    string `json:"message"`
		} `json:"links"`
	} `json:"result"`
	Error string `json:"error"`
}

// PartnerLink превращает ссылку на сайт бренда (например, поиск на Островке) в партнёрскую.
func (l *Linker) PartnerLink(ctx context.Context, link string) (string, error) {
	if l.c.token == "" {
		return "", ErrNoToken
	}
	if l.project == "" {
		return "", ErrNoProject
	}
	body := linksRequest{TRS: l.project, Marker: l.c.marker}
	body.Links = append(body.Links, struct {
		URL string `json:"url"`
	}{URL: link})
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Token", l.c.token)
	resp, err := l.c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("travelpayouts links: %w", err)
	}
	defer resp.Body.Close()
	var r linksResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("travelpayouts links: decode (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || len(r.Result.Links) == 0 {
		return "", fmt.Errorf("travelpayouts links: status %d: %s %s", resp.StatusCode, r.Code, r.Error)
	}
	got := r.Result.Links[0]
	if got.PartnerURL == "" {
		return "", fmt.Errorf("travelpayouts links: %s %s", got.Code, got.Message)
	}
	return got.PartnerURL, nil
}
