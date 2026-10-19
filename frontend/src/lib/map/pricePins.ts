import type {
  ExpressionSpecification, GeoJSONSource, LayerSpecification, Map as MapLibreMap, MapLayerMouseEvent,
} from 'maplibre-gl'
import { FONT, FONT_BOLD } from './fonts'

/**
 * Плашки-балуны «значок · цена» над гербами — слои WebGL, как у быстрых карт (Wildberries, Яндекс):
 * сотни меток не нагружают страницу, их рисует видеокарта вместе с картой. Наезжающие плашки собирает
 * встроенная кластеризация MapLibre (в фоновом потоке): группа пишет «от <самая низкая цена> ₽» и сколько
 * в ней мест, клик приближает карту, и группа распадается. Отдельный слой — серые значки городов без цены
 * (цена по клику). HTML-маркеры были в 4+ раза медленнее: каждый кадр браузер двигал сотни элементов.
 */

export interface PricePin {
  key: string
  lon: number
  lat: number
  name: string
  label: string // «5 447 ₽» или «≈ 663 ₽»
  price: number // рубли — для «от …» у группы и порядка
  by: 'fly' | 'car'
}

export interface LookupPin {
  key: string
  lon: number
  lat: number
  name: string
}

export const PRICE_LAYER = 'price-pins'
export const LOOKUP_LAYER = 'lookup-pins'
const ACTIVE_LAYER = 'price-active'
const PRICES = 'price-pins'
const LOOKUPS = 'lookup-pins'
const ACTIVE = 'price-active' // выбранная плашка — отдельно, чтобы никогда не пряталась в группу

export const PIN_FONT = FONT_BOLD // шрифт приложения, грузится вместе со стилем карты (fonts.ts)
const LIGHT_FONT = FONT // «от» в группах — светлее цены

export const RATIO = 2
const FLIGHT = '#0b57d0'
const ROAD = '#e37400'
const INK = '#1f1f1f'
const MUTED = '#5f6368'

// Значки lucide (plane, car)
const PLANE = 'M17.8 19.2 16 11l3.5-3.5C21 6 21.5 4 21 3c-1-.5-3 0-4.5 1.5L13 8 4.8 6.2c-.5-.1-.9.1-1.1.5l-.3.5c-.2.5-.1 1 .3 1.3L9 12l-2 3H4l-1 1 3 2 2 3 1-1v-3l3-2 3.5 5.3c.3.4.8.5 1.3.3l.5-.2c.4-.3.6-.7.5-1.2z'
const CAR = ['M19 17h2c.6 0 1-.4 1-1v-3c0-.9-.7-1.7-1.5-1.9C18.7 10.6 16 10 16 10s-1.3-1.4-2.2-2.3c-.5-.4-1.1-.7-1.8-.7H5c-.6 0-1.1.4-1.4.9l-1.4 2.9A3.7 3.7 0 0 0 2 12v4c0 .6.4 1 1 1h2', 'M9 17h6']
const CAR_WHEELS: [number, number][] = [[7, 17], [17, 17]]

export function canvas(w: number, h: number) {
  const c = document.createElement('canvas')
  c.width = w * RATIO
  c.height = h * RATIO
  const ctx = c.getContext('2d')!
  ctx.scale(RATIO, RATIO)
  return { c, ctx }
}

/** Круглый значок способа поездки — вставляется прямо в текст плашки. */
function badge(color: string, by: 'fly' | 'car', size = 18, iconColor = '#fff') {
  const { ctx } = canvas(size, size)
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, size / 2, 0, Math.PI * 2)
  ctx.fillStyle = color
  ctx.fill()
  const s = (size * 0.6) / 24
  ctx.translate(size * 0.2, size * 0.2)
  ctx.scale(s, s)
  ctx.strokeStyle = iconColor
  ctx.lineWidth = 2.2
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  if (by === 'fly') ctx.stroke(new Path2D(PLANE))
  else {
    for (const d of CAR) ctx.stroke(new Path2D(d))
    for (const [x, y] of CAR_WHEELS) {
      ctx.beginPath()
      ctx.arc(x, y, 2, 0, Math.PI * 2)
      ctx.stroke()
    }
  }
  return ctx.getImageData(0, 0, size * RATIO, size * RATIO)
}

/**
 * Плашка-балун: скругление и хвостик снизу по центру. Растягиваются две одинаковые зоны по бокам от хвостика,
 * поэтому хвостик всегда под серединой плашки — то есть под городом.
 */
export function pill(fill: string) {
  const R = 14 // скругление и неподвижные края
  const T = 12 // ширина хвостика
  const H = 28
  const TAIL = 6
  const w = R * 2 + T + 8
  const pad = 3 // место под тень
  const { ctx } = canvas(w + pad * 2, H + TAIL + pad * 2)
  ctx.translate(pad, pad)
  ctx.shadowColor = 'rgba(0, 0, 0, 0.18)'
  ctx.shadowBlur = 5
  ctx.shadowOffsetY = 1.5
  ctx.beginPath()
  ctx.roundRect(0, 0, w, H, H / 2)
  const mid = w / 2
  ctx.moveTo(mid - T / 2, H - 1)
  ctx.lineTo(mid, H + TAIL)
  ctx.lineTo(mid + T / 2, H - 1)
  ctx.fillStyle = fill
  ctx.fill()
  const px = (v: number) => (v + pad) * RATIO
  return {
    data: ctx.getImageData(0, 0, (w + pad * 2) * RATIO, (H + TAIL + pad * 2) * RATIO),
    options: {
      pixelRatio: RATIO,
      stretchX: [[px(R), px(mid - T / 2)], [px(mid + T / 2), px(w - R)]] as [number, number][],
      stretchY: [[px(H / 2 - 1), px(H / 2 + 1)]] as [number, number][],
      content: [px(8), px(4), px(w - 8), px(H - 4)] as [number, number, number, number],
    },
  }
}

/** Серый значок города без цены. */
function lookupIcon() {
  const size = 22
  const { ctx } = canvas(size + 4, size + 4)
  ctx.translate(2, 2)
  ctx.shadowColor = 'rgba(0, 0, 0, 0.2)'
  ctx.shadowBlur = 3
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, size / 2, 0, Math.PI * 2)
  ctx.fillStyle = '#fff'
  ctx.fill()
  ctx.shadowColor = 'transparent'
  const b = badge('#9aa0a6', 'fly', 16)
  const tmp = document.createElement('canvas')
  tmp.width = b.width
  tmp.height = b.height
  tmp.getContext('2d')!.putImageData(b, 0, 0)
  ctx.drawImage(tmp, 3, 3, 16, 16)
  return ctx.getImageData(0, 0, (size + 4) * RATIO, (size + 4) * RATIO)
}

const isActive: ExpressionSpecification = ['==', ['get', 'active'], true]
const isCluster: ExpressionSpecification = ['has', 'point_count']
const rubles = (v: ExpressionSpecification): ExpressionSpecification =>
  ['concat', ['number-format', v, { locale: 'ru-RU', 'max-fraction-digits': 0 }], ' ₽']


function priceLayer(id: string, source: string): LayerSpecification & { type: 'symbol' } {
  return {
    id,
    type: 'symbol',
    source,
    layout: {
      'icon-image': ['case', isActive, 'pill-active', 'pill'],
      'icon-text-fit': 'both',
      'icon-anchor': 'bottom',
      'text-anchor': 'bottom',
      // Над гербом, не задевая его: метки участвуют в общей проверке наложений и иначе прятали бы гербы.
      // Фон двигать не нужно — icon-text-fit подгоняет его под текст
      // Подъём — по размеру герба на этом зуме (cities.ts): ниже хвостик и тень плашки задевают герб
      'text-offset': ['interpolate', ['linear'], ['zoom'], 3, ['literal', [0, -2]], 6, ['literal', [0, -3]], 9, ['literal', [0, -3.5]]],
      'text-font': [PIN_FONT],
      'text-size': 12.5,
      'text-max-width': 40, // одной строкой: «Минск  26 336 ₽», а не в две
      'text-padding': 3,
      'text-field': ['case',
        isCluster,
        ['format',
          ['image', ['case', ['>', ['get', 'fly'], 0], 'pin-fly', 'pin-car']], {},
          '  от ', { 'text-color': MUTED, 'text-font': ['literal', [LIGHT_FONT]] },
          rubles(['get', 'min']), {},
          ['concat', '  +', ['to-string', ['get', 'point_count']]], { 'text-color': MUTED, 'font-scale': 0.9 },
        ],
        // Только значок и цена: название города и так подписано на карте под гербом
        ['format',
          ['image', ['concat', 'pin-', ['get', 'by'], ['case', isActive, '-active', '']]], {},
          ['concat', '  ', ['get', 'label']], {},
        ],
      ],
      // Приоритет при наложении: дешёвые первыми
      'symbol-sort-key': ['case', isCluster, ['get', 'min'], ['get', 'price']],
    },
    paint: {
      'text-color': ['case', isActive, '#ffffff', INK],
    },
  }
}

/** Источники и слои плашек поверх стиля; вызывать, когда стиль загружен. */
export function addPricePins(map: MapLibreMap, onPick: (key: string) => void) {
  // У выбранной (синей) плашки значок наоборот: белый кружок с цветным самолётом или машиной
  const images: [string, ImageData][] = [
    ['pin-fly', badge(FLIGHT, 'fly')],
    ['pin-car', badge(ROAD, 'car')],
    ['pin-fly-active', badge('#ffffff', 'fly', 18, FLIGHT)],
    ['pin-car-active', badge('#ffffff', 'car', 18, ROAD)],
  ]
  for (const [id, data] of images) if (!map.hasImage(id)) map.addImage(id, data, { pixelRatio: RATIO })
  for (const [id, fill] of [['pill', '#ffffff'], ['pill-active', FLIGHT]] as const) {
    const { data, options } = pill(fill)
    if (!map.hasImage(id)) map.addImage(id, data, options)
  }
  if (!map.hasImage('pin-lookup')) map.addImage('pin-lookup', lookupIcon(), { pixelRatio: RATIO })

  map.addSource(LOOKUPS, { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
  map.addSource(PRICES, {
    type: 'geojson',
    data: { type: 'FeatureCollection', features: [] },
    cluster: true,
    clusterRadius: 56,
    clusterMaxZoom: 9,
    // У группы — самая низкая цена и сколько в ней перелётов (для значка)
    clusterProperties: {
      min: ['min', ['get', 'price']],
      fly: ['+', ['case', ['==', ['get', 'by'], 'fly'], 1, 0]],
    },
  })

  map.addLayer({
    id: LOOKUP_LAYER,
    type: 'symbol',
    source: LOOKUPS,
    layout: {
      'icon-image': 'pin-lookup',
      'icon-anchor': 'bottom',
      'icon-offset': [0, -8],
      'icon-padding': 2,
    },
  })

  map.addSource(ACTIVE, { type: 'geojson', data: { type: 'FeatureCollection', features: [] } })
  map.addLayer(priceLayer(PRICE_LAYER, PRICES))
  // Выбранная видна всегда и сама никого не прячет — ни соседей, ни герб своего города
  const active = priceLayer(ACTIVE_LAYER, ACTIVE)
  map.addLayer({
    ...active,
    layout: { ...active.layout, 'icon-allow-overlap': true, 'text-allow-overlap': true, 'icon-ignore-placement': true, 'text-ignore-placement': true },
  } as LayerSpecification)

  const pick = (e: MapLayerMouseEvent) => {
    const f = e.features?.[0]
    if (!f || f.geometry.type !== 'Point') return
    const center = f.geometry.coordinates as [number, number]
    if (f.properties?.cluster) {
      const src = map.getSource<GeoJSONSource>(PRICES)!
      src.getClusterExpansionZoom(f.properties.cluster_id).then((zoom) => map.easeTo({ center, zoom: zoom + 0.5, duration: 600 }))
      return
    }
    onPick(f.properties?.key)
  }
  map.on('click', PRICE_LAYER, pick)
  map.on('click', ACTIVE_LAYER, pick)
  map.on('click', LOOKUP_LAYER, (e: MapLayerMouseEvent) => onPick(e.features?.[0]?.properties?.key))
  for (const layer of [PRICE_LAYER, ACTIVE_LAYER, LOOKUP_LAYER]) {
    map.on('mouseenter', layer, () => (map.getCanvas().style.cursor = 'pointer'))
    map.on('mouseleave', layer, () => (map.getCanvas().style.cursor = ''))
  }
}

const points = <T extends { lon: number; lat: number }>(list: T[], props: (p: T) => Record<string, unknown>) => ({
  type: 'FeatureCollection' as const,
  features: list.map((p) => ({
    type: 'Feature' as const,
    geometry: { type: 'Point' as const, coordinates: [p.lon, p.lat] },
    properties: props(p),
  })),
})

export function setPricePins(map: MapLibreMap, pins: PricePin[], lookups: LookupPin[], activeKey?: string) {
  const props = (p: PricePin) => ({ key: p.key, name: p.name, label: p.label, price: p.price, by: p.by, active: p.key === activeKey })
  map.getSource<GeoJSONSource>(PRICES)?.setData(points(pins.filter((p) => p.key !== activeKey), props))
  map.getSource<GeoJSONSource>(ACTIVE)?.setData(points(pins.filter((p) => p.key === activeKey), props))
  map.getSource<GeoJSONSource>(LOOKUPS)?.setData(points(lookups, (p) => ({ key: p.key, name: p.name })))
}
