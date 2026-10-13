package osmimport

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"
)

// airportIndex — аэропорты по ячейкам сетки: парковки берём только в 3 км от аэропорта.
type airportIndex map[[2]int][][2]float64

const (
	airportCellDeg  = 0.05 // ~5 км по широте
	parkingRadiusKm = 3.0
)

func loadAirports(ctx context.Context, pool *pgxpool.Pool) (airportIndex, error) {
	rows, err := pool.Query(ctx, `SELECT ST_Y(location::geometry), ST_X(location::geometry) FROM places WHERE kind = 'airport'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := airportIndex{}
	for rows.Next() {
		var lat, lon float64
		if err := rows.Scan(&lat, &lon); err != nil {
			return nil, err
		}
		k := cell(lat, lon)
		idx[k] = append(idx[k], [2]float64{lat, lon})
	}
	return idx, rows.Err()
}

func (idx airportIndex) near(lat, lon float64) bool {
	c := cell(lat, lon)
	// по долготе ячейка уже по мере движения к полюсу — проверяем соседей с запасом
	span := int(math.Ceil(1 / math.Max(math.Cos(lat*math.Pi/180), 0.2)))
	for dy := -1; dy <= 1; dy++ {
		for dx := -span; dx <= span; dx++ {
			for _, a := range idx[[2]int{c[0] + dy, c[1] + dx}] {
				if haversineKm(lat, lon, a[0], a[1]) <= parkingRadiusKm {
					return true
				}
			}
		}
	}
	return false
}

func cell(lat, lon float64) [2]int {
	return [2]int{int(math.Floor(lat / airportCellDeg)), int(math.Floor(lon / airportCellDeg))}
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}
