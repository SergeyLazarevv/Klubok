<script setup lang="ts">
import { onUnmounted, reactive, watch } from 'vue'
import { RotateCcwIcon, SearchIcon, StarIcon, TrashIcon } from '@lucide/vue'
import { api, type Favorite, type FavoriteCurrent } from '@/api/client'
import { hours, localDay, plural, rub } from '@/lib/format'
import { useFavoritesStore } from '@/stores/favorites'
import DriveOptionCard from '@/components/DriveOptionCard.vue'
import TripOptionCard from '@/components/TripOptionCard.vue'

const favorites = useFavoritesStore()

// Актуальные цены подгружаются фоном по каждому маршруту; список сразу показывает цены на момент сохранения
const current = reactive(new Map<string, FavoriteCurrent | 'loading'>())

async function refresh(fav: Favorite) {
  current.set(fav.id, 'loading')
  const { data } = await api.GET('/favorites/{id}/current', { params: { path: { id: fav.id } } })
  current.set(fav.id, data ?? { status: 'unavailable' })
}

// Пересчёт по два маршрута одновременно — не нагружаем источник цен
async function refreshAll(list: Favorite[]) {
  const queue = list.filter((f) => !current.has(f.id))
  const worker = async () => {
    for (let f = queue.shift(); f; f = queue.shift()) await refresh(f)
  }
  await Promise.all([worker(), worker()])
}
watch(() => favorites.items, refreshAll, { immediate: true })

async function undo() {
  const fav = await favorites.undoRemove()
  if (fav) await refresh(fav)
}
onUnmounted(() => (favorites.lastRemoved = null)) // «Отменить» — только пока страница открыта

const cur = (f: Favorite) => {
  const c = current.get(f.id)
  return c === 'loading' ? undefined : c
}
const searchLink = (f: Favorite) => ({
  path: '/',
  query: {
    from: f.params.from,
    to: f.params.to,
    date_from: f.params.date_from,
    date_to: f.params.date_to,
    min: f.params.min_days,
    max: f.params.max_days,
  },
})
const statusText: Record<string, string> = {
  replaced: 'Сохранённого варианта больше нет — вот лучший с теми же параметрами',
  past: 'Даты поездки уже прошли',
  empty: 'Сейчас вариантов нет — источник не нашёл цен на эти даты',
  unavailable: 'Не удалось обновить цену — источник недоступен, попробуйте позже',
}
</script>

<template>
  <section class="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-8 sm:py-12">
    <header class="flex flex-col gap-1.5">
      <h1 class="flex items-center gap-2 text-3xl font-bold tracking-tight sm:text-4xl">
        <StarIcon class="size-7 fill-amber-400 text-amber-500" /> Избранное
      </h1>
      <p class="text-muted-foreground">
        Сохранённые маршруты с актуальной ценой. Нажмите ☆ на любом варианте в поиске, чтобы добавить его сюда.
      </p>
    </header>

    <div v-if="favorites.lastRemoved" class="flex items-center justify-between gap-3 rounded-2xl bg-muted px-4 py-3 text-sm">
      <span>«{{ favorites.lastRemoved.title }}» убран из избранного</span>
      <button type="button" class="flex items-center gap-1.5 font-medium text-primary" @click="undo">
        <RotateCcwIcon class="size-4" /> Отменить
      </button>
    </div>

    <p v-if="favorites.loaded && !favorites.items.length" class="rounded-3xl bg-muted p-6 text-center text-muted-foreground">
      Пока пусто. Найдите поездку и нажмите ☆ на понравившемся варианте.
      <RouterLink to="/" class="mt-3 flex items-center justify-center gap-1.5 font-medium text-primary">
        <SearchIcon class="size-4" /> К поиску
      </RouterLink>
    </p>

    <article v-for="f in favorites.items" :key="f.id" class="flex flex-col gap-3">
      <header class="flex flex-wrap items-end justify-between gap-x-4 gap-y-2 px-1">
        <div class="flex flex-col gap-0.5">
          <h2 class="text-xl font-semibold tracking-tight">{{ f.title }}</h2>
          <span class="text-sm text-muted-foreground">
            <template v-if="f.pick.kind === 'drive'">на машине<template v-if="f.snapshot.drive_hours">, {{ hours(f.snapshot.drive_hours) }} в одну сторону</template></template>
            <template v-else>
              вылет: {{ f.snapshot.hub_name ?? f.pick.hub_iata }}
              <template v-if="f.snapshot.depart_at && f.snapshot.return_at">· {{ localDay(f.snapshot.depart_at) }} – {{ localDay(f.snapshot.return_at) }}</template>
            </template>
            · {{ f.params.travelers ?? 1 }} {{ plural(f.params.travelers ?? 1, 'человек', 'человека', 'человек') }}
            · сохранено {{ localDay(f.created_at) }} за {{ rub(f.snapshot.total) }}
          </span>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <span v-if="current.get(f.id) === 'loading'" class="text-sm text-muted-foreground">обновляем цену…</span>
          <span
            v-else-if="cur(f)?.status === 'found' && cur(f)?.diff"
            class="rounded-full px-3 py-1 text-sm font-medium whitespace-nowrap tabular-nums"
            :class="cur(f)!.diff!.amount < 0 ? 'bg-[#c4eed0] text-[#072711]' : cur(f)!.diff!.amount > 0 ? 'bg-amber-100 text-amber-900' : 'bg-muted text-muted-foreground'"
          >
            <template v-if="cur(f)!.diff!.amount < 0">подешевело на {{ rub(-cur(f)!.diff!.amount) }}</template>
            <template v-else-if="cur(f)!.diff!.amount > 0">подорожало на {{ rub(cur(f)!.diff!.amount) }}</template>
            <template v-else>цена не изменилась</template>
          </span>
          <RouterLink
            :to="searchLink(f)"
            class="flex h-10 items-center gap-1.5 rounded-full px-3 text-sm font-medium whitespace-nowrap ring-1 ring-input hover:bg-muted sm:px-4"
            title="Открыть поиск с этими параметрами"
            aria-label="Искать заново"
          >
            <SearchIcon class="size-4" /> <span class="hidden sm:inline">Искать заново</span>
          </RouterLink>
          <button
            type="button"
            class="flex size-10 items-center justify-center rounded-full ring-1 ring-input hover:bg-muted"
            aria-label="Удалить из избранного"
            title="Удалить из избранного"
            @click="favorites.remove(f)"
          >
            <TrashIcon class="size-4 text-muted-foreground" />
          </button>
        </div>
      </header>

      <p v-if="cur(f) && cur(f)!.status !== 'found'" class="px-1 text-sm text-muted-foreground">{{ statusText[cur(f)!.status] }}</p>

      <template v-if="cur(f)?.result">
        <TripOptionCard
          v-if="cur(f)!.option"
          :option="cur(f)!.option!"
          :home="cur(f)!.result!.home"
          :destination="cur(f)!.result!.destination"
          :travelers="f.params.travelers ?? 1"
          :date-from="f.params.date_from"
          :search-params="f.params"
          :favorite="f"
        />
        <DriveOptionCard
          v-else-if="cur(f)!.drive"
          :drive="cur(f)!.drive!"
          :home="cur(f)!.result!.home"
          :destination="cur(f)!.result!.destination"
          :travelers="f.params.travelers ?? 1"
          :date-from="f.params.date_from"
          :search-params="f.params"
          :favorite="f"
        />
      </template>
    </article>
  </section>
</template>
