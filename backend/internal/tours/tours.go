// Package tours — горящие туры из ближайшего к пользователю города вылета (ADR-0027).
//
// Сейчас источник один — ссылки на поиск Level.Travel без цен (LevelTravelLinks): API туров
// Level.Travel выдаёт партнёрам по заявке. Когда доступ появится, его адаптер реализует Provider
// и вернёт предложения с ценами — остальная цепочка (сервис, API, интерфейс) не меняется.
package tours

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"
)

// Departure — город вылета туров.
type Departure struct {
	Slug     string // как город называется в адресах Level.Travel: Kazan
	Name     string // Казань
	NameFrom string // «из Казани»
	Lat, Lon float64
}

// Destination — направление туров: страна и месяцы, когда туда обычно едут на отдых.
type Destination struct {
	Country string // код ISO 3166-1: TR
	Name    string // Турция
	Nights  int
	Months  []time.Month // пусто — круглый год
}

func (d Destination) InSeason(m time.Month) bool {
	if len(d.Months) == 0 {
		return true
	}
	for _, x := range d.Months {
		if x == m {
			return true
		}
	}
	return false
}

// Offer — тур или ссылка на поиск туров по направлению.
type Offer struct {
	Destination Destination
	Departure   time.Time
	Nights      int
	URL         string
	PriceKop    int64 // 0 — цены нет (только ссылка на поиск)
}

// Query — что ищем: откуда, на сколько человек, относительно какого дня.
type Query struct {
	From      Departure
	Travelers int
	Today     time.Time
}

// Provider — источник горящих туров.
type Provider interface {
	Hot(ctx context.Context, q Query) ([]Offer, error)
	// AllURL — страница всех горящих туров из города вылета.
	AllURL(from Departure) string
}

// PartnerLinker превращает ссылку на сайт бренда в партнёрскую (Travelpayouts).
type PartnerLinker interface {
	PartnerLink(ctx context.Context, link string) (string, error)
}

// Service выбирает город вылета и отдаёт горящие туры из него.
type Service struct {
	provider   Provider
	departures []Departure
	linker     PartnerLinker // nil — ссылки без партнёрской метки
	log        *slog.Logger

	mu    sync.Mutex
	links map[string]string // ссылка бренда → партнёрская
}

func NewService(provider Provider, departures []Departure, linker PartnerLinker, log *slog.Logger) *Service {
	return &Service{provider: provider, departures: departures, linker: linker, log: log, links: map[string]string{}}
}

// Nearest — ближайший к точке город вылета и расстояние до него по прямой, км.
func (s *Service) Nearest(lat, lon float64) (Departure, float64) {
	var best Departure
	bestKm := math.Inf(1)
	for _, d := range s.departures {
		if km := distanceKm(lat, lon, d.Lat, d.Lon); km < bestKm {
			best, bestKm = d, km
		}
	}
	return best, bestKm
}

// Hot — горящие туры из города вылета.
func (s *Service) Hot(ctx context.Context, q Query) ([]Offer, error) {
	return s.provider.Hot(ctx, q)
}

// AllURL — страница всех горящих туров из города вылета.
func (s *Service) AllURL(from Departure) string {
	return s.provider.AllURL(from)
}

// Partner — партнёрская ссылка вместо ссылки бренда; не вышло — исходная ссылка и false.
func (s *Service) Partner(ctx context.Context, link string) (string, bool) {
	if s.linker == nil {
		return link, false
	}
	s.mu.Lock()
	cached, ok := s.links[link]
	s.mu.Unlock()
	if ok {
		return cached, true
	}
	pl, err := s.linker.PartnerLink(ctx, link)
	if err != nil {
		s.log.WarnContext(ctx, "tours partner link", slog.String("link", link), slog.Any("err", err))
		return link, false
	}
	s.mu.Lock()
	if len(s.links) > 10000 { // ссылки с датами меняются каждый день — не копим их бесконечно
		clear(s.links)
	}
	s.links[link] = pl
	s.mu.Unlock()
	return pl, true
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
