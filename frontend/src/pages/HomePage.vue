<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { api, type TripsResponse } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import { upcomingMonths, useSearchStore } from '@/stores/search'
import CarSettings from '@/components/CarSettings.vue'
import SearchForm from '@/components/SearchForm.vue'
import TripResults from '@/components/TripResults.vue'

const profile = useProfileStore()
const { homePlace, travelers } = storeToRefs(profile)
const search = useSearchStore()
const route = useRoute()
const router = useRouter()

const result = ref<TripsResponse>()
const searchedMonth = ref('')
const loading = ref(false)
const error = ref<string>()
const referenceFuelPrice = ref<number>()

async function runSearch() {
  if (!homePlace.value || !search.destination) return
  loading.value = true
  error.value = undefined
  const month = search.month
  router.replace({
    query: { from: homePlace.value.id, to: search.destination.id, month, min: search.minDays, max: search.maxDays },
  })
  const { data, error: problem } = await api.GET('/trips', {
    params: {
      query: {
        from: homePlace.value.id,
        to: search.destination.id,
        month,
        min_days: search.minDays,
        max_days: search.maxDays,
        travelers: travelers.value,
        use_car: profile.hasCar,
        radius_km: profile.radiusKm,
        max_drive_hours: profile.maxDriveHours,
        consumption_l100: profile.consumptionL100,
        fuel_type: profile.fuelType,
        fuel_price_rub: profile.fuelPriceRub ?? undefined,
        allow_border: profile.allowBorder,
      },
    },
  })
  loading.value = false
  if (problem) {
    error.value = problem.errors?.[0]?.message ?? problem.detail ?? 'Не удалось выполнить поиск'
    result.value = undefined
    return
  }
  result.value = data
  searchedMonth.value = month
  if (data?.fuel_price && profile.fuelPriceRub == null) referenceFuelPrice.value = data.fuel_price.amount
}

// Открыли ссылку с параметрами поиска — восстанавливаем их и сразу ищем
onMounted(async () => {
  const { from, to, month, min, max } = route.query
  if (typeof to !== 'string') return
  const { data } = await api.GET('/places/{id}', { params: { path: { id: to } } })
  if (!data) return
  search.destination = data
  // Ссылку открыл человек без своего города — считаем от города автора ссылки
  if (!homePlace.value && typeof from === 'string') {
    const { data: home } = await api.GET('/places/{id}', { params: { path: { id: from } } })
    if (home) homePlace.value = home
  }
  if (typeof month === 'string' && upcomingMonths().includes(month)) search.month = month
  if (Number(min) > 0) search.minDays = Number(min)
  if (Number(max) >= search.minDays) search.maxDays = Number(max)
  await runSearch()
})
</script>

<template>
  <section class="bg-linear-to-b from-[#eef3fb] to-background">
  <div class="mx-auto flex max-w-6xl flex-col gap-5 px-4 pt-6 pb-2 sm:pt-12">
    <header class="flex flex-col gap-1.5">
      <h1 class="text-3xl font-bold tracking-tight sm:text-5xl">Куда поедем?</h1>
      <p class="text-muted-foreground sm:text-lg">
        Считаем всю дорогу: билеты из вашего и соседних городов, бензин и парковку.
      </p>
    </header>
    <div class="flex flex-col gap-3 rounded-3xl bg-card p-2 shadow-[0_1px_3px_rgb(0_0_0/0.08),0_8px_24px_rgb(11_87_208/0.08)] sm:p-3">
      <SearchForm :loading="loading" :disabled="!homePlace || !search.destination" @search="runSearch" />
      <CarSettings :reference-fuel-price="referenceFuelPrice" class="px-1.5 pb-1" />
    </div>
  </div>
  </section>

  <section class="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8">
    <p v-if="error" class="rounded-xl border border-destructive/30 bg-card p-3 text-sm text-destructive">
      {{ error }}
    </p>
    <TripResults v-if="result" :result="result" :travelers="travelers" :month="searchedMonth" />
    <p v-else-if="!loading" class="py-6 text-center text-sm text-muted-foreground">
      Укажите, откуда и куда летите, — найдём самый дешёвый вариант. Настройки сохраняются в этом браузере.
    </p>
  </section>
</template>
