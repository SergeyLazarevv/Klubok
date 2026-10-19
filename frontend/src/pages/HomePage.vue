<script setup lang="ts">
import { computed, onMounted, provide, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { CalendarDaysIcon, PencilIcon, XIcon } from '@lucide/vue'
import { api, type Problem, type TripsResponse } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import { maxRangeEnd, maxSearchDate, stopsEditorKey, todayISO, useSearchStore, VIA_RE } from '@/stores/search'
import { dateRangeLabel, plural } from '@/lib/format'
import { collapseAll, useIsPhone } from '@/lib/tripOpen'
import CarSettings from '@/components/CarSettings.vue'
import SearchLoader from '@/components/SearchLoader.vue'
import SearchForm from '@/components/SearchForm.vue'
import TripResults from '@/components/TripResults.vue'

const profile = useProfileStore()
const { homePlace, travelers } = storeToRefs(profile)
const search = useSearchStore()
const route = useRoute()
const router = useRouter()

const result = ref<TripsResponse>()
const searchedDateFrom = ref('')
const searchedDateTo = ref('')
const loading = ref(false)
const error = ref<string>()
const referenceFuelPrice = ref<number>()
// На телефоне после поиска форма сворачивается в строку с параметрами — результаты сразу на первом экране
const phone = useIsPhone()
const editing = ref(true)
const collapsed = computed(() => phone.value && !!result.value && !editing.value)

function editSearch() {
  editing.value = true
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

/**
 * keepTrip — открытый вариант (?trip=) остаётся открытым: поиск по ссылке или пересчёт.
 * quiet — пересчёт без экрана загрузки: поменяли остановки по пути в открытом варианте.
 */
async function runSearch(keepTrip = false, quiet = false): Promise<boolean> {
  if (!homePlace.value || !search.destination) return false
  // Остановки — для конкретной дороги: поменяли откуда или куда — их больше нет
  const last = search.lastQuery
  if (!quiet && last && (last.from !== homePlace.value.id || last.to !== search.destination.id)) search.via = []
  if (quiet) search.refreshing = true
  else {
    loading.value = true
    collapseAll()
  }
  error.value = undefined
  const { dateFrom, dateTo } = search
  const via = search.via.length ? [...search.via] : undefined
  const url = {
    query: {
      from: homePlace.value.id,
      to: search.destination.id,
      date_from: dateFrom,
      date_to: dateTo,
      min: search.minDays,
      max: search.maxDays,
      weekends: search.weekends ? '1' : undefined,
      via,
      trip: keepTrip ? route.query.trip : undefined,
    },
  }
  // Пересчёт меняет адрес только после ответа: не вышло — ссылка остаётся на то, что на экране
  if (!quiet) router.replace(url)
  const query = {
    from: homePlace.value.id,
    to: search.destination.id,
    date_from: dateFrom,
    date_to: dateTo,
    min_days: search.minDays,
    max_days: search.maxDays,
    weekends: search.weekends || undefined,
    travelers: travelers.value,
    use_car: profile.hasCar,
    radius_km: profile.radiusKm,
    max_drive_hours: profile.maxDriveHours,
    consumption_l100: profile.consumptionL100,
    fuel_type: profile.fuelType,
    fuel_price_rub: profile.fuelPriceRub ?? undefined,
    allow_border: profile.allowBorder,
    via,
  }
  const { data, error: problem } = await api.GET('/trips', { params: { query } }).catch(() => ({ data: undefined, error: {} as Problem }))
  loading.value = false
  search.refreshing = false
  if (problem || !data) {
    // Пересчёт не удался — экран не трогаем: прежний результат остаётся, сообщение — рядом с тем, что меняли
    if (quiet) return false
    error.value = problem?.errors?.[0]?.message ?? problem?.detail ?? 'Не удалось выполнить поиск'
    result.value = undefined
    return false
  }
  if (quiet) router.replace(url)
  result.value = data
  searchedDateFrom.value = dateFrom
  searchedDateTo.value = dateTo
  search.lastQuery = query
  editing.value = false
  if (data.fuel_price && profile.fuelPriceRub == null) referenceFuelPrice.value = data.fuel_price.amount
  return true
}

// Открыли ссылку с параметрами поиска — восстанавливаем их и сразу ищем
onMounted(async () => {
  const { from, to, date_from: dateFrom, date_to: dateTo, min, max, weekends } = route.query
  if (typeof to !== 'string') return
  const { data } = await api.GET('/places/{id}', { params: { path: { id: to } } })
  if (!data) return
  search.destination = data
  // Ссылку открыл человек без своего города — считаем от города автора ссылки
  if (!homePlace.value && typeof from === 'string') {
    const { data: home } = await api.GET('/places/{id}', { params: { path: { id: from } } })
    if (home) homePlace.value = home
  }
  const minDate = todayISO()
  const maxDate = maxSearchDate()
  if (typeof dateFrom === 'string' && dateFrom >= minDate && dateFrom <= maxDate) search.dateFrom = dateFrom
  if (typeof dateTo === 'string' && dateTo >= search.dateFrom && dateTo <= maxRangeEnd(search.dateFrom)) search.dateTo = dateTo
  if (Number(min) > 0) search.minDays = Number(min)
  if (Number(max) >= search.minDays) search.maxDays = Number(max)
  search.weekends = weekends === '1'
  const via = route.query.via
  search.via = (Array.isArray(via) ? via : [via]).filter((v): v is string => typeof v === 'string' && VIA_RE.test(v))
  search.lastQuery = null // остановки из ссылки — для её маршрута, а не прошлого поиска
  await runSearch(true)
})

// Остановки в результатах поиска — в параметрах: меняем их и пересчитываем без экрана загрузки;
// не вышло — остановки как были
provide(stopsEditorKey, {
  via: () => search.via,
  set: async (via) => {
    const before = search.via
    search.via = via
    if (await runSearch(true, true)) return undefined
    search.via = before
    return 'Не получилось пересчитать поездку — остановки остались как были. Попробуйте ещё раз.'
  },
})
</script>

<template>
  <SearchLoader v-if="loading" />
  <!-- Свёрнутый поиск: что ищем, по нажатию — снова форма -->
  <div v-if="collapsed" class="sticky top-0 z-30 bg-background/95 px-4 py-2 backdrop-blur">
    <button
      type="button"
      class="flex w-full items-center gap-3 rounded-2xl bg-card px-4 py-2.5 text-left shadow-[0_1px_3px_rgb(0_0_0/0.08),0_4px_16px_rgb(11_87_208/0.08)] ring-1 ring-border active:bg-muted"
      aria-label="Изменить параметры поиска"
      @click="editSearch"
    >
      <span class="flex min-w-0 flex-1 flex-col">
        <span class="truncate font-semibold">{{ homePlace?.name }} → {{ search.destination?.name }}</span>
        <span class="truncate text-xs text-muted-foreground">
          {{ dateRangeLabel(search.dateFrom, search.dateTo) }} · {{ search.minDays }}–{{ search.maxDays }}
          {{ plural(search.maxDays, 'день', 'дня', 'дней') }} · {{ travelers }} {{ plural(travelers, 'человек', 'человека', 'человек') }}
        </span>
      </span>
      <span class="flex size-10 shrink-0 items-center justify-center rounded-full bg-muted text-primary">
        <PencilIcon class="size-4" />
      </span>
    </button>
  </div>
  <section v-else class="bg-linear-to-b from-[#eef3fb] to-background">
  <div class="mx-auto flex max-w-6xl flex-col gap-4 px-4 pt-4 pb-2 sm:gap-5 sm:pt-12">
    <header class="flex flex-col gap-1.5">
      <h1 class="text-2xl font-bold tracking-tight sm:text-5xl">Куда поедем?</h1>
      <p class="text-muted-foreground max-sm:hidden sm:text-lg">
        Считаем всю дорогу: билеты из вашего и соседних городов, бензин и парковку.
      </p>
    </header>
    <div class="flex flex-col gap-3 rounded-3xl bg-card p-2 shadow-[0_1px_3px_rgb(0_0_0/0.08),0_8px_24px_rgb(11_87_208/0.08)] sm:p-3">
      <SearchForm :loading="loading" :disabled="!homePlace || !search.destination" @search="runSearch" />
      <div class="flex flex-col items-start gap-2 px-1.5 pb-1">
        <CarSettings :reference-fuel-price="referenceFuelPrice" class="self-stretch" />
        <button
          v-if="search.weekends"
          type="button"
          class="flex items-center gap-1.5 rounded-full bg-muted px-3 py-1 text-sm transition-colors hover:bg-muted/70"
          title="Убрать фильтр и искать на любые дни"
          @click="search.weekends = false; runSearch()"
        >
          <CalendarDaysIcon class="size-4 text-muted-foreground" />
          Только выходные: вылет чт–сб, обратно вс–пн
          <XIcon class="size-3.5 text-muted-foreground" />
        </button>
      </div>
    </div>
  </div>
  </section>

  <section class="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-6 sm:py-8">
    <p v-if="error" class="rounded-xl border border-destructive/30 bg-card p-3 text-sm text-destructive">
      {{ error }}
    </p>
    <TripResults v-if="result" :result="result" :travelers="travelers" :date-from="searchedDateFrom" :date-to="searchedDateTo" />
    <p v-else-if="!loading" class="py-6 text-center text-sm text-muted-foreground">
      Укажите, откуда и куда летите, — найдём самый дешёвый вариант. Настройки сохраняются в этом браузере.
    </p>
  </section>
</template>
