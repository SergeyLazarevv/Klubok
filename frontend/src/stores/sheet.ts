import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type TripItem, type TripItemCreate } from '@/api/client'
import { hasPreview, isAutoTitle } from '@/lib/sheet'

/**
 * Путевые листы сохранённых поездок: пункты по id избранного. Одна копия на всё приложение —
 * поездка может быть раскрыта и в результатах поиска, и в избранном.
 */
export const useSheetStore = defineStore('sheet', () => {
  const byFavorite = ref<Record<string, TripItem[]>>({})
  const pending = new Map<string, Promise<void>>()

  /** Загружает один раз; повторный вызов ждёт тот же запрос. */
  function load(favoriteId: string): Promise<void> {
    if (byFavorite.value[favoriteId]) return Promise.resolve()
    let p = pending.get(favoriteId)
    if (!p) {
      p = api
        .GET('/favorites/{id}/items', { params: { path: { id: favoriteId } } })
        .then(({ data }) => {
          if (!data) return
          byFavorite.value[favoriteId] = data.items
          void backfill(favoriteId)
        })
        .finally(() => pending.delete(favoriteId))
      pending.set(favoriteId, p)
    }
    return p
  }

  // Ссылки, добавленные без превью (раньше, чем оно появилось, или сайт тогда не ответил), — подтягиваем
  // картинку и описание один раз за сессию и сохраняем. По одной: страницы объявлений отвечают по несколько секунд
  const tried = new Set<string>()
  async function backfill(favoriteId: string) {
    const todo = (byFavorite.value[favoriteId] ?? []).filter((i) => i.kind === 'link' && i.url && !i.image_url && hasPreview(i.url) && !tried.has(i.id))
    for (const item of todo) {
      tried.add(item.id)
      const { data: p } = await api.GET('/link-preview', { params: { query: { url: item.url! } } })
      if (!p?.image_url && !p?.description) continue
      const { data } = await api.PATCH('/favorites/{id}/items/{item_id}', {
        params: { path: { id: favoriteId, item_id: item.id } },
        body: {
          image_url: p.image_url,
          description: p.description?.slice(0, 500),
          title: isAutoTitle(item.title) && p.title ? p.title.slice(0, 300) : undefined,
        },
      })
      if (data) byFavorite.value[favoriteId] = (byFavorite.value[favoriteId] ?? []).map((i) => (i.id === data.id ? data : i))
    }
  }

  async function add(favoriteId: string, body: TripItemCreate) {
    await load(favoriteId)
    const { data } = await api.POST('/favorites/{id}/items', { params: { path: { id: favoriteId } }, body })
    if (data) byFavorite.value[favoriteId] = [...(byFavorite.value[favoriteId] ?? []), data]
    return data
  }

  /** Отметка «сделано» — сразу в интерфейсе, при ошибке откатываем. */
  async function toggle(favoriteId: string, item: TripItem) {
    const set = (done: boolean) =>
      (byFavorite.value[favoriteId] = (byFavorite.value[favoriteId] ?? []).map((i) => (i.id === item.id ? { ...i, done } : i)))
    set(!item.done)
    const { error } = await api.PATCH('/favorites/{id}/items/{item_id}', {
      params: { path: { id: favoriteId, item_id: item.id } },
      body: { done: !item.done },
    })
    if (error) set(item.done)
  }

  async function remove(favoriteId: string, item: TripItem) {
    const before = byFavorite.value[favoriteId] ?? []
    byFavorite.value[favoriteId] = before.filter((i) => i.id !== item.id)
    const { error } = await api.DELETE('/favorites/{id}/items/{item_id}', { params: { path: { id: favoriteId, item_id: item.id } } })
    if (error) byFavorite.value[favoriteId] = before
  }

  return { byFavorite, load, add, toggle, remove }
})
