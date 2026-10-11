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
