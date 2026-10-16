import type { GeoJSONSource, LngLatLike, Map as MapLibreMap } from 'maplibre-gl'

/**
 * Маршрут от дома до выбранного города: дорога — оранжевой линией по реальным дорогам, перелёт — синей
 * пунктирной дугой. Слои лежат под гербами и плашками (перед слоем cities), чтобы их не закрывать.
 */

export interface RouteSegment {
  kind: 'road' | 'flight'
  coords: [number, number][] // [lon, lat]
}

const SOURCE = 'trip-route'
const ROAD = '#e37400'
const FLIGHT = '#0b57d0'

export function addRouteLayers(map: MapLibreMap, beforeId?: string) {
  map.addSource(SOURCE, { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
  const before = beforeId && map.getLayer(beforeId) ? beforeId : undefined
  const isRoad = ['==', ['get', 'kind'], 'road'] as const
  map.addLayer({
    id: 'trip-route-casing',
    type: 'line',
    source: SOURCE,
    filter: isRoad as never,
    layout: { 'line-cap': 'round', 'line-join': 'round' },
    paint: { 'line-color': '#ffffff', 'line-width': ['interpolate', ['linear'], ['zoom'], 4, 5, 10, 9], 'line-opacity': 0.9 },
  }, before)
  map.addLayer({
    id: 'trip-route-road',
    type: 'line',
    source: SOURCE,
    filter: isRoad as never,
    layout: { 'line-cap': 'round', 'line-join': 'round' },
    paint: { 'line-color': ROAD, 'line-width': ['interpolate', ['linear'], ['zoom'], 4, 3, 10, 5.5] },
  }, before)
  map.addLayer({
    id: 'trip-route-flight',
    type: 'line',
    source: SOURCE,
    filter: ['==', ['get', 'kind'], 'flight'],
    layout: { 'line-cap': 'round' },
    paint: { 'line-color': FLIGHT, 'line-width': 2.5, 'line-dasharray': [2, 2] },
  }, before)
}

export function setRoute(map: MapLibreMap, segments: RouteSegment[]) {
  map.getSource<GeoJSONSource>(SOURCE)?.setData({
    type: 'FeatureCollection',
    features: segments.map((s) => ({
      type: 'Feature',
      geometry: { type: 'LineString', coordinates: s.coords },
      properties: { kind: s.kind },
    })),
  })
}

/** Показать весь путь: точки — дом, пересадки и место назначения; extraLeft — запас слева, px. */
export function fitRoute(map: MapLibreMap, points: LngLatLike[], extraLeft: number) {
  if (points.length < 2) return
  const lons = points.map((p) => (Array.isArray(p) ? p[0] : 'lng' in p ? p.lng : p.lon))
  const lats = points.map((p) => (Array.isArray(p) ? p[1] : p.lat))
  map.fitBounds(
    [[Math.min(...lons), Math.min(...lats)], [Math.max(...lons), Math.max(...lats)]],
    // Сверху — место под плашку над городом
    { padding: { top: 90, bottom: 50, left: extraLeft + 50, right: 70 }, maxZoom: 9, duration: 800 },
  )
}
