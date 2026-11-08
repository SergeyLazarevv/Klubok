import type { MapSearchItem } from '@/api/client'

const num = String.raw`(-?\d{1,3}(?:[.,]\d+)?)`
// «53.19, 50.10», «53.19 50.10», «53,19 50,10» — широта, затем долгота, как в Яндекс и Google Картах
const COORDS = new RegExp(String.raw`^\s*${num}\s*(?:[;,]\s*|\s+)${num}\s*$`)

/** Координаты из строки поиска: их не нужно отправлять на сервер. */
export function parseCoords(q: string): MapSearchItem | null {
  const m = COORDS.exec(q)
  if (!m) return null
  const lat = Number(m[1]!.replace(',', '.'))
  const lon = Number(m[2]!.replace(',', '.'))
  if (Math.abs(lat) > 90 || Math.abs(lon) > 180) return null
  return { name: `${lat.toFixed(5)}, ${lon.toFixed(5)}`, subtitle: 'Координаты', kind: 'other', location: { lat, lon } }
}

/** Масштаб, на котором видно найденное место без границ. */
export const zoomFor: Record<MapSearchItem['kind'], number> = {
  city: 11,
  village: 13.5,
  nature: 9, // озеро или гора без границ: шире, иначе видна только вода
  sight: 16,
  street: 16,
  house: 17,
  other: 15,
}

const HISTORY_KEY = 'klubok:map-search'
const HISTORY_MAX = 6

export function loadHistory(): MapSearchItem[] {
  try {
    const v = JSON.parse(localStorage.getItem(HISTORY_KEY) ?? '[]')
    return Array.isArray(v) ? v : []
  } catch {
    return []
  }
}

export function remember(it: MapSearchItem): MapSearchItem[] {
  const same = (a: MapSearchItem) => a.name === it.name && a.subtitle === it.subtitle
  const list = [it, ...loadHistory().filter((a) => !same(a))].slice(0, HISTORY_MAX)
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(list))
  } catch {
    // приватный режим — история просто не сохраняется
  }
  return list
}
