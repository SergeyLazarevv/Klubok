import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import './tripMap.css'
import { api, type Place, type RoadRoute, type TripOption } from '@/api/client'
import { decodePolyline, greatCircleArc, type LatLon } from '@/lib/geo'

/** Что рисуем: дорога от дома до аэропорта вылета и перелёт до аэропорта прилёта. */
export interface Trip {
  home: Place
  option: TripOption
  destination: Place
}

export const at = (p: Place): LatLon => [p.location.lat, p.location.lon]
export const label = (p: Place) => p.city_name ?? p.name
export const arrivalOf = (t: Trip) => t.option.arrival ?? t.destination
/** Едем ли до аэропорта на машине — тогда на карте есть дорога. */
export const byCar = (t: Trip) => !!t.option.car && t.home.id !== t.option.hub.id

// Линии дорог одинаковы для всех поисков — храним, пока открыта страница
const roadCache = new Map<string, Promise<RoadRoute | undefined>>()

function fetchRoad(from: string, to: string) {
  const key = `${from}:${to}`
  let route = roadCache.get(key)
  if (!route) {
    route = api.GET('/routes/road', { params: { query: { from, to } } }).then(({ data }) => data)
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
  const arrival = arrivalOf(t)
  const layers: L.Layer[] = [
    L.polyline(greatCircleArc(at(t.option.hub), at(arrival)), { className: 'trip-map-flight', weight: 3, dashArray: '6 8' }),
  ]
  const car = byCar(t)
  // Аэропорт своего города: подпись одна, у аэропорта
  if (car) layers.push(marker(t.home, 'trip-map-home', label(t.home) === label(t.option.hub) ? undefined : 'left'))
  layers.push(marker(t.option.hub, 'trip-map-hub', car ? 'right' : 'left'))
  layers.push(marker(arrival, 'trip-map-arrival', 'right'))
  return L.featureGroup(layers).addTo(map).getBounds()
}

/**
 * Загружает дорогу и рисует её под точками. Если сервер не ответил — прямая с пометкой approximate.
 * undefined — до аэропорта не на машине.
 */
export async function drawRoad(map: L.Map, t: Trip) {
  if (!byCar(t)) return undefined
  const fetched = await fetchRoad(t.home.id, t.option.hub.id)
  const route: RoadRoute = fetched ?? { road: { ...t.option.car!.road, approximate: true }, polyline: '' }
  const line = fetched?.polyline ? decodePolyline(fetched.polyline) : [at(t.home), at(t.option.hub)]
  roadLine(line, route.road.approximate).addTo(map).bringToBack() // точки и подписи — поверх дороги
  return { route, line }
}
