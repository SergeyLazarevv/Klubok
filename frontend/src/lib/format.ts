import type { Money } from '@/api/client'

const rubFmt = new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB', maximumFractionDigits: 0 })

/** 327360 копеек → «3 274 ₽» */
export const rub = (m: Money | number) => rubFmt.format((typeof m === 'number' ? m : m.amount) / 100)

/** 6.6 → «6 ч 36 мин» */
export function hours(h: number) {
  const total = Math.round(h * 60)
  const hh = Math.floor(total / 60)
  const mm = total % 60
  return hh ? (mm ? `${hh} ч ${mm} мин` : `${hh} ч`) : `${mm} мин`
}

export const km = (v: number) => `${Math.round(v).toLocaleString('ru-RU')} км`

/** 1 → «1 человек», 2 → «2 человека» */
export function plural(n: number, one: string, few: string, many: string) {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return one
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return few
  return many
}

const dayFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', timeZone: 'UTC' })

/** Дата по местному времени аэропорта: «20 нояб.» — время берём из строки, а не из часового пояса браузера */
export const localDay = (iso: string) => dayFmt.format(new Date(iso.slice(0, 10) + 'T00:00:00Z'))

/** «2026-11-15», «2026-11-22» → «15 нояб. – 22 нояб.»; одна дата — просто «15 нояб.» */
export const dateRangeLabel = (from: string, to: string) => (from === to ? localDay(from) : `${localDay(from)} – ${localDay(to)}`)

export const transfersLabel = (n: number) =>
  n === 0 ? 'прямой' : `${n} ${plural(n, 'пересадка', 'пересадки', 'пересадок')}`

const dayWeekFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', weekday: 'short', timeZone: 'UTC' })

/** «23 нояб., вс» — по местной дате из строки, как localDay */
export const localDayWeek = (iso: string) => dayWeekFmt.format(new Date(iso.slice(0, 10) + 'T00:00:00Z'))

/** «2026-11-23T06:40:00+04:00» → «06:40» — местное время аэропорта из строки */
export const localTime = (iso: string) => iso.slice(11, 16)

/** Дни на месте по светлому времени (days_there): «~2 дня», «~1,5 дня», «меньше полдня» */
export function daysThere(n: number) {
  if (n < 0.5) return 'меньше полдня'
  if (!Number.isInteger(n)) return `~${String(n).replace('.', ',')} дня`
  return `~${n} ${plural(n, 'день', 'дня', 'дней')}`
}
