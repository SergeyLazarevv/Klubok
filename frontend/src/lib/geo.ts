/** Точка [широта, долгота] — как в Leaflet. */
export type LatLon = [number, number]

/** Раскодирует Encoded Polyline; OSRM отдаёт точность 6 знаков (polyline6). */
export function decodePolyline(encoded: string, precision = 1e6): LatLon[] {
  const points: LatLon[] = []
  let index = 0
  let lat = 0
  let lon = 0
  const next = () => {
    let result = 0
    let shift = 0
    let byte: number
    do {
      byte = encoded.charCodeAt(index++) - 63
      result |= (byte & 0x1f) << shift
      shift += 5
    } while (byte >= 0x20)
    return result & 1 ? ~(result >> 1) : result >> 1
  }
  while (index < encoded.length) {
    lat += next()
    lon += next()
    points.push([lat / precision, lon / precision])
  }
  return points
}

const rad = Math.PI / 180

/** Расстояние по дуге большого круга, км. */
export function greatCircleKm([lat1, lon1]: LatLon, [lat2, lon2]: LatLon) {
  const dLat = (lat2 - lat1) * rad
  const dLon = (lon2 - lon1) * rad
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2
  return 2 * 6371 * Math.asin(Math.sqrt(h))
}

/**
 * Дуга большого круга — так самолёт летит на самом деле и так перелёт выглядит на карте.
 * Долготы идут без скачка через 180°, чтобы линия не пересекала всю карту.
 */
export function greatCircleArc(from: LatLon, to: LatLon, segments = 64): LatLon[] {
  const toVec = ([lat, lon]: LatLon) => [
    Math.cos(lat * rad) * Math.cos(lon * rad),
    Math.cos(lat * rad) * Math.sin(lon * rad),
    Math.sin(lat * rad),
  ]
  const a = toVec(from)
  const b = toVec(to)
  const angle = Math.acos(Math.min(1, Math.max(-1, a[0]! * b[0]! + a[1]! * b[1]! + a[2]! * b[2]!)))
  if (angle < 1e-9) return [from, to]

  const points: LatLon[] = []
  let prevLon = from[1]
  for (let i = 0; i <= segments; i++) {
    const t = i / segments
    const k1 = Math.sin((1 - t) * angle) / Math.sin(angle)
    const k2 = Math.sin(t * angle) / Math.sin(angle)
    const [x, y, z] = [0, 1, 2].map((j) => k1 * a[j]! + k2 * b[j]!) as [number, number, number]
    const lat = Math.atan2(z, Math.hypot(x, y)) / rad
    let lon = Math.atan2(y, x) / rad
    while (lon - prevLon > 180) lon -= 360
    while (lon - prevLon < -180) lon += 360
    points.push([lat, lon])
    prevLon = lon
  }
  return points
}

/** Маршрут на машине в Яндекс Картах — точный путь и пробки там. */
export const yandexRouteUrl = (from: { lat: number; lon: number }, to: { lat: number; lon: number }) =>
  `https://yandex.ru/maps/?rtext=${from.lat},${from.lon}~${to.lat},${to.lon}&rtt=auto`
