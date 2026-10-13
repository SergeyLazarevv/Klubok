package guides

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Listing — место из статьи путеводителя (шаблон {{listing}} и его варианты {{see}}, {{eat}}…).
type Listing struct {
	Type        string // see | do | eat | drink | sleep | buy | go | other
	Name        string
	Alt         string
	Description string
	Address     string
	URL         string
	Hours       string
	Price       string
	Lat, Lon    float64
	HasLocation bool
	Wikidata    string
	ImageFile   string // файл на Commons, без «Файл:»
}

// Article — то, что берём из статьи: обзор города и места.
type Article struct {
	Overview string
	Listings []Listing
}

var listingTypes = map[string]bool{"see": true, "do": true, "eat": true, "drink": true, "sleep": true, "buy": true, "go": true}

// Разделы статьи русского Викигида → тип места, если в шаблоне тип не указан.
var sectionTypes = map[string]string{
	"достопримечательности": "see",
	"чем заняться":          "do",
	"развлечения":           "do",
	"еда":                   "eat",
	"где поесть":            "eat",
	"ночная жизнь":          "drink",
	"где остановиться":      "sleep",
	"жильё":                 "sleep",
	"покупки":               "buy",
	"как добраться":         "go",
	"транспорт":             "go",
}

const overviewMaxRunes = 700

var headingRe = regexp.MustCompile(`(?m)^==\s*([^=].*?)\s*==\s*$`)

// Parse разбирает вики-текст статьи Викигида.
func Parse(wikitext string) Article {
	var a Article
	// Разделы второго уровня: [заголовок, текст]; до первого заголовка — вступление
	locs := headingRe.FindAllStringSubmatchIndex(wikitext, -1)
	type section struct{ title, body string }
	sections := []section{{"", wikitext}}
	if len(locs) > 0 {
		sections = []section{{"", wikitext[:locs[0][0]]}}
		for i, l := range locs {
			end := len(wikitext)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			sections = append(sections, section{strings.ToLower(strings.TrimSpace(wikitext[l[2]:l[3]])), wikitext[l[1]:end]})
		}
	}

	var intro, understand string
	for _, sec := range sections {
		switch sec.title {
		case "":
			intro = plainText(sec.body)
		case "понять":
			understand = plainText(sec.body)
		}
		for _, t := range templates(sec.body) {
			if l, ok := listing(t, sectionTypes[sec.title]); ok {
				a.Listings = append(a.Listings, l)
			}
		}
	}
	a.Overview = clip(strings.TrimSpace(joinParagraphs(intro, understand)), overviewMaxRunes)
	return a
}

type template struct {
	name   string
	params map[string]string
}

// templates находит шаблоны {{…}} верхнего уровня с учётом вложенных {{…}} и [[…]].
func templates(text string) []template {
	var out []template
	for i := 0; i < len(text)-1; i++ {
		if text[i] != '{' || text[i+1] != '{' {
			continue
		}
		end := matchBraces(text, i)
		if end < 0 {
			break
		}
		parts := splitTop(text[i+2 : end])
		t := template{name: strings.ToLower(strings.TrimSpace(parts[0])), params: make(map[string]string)}
		for _, p := range parts[1:] {
			if k, v, ok := strings.Cut(p, "="); ok {
				t.params[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}
		}
		out = append(out, t)
		i = end + 1
	}
	return out
}

// matchBraces возвращает позицию закрывающих «}}» для «{{» в start; -1 — шаблон не закрыт.
func matchBraces(text string, start int) int {
	depth := 0
	for i := start; i < len(text)-1; i++ {
		switch {
		case text[i] == '{' && text[i+1] == '{':
			depth++
			i++
		case text[i] == '}' && text[i+1] == '}':
			depth--
			if depth == 0 {
				return i
			}
			i++
		}
	}
	return -1
}

// splitTop делит содержимое шаблона по «|» верхнего уровня (не внутри вложенных шаблонов и ссылок).
func splitTop(s string) []string {
	var parts []string
	depth, last := 0, 0
	for i := 0; i < len(s); i++ {
		switch {
		case i+1 < len(s) && (s[i:i+2] == "{{" || s[i:i+2] == "[["):
			depth++
			i++
		case i+1 < len(s) && (s[i:i+2] == "}}" || s[i:i+2] == "]]"):
			depth--
			i++
		case s[i] == '|' && depth == 0:
			parts = append(parts, s[last:i])
			last = i + 1
		}
	}
	return append(parts, s[last:])
}

func listing(t template, sectionType string) (Listing, bool) {
	typ := t.name
	if typ == "listing" || typ == "объект" {
		typ = strings.ToLower(t.params["type"])
		if !listingTypes[typ] {
			typ = sectionType
		}
	} else if !listingTypes[typ] {
		return Listing{}, false // не шаблон места
	}
	if typ == "" {
		typ = "other"
	}
	l := Listing{
		Type:        typ,
		Name:        plainText(t.params["name"]),
		Alt:         plainText(t.params["alt"]),
		Description: plainText(first(t.params, "content", "description")),
		Address:     plainText(t.params["address"]),
		URL:         strings.TrimSpace(t.params["url"]),
		Hours:       plainText(t.params["hours"]),
		Price:       plainText(t.params["price"]),
		Wikidata:    strings.TrimSpace(first(t.params, "wikidata", "wdid")),
		ImageFile:   imageFile(t.params["image"]),
	}
	if l.Name == "" {
		return Listing{}, false
	}
	if !strings.HasPrefix(l.URL, "http://") && !strings.HasPrefix(l.URL, "https://") {
		l.URL = ""
	}
	lat, errLat := strconv.ParseFloat(strings.TrimSpace(t.params["lat"]), 64)
	lon, errLon := strconv.ParseFloat(strings.TrimSpace(first(t.params, "long", "lon")), 64)
	if errLat == nil && errLon == nil && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 && (lat != 0 || lon != 0) {
		l.Lat, l.Lon, l.HasLocation = lat, lon, true
	}
	return l, true
}

func first(params map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := params[k]; strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func imageFile(v string) string {
	v = strings.TrimSpace(v)
	for _, prefix := range []string{"Файл:", "File:", "Изображение:", "Image:"} {
		v = strings.TrimPrefix(v, prefix)
	}
	return strings.ReplaceAll(v, "_", " ")
}

var (
	refRe      = regexp.MustCompile(`(?s)<ref[^>]*/>|<ref[^>]*>.*?</ref>`)
	commentRe  = regexp.MustCompile(`(?s)<!--.*?-->`)
	tagRe      = regexp.MustCompile(`<[^>]+>`)
	extLinkRe  = regexp.MustCompile(`\[https?://\S+\s+([^\]]+)\]`)
	bareLinkRe = regexp.MustCompile(`\[https?://[^\]]*\]`)
	spacesRe   = regexp.MustCompile(`[ \t]+`)
	blankRe    = regexp.MustCompile(`\n\s*\n+`)
)

// Пространства имён ссылок, которые в тексте не нужны: картинки, категории, межъязыковые.
var dropLinkPrefixes = []string{"файл:", "file:", "изображение:", "image:", "категория:", "category:"}

// plainText превращает вики-разметку в простой текст: без шаблонов, картинок, сносок и оформления.
func plainText(s string) string {
	s = commentRe.ReplaceAllString(s, "")
	s = refRe.ReplaceAllString(s, "")
	s = stripTemplates(s)
	s = stripLinks(s)
	s = extLinkRe.ReplaceAllString(s, "$1")
	s = bareLinkRe.ReplaceAllString(s, "")
	s = tagRe.ReplaceAllString(s, "")
	s = strings.NewReplacer("'''", "", "''", "", "&nbsp;", " ", "&mdash;", "—", "&ndash;", "–").Replace(s)
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		// Списки, таблицы и подзаголовки — не обзорный текст
		if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "{|") ||
			strings.HasPrefix(line, "|") || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "=") {
			continue
		}
		lines = append(lines, spacesRe.ReplaceAllString(line, " "))
	}
	return strings.TrimSpace(blankRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func stripTemplates(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			if end := matchBraces(s, i); end >= 0 {
				i = end + 1
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// stripLinks: [[Цель|подпись]] → подпись, [[Цель]] → Цель, [[Файл:…]] — убираем целиком.
func stripLinks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '[' && s[i+1] == '[' {
			depth, j := 0, i
			for ; j < len(s)-1; j++ {
				if s[j] == '[' && s[j+1] == '[' {
					depth++
					j++
				} else if s[j] == ']' && s[j+1] == ']' {
					depth--
					j++
					if depth == 0 {
						break
					}
				}
			}
			if depth != 0 {
				b.WriteString(s[i:])
				break
			}
			inner := s[i+2 : j-1]
			lower := strings.ToLower(inner)
			drop := false
			for _, p := range dropLinkPrefixes {
				if strings.HasPrefix(lower, p) {
					drop = true
				}
			}
			if !drop {
				parts := splitTop(inner)
				b.WriteString(parts[len(parts)-1])
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func joinParagraphs(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}

// clip обрезает текст до max символов по концу предложения.
func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)[:max]
	cut := string(r)
	if i := strings.LastIndexAny(cut, ".!?"); i > len(cut)/2 {
		return cut[:i+1]
	}
	return strings.TrimSpace(cut) + "…"
}
