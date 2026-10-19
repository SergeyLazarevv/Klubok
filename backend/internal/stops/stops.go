// Package stops — города, где можно остановиться по дороге на машине (ADR-0023).
//
// Кандидаты — города карты (cities.geojson) в коридоре вдоль линии дороги от дома до места, как «поиск
// вдоль маршрута» в Google Maps. Для лучших из них крюк считается по дорогам (две table-матрицы OSRM),
// затем каждому — оценка: интерес города минус крюк и удалённость от середины пути. Если дорога длиннее
// дневного лимита, выше города, где удобно заночевать: оба участка не длиннее лимита. Выбирает пользователь.
package stops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
	"github.com/SergeyLazarevv/Klubok/backend/internal/stay"
)

const (
	minRouteKm     = 80   // короче — останавливаться по пути незачем
	endpointKm     = 25   // город у самого дома или места — это не остановка
	corridorShare  = 0.08 // ширина коридора от линии дороги — 8% её длины, в пределах corridorMinKm…corridorMaxKm
	corridorMinKm  = 25
	corridorMaxKm  = 60
	roadCandidates = 30 // сколько кандидатов проверять по дорогам
	positionBins   = 10 // путь делится на участки, в каждом — свои лучшие кандидаты
	perBin         = 2
	detourShare    = 0.2
	detourMinS     = 40 * 60 // крюк по дороге — до 20% пути, но 40 минут допустимы всегда
	resultLimit    = 6
	sameStopKm     = 30 // два предложения ближе — одно и то же место
	cacheTTL       = 7 * 24 * time.Hour
)

// Candidate — город в коридоре дороги и где он на ней: доля длины линии до ближайшей точки, 0–1.
type Candidate struct {
	City stay.MapCity
	Pos  float64
}

// Suggestion — город, где можно остановиться.
type Suggestion struct {
	City      stay.MapCity
	Detour    road.Leg // сколько добавит заезд к прямой дороге
	Position  float64  // доля пути до города по времени, 0–1
	ToHours   float64  // от дома (или прошлой ночёвки) до города
	FromHours float64  // от города до места
	Overnight bool     // дорога длиннее лимита, а здесь оба участка в лимите
	Score     float64
}

type Service struct {
	cities []stay.MapCity
	byID   map[string]stay.MapCity
	router road.Router
	stays  *stay.Service
	cache  *redis.Client // nil — без кэша
	log    *slog.Logger
}

// NewService — подбор по городам карты; в остановки идут только города России (Country пустой):
// граница посреди дороги — отдельная история со страховкой и очередями.
func NewService(cities []stay.MapCity, router road.Router, stays *stay.Service, log *slog.Logger) *Service {
	s := &Service{router: router, stays: stays, log: log, byID: make(map[string]stay.MapCity, len(cities))}
	for _, c := range cities {
		s.byID[c.ID] = c
		if c.Country == "" {
			s.cities = append(s.cities, c)
		}
	}
	return s
}

func (s *Service) UseCache(c *redis.Client) { s.cache = c }

// City — город карты по id Wikidata.
func (s *Service) City(id string) (stay.MapCity, bool) {
	c, ok := s.byID[id]
	return c, ok
}

// StayPerNight — сутки в гостинице 3* с человека по городам карты, копейки; города без цены нет в ответе.
func (s *Service) StayPerNight(ctx context.Context) (map[string]int64, error) {
	if s.stays == nil {
		return nil, nil
	}
	prices, err := s.stays.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(prices))
	for _, p := range prices {
		out[p.CityID] = p.Kop
	}
	return out, nil
}

// Suggest — лучшие города для остановки на дороге from → to; direct — сама дорога с линией.
func (s *Service) Suggest(ctx context.Context, from, to road.Point, direct road.Route, maxDriveHours float64) ([]Suggestion, error) {
	key := fmt.Sprintf("klubok:stops:v3:%.3f,%.3f:%.3f,%.3f:%.1f", from.Lat, from.Lon, to.Lat, to.Lon, maxDriveHours)
	if cached, ok := s.cached(ctx, key); ok {
		return cached, nil
	}
	cands := Candidates(s.cities, line(from, to, direct))
	back, err := s.table(ctx, to, cands)
	if err != nil {
		return nil, err
	}
	list, err := s.rate(ctx, from, cands, back, direct.Leg, maxDriveHours)
	if err != nil {
		return nil, err
	}
	out := Pick(list)
	if s.cache != nil && !direct.Approximate {
		if data, err := json.Marshal(out); err == nil {
			if err := s.cache.Set(ctx, key, data, cacheTTL).Err(); err != nil {
				s.log.WarnContext(ctx, "stops cache set", slog.Any("err", err))
			}
		}
	}
	return out, nil
}

func (s *Service) cached(ctx context.Context, key string) ([]Suggestion, bool) {
	if s.cache == nil {
		return nil, false
	}
	data, err := s.cache.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			s.log.WarnContext(ctx, "stops cache get", slog.Any("err", err))
		}
		return nil, false
	}
	var out []Suggestion
	if json.Unmarshal(data, &out) != nil {
		return nil, false
	}
	return out, true
}

// line — линия дороги; маршрутизатор её не дал — прямая.
func line(from, to road.Point, r road.Route) []road.Point {
	if r.Polyline != "" {
		if pts, err := road.DecodePolyline(r.Polyline); err == nil && len(pts) >= 2 {
			return pts
		}
	}
	return []road.Point{from, to}
}

// table — дороги от точки до кандидатов; маршрутизатор недоступен — оценка по прямой.
func (s *Service) table(ctx context.Context, from road.Point, cands []Candidate) ([]*road.Leg, error) {
	points := make([]road.Point, len(cands))
	for i, c := range cands {
		points[i] = point(c.City)
	}
	if len(points) == 0 {
		return nil, nil
	}
	legs, err := s.router.Table(ctx, from, points)
	if err != nil {
		s.log.WarnContext(ctx, "router failed, stops by straight line", slog.Any("err", err))
		legs = make([]*road.Leg, len(points))
		for i, p := range points {
			e := road.Estimate(from, p)
			legs[i] = &e
		}
	}
	return legs, nil
}

// rate — крюк и оценка каждого кандидата; back — дороги «место → кандидат»: в обе стороны по трассе
// почти одинаково, а одна матрица от места дешевле, чем по матрице от каждого города.
func (s *Service) rate(ctx context.Context, from road.Point, cands []Candidate, back []*road.Leg, direct road.Leg, maxDriveHours float64) ([]Suggestion, error) {
	fwd, err := s.table(ctx, from, cands)
	if err != nil {
		return nil, err
	}
	var out []Suggestion
	for i, c := range cands {
		if fwd[i] == nil || back[i] == nil {
			continue // проезда нет
		}
		if sg, ok := Rate(c.City, *fwd[i], *back[i], direct, maxDriveHours); ok {
			out = append(out, sg)
		}
	}
	return out, nil
}

// Candidates — города в коридоре вдоль линии дороги (8% её длины, 25–60 км), кроме тех, что у самых
// её концов. Не больше roadCandidates: сначала по perBin лучших на каждом участке пути — чтобы для ночёвки
// были города и в середине длинной дороги, а не только крупные у её концов, — затем лучшие из остальных.
func Candidates(cities []stay.MapCity, line []road.Point) []Candidate {
	total := lineKm(line)
	if total < minRouteKm {
		return nil
	}
	corridor := math.Min(math.Max(total*corridorShare, corridorMinKm), corridorMaxKm)
	type cand struct {
		Candidate
		score float64
		bin   int
	}
	var list []cand
	for _, c := range cities {
		off, along := nearest(line, point(c))
		if off > corridor || along < endpointKm || total-along < endpointKm {
			continue
		}
		pos := along / total
		list = append(list, cand{Candidate{c, pos}, Interest(c) - off/corridor, min(int(pos*positionBins), positionBins-1)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].score > list[j].score })
	out := make([]Candidate, 0, min(len(list), roadCandidates))
	taken := make([]bool, len(list))
	inBin := make([]int, positionBins)
	for i, x := range list {
		if inBin[x.bin] < perBin && len(out) < roadCandidates {
			inBin[x.bin]++
			taken[i] = true
			out = append(out, x.Candidate)
		}
	}
	for i, x := range list {
		if !taken[i] && len(out) < roadCandidates {
			out = append(out, x.Candidate)
		}
	}
	return out
}

// nearest — расстояние от точки до линии и сколько километров по линии до ближайшей её точки.
// Каждый отрезок — в локальной плоской проекции: для отрезков в десятки километров ошибка мала.
func nearest(line []road.Point, p road.Point) (offKm, alongKm float64) {
	offKm = math.Inf(1)
	var walked float64
	for i := 1; i < len(line); i++ {
		a, b := line[i-1], line[i]
		kx := 111.32 * math.Cos((a.Lat+b.Lat)/2*math.Pi/180) // км в градусе долготы
		const ky = 110.57                                    // км в градусе широты
		bx, by := (b.Lon-a.Lon)*kx, (b.Lat-a.Lat)*ky
		px, py := (p.Lon-a.Lon)*kx, (p.Lat-a.Lat)*ky
		seg := math.Hypot(bx, by)
		t := 0.0
		if seg > 0 {
			t = math.Max(0, math.Min(1, (px*bx+py*by)/(seg*seg)))
		}
		if d := math.Hypot(px-t*bx, py-t*by); d < offKm {
			offKm, alongKm = d, walked+t*seg
		}
		walked += seg
	}
	return offKm, alongKm
}

func lineKm(line []road.Point) float64 {
	var km float64
	for i := 1; i < len(line); i++ {
		km += road.StraightKm(line[i-1], line[i])
	}
	return km
}

func point(c stay.MapCity) road.Point { return road.Point{Lat: c.Lat, Lon: c.Lon} }

// Interest — насколько город интересен как остановка: размер (логарифм населения) и то, что о нём есть
// что показать — герб, фото, главная достопримечательность. Казань ≈ 6,9, райцентр на 10 тыс. ≈ 4,5.
func Interest(c stay.MapCity) float64 {
	v := math.Log10(math.Max(float64(c.Pop), 1000))
	if c.Arms {
		v += 0.3
	}
	if c.Photo != "" {
		v += 0.3
	}
	if c.Landmark != "" {
		v += 0.4
	}
	return v
}

// Rate — крюк и оценка города по дорогам; ok = false — крюк слишком большой.
func Rate(c stay.MapCity, toCity, fromCity, direct road.Leg, maxDriveHours float64) (Suggestion, bool) {
	detour := road.Leg{
		DistanceM:   max(toCity.DistanceM+fromCity.DistanceM-direct.DistanceM, 0),
		DurationS:   max(toCity.DurationS+fromCity.DurationS-direct.DurationS, 0),
		Approximate: toCity.Approximate || fromCity.Approximate || direct.Approximate,
	}
	if float64(detour.DurationS) > math.Max(detourMinS, float64(direct.DurationS)*detourShare) {
		return Suggestion{}, false
	}
	sg := Suggestion{
		City:      c,
		Detour:    detour,
		Position:  float64(toCity.DurationS) / float64(max(toCity.DurationS+fromCity.DurationS, 1)),
		ToHours:   float64(toCity.DurationS) / 3600,
		FromHours: float64(fromCity.DurationS) / 3600,
	}
	// Подсказка «удобно заночевать»: дорога длиннее лимита, а до города и от него — в лимите. Решает пользователь
	if maxDriveHours > 0 && float64(direct.DurationS)/3600 > maxDriveHours {
		sg.Overnight = sg.ToHours <= maxDriveHours && sg.FromHours <= maxDriveHours
	}
	// Полчаса крюка стоят как разница между крупным городом и вдесятеро меньшим; лучше — ближе к середине
	sg.Score = Interest(c) - float64(detour.DurationS)/1800 - 2*math.Abs(sg.Position-0.5)
	if sg.Overnight {
		sg.Score += 3 // на длинной дороге сначала то, где можно заночевать
	}
	return sg, true
}

// Pick — лучшие предложения без повторов: из городов ближе sameStopKm друг к другу — один, с лучшей оценкой.
func Pick(list []Suggestion) []Suggestion {
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	var out []Suggestion
	for _, sg := range list {
		dup := false
		for _, o := range out {
			if road.StraightKm(point(sg.City), point(o.City)) < sameStopKm {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, sg)
		}
		if len(out) == resultLimit {
			break
		}
	}
	return out
}
