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
const monthFmt = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric' })

/** Дата по местному времени аэропорта: «20 нояб.» — время берём из строки, а не из часового пояса браузера */
export const localDay = (iso: string) => dayFmt.format(new Date(iso.slice(0, 10) + 'T00:00:00Z'))

/** «2026-11» → «ноябрь 2026» */
export const monthLabel = (ym: string) => {
  const [y, m] = ym.split('-').map(Number)
  return monthFmt.format(new Date(y!, m! - 1, 1))
}

export const transfersLabel = (n: number) =>
  n === 0 ? 'прямой' : `${n} ${plural(n, 'пересадка', 'пересадки', 'пересадок')}`

const monthsIn = ['январе', 'феврале', 'марте', 'апреле', 'мае', 'июне', 'июле', 'августе', 'сентябре', 'октябре', 'ноябре', 'декабре']
const monthsShort = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек']

/** «2026-11» → «в ноябре» */
export const inMonth = (ym: string) => `в ${monthsIn[Number(ym.slice(5, 7)) - 1]}`

/** «2026-11» → «Ноя» */
export const monthShort = (ym: string) => monthsShort[Number(ym.slice(5, 7)) - 1]

const dayWeekFmt = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', weekday: 'short', timeZone: 'UTC' })

/** «23 нояб., вс» — по местной дате из строки, как localDay */
export const localDayWeek = (iso: string) => dayWeekFmt.format(new Date(iso.slice(0, 10) + 'T00:00:00Z'))

/** «2026-11-23T06:40:00+04:00» → «06:40» — местное время аэропорта из строки */
export const localTime = (iso: string) => iso.slice(11, 16)
