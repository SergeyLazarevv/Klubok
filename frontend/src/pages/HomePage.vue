<script setup lang="ts">
import { computed, nextTick, onMounted, provide, ref } from 'vue'
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
import DestinationPostcards from '@/components/home/DestinationPostcards.vue'
import ExploreBanner from '@/components/home/ExploreBanner.vue'
import HeroScene from '@/components/home/HeroScene.vue'
import HowItWorks from '@/components/home/HowItWorks.vue'
import type { Postcard } from '@/lib/home'

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

// Открытка с главной: город встаёт в «Куда». Свой город известен — сразу ищем, нет — просим его указать
const picking = ref<string>()
async function pickPostcard(card: Postcard) {
  picking.value = card.id
  const { data } = await api
    .GET('/places', { params: { query: { q: card.name, kind: 'city', limit: 1 } } })
    .catch(() => ({ data: undefined }))
  picking.value = undefined
  const place = data?.items[0]
  if (!place) {
    error.value = `Не нашли ${card.name} — попробуйте ввести город вручную`
    return
  }
  search.destination = place
  window.scrollTo({ top: 0, behavior: 'smooth' })
  if (homePlace.value) {
    await runSearch()
    return
  }
  await nextTick()
  document.getElementById('search-from')?.focus({ preventScroll: true })
}

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
  <div v-if="collapsed" class="sticky top-0 z-30 bg-background/90 px-4 py-2 backdrop-blur">
    <button
      type="button"
      class="flex w-full items-center gap-3 rounded-[20px] bg-card px-4 py-2.5 text-left shadow-[0_8px_24px_-12px_rgb(31_58_95/0.3)] ring-1 ring-border active:bg-muted"
      aria-label="Изменить параметры поиска"
      @click="editSearch"
    >
      <span class="flex min-w-0 flex-1 flex-col">
        <span class="truncate font-extrabold text-ink">{{ homePlace?.name }} → {{ search.destination?.name }}</span>
        <span class="truncate text-xs text-muted-foreground">
          {{ dateRangeLabel(search.dateFrom, search.dateTo) }} · {{ search.minDays }}–{{ search.maxDays }}
          {{ plural(search.maxDays, 'день', 'дня', 'дней') }} · {{ travelers }} {{ plural(travelers, 'человек', 'человека', 'человек') }}
        </span>
      </span>
      <span class="flex size-10 shrink-0 items-center justify-center rounded-full bg-accent text-accent-foreground">
        <PencilIcon class="size-4" />
      </span>
    </button>
  </div>
  <section v-else class="hero relative">
    <div class="mx-auto flex max-w-6xl flex-col gap-5 px-4 pt-5 pb-2 sm:gap-8 sm:pt-10 lg:pt-6">
      <div class="grid items-center gap-6 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
        <header class="flex flex-col gap-3 sm:gap-5">
          <h1 class="font-display text-[2.15rem] leading-[1.05] font-semibold tracking-tight text-ink sm:text-6xl lg:text-7xl">
            Куда
            <span class="relative inline-block whitespace-nowrap">
              поедем<span class="text-cta">?</span>
              <!-- Нить под словом: волна с петелькой -->
              <svg
                class="absolute -bottom-[0.2em] left-0 h-[0.34em] w-full overflow-visible"
                viewBox="0 0 200 24"
                preserveAspectRatio="none"
                fill="none"
                aria-hidden="true"
              >
                <path
                  class="underline-thread"
                  d="M2 14 C30 4 50 22 78 12 S120 2 132 12 C140 20 128 26 124 18 C120 10 140 6 160 10 S190 14 198 8"
                  pathLength="1"
                  stroke="var(--yarn)"
                  stroke-width="5"
                  stroke-linecap="round"
                  vector-effect="non-scaling-stroke"
                />
              </svg>
            </span>
          </h1>
          <p class="max-w-xl text-[17px] leading-relaxed text-foreground/75 max-sm:hidden sm:text-xl">
            Клубок соберёт цены на всю дорогу — билеты из вашего и соседних городов, бензин, парковку и ночёвки, — чтобы вы
            сразу увидели, как выйдет дешевле.
          </p>
        </header>
        <HeroScene class="max-lg:hidden" />
      </div>

      <div
        class="relative flex flex-col gap-3 rounded-[28px] bg-card p-2.5 shadow-[0_2px_4px_rgb(31_58_95/0.04),0_20px_50px_-16px_rgb(31_58_95/0.28)] ring-1 ring-border/80 sm:p-3"
      >
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

  <section v-if="error || result" class="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-6 sm:py-8">
    <p v-if="error" class="rounded-2xl border border-destructive/30 bg-card p-3 text-sm text-destructive">
      {{ error }}
    </p>
    <TripResults v-if="result" :result="result" :travelers="travelers" :date-from="searchedDateFrom" :date-to="searchedDateTo" />
  </section>

  <!-- Пока ничего не искали — главная рассказывает, куда и как можно поехать -->
  <template v-if="!result">
    <div class="mx-auto max-w-6xl pt-10 sm:px-4 sm:pt-16">
      <DestinationPostcards :picking="picking" @pick="pickPostcard" />
    </div>
    <div class="mx-auto max-w-6xl px-4 pt-14 sm:pt-24">
      <HowItWorks />
    </div>
    <div class="mx-auto max-w-6xl px-4 pt-14 pb-10 sm:pt-24 sm:pb-16">
      <ExploreBanner />
    </div>
    <footer class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 border-t px-4 py-6 text-sm text-muted-foreground">
      <span><span class="font-display font-semibold text-ink">клубок</span> — ведёт туда, где дешевле</span>
      <span>Настройки сохраняются в этом браузере</span>
    </footer>
  </template>
</template>

<style scoped>
/* Небо над героем переходит в бумагу; мягкие пятна неба и солнца — без картинок */
.hero {
  background:
    radial-gradient(60rem 28rem at 85% -10%, rgb(220 239 255 / 0.9), transparent 70%),
    radial-gradient(40rem 22rem at 0% 0%, rgb(252 235 198 / 0.7), transparent 70%);
}

.underline-thread {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: underline-draw 1.1s cubic-bezier(0.65, 0, 0.35, 1) 0.3s forwards;
}
@keyframes underline-draw {
  to {
    stroke-dashoffset: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .underline-thread {
    animation: none;
    stroke-dashoffset: 0;
  }
}
</style>
