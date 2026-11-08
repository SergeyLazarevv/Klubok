import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, type AuthProvider, type Place, type User } from '@/api/client'
import { useFavoritesStore } from '@/stores/favorites'
import { useProfileStore } from '@/stores/profile'

/**
 * Кто вошёл (ADR-0026). Сам вход — переход браузера на /api/v1/auth/{provider}/start: сервер уводит на Яндекс
 * или VK и возвращает обратно уже с сессией пользователя. Маршруты, сохранённые до входа, остаются с ним.
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const providers = ref<AuthProvider[]>([])
  const loaded = ref(false)
  const loginOpen = ref(false) // лист «Войти» — открывается из шапки, ЛК и после неудачного входа
  const loginFailed = ref(false)
  const homePlaceSaving = ref(false)
  const homePlaceSaveError = ref(false)

  const signedIn = computed(() => user.value !== null)
  let homePlaceSaveQueue = Promise.resolve()
  let homePlaceSaveSequence = 0

  async function saveHomePlace(place: Place): Promise<boolean> {
    const userId = user.value?.id
    if (!userId) return false

    const sequence = ++homePlaceSaveSequence
    homePlaceSaving.value = true
    homePlaceSaveError.value = false
    const save = homePlaceSaveQueue.then(async () => {
      if (user.value?.id !== userId) return false
      if (user.value.home_place?.id === place.id) return true
      try {
        const { error } = await api.PUT('/me/home-place', { body: { place_id: place.id } })
        if (error) return false
      } catch {
        return false
      }
      if (user.value?.id === userId) user.value = { ...user.value, home_place: place }
      return user.value?.id === userId
    })
    homePlaceSaveQueue = save.then(() => undefined, () => undefined)
    const saved = await save
    if (sequence === homePlaceSaveSequence) {
      homePlaceSaving.value = false
      homePlaceSaveError.value = !saved
    }
    return saved
  }

  async function load() {
    const { data } = await api.GET('/me').catch(() => ({ data: undefined }))
    if (data) {
      user.value = data.user ?? null
      providers.value = data.providers
      if (user.value?.home_place) {
        useProfileStore().homePlace = user.value.home_place
      } else if (user.value && useProfileStore().homePlace) {
        await saveHomePlace(useProfileStore().homePlace!)
      }
    }
    loaded.value = true
  }

  /** Адрес входа: после него вернёмся на ту же страницу. */
  const loginUrl = (provider: AuthProvider, returnTo: string) =>
    `/api/v1/auth/${provider}/start?return=${encodeURIComponent(returnTo)}`

  async function logout() {
    await fetch('/api/v1/auth/logout', { method: 'POST' })
    user.value = null
    // После выхода браузер начинает с чистой сессии — избранное пользователя больше не показываем
    await useFavoritesStore().load()
  }

  return {
    user, providers, loaded, loginOpen, loginFailed, signedIn, homePlaceSaving, homePlaceSaveError,
    load, loginUrl, logout, saveHomePlace,
  }
})
