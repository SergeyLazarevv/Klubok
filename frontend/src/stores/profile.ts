import { defineStore } from 'pinia'
import { useLocalStorage } from '@vueuse/core'
import type { FuelType, Place } from '@/api/client'

const key = (name: string) => `klubok.${name}`

/**
 * Гостевой профиль: хранится в браузере, при регистрации переносится в аккаунт (SRCH-11, SRCH-12).
 * Значения по умолчанию — docs/03-requirements.md, раздел 3.2.
 */
export const useProfileStore = defineStore('profile', () => {
  const homePlace = useLocalStorage<Place | null>(key('home'), null, {
    serializer: { read: (v) => (v ? JSON.parse(v) : null), write: (v) => JSON.stringify(v) },
  })
  const travelers = useLocalStorage(key('travelers'), 1)

  const hasCar = useLocalStorage(key('hasCar'), true)
  const radiusKm = useLocalStorage(key('radiusKm'), 100)
  const maxDriveHours = useLocalStorage(key('maxDriveHours'), 8)
  const consumptionL100 = useLocalStorage(key('consumptionL100'), 8)
  const fuelType = useLocalStorage<FuelType>(key('fuelType'), 'ai95')
  const fuelPriceRub = useLocalStorage<number | null>(key('fuelPriceRub'), null, {
    serializer: { read: (v) => (v ? Number(v) : null), write: (v) => (v == null ? '' : String(v)) },
  })
  const allowBorder = useLocalStorage(key('allowBorder'), false)

  return {
    homePlace, travelers,
    hasCar, radiusKm, maxDriveHours, consumptionL100, fuelType, fuelPriceRub, allowBorder,
  }
})
