import { api, type DriveTrip, type Place, type RoadRoute, type TripOption } from '@/api/client'
import { decodePolyline, greatCircleKm, type LatLon } from '@/lib/geo'

/**
 * Что рисуем: с перелётом (option) — дорога от дома до аэропорта вылета и перелёт до аэропорта прилёта;
 * без него (drive) — дорога от дома прямо до места назначения. Рисует lib/map/tripLayers.ts.
 */
export interface Trip {
  home: Place
  destination: Place
  option?: TripOption
  drive?: DriveTrip
  stops?: Stop[] // места из путевого листа и города-остановки
}

/** Место из путевого листа: по пути (road) — дорога идёт через него; в городе (see) — просто отметка «хочу побывать». */
export interface Stop {
  id: string
  title: string
  at: LatLon
  onRoad: boolean
  city?: boolean // город-остановка (ADR-0023): подпись видна всегда
}

export const at = (p: Place): LatLon => [p.location.lat, p.location.lon]
export const label = (p: Place) => p.city_name ?? p.name
export const arrivalOf = (t: Trip) => t.option?.arrival ?? t.destination
/** Куда ведёт дорога: до аэропорта вылета или, без перелёта, до места назначения. */
export const roadEnd = (t: Trip) => t.option?.hub ?? t.destination
/** Есть ли на карте дорога на машине. */
export const byCar = (t: Trip) => (t.option ? !!t.option.car && t.home.id !== t.option.hub.id : true)

// Линии дорог одинаковы для всех поисков — храним, пока открыта страница
const roadCache = new Map<string, Promise<RoadRoute | undefined>>()

function fetchRoad(from: string, to: string, via: LatLon[]) {
  const viaParam = via.map(([lat, lon]) => `${lat.toFixed(5)},${lon.toFixed(5)}`)
  const key = [from, to, ...viaParam].join(':')
  let route = roadCache.get(key)
  if (!route) {
    const query = { from, to, ...(viaParam.length ? { via: viaParam } : {}) }
    route = api.GET('/routes/road', { params: { query } }).then(({ data }) => data)
    roadCache.set(key, route)
    route.then((r) => r || roadCache.delete(key)) // ошибку не кэшируем
  }
  return route
}

/**
 * Места по пути — в порядке удаления от дома: для дороги между городами этого достаточно,
 * чтобы не петлять туда-обратно.
 */
export function roadStops(t: Trip): LatLon[] {
  const home = at(t.home)
  return (t.stops ?? [])
    .filter((s) => s.onRoad)
    .map((s) => s.at)
    .sort((a, b) => greatCircleKm(home, a) - greatCircleKm(home, b))
}

/**
 * Дорога через места по пути с линией. Сервер не ответил — ломаная через точки с пометкой approximate.
 * undefined — до аэропорта не на машине.
 */
export async function loadRoad(t: Trip): Promise<{ route: RoadRoute; line: LatLon[] } | undefined> {
  if (!byCar(t)) return undefined
  const end = roadEnd(t)
  const via = roadStops(t)
  const fetched = await fetchRoad(t.home.id, end.id, via)
  const known = t.option?.car?.road ?? t.drive?.road ?? { distance_km: 0, duration_min: 0, approximate: true }
  const route: RoadRoute = fetched ?? { road: { ...known, approximate: true }, polyline: '' }
  const line = fetched?.polyline ? decodePolyline(fetched.polyline) : [at(t.home), ...via, at(end)]
  return { route, line }
}
