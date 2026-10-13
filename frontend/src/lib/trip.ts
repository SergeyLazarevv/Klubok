import type { DriveTrip, Place, TripOption, TripsResponse } from '@/api/client'

/** Вариант поездки: с перелётом или целиком на машине. */
export type Choice = { kind: 'flight'; key: string; option: TripOption } | { kind: 'drive'; key: 'drive'; drive: DriveTrip }

export const totalOf = (c: Choice) => (c.kind === 'drive' ? c.drive.total.amount : c.option.total.amount)

/**
 * Время в пути туда от двери до места: дорога до аэропорта + перелёт, или вся дорога на машине.
 * Ожидание в аэропорту не считаем. Длительность перелёта неизвестна — вариант не может быть «самым быстрым».
 */
export function doorToDoorHours(o: TripOption): number | undefined {
  if (!o.duration_to_min) return undefined
  return (o.car?.drive_hours ?? 0) + o.duration_to_min / 60
}

export const hoursOf = (c: Choice) => (c.kind === 'drive' ? c.drive.drive_hours : doorToDoorHours(c.option))

export type Highlight = 'cheapest' | 'fastest' | 'home'

export interface Pick {
  choice: Choice
  tags: Highlight[]
  /** Цена выбора: дешёвый — насколько дольше самого быстрого, быстрый — насколько дороже самого дешёвого. */
  tradeoff?: { slowerHours?: number; pricierKop?: number }
}

const isHome = (o: TripOption, home: Place) => (o.hub.city_iata ?? o.hub.iata) === home.iata

/**
 * Что показать: самый дешёвый, самый быстрый, вылет из своего города и поездка на машине (если она
 * подходит под настройки). Совпадающие варианты — одна карточка с несколькими метками. Остальное
 * (соседние аэропорты, которые ни дешевле, ни быстрее) — в rest, показываются ниже.
 */
export function highlights(result: TripsResponse): { picks: Pick[]; rest: Choice[] } {
  const choices: Choice[] = result.options.map((o) => ({ kind: 'flight', key: o.hub.id, option: o }))
  if (result.drive) choices.push({ kind: 'drive', key: 'drive', drive: result.drive })
  if (!choices.length) return { picks: [], rest: [] }

  const cheapest = choices.reduce((a, b) => (totalOf(b) < totalOf(a) ? b : a))
  let fastest: Choice | undefined
  for (const c of choices) {
    const h = hoursOf(c)
    if (h === undefined) continue
    const best = fastest && hoursOf(fastest)
    if (best === undefined || h < best) fastest = c
  }
  const home = choices.find((c) => c.kind === 'flight' && isHome(c.option, result.home))
  const drive = choices.find((c) => c.kind === 'drive')

  const picks: Pick[] = []
  const add = (c: Choice | undefined, tag: Highlight) => {
    if (!c) return
    const existing = picks.find((p) => p.choice.key === c.key)
    if (existing) existing.tags.push(tag)
    else picks.push({ choice: c, tags: [tag] })
  }
  add(cheapest, 'cheapest')
  add(fastest, 'fastest')
  add(home, 'home')
  // Машина — всегда отдельной карточкой; без метки, если она не дешевле и не быстрее
  if (drive && !picks.some((p) => p.choice.key === drive.key)) picks.push({ choice: drive, tags: [] })

  if (fastest && fastest.key !== cheapest.key) {
    const slower = (hoursOf(cheapest) ?? 0) - (hoursOf(fastest) ?? 0)
    const cheapPick = picks.find((p) => p.choice.key === cheapest.key)!
    const fastPick = picks.find((p) => p.choice.key === fastest!.key)!
    if (hoursOf(cheapest) !== undefined && slower > 0.25) cheapPick.tradeoff = { slowerHours: slower }
    fastPick.tradeoff = { pricierKop: totalOf(fastest) - totalOf(cheapest) }
  }
  const drivePick = picks.find((p) => p.choice.kind === 'drive' && !p.tags.length)
  if (drivePick) {
    const slower = (hoursOf(drivePick.choice) ?? 0) - (fastest ? (hoursOf(fastest) ?? 0) : 0)
    drivePick.tradeoff = {
      slowerHours: fastest && slower > 0.25 ? slower : undefined,
      pricierKop: totalOf(drivePick.choice) - totalOf(cheapest),
    }
  }
  const shown = new Set(picks.map((p) => p.choice.key))
  return { picks, rest: choices.filter((c) => !shown.has(c.key)) }
}
