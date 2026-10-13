package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/fares"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
	"github.com/SergeyLazarevv/Klubok/backend/internal/road"
)

// homeAirportsRadiusKm — в каком радиусе искать аэропорты своего города, когда машины нет.
const homeAirportsRadiusKm = 80

// staleAfter — цена старше этого считается устаревшей и помечается предупреждением.
const staleAfter = 3 * 24 * time.Hour

// fetchConcurrency — сколько запросов к источнику цен идёт одновременно.
const fetchConcurrency = 4

var ErrSameCity = errors.New("destination is the home city")

type Warning string

const (
	WarnBorder           Warning = "border_crossing_by_car"
	WarnStalePrice       Warning = "stale_price"
	WarnParkingEstimated Warning = "parking_estimated"
	WarnRoadApproximate  Warning = "road_approximate"
)

// TripsQuery — поиск поездки туда и обратно: направление, месяц вылета, длительность.
type TripsQuery struct {
	HomeID        uuid.UUID
	DestinationID uuid.UUID
	Month         time.Time
	MinDays       int
	MaxDays       int
	Travelers     int
	UseCar        bool
	Car           CarParams
}

// TripOption — лучший вариант поездки через конкретный аэропорт вылета.
type TripOption struct {
	Hub       reachable
	Arrival   *geo.Place // аэропорт прилёта; nil — его нет в справочнике
	ByCar     bool
	Quote     fares.Quote
	TripDays  int
	Car       carcost.Cost // нули, если без машины
	Tickets   carcost.Kop  // на всех
	Total     carcost.Kop
	PerPerson carcost.Kop
	Savings   *carcost.Kop // относительно лучшего вылета из своего города; nil — сравнить не с чем
	Warnings  []Warning
}

type TripsResult struct {
	Home        geo.Place
	Destination geo.Place
	FuelPrice   carcost.Kop // 0 — без машины
	Options     []TripOption
	HomeBest    *TripOption
	NoData      []geo.Place // хабы, по которым у источника нет цен
	Partial     bool        // часть запросов к источнику не удалась

	Drive         *DriveOption // поездка целиком на машине; nil — без машины или не подходит (DriveSkip)
	DriveSkip     DriveSkip
	HiddenFlights int // перелёты, которые не дешевле и не заметно быстрее машины
}

// Trips ищет самые дешёвые поездки с учётом дороги до соседних аэропортов.
func (s *Service) Trips(ctx context.Context, q TripsQuery) (TripsResult, error) {
	home, err := s.geo.Get(ctx, q.HomeID)
	if err != nil {
		return TripsResult{}, err
	}
	dest, err := s.geo.Get(ctx, q.DestinationID)
	if err != nil {
		return TripsResult{}, err
	}
	destCity := cityCode(dest)
	if destCity == cityCode(home) {
		return TripsResult{}, ErrSameCity
	}
	res := TripsResult{Home: home, Destination: dest}

	var hubs []reachable
	if q.UseCar {
		if res.FuelPrice, _, err = s.fuelPrice(ctx, home, q.Car); err != nil {
			return TripsResult{}, err
		}
		if hubs, err = s.reachableByCar(ctx, home, q.Car); err != nil {
			return TripsResult{}, err
		}
	} else {
		if hubs, err = s.homeAirports(ctx, home); err != nil {
			return TripsResult{}, err
		}
	}
	hubs = filter(hubs, func(r reachable) bool { return cityCode(r.Hub.Place) != destCity })

	if q.UseCar {
		if res.Drive, res.DriveSkip, err = s.drive(ctx, home, dest, q, res.FuelPrice); err != nil {
			return TripsResult{}, err
		}
	}

	quotes, partial, err := s.fetchQuotes(ctx, hubs, destCity, q.Month)
	if err != nil {
		if res.Drive == nil {
			return TripsResult{}, err
		}
		// Цены на билеты недоступны, но доехать можно — показываем машину
		s.log.WarnContext(ctx, "fares unavailable, drive only", slog.Any("err", err))
		quotes, partial = nil, true
	}
	res.Partial = partial

	best := make(map[uuid.UUID]TripOption)
	for _, r := range hubs {
		for _, quote := range quotes[r.Hub.Place.IATA] {
			days := quote.TripDays()
			if days < q.MinDays || days > q.MaxDays || !quote.DepartAt.After(time.Now()) {
				continue
			}
			opt := s.option(r, q, res.FuelPrice, quote, days)
			if cur, ok := best[r.Hub.Place.ID]; !ok || opt.Total < cur.Total {
				best[r.Hub.Place.ID] = opt
			}
		}
	}
	if err := s.resolveArrivals(ctx, best); err != nil {
		return TripsResult{}, err
	}
	for _, r := range hubs {
		opt, ok := best[r.Hub.Place.ID]
		if !ok {
			res.NoData = append(res.NoData, r.Hub.Place)
			continue
		}
		res.Options = append(res.Options, opt)
		if cityCode(r.Hub.Place) == cityCode(home) && (res.HomeBest == nil || opt.Total < res.HomeBest.Total) {
			o := opt
			res.HomeBest = &o
		}
	}
	// Можно доехать на машине — перелёт предлагаем, только если он дешевле или заметно быстрее
	if hideUnworthyFlights && res.Drive != nil {
		kept := res.Options[:0]
		for _, o := range res.Options {
			if worthFlying(o, res.Drive) {
				kept = append(kept, o)
			} else {
				res.HiddenFlights++
			}
		}
		res.Options = kept
	}
	sort.SliceStable(res.Options, func(i, j int) bool { return res.Options[i].Total < res.Options[j].Total })
	if res.HomeBest != nil {
		for i := range res.Options {
			diff := res.HomeBest.Total - res.Options[i].Total
			res.Options[i].Savings = &diff
		}
	}
	return res, nil
}

func (s *Service) option(r reachable, q TripsQuery, fuelPrice carcost.Kop, quote fares.Quote, days int) TripOption {
	opt := TripOption{Hub: r, ByCar: q.UseCar, Quote: quote, TripDays: days}
	if q.UseCar {
		opt.Car = r.carCost(fuelPrice, q.Car.ConsumptionL100, days)
		if r.Border != nil {
			opt.Warnings = append(opt.Warnings, WarnBorder)
		}
		if r.ParkingEstimated {
			opt.Warnings = append(opt.Warnings, WarnParkingEstimated)
		}
		if r.Leg.Approximate {
			opt.Warnings = append(opt.Warnings, WarnRoadApproximate)
		}
	}
	if !quote.FoundOn.IsZero() && time.Since(quote.FoundOn) > staleAfter {
		opt.Warnings = append(opt.Warnings, WarnStalePrice)
	}
	travelers := carcost.Kop(max(q.Travelers, 1))
	opt.Tickets = quote.Price * travelers
	opt.Total = opt.Tickets + opt.Car.Total
	opt.PerPerson = opt.Total / travelers
	return opt
}

// resolveArrivals подставляет аэропорты прилёта по кодам из котировок — для карты.
func (s *Service) resolveArrivals(ctx context.Context, opts map[uuid.UUID]TripOption) error {
	if len(opts) == 0 {
		return nil
	}
	codes := make([]string, 0, len(opts))
	for _, o := range opts {
		codes = append(codes, o.Quote.Destination)
	}
	airports, err := s.geo.AirportsByIATA(ctx, codes)
	if err != nil {
		return err
	}
	for id, o := range opts {
		if a, ok := airports[o.Quote.Destination]; ok {
			o.Arrival = &a
			opts[id] = o
		}
	}
	return nil
}

// RoadRoute — путь на машине между точками справочника с линией для карты.
func (s *Service) RoadRoute(ctx context.Context, fromID, toID uuid.UUID) (road.Route, error) {
	from, err := s.geo.Get(ctx, fromID)
	if err != nil {
		return road.Route{}, err
	}
	to, err := s.geo.Get(ctx, toID)
	if err != nil {
		return road.Route{}, err
	}
	return s.road.Route(ctx, roadPoint(from), roadPoint(to))
}

// homeAirports — аэропорты своего города, когда машины нет.
func (s *Service) homeAirports(ctx context.Context, home geo.Place) ([]reachable, error) {
	hubs, err := s.geo.AirportsWithin(ctx, home.ID, homeAirportsRadiusKm)
	if err != nil {
		return nil, err
	}
	var out []reachable
	for _, h := range hubs {
		if cityCode(h.Place) == cityCode(home) {
			out = append(out, reachable{Hub: h})
		}
	}
	return out, nil
}

// fetchQuotes запрашивает цены по каждому городу вылета (у города может быть несколько аэропортов)
// и раскладывает их по IATA аэропорта. partial — часть запросов не удалась.
func (s *Service) fetchQuotes(ctx context.Context, hubs []reachable, destCity string, month time.Time) (map[string][]fares.Quote, bool, error) {
	cities := make(map[string]struct{})
	for _, r := range hubs {
		cities[cityCode(r.Hub.Place)] = struct{}{}
	}

	var (
		mu       sync.Mutex
		byOrigin = make(map[string][]fares.Quote)
		failed   int
		lastErr  error
	)
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(fetchConcurrency)
	for city := range cities {
		g.Go(func() error {
			quotes, err := s.fares.RoundTrips(gctx, fares.RoundTripQuery{Origin: city, Destination: destCity, Month: month})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				lastErr = err
				s.log.WarnContext(ctx, "fares request failed", slog.String("origin", city), slog.Any("err", err))
				return nil // один город не должен ронять весь поиск
			}
			for _, q := range quotes {
				byOrigin[q.Origin] = append(byOrigin[q.Origin], q)
			}
			if err := s.history.Save(gctx, quotes); err != nil {
				s.log.WarnContext(ctx, "save price history", slog.Any("err", err))
			}
			return nil
		})
	}
	_ = g.Wait()
	if len(cities) > 0 && failed == len(cities) {
		return nil, false, fmt.Errorf("all fare requests failed: %w", lastErr)
	}
	return byOrigin, failed > 0, nil
}

// cityCode — код города точки: для аэропорта — его город, для города — он сам.
func cityCode(p geo.Place) string {
	if p.Kind == geo.KindAirport && p.CityIATA != "" {
		return p.CityIATA
	}
	return p.IATA
}
