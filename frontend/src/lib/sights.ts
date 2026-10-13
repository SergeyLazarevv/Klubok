import type { GuideListing, Poi } from '@/api/client'
import { kindLabels } from '@/lib/poiLayer'

/** Карточка места в ленте: из путеводителя или из OSM. */
export interface SightItem {
  id: string
  name: string
  kind?: string
  description?: string
  photoUrl?: string
  url?: string
}

export const fromPoi = (p: Poi): SightItem => ({
  id: p.id,
  name: p.name ?? '',
  kind: kindLabels[p.kind] ?? 'Достопримечательность',
  description: p.description,
  photoUrl: p.photo?.url,
})

const listingKinds: Partial<Record<GuideListing['type'], string>> = { see: 'Достопримечательность', do: 'Чем заняться' }

export const fromListing = (l: GuideListing, i: number): SightItem => ({
  id: `guide-${i}`,
  name: l.name,
  kind: listingKinds[l.type],
  description: l.description,
  photoUrl: l.photo?.url,
  url: l.url,
})

const norm = (s: string) => s.toLowerCase().replace(/[«»"'.,()-]/g, ' ').replace(/\s+/g, ' ').trim()

/** Места OSM, которых нет в путеводителе: совпадение по Wikidata или по названию. */
export function notInGuide(pois: Poi[], listings: GuideListing[]): Poi[] {
  const ids = new Set(listings.map((l) => l.wikidata).filter(Boolean))
  const names = listings.map((l) => norm(l.name))
  return pois.filter((p) => {
    if (p.wikidata && ids.has(p.wikidata)) return false
    const n = norm(p.name ?? '')
    return !names.some((g) => g === n || (n.length > 5 && g.includes(n)) || (g.length > 5 && n.includes(g)))
  })
}
