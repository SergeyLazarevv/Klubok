import type { BorderRule, DriveTrip, ExploreItem, StayPrice, TripOption } from '@/api/client'
import { addDays, maxSearchDate, todayISO } from '@/stores/search'
import { greatCircleKm } from '@/lib/geo'
import type { City } from '@/lib/map/cities'

/**
 * «Куда угодно»: периоды и длительности поиска, оценка поездок целиком на машине и слияние их с перелётами.
 * Перелёты считает backend (/explore), машину — клиент по справочнику городов карты: в нём есть
 * небольшие города без аэропорта, которых нет в справочнике backend. Самолёт и машина — отдельные подборки.
 */

/** Во сколько раз перелёт (с дорогой до аэропорта) должен быть быстрее машины — как flightTimeGain на backend. */
const FLIGHT_TIME_GAIN = 0.75

/**
 * Стоит ли предлагать перелёт, если до города можно доехать на машине: он дешевле или заметно быстрее.
 * То же правило, что worthFlying на backend, — чтобы уточнённая цена совпадала с подборкой.
 */
export function worthFlying(o: TripOption, drive?: DriveTrip) {
  if (!drive || o.total.amount < drive.total.amount) return true
  if (o.duration_to_min <= 0) return false
  return o.duration_to_min / 60 + (o.car?.drive_hours ?? 0) <= drive.drive_hours * FLIGHT_TIME_GAIN
}

export interface Period {
  key: string
  label: string
  from: string
  to: string
  days?: number // длительность задана самим периодом — выбор длительности не нужен
}

const monthFmt = new Intl.DateTimeFormat('ru-RU', { month: 'long', timeZone: 'UTC' })
const capitalize = (s: string) => s[0].toUpperCase() + s.slice(1)

/** Ближайшая суббота: сегодня, если сегодня суббота. Вылет — в субботу, возвращение — в воскресенье. */
export function nextWeekend(today = todayISO()): Period {
  const dow = new Date(today + 'T00:00:00Z').getUTCDay() // 0 — воскресенье, 6 — суббота
  const sat = addDays(today, (6 - dow + 7) % 7)
  return { key: 'weekend', label: 'Ближайшие выходные', from: sat, to: addDays(sat, 1), days: 2 }
}

/** «Ближайший месяц», ближайшие выходные и следующие календарные месяцы в пределах горизонта цен (90 дней на начало). */
export function periods(): Period[] {
  const today = todayISO()
  const start = addDays(today, 1)
  const out: Period[] = [{ key: 'soon', label: 'Ближайший месяц', from: start, to: addDays(start, 30) }, nextWeekend(today)]
  const horizon = maxSearchDate()
  const d = new Date(start + 'T00:00:00Z')
  for (let i = 1; i <= 3; i++) {
    const first = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + i, 1))
    const from = first.toISOString().slice(0, 10)
    if (from > horizon) break
    const last = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0)).toISOString().slice(0, 10)
    out.push({ key: from.slice(0, 7), label: capitalize(monthFmt.format(first)), from, to: last })
  }
  return out
}

export interface Duration {
  key: string
  label: string
  min: number
  max: number
  weekends?: boolean
}

export const durations: Duration[] = [
  { key: 'weekend', label: 'Выходные', min: 2, max: 4, weekends: true },
  { key: '3-5', label: '3–5 дней', min: 3, max: 5 },
  { key: '6-9', label: '6–9 дней', min: 6, max: 9 },
  { key: '10-14', label: '10–14 дней', min: 10, max: 14 },
]

export type Mode = 'all' | 'fly' | 'car'

// Как backend оценивает дорогу без маршрутизатора (road.go): дороги на 30 % длиннее прямой, 70 км/ч
const DETOUR = 1.3
const SPEED_KMH = 70
/** Ближе этого — тот же город или пригород, ехать «в путешествие» туда незачем. */
const MIN_DRIVE_KM = 40
/** Мелкие города без достопримечательности в подборку не берём — их сотни, и они заслоняют интересное. */
const MIN_POP = 20_000

export interface DriveEstimate {
  roadKm: number
  hours: number // в одну сторону, с границей
  fuel: number // копейки, туда и обратно
  total: number // бензин + страховка на границе
  border?: { insurance: number; hours: number; note: string; known: boolean }
}

/** Одно направление на карте и в списке: перелёт или поездка на машине. */
export interface Destination {
  key: string
  name: string
  lon: number
  lat: number
  city?: City // из справочника карты: герб, фото, достопримечательность
  flight?: ExploreItem
  drive?: DriveEstimate
  price: number // полная стоимость на всех (дорога + жильё, если учитываем), копейки; у lookup — Infinity
  travel?: number // только дорога, копейки
  stay?: StayEstimate
  lookup?: boolean // город за рубежом без цены в подборке: цену ищем по клику
  unreachable?: { drive?: DriveEstimate } // город России без цены: в карточке объясняем почему
  by: 'fly' | 'car'
}

/** Жильё на поездку: средняя цена суток в гостинице 3* (Росстат) × путешественники × ночи. */
export interface StayEstimate {
  nights: number
  perNight: number // на всех, копейки
  total: number
  source: StayPrice
}

export interface DriveParams {
  home: { lat: number; lon: number }
  maxHours: number
  consumptionL100: number
  fuelPrice: number // копейки за литр
  allowBorder?: boolean // ехать в другие страны (настройки машины)
  borders?: Map<string, BorderRule> // правила въезда по коду страны (из /explore)
}

export const distanceKm = (a: { lat: number; lon: number }, b: { lat: number; lon: number }) =>
  greatCircleKm([a.lat, a.lon], [b.lat, b.lon])

export function estimateDrive(straightKm: number, p: DriveParams): DriveEstimate {
  const roadKm = straightKm * DETOUR
  const fuel = Math.round(((roadKm * 2 * p.consumptionL100) / 100) * p.fuelPrice)
  return { roadKm, hours: roadKm / SPEED_KMH, fuel, total: fuel }
}

export interface CityFeature {
  city: City
  lon: number
  lat: number
}

/** Города России, до которых можно доехать за отведённое время. */
export function driveDestinations(cities: CityFeature[], p: DriveParams): Destination[] {
  const out: Destination[] = []
  for (const c of cities) {
    // Другая страна — только если в настройках разрешена граница: + время на КПП и страховка
    if (c.city.country && (!p.allowBorder || !c.city.cc)) continue
    if (c.city.pop < MIN_POP && !c.city.landmark) continue
    const km = distanceKm(p.home, c)
    if (km < MIN_DRIVE_KM) continue
    const drive = estimateDrive(km, p)
    if (c.city.cc) {
      const rule = p.borders?.get(c.city.cc)
      drive.border = rule
        ? { insurance: rule.insurance.amount, hours: rule.crossing_hours, note: rule.note, known: true }
        : { insurance: 0, hours: 0, note: 'Нет данных о правилах въезда на машине в эту страну — проверьте самостоятельно.', known: false }
      drive.hours += drive.border.hours
      drive.total += drive.border.insurance
    }
    if (p.maxHours > 0 && drive.hours > p.maxHours) continue
    out.push({ key: c.city.id, name: c.city.name, lon: c.lon, lat: c.lat, city: c.city, drive, price: drive.total, by: 'car' })
  }
  return out.sort((a, b) => a.price - b.price)
}

/** Перелёт и город карты — одно место, если названия совпадают и до центра недалеко. */
const SAME_PLACE_KM = 40

/** Перелёты, привязанные к городам карты (герб, фото), — дешёвые первыми. */
export function flightDestinations(flights: ExploreItem[], cities: CityFeature[]): Destination[] {
  const byName = new Map<string, CityFeature[]>()
  for (const c of cities) {
    const list = byName.get(c.city.name) ?? []
    list.push(c)
    byName.set(c.city.name, list)
  }
  return flights
    .map((f): Destination => {
      const { lat, lon } = f.city.location
      const match = byName.get(f.city.name)?.find((c) => distanceKm(c, { lat, lon }) < SAME_PLACE_KM)
      return {
        key: match?.city.id ?? `iata-${f.city.iata}`,
        name: f.city.name,
        lon: match?.lon ?? lon,
        lat: match?.lat ?? lat,
        city: match?.city,
        flight: f,
        price: f.best.total.amount,
        by: 'fly',
      }
    })
    .sort((a, b) => a.price - b.price)
}

/** Самый дешёвый способ по каждому городу: машина или перелёт — как карточка «Самый дешёвый» в поиске. */
export function cheapestByCity(flights: Destination[], drives: Destination[]): Destination[] {
  const out = new Map<string, Destination>()
  for (const d of drives) out.set(d.key, d)
  for (const f of flights) {
    const cur = out.get(f.key)
    if (!cur || f.price < cur.price) out.set(f.key, f)
  }
  return [...out.values()].sort((a, b) => a.price - b.price)
}

/** Зарубежные города карты без цены в подборке — подсвечиваем, цену ищем по клику. */
export function lookupDestinations(cities: CityFeature[], priced: Set<string>): Destination[] {
  return cities
    .filter((c) => c.city.country && !priced.has(c.city.id))
    .map((c) => ({ key: c.city.id, name: c.city.name, lon: c.lon, lat: c.lat, city: c.city, price: Infinity, by: 'fly' as const, lookup: true }))
}

/**
 * Добавляет к направлениям жильё и пересчитывает итог. Ночей у перелёта — по датам рейса, у машины (даты
 * свободные) — по минимальной длительности поездки. Цены жилья нет (зарубежный город) — итог без жилья.
 */
export function withStay(
  list: Destination[], prices: Map<string, StayPrice>, travelers: number, minDays: number, include: boolean,
): Destination[] {
  const out = list.map((d) => {
    const travel = d.travel ?? d.price
    const source = d.city && prices.get(d.city.id)
    if (!source || d.lookup) return { ...d, travel, stay: undefined, price: travel }
    const days = d.flight ? d.flight.best.trip_days : minDays
    const nights = Math.max(1, days - 1)
    const perNight = source.per_night.amount * Math.max(travelers, 1)
    const stay = { nights, perNight, total: perNight * nights, source }
    return { ...d, travel, stay, price: include ? travel + stay.total : travel }
  })
  return out.sort((a, b) => a.price - b.price)
}
