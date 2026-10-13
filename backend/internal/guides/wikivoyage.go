package guides

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Wikivoyage — клиент MediaWiki API русского Викигида и Wikimedia Commons (фото мест).
type Wikivoyage struct {
	client     *http.Client
	api        string // https://ru.wikivoyage.org/w/api.php
	commonsAPI string
	pageBase   string // https://ru.wikivoyage.org/wiki/
}

func NewWikivoyage() *Wikivoyage {
	return &Wikivoyage{
		client:     &http.Client{Timeout: 20 * time.Second},
		api:        "https://ru.wikivoyage.org/w/api.php",
		commonsAPI: "https://commons.wikimedia.org/w/api.php",
		pageBase:   "https://ru.wikivoyage.org/wiki/",
	}
}

// Page — найденная статья о городе.
type Page struct {
	Title    string
	URL      string
	Revision int64
}

// Photo — уменьшенная копия файла с Commons с автором и лицензией.
type Photo struct{ URL, Page, Credit string }

// maxCityDistanceKm — статья с координатами дальше этого от города — о другом месте с тем же названием.
const maxCityDistanceKm = 60

// FindPage ищет статью о городе по названию: «Уральск», затем «Уральск (город)». nil — статьи нет.
func (w *Wikivoyage) FindPage(ctx context.Context, name string, lat, lon float64) (*Page, error) {
	for _, title := range []string{name, name + " (город)"} {
		var r struct {
			Query struct {
				Pages []struct {
					Title     string `json:"title"`
					Missing   bool   `json:"missing"`
					Revisions []struct {
						RevID int64 `json:"revid"`
					} `json:"revisions"`
					Coordinates []struct {
						Lat float64 `json:"lat"`
						Lon float64 `json:"lon"`
					} `json:"coordinates"`
					PageProps map[string]string `json:"pageprops"`
				} `json:"pages"`
			} `json:"query"`
		}
		err := w.get(ctx, w.api, url.Values{
			"action": {"query"}, "titles": {title}, "redirects": {"1"},
			"prop": {"revisions|coordinates|pageprops"}, "rvprop": {"ids"}, "ppprop": {"disambiguation"},
		}, &r)
		if err != nil {
			return nil, err
		}
		if len(r.Query.Pages) == 0 {
			continue
		}
		p := r.Query.Pages[0]
		if p.Missing || len(p.Revisions) == 0 {
			continue
		}
		if _, disambig := p.PageProps["disambiguation"]; disambig {
			continue
		}
		if len(p.Coordinates) > 0 && distanceKm(lat, lon, p.Coordinates[0].Lat, p.Coordinates[0].Lon) > maxCityDistanceKm {
			continue
		}
		return &Page{
			Title:    p.Title,
			URL:      w.pageBase + url.PathEscape(strings.ReplaceAll(p.Title, " ", "_")),
			Revision: p.Revisions[0].RevID,
		}, nil
	}
	return nil, nil
}

// Wikitext возвращает текст статьи в вики-разметке.
func (w *Wikivoyage) Wikitext(ctx context.Context, title string) (string, error) {
	var r struct {
		Parse struct {
			Wikitext string `json:"wikitext"`
		} `json:"parse"`
	}
	if err := w.get(ctx, w.api, url.Values{"action": {"parse"}, "page": {title}, "prop": {"wikitext"}}, &r); err != nil {
		return "", err
	}
	return r.Parse.Wikitext, nil
}

// commonsBatch — сколько файлов Commons отдаёт за один запрос.
const commonsBatch = 50

// Photos — уменьшенные копии файлов Commons с автором и лицензией. Не найденные файлы в ответ не попадают.
func (w *Wikivoyage) Photos(ctx context.Context, files []string) (map[string]Photo, error) {
	out := make(map[string]Photo, len(files))
	for start := 0; start < len(files); start += commonsBatch {
		batch := files[start:min(start+commonsBatch, len(files))]
		titles := make([]string, len(batch))
		for i, f := range batch {
			titles[i] = "File:" + f
		}
		var r struct {
			Query struct {
				Normalized []struct{ From, To string } `json:"normalized"`
				Pages      []struct {
					Title     string `json:"title"`
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
			"action": {"query"}, "titles": {strings.Join(titles, "|")}, "prop": {"imageinfo"},
			"iiprop": {"url|extmetadata"}, "iiurlwidth": {"640"}, "iiextmetadatafilter": {"Artist|LicenseShortName"},
		}, &r)
		if err != nil {
			return nil, err
		}
		// Commons приводит названия к своему виду («File:a_b.jpg» → «File:A b.jpg») — сопоставляем обратно
		requested := make(map[string]string, len(batch))
		for _, f := range batch {
			requested["File:"+f] = f
		}
		for _, n := range r.Query.Normalized {
			if f, ok := requested[n.From]; ok {
				requested[n.To] = f
			}
		}
		for _, p := range r.Query.Pages {
			f, ok := requested[p.Title]
			if !ok || len(p.ImageInfo) == 0 || p.ImageInfo[0].ThumbURL == "" {
				continue
			}
			ii := p.ImageInfo[0]
			var credit []string
			if a := stripHTML(ii.ExtMetadata["Artist"].Value); a != "" {
				credit = append(credit, a)
			}
			if l := ii.ExtMetadata["LicenseShortName"].Value; l != "" {
				credit = append(credit, l)
			}
			out[f] = Photo{URL: ii.ThumbURL, Page: ii.DescriptionURL, Credit: strings.Join(credit, " · ")}
		}
	}
	return out, nil
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(htmlTagRe.ReplaceAllString(s, ""))), " ")
}

func (w *Wikivoyage) get(ctx context.Context, endpoint string, params url.Values, out any) error {
	params.Set("format", "json")
	params.Set("formatversion", "2")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	// Wikimedia требует User-Agent с контактом: https://meta.wikimedia.org/wiki/User-Agent_policy
	req.Header.Set("User-Agent", "Klubok/0.1 (+https://github.com/SergeyLazarevv/Klubok)")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("wikivoyage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wikivoyage %s: status %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("wikivoyage %s: decode: %w", endpoint, err)
	}
	return nil
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
