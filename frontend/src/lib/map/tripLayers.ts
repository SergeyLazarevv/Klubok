import type { ExpressionSpecification, GeoJSONSource, LngLatBoundsLike, Map as MapLibreMap, MapLayerMouseEvent } from 'maplibre-gl'
import type { Feature, FeatureCollection, Point } from 'geojson'
import type { Poi, StopSuggestion } from '@/api/client'
import { greatCircleArc, type LatLon } from '@/lib/geo'
import { categories, fetchDetails, poiColors, poiPopup, poiSvg, type PoiCategory } from '@/lib/poiLayer'
import { detourLabel } from '@/lib/stopPicker'
import { arrivalOf, at, byCar, label, type Stop, type Trip } from '@/lib/tripMap'
import { PASTEL, cartoonStyle } from './cartoonStyle'
import { maplibregl } from './maplibre'
import { provideImages } from './patterns'
import { PIN_FONT, RATIO, pill } from './pricePins'
import { addRouteLayers, setRoute, type RouteSegment } from './routeLine'
import './tripLayers.css'

/**
 * Карта поездки в деталях варианта — в том же пастельном стиле, что «Куда угодно», но без гербов городов:
 * дорога и перелёт, точки маршрута с подписями-плашками, места из путевого листа, города для остановки
 * (ADR-0023) и, на полноэкранной карте, заправки, кафе и места из OSM. Всё — слои WebGL, близкие места
 * собирает кластеризация MapLibre: сотни HTML-маркеров заметно тормозили бы карту.
 */

const ROAD = '#e37400'
const FLIGHT = '#0b57d0'
const PRIMARY = '#0b57d0'
const INK = '#1f1f1f'

const POINTS = 'trip-points'
const STOPS = 'trip-stops'
const CITIES = 'trip-cities'
const CITY_LAYER = 'trip-cities-label'
const CITY_CHOSEN = 'trip-cities-chosen'
const CITY_DOT = 'trip-cities-dot'
const isChosen: ExpressionSpecification = ['==', ['get', 'chosen'], true]
const offeredOnly: ExpressionSpecification = ['==', ['get', 'chosen'], false]

type FC = FeatureCollection<Point>
const empty = (): FC => ({ type: 'FeatureCollection', features: [] })
const feature = ([lat, lon]: LatLon, properties: Record<string, unknown>): Feature<Point> => ({
  type: 'Feature',
  geometry: { type: 'Point', coordinates: [lon, lat] },
  properties,
})
const toLonLat = ([lat, lon]: LatLon): [number, number] => [lon, lat]

/** Отступы при подгонке масштаба: сверху — место под плашки над точками. */
export const FIT_PADDING = { top: 56, bottom: 24, left: 32, right: 32 }

export interface TripMapOptions {
  /** Карта в карточке: двигается двумя пальцами (страница под ней прокручивается одним), без кнопок наклона. */
  compact?: boolean
  /** Врезка: без управления и подписей. */
  static?: boolean
  pois?: boolean
}

/** Карта и обещание готовности: слои добавляются, когда разобран стиль (тайлы ещё грузятся). */
export function createTripMap(container: HTMLElement, t: Trip, opts: TripMapOptions = {}) {
  const map = new maplibregl.Map({
    container,
    style: cartoonStyle(PASTEL),
    bounds: tripBounds(t),
    fitBoundsOptions: { padding: opts.static ? 14 : FIT_PADDING },
    interactive: !opts.static,
    cooperativeGestures: opts.compact,
    attributionControl: opts.static ? false : { compact: true },
    maxPitch: 60,
    dragRotate: false,
    pitchWithRotate: false,
  })
  provideImages(map)
  if (!opts.static) map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-left')
  const ready = new Promise<void>((resolve) => {
    const init = () => {
      addTripLayers(map, opts)
      resolve()
    }
    if (map.isStyleLoaded()) init()
    else map.once('style.load', init)
  })
  return { map, ready }
}

function addTripLayers(map: MapLibreMap, opts: TripMapOptions) {
  for (const [id, fill] of [['trip-pill', '#ffffff'], ['trip-pill-road', ROAD]] as const) {
    const { data, options } = pill(fill)
    if (!map.hasImage(id)) map.addImage(id, data, options)
  }
  addRouteLayers(map)
  if (opts.pois) addPoiLayers(map)

  // Места из путевого листа: по пути — точки цвета дороги, «хочу побывать» — синие
  map.addSource(STOPS, { type: 'geojson', data: empty() })
  map.addLayer({
    id: STOPS,
    type: 'circle',
    source: STOPS,
    paint: {
      'circle-radius': ['case', ['==', ['get', 'onRoad'], true], 6, 7],
      'circle-color': ['case', ['==', ['get', 'onRoad'], true], ROAD, PRIMARY],
      'circle-stroke-color': '#ffffff',
      'circle-stroke-width': 2,
    },
  })

  // Города для остановки и выбранные города-остановки: точка и плашка «Казань · +15 мин»
  map.addSource(CITIES, { type: 'geojson', data: empty() })
  map.addLayer({
    id: CITY_DOT,
    type: 'circle',
    source: CITIES,
    paint: {
      'circle-radius': 5,
      'circle-color': ['case', isChosen, ROAD, '#ffffff'],
      'circle-stroke-color': ROAD,
      'circle-stroke-width': 2,
    },
  })
  if (!opts.static) {
    const layout = {
      'icon-text-fit': 'both' as const,
      'icon-anchor': 'bottom' as const,
      'text-anchor': 'bottom' as const,
      'text-offset': [0, -1.1] as [number, number],
      'text-field': ['get', 'label'] as ExpressionSpecification,
      'text-font': [PIN_FONT],
      'text-size': 12,
      'text-max-width': 40,
      'text-padding': 2,
    }
    // Предложенные — прячутся, если наезжают друг на друга (в карточке — только точки, подписи на большой
    // карте); выбранные — видны всегда, в цвете дороги
    map.addLayer({
      id: CITY_LAYER,
      type: 'symbol',
      source: CITIES,
      filter: offeredOnly,
      layout: { ...layout, 'icon-image': 'trip-pill', visibility: opts.compact ? 'none' : 'visible' },
      paint: { 'text-color': INK },
    })
    map.addLayer({
      id: CITY_CHOSEN,
      type: 'symbol',
      source: CITIES,
      filter: isChosen,
      layout: { ...layout, 'icon-image': 'trip-pill-road', 'icon-allow-overlap': true, 'text-allow-overlap': true },
      paint: { 'text-color': '#ffffff' },
    })
    for (const layer of [CITY_LAYER, CITY_CHOSEN, CITY_DOT]) pointer(map, layer)
  }

  // Точки маршрута — поверх всего, подписи не прячутся
  map.addSource(POINTS, { type: 'geojson', data: empty() })
  map.addLayer({
    id: POINTS,
    type: 'circle',
    source: POINTS,
    paint: {
      'circle-radius': 6,
      'circle-color': '#ffffff',
      'circle-stroke-color': ['case', ['==', ['get', 'kind'], 'arrival'], FLIGHT, ROAD],
      'circle-stroke-width': 2.5,
    },
  })
  if (!opts.static) {
    map.addLayer({
      id: `${POINTS}-label`,
      type: 'symbol',
      source: POINTS,
      filter: ['!=', ['get', 'label'], ''],
      layout: {
        'icon-image': 'trip-pill',
        'icon-text-fit': 'both',
        'icon-anchor': 'bottom',
        'text-anchor': 'bottom',
        'text-offset': [0, -1.1],
        'text-field': ['get', 'label'],
        'text-font': [PIN_FONT],
        'text-size': 12.5,
        'icon-allow-overlap': true,
        'text-allow-overlap': true,
      },
      paint: { 'text-color': INK },
    })
  }
}

function pointer(map: MapLibreMap, layer: string) {
  map.on('mouseenter', layer, () => (map.getCanvas().style.cursor = 'pointer'))
  map.on('mouseleave', layer, () => (map.getCanvas().style.cursor = ''))
}

/** Дом, аэропорты и место назначения; у аэропорта своего города подпись одна. */
export function setTripPoints(map: MapLibreMap, t: Trip) {
  const features: Feature<Point>[] = []
  if (!t.option) {
    features.push(feature(at(t.home), { kind: 'home', label: label(t.home) }))
    features.push(feature(at(t.destination), { kind: 'hub', label: label(t.destination) }))
  } else {
    const hub = t.option.hub
    if (byCar(t)) features.push(feature(at(t.home), { kind: 'home', label: label(t.home) === label(hub) ? '' : label(t.home) }))
    features.push(feature(at(hub), { kind: 'hub', label: label(hub) }))
    features.push(feature(at(arrivalOf(t)), { kind: 'arrival', label: label(arrivalOf(t)) }))
  }
  map.getSource<GeoJSONSource>(POINTS)?.setData({ type: 'FeatureCollection', features })
}

/** Места из путевого листа; города-остановки рисует setCities. */
export function setStops(map: MapLibreMap, stops: Stop[] = []) {
  map.getSource<GeoJSONSource>(STOPS)?.setData({
    type: 'FeatureCollection',
    features: stops.filter((s) => !s.city).map((s) => feature(s.at, { id: s.id, title: s.title, onRoad: s.onRoad })),
  })
}

/** Города для остановки (если есть) и выбранные города-остановки; id — Wikidata. */
export function setCities(map: MapLibreMap, suggestions: StopSuggestion[] = [], chosen: string[] = [], stops: Stop[] = []) {
  const features = suggestions.map((s) => {
    const picked = chosen.includes(s.city.id)
    return feature([s.city.location.lat, s.city.location.lon], {
      id: s.city.id,
      chosen: picked,
      label: picked ? s.city.name : `${s.city.name} · ${detourLabel(s)}`,
    })
  })
  const offered = new Set(suggestions.map((s) => s.city.id))
  for (const s of stops) {
    const id = s.id.replace(/^city-/, '')
    if (s.city && !offered.has(id)) features.push(feature(s.at, { id, chosen: true, label: s.title }))
  }
  map.getSource<GeoJSONSource>(CITIES)?.setData({ type: 'FeatureCollection', features })
}

/** Нажатие на город — его id. */
export function onCityClick(map: MapLibreMap, pick: (id: string) => void) {
  const handler = (e: MapLayerMouseEvent) => {
    const id = e.features?.[0]?.properties?.id
    if (typeof id === 'string') pick(id)
  }
  for (const layer of [CITY_LAYER, CITY_CHOSEN, CITY_DOT]) map.on('click', layer, handler)
}

/** Дорога (линия — от loadRoad) и дуга перелёта. */
export function setTripRoute(map: MapLibreMap, t: Trip, road?: LatLon[]) {
  const segments: RouteSegment[] = []
  if (road?.length) segments.push({ kind: 'road', coords: road.map(toLonLat) })
  if (t.option) segments.push({ kind: 'flight', coords: greatCircleArc(at(t.option.hub), at(arrivalOf(t))).map(toLonLat) })
  setRoute(map, segments)
}

/** Границы поездки: точки маршрута и, если есть, линия дороги. */
export function tripBounds(t: Trip, road: LatLon[] = []): LngLatBoundsLike {
  const pts: LatLon[] = [at(t.home), at(t.destination), ...road]
  if (t.option) pts.push(at(t.option.hub), at(arrivalOf(t)))
  return bounds(pts)
}

export function bounds(pts: LatLon[]): LngLatBoundsLike {
  const lats = pts.map((p) => p[0])
  const lons = pts.map((p) => p[1])
  return [[Math.min(...lons), Math.min(...lats)], [Math.max(...lons), Math.max(...lats)]]
}

// --- Заправки, кафе и места (полноэкранная карта) ---

const poiSource = (c: PoiCategory) => `poi-${c}`
const byId = new WeakMap<MapLibreMap, Map<string, Poi>>()

/** Значок категории: цветной кружок с белой иконкой Lucide. */
function poiImage(c: PoiCategory): Promise<HTMLImageElement> {
  const size = 26
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${size * RATIO}" height="${size * RATIO}" viewBox="0 0 ${size} ${size}">
    <circle cx="13" cy="13" r="11.5" fill="${poiColors[c]}" stroke="#fff" stroke-width="2"/>
    <g transform="translate(6 6)" color="#fff">${poiSvg(c)}</g></svg>`
  const img = new Image(size * RATIO, size * RATIO)
  img.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
  return img.decode().then(() => img)
}

function addPoiLayers(map: MapLibreMap) {
  byId.set(map, new Map())
  for (const { id: c } of categories) {
    const src = poiSource(c)
    map.addSource(src, { type: 'geojson', data: empty(), cluster: true, clusterRadius: 45, clusterMaxZoom: 15 })
    map.addLayer({
      id: `${src}-cluster`,
      type: 'circle',
      source: src,
      filter: ['has', 'point_count'],
      paint: {
        'circle-color': poiColors[c],
        'circle-radius': ['step', ['get', 'point_count'], 12, 10, 14, 50, 17],
        'circle-stroke-color': '#ffffff',
        'circle-stroke-width': 2,
      },
    })
    map.addLayer({
      id: `${src}-count`,
      type: 'symbol',
      source: src,
      filter: ['has', 'point_count'],
      layout: { 'text-field': ['get', 'point_count_abbreviated'], 'text-font': [PIN_FONT], 'text-size': 11, 'text-allow-overlap': true },
      paint: { 'text-color': '#ffffff' },
    })
    map.addLayer({
      id: `${src}-point`,
      type: 'symbol',
      source: src,
      filter: ['!', ['has', 'point_count']],
      layout: { 'icon-image': src, 'icon-allow-overlap': true, 'icon-size': 1 },
    })
    void poiImage(c).then((img) => map.hasImage(src) || map.addImage(src, img, { pixelRatio: RATIO }))

    map.on('click', `${src}-cluster`, (e) => {
      const f = e.features?.[0]
      if (!f || f.geometry.type !== 'Point') return
      const center = f.geometry.coordinates as [number, number]
      void map.getSource<GeoJSONSource>(src)!.getClusterExpansionZoom(f.properties!.cluster_id).then((zoom) => map.easeTo({ center, zoom }))
    })
    map.on('click', `${src}-point`, (e) => {
      const f = e.features?.[0]
      const p = f && byId.get(map)?.get(f.properties?.id)
      if (p) openPoi(map, p)
    })
    pointer(map, `${src}-cluster`)
    pointer(map, `${src}-point`)
  }
}

function openPoi(map: MapLibreMap, p: Poi) {
  const popup = new maplibregl.Popup({ className: 'poi-popup', maxWidth: '280px', offset: 14 })
    .setLngLat([p.location.lon, p.location.lat])
    .setDOMContent(poiPopup(p, !!p.details_pending))
    .addTo(map)
  if (p.details_pending) void fetchDetails(p.id).then((full) => full && popup.isOpen() && popup.setDOMContent(poiPopup(full, false)))
}

export function setPois(map: MapLibreMap, pois: Poi[]) {
  const index = byId.get(map)
  for (const p of pois) index?.set(p.id, p)
  for (const { id: c } of categories) {
    map.getSource<GeoJSONSource>(poiSource(c))?.setData({
      type: 'FeatureCollection',
      features: pois.filter((p) => p.category === c).map((p) => feature([p.location.lat, p.location.lon], { id: p.id })),
    })
  }
}

export function setPoiVisible(map: MapLibreMap, c: PoiCategory, visible: boolean) {
  for (const suffix of ['cluster', 'count', 'point']) {
    const layer = `${poiSource(c)}-${suffix}`
    if (map.getLayer(layer)) map.setLayoutProperty(layer, 'visibility', visible ? 'visible' : 'none')
  }
}

/** Показать или спрятать предложенные города; выбранные остаются. */
export function setOfferedVisible(map: MapLibreMap, visible: boolean) {
  if (map.getLayer(CITY_LAYER)) map.setLayoutProperty(CITY_LAYER, 'visibility', visible ? 'visible' : 'none')
  if (map.getLayer(CITY_DOT)) map.setFilter(CITY_DOT, visible ? null : isChosen)
}
