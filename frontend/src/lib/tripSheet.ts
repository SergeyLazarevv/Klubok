import { computed, ref, watch, type InjectionKey, type Ref } from 'vue'
import type { Favorite, FavoritePick, FavoriteSnapshot, TripItem, TripItemCreate, TripItemStage, TripSearchParams } from '@/api/client'
import { useFavoritesStore } from '@/stores/favorites'
import { useSheetStore } from '@/stores/sheet'

/**
 * Путевой лист поездки. Это не отдельная сущность, а продолжение избранного: пункты привязаны к сохранённой
 * поездке. Первый же добавленный пункт сохраняет поездку в избранное — пользователю не нужно сначала жать ☆.
 */
export function useTripSheet(opts: {
  favorite: Ref<Favorite | undefined> // явная запись: в избранном вариант мог смениться (replaced), а лист — тот же
  params: Ref<TripSearchParams | undefined>
  pick: Ref<FavoritePick>
  snapshot: Ref<FavoriteSnapshot>
  title: Ref<string>
}) {
  const favorites = useFavoritesStore()
  const sheets = useSheetStore()

  const favorite = computed(() => opts.favorite.value ?? (opts.params.value ? favorites.find(opts.params.value, opts.pick.value) : undefined))
  /** Без параметров поиска сохранить поездку нечем — тогда лист только для чтения (и пустой). */
  const enabled = computed(() => !!favorite.value || !!opts.params.value)
  const items = computed<TripItem[]>(() => (favorite.value && sheets.byFavorite[favorite.value.id]) || [])
  const justSaved = ref(false)

  watch(
    () => favorite.value?.id,
    (id) => id && void sheets.load(id),
    { immediate: true },
  )

  async function add(body: TripItemCreate) {
    let fav = favorite.value
    if (!fav && opts.params.value) {
      fav = await favorites.add(opts.title.value, opts.params.value, opts.pick.value, opts.snapshot.value)
      justSaved.value = !!fav
    }
    if (fav) return sheets.add(fav.id, body)
  }

  const byStage = (stage: TripItemStage) => items.value.filter((i) => i.stage === stage)
  const toggle = (item: TripItem) => favorite.value && sheets.toggle(favorite.value.id, item)
  const remove = (item: TripItem) => favorite.value && sheets.remove(favorite.value.id, item)
  // Ссылки — варианты (жильё, аренда): в итог идёт только выбранный. Дела и места с ценой — всегда
  const counted = (i: TripItem) => i.kind !== 'link' || i.done
  const extra = computed(() => items.value.reduce((sum, i) => sum + (counted(i) ? (i.price?.amount ?? 0) : 0), 0))
  /** Варианты с ценой, которые не выбраны и в итог не вошли. */
  const unchosen = computed(() => items.value.filter((i) => !counted(i) && i.price).length)

  return { favorite, enabled, items, justSaved, add, byStage, toggle, remove, extra, unchosen }
}

export type TripSheet = ReturnType<typeof useTripSheet>
export const tripSheetKey: InjectionKey<TripSheet> = Symbol('tripSheet')
