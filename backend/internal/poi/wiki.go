package poi

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Wiki достаёт описание и фото места из Wikidata, Википедии и Wikimedia Commons.
// Порядок: ссылка на статью → элемент Wikidata → фото (P18) и статья на русском → первые предложения статьи
// и уменьшенная копия фото с автором и лицензией.
type Wiki struct {
	client *http.Client
	// Адреса API; в тестах подменяются.
	wikidataAPI  string
	commonsAPI   string
	wikipediaAPI func(lang string) string
}

func NewWiki() *Wiki {
	return &Wiki{
		client:       &http.Client{Timeout: 15 * time.Second},
		wikidataAPI:  "https://www.wikidata.org/w/api.php",
		commonsAPI:   "https://commons.wikimedia.org/w/api.php",
		wikipediaAPI: func(lang string) string { return "https://" + lang + ".wikipedia.org/w/api.php" },
	}
}

const (
	photoWidth       = 640
	extractSentences = 3
)

// Lookup возвращает подробности и найденный элемент Wikidata (если его не было в OSM).
func (w *Wiki) Lookup(ctx context.Context, p POI) (Details, string, error) {
	var d Details
	qid := p.Wikidata
	lang, title := splitWikipedia(p.Wikipedia)
	if qid == "" && title != "" {
		var err error
		if qid, err = w.wikidataID(ctx, lang, title); err != nil {
			return d, "", err
		}
	}

	file := p.CommonsFile
	var wikidataDesc string
	if qid != "" {
		item, err := w.entity(ctx, qid)
		if err != nil {
			return d, "", err
		}
		if file == "" {
			file = item.image
		}
		// Статья на русском важнее той, что указана в OSM на другом языке
		if item.ruTitle != "" {
			lang, title = "ru", item.ruTitle
		}
		wikidataDesc = item.description
	}

	if title != "" {
		extract, err := w.extract(ctx, lang, title)
		if err != nil {
			return d, "", err
		}
		d.Description = extract
	}
	if d.Description == "" {
		d.Description = wikidataDesc
	}
	if file != "" {
		photo, err := w.photo(ctx, file)
		if err != nil {
			return d, "", err
		}
		d.PhotoURL, d.PhotoPage, d.PhotoCredit = photo.url, photo.page, photo.credit
	}
	return d, qid, nil
}

// splitWikipedia: «ru:Самарская площадь» → ru, Самарская площадь.
func splitWikipedia(tag string) (lang, title string) {
	lang, title, ok := strings.Cut(tag, ":")
	if !ok || len(lang) < 2 || len(lang) > 12 || title == "" {
		return "", ""
	}
	return lang, title
}

func (w *Wiki) wikidataID(ctx context.Context, lang, title string) (string, error) {
	var r struct {
		Query struct {
			Pages map[string]struct {
				PageProps struct {
					Item string `json:"wikibase_item"`
				} `json:"pageprops"`
			} `json:"pages"`
		} `json:"query"`
	}
	err := w.get(ctx, w.wikipediaAPI(lang), url.Values{
		"action": {"query"}, "prop": {"pageprops"}, "ppprop": {"wikibase_item"},
		"redirects": {"1"}, "titles": {title},
	}, &r)
	if err != nil {
		return "", err
	}
	for _, p := range r.Query.Pages {
		return p.PageProps.Item, nil
	}
	return "", nil
}

type wikidataItem struct {
	image       string
	ruTitle     string
	description string
}

func (w *Wiki) entity(ctx context.Context, qid string) (wikidataItem, error) {
	var r struct {
		Entities map[string]struct {
			Claims map[string][]struct {
				MainSnak struct {
					DataValue struct {
						Value json.RawMessage `json:"value"`
					} `json:"datavalue"`
				} `json:"mainsnak"`
			} `json:"claims"`
			Sitelinks map[string]struct {
				Title string `json:"title"`
			} `json:"sitelinks"`
			Descriptions map[string]struct {
				Value string `json:"value"`
			} `json:"descriptions"`
		} `json:"entities"`
	}
	err := w.get(ctx, w.wikidataAPI, url.Values{
		"action": {"wbgetentities"}, "ids": {qid}, "props": {"claims|sitelinks|descriptions"},
		"languages": {"ru"}, "sitefilter": {"ruwiki"},
	}, &r)
	if err != nil {
		return wikidataItem{}, err
	}
	e, ok := r.Entities[qid]
	if !ok {
		return wikidataItem{}, nil
	}
	var item wikidataItem
	if images := e.Claims["P18"]; len(images) > 0 { // P18 — изображение
		_ = json.Unmarshal(images[0].MainSnak.DataValue.Value, &item.image)
	}
	item.ruTitle = e.Sitelinks["ruwiki"].Title
	item.description = e.Descriptions["ru"].Value
	return item, nil
}

func (w *Wiki) extract(ctx context.Context, lang, title string) (string, error) {
	var r struct {
		Query struct {
			Pages map[string]struct {
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	err := w.get(ctx, w.wikipediaAPI(lang), url.Values{
		"action": {"query"}, "prop": {"extracts"}, "exintro": {"1"}, "explaintext": {"1"},
		"exsentences": {fmt.Sprint(extractSentences)}, "redirects": {"1"}, "titles": {title},
	}, &r)
	if err != nil {
		return "", err
	}
	for _, p := range r.Query.Pages {
		return cleanExtract(p.Extract), nil
	}
	return "", nil
}

var (
	// Произношение и даты в скобках сразу после названия: «Собор (рус. …; 1894—1900) — …» → «Собор — …»
	leadingParens = regexp.MustCompile(`^([^(]{1,80}?)\s*\([^()]*\)\s*`)
	spaces        = regexp.MustCompile(`\s+`)
)

func cleanExtract(s string) string {
	s = spaces.ReplaceAllString(strings.TrimSpace(s), " ")
	return leadingParens.ReplaceAllString(s, "$1 ")
}

type commonsPhoto struct{ url, page, credit string }

func (w *Wiki) photo(ctx context.Context, file string) (commonsPhoto, error) {
	var r struct {
		Query struct {
			Pages map[string]struct {
				ImageInfo []struct {
					ThumbURL       string `json:"thumburl"`
					DescriptionURL string `json:"descriptionurl"`
					ExtMetadata    map[string]struct {
						Value string `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	err := w.get(ctx, w.commonsAPI, url.Values{
		"action": {"query"}, "titles": {"File:" + file}, "prop": {"imageinfo"},
		"iiprop": {"url|extmetadata"}, "iiurlwidth": {fmt.Sprint(photoWidth)},
		"iiextmetadatafilter": {"Artist|LicenseShortName"},
	}, &r)
	if err != nil {
		return commonsPhoto{}, err
	}
	for _, p := range r.Query.Pages {
		if len(p.ImageInfo) == 0 {
			continue
		}
		ii := p.ImageInfo[0]
		credit := []string{}
		if a := stripHTML(ii.ExtMetadata["Artist"].Value); a != "" {
			credit = append(credit, a)
		}
		if l := ii.ExtMetadata["LicenseShortName"].Value; l != "" {
			credit = append(credit, l)
		}
		return commonsPhoto{url: ii.ThumbURL, page: ii.DescriptionURL, credit: strings.Join(credit, " · ")}, nil
	}
	return commonsPhoto{}, nil
}

var tags = regexp.MustCompile(`<[^>]*>`)

// stripHTML: автор на Commons часто записан ссылкой на свою страницу.
func stripHTML(s string) string {
	s = html.UnescapeString(tags.ReplaceAllString(s, ""))
	return spaces.ReplaceAllString(strings.TrimSpace(s), " ")
}

func (w *Wiki) get(ctx context.Context, endpoint string, params url.Values, out any) error {
	params.Set("format", "json")
	params.Set("formatversion", "1")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	// Wikimedia требует User-Agent с контактом: https://meta.wikimedia.org/wiki/User-Agent_policy
	req.Header.Set("User-Agent", "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("wiki: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wiki %s: status %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("wiki %s: decode: %w", endpoint, err)
	}
	return nil
}
