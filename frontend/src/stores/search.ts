import { defineStore } from 'pinia'
import { ref, watch, type InjectionKey } from 'vue'
import type { Favorite, Place, TripSearchParams } from '@/api/client'

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

/**
 * Остановки по пути (ADR-0023) — их даёт менять страница: поиск меняет параметры и пересчитывает выдачу
 * без экрана загрузки, избранное сохраняет остановки в запись. set возвращает текст ошибки или undefined:
 * не вышло — на экране всё как было, ошибку показывает план поездки рядом с городом.
 */
export interface StopsEditor {
  via: (favorite?: Favorite) => string[]
  set: (via: string[], favorite?: Favorite) => Promise<string | undefined>
}
export const stopsEditorKey: InjectionKey<StopsEditor> = Symbol('stopsEditor')

/** Остановка в параметре via: «Q900:1» → город и ночи; «Q900:1:back» — на обратном пути. */
export const VIA_RE = /^(Q\d+):([0-3])(:back)?$/
export const parseVia = (v: string) => {
  const m = VIA_RE.exec(v)
  return m ? { id: m[1]!, nights: Number(m[2]), back: !!m[3] } : undefined
}
export const formatVia = (s: { id: string; nights: number; back: boolean }) => `${s.id}:${s.nights}${s.back ? ':back' : ''}`

/** Параметры поиска: куда, когда и на сколько. Хранятся в URL, чтобы результатом можно было поделиться. */
export const useSearchStore = defineStore('search', () => {
  const destination = ref<Place | null>(null)
  const dateFrom = ref(addDays(todayISO(), 7))
  const dateTo = ref(addDays(todayISO(), 37))
  const minDays = ref(5)
  const maxDays = ref(10)
  // Только выходные (вылет чт–сб, возвращение вс–пн) — приходит из подборки «Куда угодно»
  const weekends = ref(false)
  // Остановки по дороге на машине: «<id города карты>:<ночей>[:back]», туда — по порядку от дома,
  // обратно (back) — от места назначения (ADR-0023)
  const via = ref<string[]>([])
  // Параметры последнего выполненного поиска — с ними вариант сохраняется в избранное
  const lastQuery = ref<TripSearchParams | null>(null)
  // Поиск пересчитывается без экрана загрузки (поменяли остановки) — карточки показывают, что цена обновляется
  const refreshing = ref(false)

  // Начало сдвинулось — конец остаётся в пределах [начало, начало + 60 дней]
  watch(dateFrom, (from) => {
    if (dateTo.value < from) dateTo.value = from
    else if (dateTo.value > maxRangeEnd(from)) dateTo.value = maxRangeEnd(from)
  })

  return { destination, dateFrom, dateTo, minDays, maxDays, weekends, via, lastQuery, refreshing }
})
