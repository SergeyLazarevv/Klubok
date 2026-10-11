import { defineStore } from 'pinia'
import { useLocalStorage } from '@vueuse/core'
import type { Place } from '@/api/client'

/**
 * Гостевой профиль: хранится в браузере, при регистрации переносится в аккаунт (SRCH-11, SRCH-12).
 */
export const useProfileStore = defineStore('profile', () => {
  const homePlace = useLocalStorage<Place | null>('klubok.home', null, {
    serializer: { read: (v) => (v ? JSON.parse(v) : null), write: (v) => JSON.stringify(v) },
  })
  const hasCar = useLocalStorage('klubok.hasCar', true)
  const radiusKm = useLocalStorage('klubok.radiusKm', 100)

  return { homePlace, hasCar, radiusKm }
})
