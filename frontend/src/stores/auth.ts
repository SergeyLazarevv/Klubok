import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, type AuthProvider, type User } from '@/api/client'
import { useFavoritesStore } from '@/stores/favorites'

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

  const signedIn = computed(() => user.value !== null)

  async function load() {
    const { data } = await api.GET('/me').catch(() => ({ data: undefined }))
    if (data) {
      user.value = data.user ?? null
      providers.value = data.providers
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

  return { user, providers, loaded, loginOpen, loginFailed, signedIn, load, loginUrl, logout }
})
