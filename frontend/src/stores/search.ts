import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Place } from '@/api/client'

/** Ближайшие месяцы вылета в формате YYYY-MM; текущий — только если до конца больше недели. */
export function upcomingMonths(count = 6, now = new Date()): string[] {
  const months: string[] = []
  const daysLeft = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate() - now.getDate()
  const start = daysLeft > 7 ? 0 : 1
  for (let i = start; months.length < count; i++) {
    const d = new Date(now.getFullYear(), now.getMonth() + i, 1)
    months.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`)
  }
  return months
}

/** Параметры поиска: куда, когда и на сколько. Хранятся в URL, чтобы результатом можно было поделиться. */
export const useSearchStore = defineStore('search', () => {
  const destination = ref<Place | null>(null)
  const month = ref(upcomingMonths()[0]!)
  const minDays = ref(5)
  const maxDays = ref(10)
  return { destination, month, minDays, maxDays }
})
