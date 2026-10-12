package road

import (
	"errors"
	"math"
	"strings"
)

// polylinePrecision — 6 знаков после запятой, как polyline6 в OSRM.
const polylinePrecision = 1e6

// EncodePolyline кодирует точки в формат Encoded Polyline
// (https://developers.google.com/maps/documentation/utilities/polylinealgorithm).
func EncodePolyline(points []Point) string {
	return encodePolyline(points, polylinePrecision)
}

func encodePolyline(points []Point, precision float64) string {
	var b strings.Builder
	var prevLat, prevLon int64
	for _, p := range points {
		lat := int64(math.Round(p.Lat * precision))
		lon := int64(math.Round(p.Lon * precision))
		writePolylineValue(&b, lat-prevLat)
		writePolylineValue(&b, lon-prevLon)
		prevLat, prevLon = lat, lon
	}
	return b.String()
}

func writePolylineValue(b *strings.Builder, v int64) {
	u := uint64(v) << 1
	if v < 0 {
		u = ^u
	}
	for u >= 0x20 {
		b.WriteByte(byte((u&0x1f)|0x20) + 63)
		u >>= 5
	}
	b.WriteByte(byte(u) + 63)
}

// DecodePolyline раскодирует линию в формате Encoded Polyline с точностью 6 знаков.
func DecodePolyline(s string) ([]Point, error) {
	return decodePolyline(s, polylinePrecision)
}

func decodePolyline(s string, precision float64) ([]Point, error) {
	var points []Point
	var lat, lon int64
	for i := 0; i < len(s); {
		dLat, n, err := readPolylineValue(s[i:])
		if err != nil {
			return nil, err
		}
		i += n
		dLon, n, err := readPolylineValue(s[i:])
		if err != nil {
			return nil, err
		}
		i += n
		lat, lon = lat+dLat, lon+dLon
		points = append(points, Point{Lat: float64(lat) / precision, Lon: float64(lon) / precision})
	}
	return points, nil
}

func readPolylineValue(s string) (v int64, n int, err error) {
	var u uint64
	var shift uint
	for {
		if n >= len(s) {
			return 0, 0, errors.New("polyline: unexpected end")
		}
		b := uint64(s[n]) - 63
		n++
		u |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	v = int64(u >> 1)
	if u&1 != 0 {
		v = ^v
	}
	return v, n, nil
}
