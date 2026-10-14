import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, type Favorite, type FavoritePick, type FavoriteSnapshot, type TripSearchParams } from '@/api/client'
import { favoriteKey } from '@/lib/favorites'

/**
 * Избранное. Хранится на сервере по анонимной сессии (cookie klubok_sid); после авторизации
 * перейдёт в аккаунт. Здесь — копия списка, чтобы звёздочки в результатах загорались без запросов.
 */
export const useFavoritesStore = defineStore('favorites', () => {
  const items = ref<Favorite[]>([])
  const loaded = ref(false)
  const busy = ref(new Set<string>()) // ключи, по которым идёт запрос — от двойных нажатий
  const lastRemoved = ref<Favorite | null>(null) // для «Отменить», откуда бы ни удалили

  const byKey = computed(() => new Map(items.value.map((f) => [favoriteKey(f.params, f.pick), f])))

  async function load() {
    const { data } = await api.GET('/favorites')
    if (data) items.value = data.items
    loaded.value = true
  }

  const find = (params: TripSearchParams, pick: FavoritePick) => byKey.value.get(favoriteKey(params, pick))

  async function add(title: string, params: TripSearchParams, pick: FavoritePick, snapshot: FavoriteSnapshot) {
    const key = favoriteKey(params, pick)
    if (busy.value.has(key)) return
    busy.value.add(key)
    try {
      const { data } = await api.POST('/favorites', { body: { title, params, pick, snapshot } })
      if (data && !items.value.some((f) => f.id === data.id)) items.value = [data, ...items.value]
      return data
    } finally {
      busy.value.delete(key)
    }
  }

  async function remove(fav: Favorite) {
    const before = items.value
    items.value = items.value.filter((f) => f.id !== fav.id) // сразу убираем из списка, при ошибке вернём
    const { error } = await api.DELETE('/favorites/{id}', { params: { path: { id: fav.id } } })
    if (error) items.value = before
    else lastRemoved.value = fav
  }

  /** Вернуть последнее удалённое (кнопка «Отменить»): сервер заведёт запись заново с теми же данными. */
  async function undoRemove() {
    const fav = lastRemoved.value
    if (!fav) return
    lastRemoved.value = null
    return add(fav.title, fav.params, fav.pick, fav.snapshot)
  }

  return { items, loaded, lastRemoved, load, find, add, remove, undoRemove }
})
