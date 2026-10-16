import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import type { Place, TripSearchParams } from '@/api/client'

const isoDate = (d: Date) => d.toISOString().slice(0, 10)

export function addDays(iso: string, days: number): string {
  const d = new Date(iso + 'T00:00:00Z')
  d.setUTCDate(d.getUTCDate() + days)
  return isoDate(d)
}

export const todayISO = () => isoDate(new Date())

/** Те же лимиты, что на backend (trips.go): начало — не дальше 90 дней, диапазон — до 60 дней. */
export const maxSearchHorizonDays = 90
export const maxRangeDays = 60

/** Самая поздняя дата начала диапазона. */
export const maxSearchDate = () => addDays(todayISO(), maxSearchHorizonDays)

/** Самая поздняя дата конца диапазона при данном начале. */
export const maxRangeEnd = (from: string) => addDays(from, maxRangeDays)

/** Параметры поиска: куда, когда и на сколько. Хранятся в URL, чтобы результатом можно было поделиться. */
export const useSearchStore = defineStore('search', () => {
  const destination = ref<Place | null>(null)
  const dateFrom = ref(addDays(todayISO(), 7))
  const dateTo = ref(addDays(todayISO(), 37))
  const minDays = ref(5)
  const maxDays = ref(10)
  // Только выходные (вылет чт–сб, возвращение вс–пн) — приходит из подборки «Куда угодно»
  const weekends = ref(false)
  // Параметры последнего выполненного поиска — с ними вариант сохраняется в избранное
  const lastQuery = ref<TripSearchParams | null>(null)

  // Начало сдвинулось — конец остаётся в пределах [начало, начало + 60 дней]
  watch(dateFrom, (from) => {
    if (dateTo.value < from) dateTo.value = from
    else if (dateTo.value > maxRangeEnd(from)) dateTo.value = maxRangeEnd(from)
  })

  return { destination, dateFrom, dateTo, minDays, maxDays, weekends, lastQuery }
})
