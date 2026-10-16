package search

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/SergeyLazarevv/Klubok/backend/internal/carcost"
	"github.com/SergeyLazarevv/Klubok/backend/internal/geo"
)

// ExploreDestination — самый дешёвый способ улететь в город за выбранные даты.
type ExploreDestination struct {
	City     geo.Place
	Best     TripOption   // дешевле всего по полной стоимости среди перелётов, которые имеет смысл предлагать
	HomeBest *TripOption  // лучший вылет из своего города; nil — из своего города цен нет
	Drive    *DriveOption // поездка целиком на машине; nil — без машины или не доехать за отведённое время
}

type ExploreResult struct {
	Home         geo.Place
	FuelPrice    carcost.Kop // 0 — без машины
	Destinations []ExploreDestination
	Partial      bool // часть запросов к источнику не удалась
	// DriveBetter — города, куда перелёты есть, но на машине и дешевле, и не намного дольше (см. worthFlying)
	DriveBetter int
	// BorderRules — правила въезда на машине из страны дома (по стране назначения), если разрешены
	// поездки за границу: клиент считает по ним машину до городов соседних стран
	BorderRules map[string]carcost.BorderRule
}

// Explore — «куда угодно»: самые дешёвые поездки из дома во все города, по которым у источника есть цены.
// С машиной перелёт в город предлагается, только если он дешевле поездки на машине или заметно быстрее
// (то же правило, что worthFlying в обычном поиске); иначе город уходит в DriveBetter.
// Источник отдаёт все направления из города одним запросом на месяц, поэтому запросов столько же,
// сколько в обычном поиске: города вылета × месяцы. DestinationID в запросе не используется.
func (s *Service) Explore(ctx context.Context, q TripsQuery) (ExploreResult, error) {
	home, err := s.geo.Get(ctx, q.HomeID)
	if err != nil {
		return ExploreResult{}, err
	}
	res := ExploreResult{Home: home}

	var hubs []reachable
	if q.UseCar {
		if res.FuelPrice, _, err = s.fuelPrice(ctx, home, q.Car); err != nil {
			return ExploreResult{}, err
		}
		if hubs, err = s.reachableByCar(ctx, home, q.Car); err != nil {
			return ExploreResult{}, err
		}
		if q.Car.AllowBorder {
			if res.BorderRules, err = s.costs.BorderRules(ctx, home.CountryCode); err != nil {
				return ExploreResult{}, err
			}
		}
	} else if hubs, err = s.homeAirports(ctx, home); err != nil {
		return ExploreResult{}, err
	}

	quotes, partial, err := s.fetchQuotes(ctx, hubs, "", q.DateFrom, q.DateTo)
	if err != nil {
		return ExploreResult{}, err
	}
	res.Partial = partial

	// Лучший вариант через каждый аэропорт вылета — по каждому городу прилёта
	homeCity := cityCode(home)
	type cityOptions struct {
		byHub    map[string]TripOption
		homeBest *TripOption
	}
	byCity := make(map[string]*cityOptions)
	for _, r := range hubs {
		hubCity := cityCode(r.Hub.Place)
		for _, quote := range quotes[r.Hub.Place.IATA] {
			dest := quote.DestinationCity
			if dest == "" || dest == homeCity || dest == hubCity {
				continue
			}
			days, ok := fits(quote, q)
			if !ok {
				continue
			}
			opt := s.option(r, q, res.FuelPrice, quote, days)
			c := byCity[dest]
			if c == nil {
				c = &cityOptions{byHub: make(map[string]TripOption)}
				byCity[dest] = c
			}
			if cur, ok := c.byHub[r.Hub.Place.IATA]; !ok || opt.Total < cur.Total {
				c.byHub[r.Hub.Place.IATA] = opt
			}
			if hubCity == homeCity && (c.homeBest == nil || opt.Total < c.homeBest.Total) {
				o := opt
				c.homeBest = &o
			}
		}
	}

	codes := make([]string, 0, len(byCity))
	for code := range byCity {
		codes = append(codes, code)
	}
	cities, err := s.geo.CitiesByIATA(ctx, codes)
	if err != nil {
		return ExploreResult{}, err
	}

	// С машиной — дорога до всех городов разом: перелёт, который не дешевле и не заметно быстрее машины,
	// не предлагаем (лететь в Ульяновск из Екатеринбурга, когда до Ульяновска 4 часа на машине)
	var drives map[uuid.UUID]*DriveOption
	if q.UseCar {
		list := make([]geo.Place, 0, len(cities))
		for _, c := range cities {
			list = append(list, c)
		}
		if drives, _, err = s.drives(ctx, home, list, q, res.FuelPrice); err != nil {
			return ExploreResult{}, err
		}
	}

	for code, c := range byCity {
		city, ok := cities[code]
		if !ok {
			continue // города нет в справочнике — не на чем показать на карте
		}
		drive := drives[city.ID]
		var best *TripOption
		for _, o := range c.byHub {
			if !worthFlying(o, drive) {
				continue
			}
			if best == nil || o.Total < best.Total || (o.Total == best.Total && o.Hub.Hub.Place.IATA < best.Hub.Hub.Place.IATA) {
				b := o
				best = &b
			}
		}
		if best == nil {
			res.DriveBetter++
			continue
		}
		if c.homeBest != nil {
			diff := c.homeBest.Total - best.Total
			best.Savings = &diff
		}
		res.Destinations = append(res.Destinations, ExploreDestination{City: city, Best: *best, HomeBest: c.homeBest, Drive: drive})
	}
	sort.Slice(res.Destinations, func(i, j int) bool {
		a, b := res.Destinations[i], res.Destinations[j]
		if a.Best.Total != b.Best.Total {
			return a.Best.Total < b.Best.Total
		}
		return a.City.IATA < b.City.IATA
	})
	return res, nil
}
