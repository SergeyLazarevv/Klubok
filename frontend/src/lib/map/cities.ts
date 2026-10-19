import type { ExpressionSpecification, Map as MapLibreMap, StyleSpecification } from 'maplibre-gl'
import { FONT_BOLD } from './fonts'

/**
 * Города России и столицы мира гербами-наклейками. Издалека видны столицы и крупные города,
 * при приближении добавляются мелкие; ближе 10-го зума гербы тают, уступая место улицам.
 * Данные — public/data (собирает scripts/build-cities.mjs); герб грузится, только когда попадает на экран.
 */
export interface City {
  id: string
  name: string
  pop: number
  capital: boolean
  arms: boolean
  country?: string
  cc?: string // код страны ISO — для правил въезда на машине
  landmark?: string
  photo?: string // файл Wikimedia Commons
}

const DATA = `${import.meta.env.BASE_URL}data/`
export const ARMS_PREFIX = 'arms-'
const NO_ARMS = 'city-dot'

export const armsUrl = (id: string) => `${DATA}arms/${id}.webp`
export const photoUrl = (file: string) =>
  `https://commons.wikimedia.org/wiki/Special:FilePath/${encodeURIComponent(file)}?width=330`

/** Гербы видны до 11-го зума и плавно исчезают между 9,5 и 10,8. */
const FADE: ExpressionSpecification = ['interpolate', ['linear'], ['zoom'], 9.5, 1, 10.8, 0]

/** Кто на карте на каком зуме: крупные столицы всегда, мелкие — с 4-го, города России — по населению. */
const visible: ExpressionSpecification = [
  'any',
  ['all', ['get', 'capital'], ['any', ['>=', ['get', 'pop'], 500_000], ['>=', ['zoom'], 4]]],
  ['>=', ['get', 'pop'], ['step', ['zoom'], 1_000_000, 4, 300_000, 5, 100_000, 6, 30_000, 7, 0]],
]

const big: ExpressionSpecification = ['any', ['get', 'capital'], ['>=', ['get', 'pop'], 1_000_000]]

/** Добавляет слой городов поверх стиля: последним, чтобы герб был важнее обычной подписи города. */
export function withCities(style: StyleSpecification, textColor: string, haloColor: string): StyleSpecification {
  return {
    ...style,
    sources: {
      ...style.sources,
      cities: {
        type: 'geojson',
        data: new URL(`${DATA}cities.geojson`, location.origin).href,
        attribution: '<a href="https://www.wikidata.org">Wikidata</a>, <a href="https://commons.wikimedia.org">Wikimedia Commons</a>',
      },
    },
    layers: [
      ...style.layers,
      {
        id: 'cities',
        type: 'symbol',
        source: 'cities',
        maxzoom: 11,
        filter: visible,
        layout: {
          'icon-image': ['case', ['get', 'arms'], ['concat', ARMS_PREFIX, ['get', 'id']], NO_ARMS],
          'icon-size': ['interpolate', ['linear'], ['zoom'],
            2, ['case', ['get', 'capital'], 0.5, 0.45],
            5, ['case', big, 0.8, 0.6],
            9, ['case', big, 1, 0.85]],
          'icon-padding': 2,
          // Сначала столицы, среди них и среди городов — по населению: Париж важнее Монако
          'symbol-sort-key': ['*', -1, ['+', ['get', 'pop'], ['case', ['get', 'capital'], 100_000_000, 0]]],
          'text-field': ['get', 'name'],
          'text-font': [FONT_BOLD],
          'text-size': ['interpolate', ['linear'], ['zoom'], 2, 10, 8, ['case', big, 15, 12]],
          'text-anchor': 'top',
          'text-offset': ['interpolate', ['linear'], ['zoom'], 2, ['literal', [0, 1.3]], 5, ['literal', [0, 1.9]], 9, ['literal', [0, 2.2]]],
          'text-optional': true,
          'text-max-width': 7,
        },
        paint: {
          'icon-opacity': FADE,
          'text-opacity': FADE,
          'text-color': textColor,
          'text-halo-color': haloColor,
          'text-halo-width': 2,
        },
      },
    ],
  }
}

const RATIO = 2

/** Готовая наклейка герба (белая кайма и тень) — её рисует scripts/build-stickers.mjs при сборке. */
export const stickerUrl = (id: string) => `${DATA}stickers/${id}.webp`

/** Картинка декодируется вне главного потока: createImageBitmap не блокирует карту, в отличие от canvas. */
async function loadBitmap(url: string) {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${res.status} ${url}`)
  return createImageBitmap(await res.blob())
}

/** Кладёт на карту наклейку герба или кружок по запросу стиля. Возвращает false, если картинка не про города. */
export async function addCityImage(map: MapLibreMap, id: string) {
  let image: ImageBitmap
  if (id === NO_ARMS) image = await loadBitmap(stickerUrl(NO_ARMS)) // кружок для города без герба
  else if (id.startsWith(ARMS_PREFIX)) image = await loadBitmap(stickerUrl(id.slice(ARMS_PREFIX.length)))
  else return false
  if (!map.hasImage(id)) map.addImage(id, image, { pixelRatio: RATIO })
  return true
}

/**
 * Прогревает кэш наклеек: после того как карта нарисовалась, в свободное время скачивает все гербы, чтобы
 * при перемещении и на следующих заходах они брались из кэша service worker, а не из сети (≈5 МБ). Не качаем
 * в режиме экономии трафика и на 2G. Один раз за вкладку.
 */
let warmed = false
export async function warmStickerCache() {
  const net = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  if (warmed || net?.saveData || /2g/.test(net?.effectiveType ?? '')) return
  warmed = true
  // Тот же файл, что у слоя городов, — уже в кэше браузера
  const fc: { features: { properties: City }[] } = await (await fetch(`${DATA}cities.geojson`)).json()
  const queue = fc.features.filter((f) => f.properties.arms).map((f) => f.properties.id)
  const idle = () => new Promise<void>((r) => ('requestIdleCallback' in window ? requestIdleCallback(() => r(), { timeout: 2000 }) : setTimeout(r, 200)))
  // Пачками в свободное время главного потока: скачивание и запись в кэш — в service worker, карте не мешают
  while (queue.length) {
    await idle()
    await Promise.all(queue.splice(0, 16).map((id) => fetch(stickerUrl(id), { priority: 'low' } as RequestInit).catch(() => undefined)))
  }
}
