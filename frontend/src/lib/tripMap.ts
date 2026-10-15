import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import './tripMap.css'
import { api, type DriveTrip, type Place, type RoadRoute, type TripOption } from '@/api/client'
import { decodePolyline, greatCircleArc, greatCircleKm, type LatLon } from '@/lib/geo'

/**
 * Что рисуем: с перелётом (option) — дорога от дома до аэропорта вылета и перелёт до аэропорта прилёта;
 * без него (drive) — дорога от дома прямо до места назначения.
 */
export interface Trip {
  home: Place
  destination: Place
  option?: TripOption
  drive?: DriveTrip
  stops?: Stop[] // места из путевого листа
}

/** Место из путевого листа: по пути (road) — дорога идёт через него; в городе (see) — просто отметка «хочу побывать». */
export interface Stop {
  id: string
  title: string
  at: LatLon
  onRoad: boolean
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

export function createMap(el: HTMLElement, options: L.MapOptions = {}) {
  const map = L.map(el, { scrollWheelZoom: false, ...options })
  tiles().addTo(map)
  map.attributionControl?.setPrefix('<a href="https://leafletjs.com">Leaflet</a>')
  return map
}

export function tiles() {
  return L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 18,
    attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
  })
}

export function marker(p: Place, className: string, direction?: L.Direction) {
  const m = L.circleMarker(at(p), { radius: 6, weight: 2, fillOpacity: 1, className })
  return direction ? m.bindTooltip(label(p), { permanent: true, direction, className: 'trip-map-label' }) : m
}

export function roadLine(points: LatLon[], approximate: boolean) {
  return L.polyline(points, { className: 'trip-map-road', weight: 4, dashArray: approximate ? '2 7' : undefined })
}

/** Рисует перелёт и точки маршрута, возвращает границы для fitBounds. */
export function drawTrip(map: L.Map, t: Trip) {
  if (!t.option) {
    const layers = [marker(t.home, 'trip-map-home', 'left'), marker(t.destination, 'trip-map-hub', 'right')]
    return L.featureGroup(layers).addTo(map).getBounds()
  }
  const hub = t.option.hub
  const arrival = arrivalOf(t)
  const layers: L.Layer[] = [
    L.polyline(greatCircleArc(at(hub), at(arrival)), { className: 'trip-map-flight', weight: 3, dashArray: '6 8' }),
  ]
  const car = byCar(t)
  // Аэропорт своего города: подпись одна, у аэропорта
  if (car) layers.push(marker(t.home, 'trip-map-home', label(t.home) === label(hub) ? undefined : 'left'))
  layers.push(marker(hub, 'trip-map-hub', car ? 'right' : 'left'))
  layers.push(marker(arrival, 'trip-map-arrival', 'right'))
  return L.featureGroup(layers).addTo(map).getBounds()
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
    .slice(0, 10)
}

/**
 * Загружает дорогу (через места по пути) и рисует её под точками. Если сервер не ответил — ломаная
 * с пометкой approximate. undefined — до аэропорта не на машине.
 */
export async function drawRoad(map: L.Map, t: Trip) {
  if (!byCar(t)) return undefined
  const end = roadEnd(t)
  const via = roadStops(t)
  const fetched = await fetchRoad(t.home.id, end.id, via)
  const known = t.option?.car?.road ?? t.drive?.road ?? { distance_km: 0, duration_min: 0, approximate: true }
  const route: RoadRoute = fetched ?? { road: { ...known, approximate: true }, polyline: '' }
  const line = fetched?.polyline ? decodePolyline(fetched.polyline) : [at(t.home), ...via, at(end)]
  const layer = roadLine(line, route.road.approximate).addTo(map).bringToBack() // точки и подписи — поверх дороги
  return { route, line, layer }
}

/** Отметки мест из путевого листа: по пути — цвета дороги, в городе — флажок «хочу побывать». */
export function drawStops(map: L.Map, stops: Stop[]) {
  const layers = stops.map((s) =>
    L.marker(s.at, {
      icon: L.divIcon({
        className: `trip-map-stop ${s.onRoad ? 'trip-map-stop-road' : 'trip-map-stop-see'}`,
        html: s.onRoad ? '' : '<span>★</span>',
        iconSize: s.onRoad ? [16, 16] : [20, 20],
      }),
      title: s.title,
      keyboard: false,
      zIndexOffset: 1000, // выбранное пользователем — поверх заправок и кафе
    }).bindTooltip(s.title, { direction: 'top', offset: [0, -10], className: 'trip-map-label' }),
  )
  return L.featureGroup(layers).addTo(map)
}
