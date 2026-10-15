// Package linkpreview — превью ссылки для путевого листа: название, описание, картинка и цена за ночь
// из og-тегов страницы (ADR-0019). Читаем только сайты из белого списка — сервер не ходит по
// произвольным адресам пользователя (защита от SSRF), и только по https.
package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	xhtml "golang.org/x/net/html"
)

// ErrNotSupported — сайт не из белого списка или закрыт для ботов (Авито): превью не будет.
var ErrNotSupported = errors.New("link preview not supported for this site")

// Preview — что удалось достать со страницы. Пустые поля — не нашлось.
type Preview struct {
	Title       string
	Description string
	Image       string
	PerNight    int64 // цена за сутки в копейках, если сайт пишет её в описании (Суточно.ру)
}

// allowed — сайты, у которых страницы объявлений отдают og-теги без JS. Авито отвечает ботам 429 — его нет.
var allowed = []string{"sutochno.ru", "ostrovok.ru", "101hotels.com", "tvil.ru", "tripster.ru", "sputnik8.com", "localrent.com"}

func Allowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, d := range allowed {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

const (
	maxBody  = 2 << 20 // og-теги в <head>, дальше не читаем
	cacheTTL = 12 * time.Hour
	maxCache = 1000
)

type entry struct {
	p   Preview
	exp time.Time
}

type Fetcher struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]entry
}

func New() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 8 * time.Second,
			// Переадресация — только на те же разрешённые сайты
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 || req.URL.Scheme != "https" || !Allowed(req.URL.Hostname()) {
					return ErrNotSupported
				}
				return nil
			},
		},
		cache: map[string]entry{},
	}
}

func (f *Fetcher) Get(ctx context.Context, raw string) (Preview, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !Allowed(u.Hostname()) || u.Port() != "" {
		return Preview{}, ErrNotSupported
	}
	u.Scheme = "https"
	u.Fragment = ""
	key := u.String()

	f.mu.Lock()
	if e, ok := f.cache[key]; ok && time.Now().Before(e.exp) {
		f.mu.Unlock()
		return e.p, nil
	}
	f.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, key, nil)
	if err != nil {
		return Preview{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Klubok/0.1; +https://github.com/SergeyLazarevv/Klubok)")
	req.Header.Set("Accept-Language", "ru")
	resp, err := f.client.Do(req)
	if err != nil {
		return Preview{}, fmt.Errorf("link preview: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Preview{}, fmt.Errorf("link preview: status %d", resp.StatusCode)
	}
	p := parse(io.LimitReader(resp.Body, maxBody), resp.Request.URL)

	f.mu.Lock()
	if len(f.cache) >= maxCache {
		clear(f.cache)
	}
	f.cache[key] = entry{p, time.Now().Add(cacheTTL)}
	f.mu.Unlock()
	return p, nil
}

// parse читает og-теги и <title> до конца <head>.
func parse(r io.Reader, base *url.URL) Preview {
	var p Preview
	var title string
	z := xhtml.NewTokenizer(r)
	for inTitle := false; ; {
		switch z.Next() {
		case xhtml.ErrorToken:
			return finish(p, title, base)
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "title":
				inTitle = true
			case "meta":
				if !hasAttr {
					continue
				}
				var prop, content string
				for {
					k, v, more := z.TagAttr()
					switch string(k) {
					case "property", "name":
						prop = string(v)
					case "content":
						content = string(v)
					}
					if !more {
						break
					}
				}
				content = strings.TrimSpace(html.UnescapeString(content))
				switch prop {
				case "og:title":
					p.Title = content
				case "og:description":
					p.Description = content
				case "description":
					if p.Description == "" {
						p.Description = content
					}
				case "og:image":
					p.Image = content
				}
			case "body":
				return finish(p, title, base)
			}
		case xhtml.TextToken:
			if inTitle {
				title += string(z.Text())
			}
		case xhtml.EndTagToken:
			if name, _ := z.TagName(); string(name) == "title" {
				inTitle = false
			} else if string(name) == "head" {
				return finish(p, title, base)
			}
		}
	}
}

func finish(p Preview, title string, base *url.URL) Preview {
	if p.Title == "" {
		p.Title = strings.TrimSpace(html.UnescapeString(title))
	}
	p.Title = cut(cleanTitle(p.Title), 300)
	p.Description = cut(p.Description, 500)
	// Картинка — только https; относительный адрес достраиваем от страницы
	if p.Image != "" {
		if img, err := base.Parse(p.Image); err == nil && img.Scheme == "https" {
			p.Image = img.String()
		} else {
			p.Image = ""
		}
	}
	p.PerNight = perNight(p.Description)
	return p
}

// SEO-хвосты заголовков: «… - забронировать онлайн», «178 отзывов об отеле, цены…», «, объявление 1331609».
var (
	titleTail  = regexp.MustCompile(`(?i)\s+[-—|]\s+[^-—|]*(забронир|бронирован|онлайн|цены|отзыв|суточно|островок|ostrovok).*$`)
	reviewTail = regexp.MustCompile(`(?i)\s+\d+\s+отзыв.*$`)
	adTail     = regexp.MustCompile(`(?i),?\s*объявление\s*№?\s*\d+\s*$`)
)

func cleanTitle(t string) string {
	t = strings.Join(strings.Fields(t), " ") // в заголовках бывают переносы строк и отступы вёрстки
	short := adTail.ReplaceAllString(reviewTail.ReplaceAllString(titleTail.ReplaceAllString(t, ""), ""), "")
	if short = strings.TrimSpace(short); len([]rune(short)) < 8 {
		return t // обрезали почти всё — оставляем как было
	}
	return short
}

// «От 3250 рублей за сутки», «Цена 5850 RUB за сутки» → копейки.
var priceRe = regexp.MustCompile(`(?i)(?:от|цена)\s+([\d][\d\s\x{00a0}]*)\s*(?:руб|rub|₽)[^.]{0,20}(?:сутки|ночь)`)

func perNight(s string) int64 {
	m := priceRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, m[1])
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || n <= 0 || n > 10_000_000 {
		return 0
	}
	return n * 100
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
