package osmimport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Geofabrik — выгрузки OpenStreetMap по странам и ежедневные дифы к ним (https://download.geofabrik.de).
type Geofabrik struct {
	baseURL string
	client  *http.Client
}

func NewGeofabrik(baseURL string) *Geofabrik {
	return &Geofabrik{
		baseURL: strings.TrimRight(baseURL, "/"),
		// Выгрузка России — 4 ГБ: общий таймаут не ставим, следим за контекстом
		client: &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: time.Minute}},
	}
}

// RemoteSequence — номер последнего опубликованного дифа региона.
func (g *Geofabrik) RemoteSequence(ctx context.Context, region string) (int, error) {
	body, err := g.get(ctx, g.baseURL+"/"+region+"-updates/state.txt")
	if err != nil {
		return 0, err
	}
	defer body.Close()
	return parseState(body)
}

// DownloadExtract скачивает выгрузку региона целиком.
func (g *Geofabrik) DownloadExtract(ctx context.Context, region, dst string) error {
	return g.download(ctx, g.baseURL+"/"+region+"-latest.osm.pbf", dst)
}

// DownloadDiff скачивает дневной диф с номером seq.
func (g *Geofabrik) DownloadDiff(ctx context.Context, region string, seq int, dst string) error {
	return g.download(ctx, g.baseURL+"/"+region+"-updates/"+sequencePath(seq)+".osc.gz", dst)
}

// Poly возвращает границу региона в формате WKT MULTIPOLYGON.
func (g *Geofabrik) Poly(ctx context.Context, region string) (string, error) {
	body, err := g.get(ctx, g.baseURL+"/"+region+".poly")
	if err != nil {
		return "", err
	}
	defer body.Close()
	return parsePoly(body)
}

// sequencePath: 4918 → 000/004/918
func sequencePath(seq int) string {
	s := fmt.Sprintf("%09d", seq)
	return s[0:3] + "/" + s[3:6] + "/" + s[6:9]
}

func parseState(r io.Reader) (int, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "sequenceNumber="); ok {
			return strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("state.txt: no sequenceNumber")
}

// parsePoly переводит формат .poly (Osmosis) в WKT. Кольца с «!» в начале — дыры в предыдущем полигоне.
func parsePoly(r io.Reader) (string, error) {
	type ring []string
	var (
		polys   [][]ring // полигон = внешнее кольцо + дыры
		current ring
		inRing  bool
		hole    bool
	)
	sc := bufio.NewScanner(r)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case first: // имя файла
			first = false
		case line == "END" && inRing:
			if len(current) > 0 && current[0] != current[len(current)-1] {
				current = append(current, current[0]) // кольцо должно быть замкнуто
			}
			if hole && len(polys) > 0 {
				polys[len(polys)-1] = append(polys[len(polys)-1], current)
			} else {
				polys = append(polys, []ring{current})
			}
			inRing, current = false, nil
		case line == "END":
			// конец файла
		case !inRing && line != "":
			inRing, hole, current = true, strings.HasPrefix(line, "!"), nil
		case inRing && line != "":
			f := strings.Fields(line)
			if len(f) != 2 {
				return "", fmt.Errorf("poly: bad coordinate line %q", line)
			}
			current = append(current, f[0]+" "+f[1])
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	if len(polys) == 0 {
		return "", fmt.Errorf("poly: no polygons")
	}
	var b strings.Builder
	b.WriteString("MULTIPOLYGON(")
	for i, p := range polys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('(')
		for j, rg := range p {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteString("(" + strings.Join(rg, ",") + ")")
		}
		b.WriteByte(')')
	}
	b.WriteByte(')')
	return b.String(), nil
}

func (g *Geofabrik) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geofabrik: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("geofabrik: %s: status %d", url, resp.StatusCode)
	}
	return resp.Body, nil
}

// download пишет во временный файл и переименовывает: оборванная загрузка не оставит битый файл.
func (g *Geofabrik) download(ctx context.Context, url, dst string) error {
	body, err := g.get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".part-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, body); err != nil {
		tmp.Close()
		return fmt.Errorf("geofabrik: download %s: %w", url, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
