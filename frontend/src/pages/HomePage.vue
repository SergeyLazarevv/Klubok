<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { api, type TripsResponse } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import { upcomingMonths, useSearchStore } from '@/stores/search'
import CarSettingsCard from '@/components/CarSettingsCard.vue'
import PlaceSearch from '@/components/PlaceSearch.vue'
import StepperField from '@/components/StepperField.vue'
import TripResults from '@/components/TripResults.vue'
import TripSearchCard from '@/components/TripSearchCard.vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

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
  <section class="mx-auto flex max-w-2xl flex-col gap-6 px-4 py-8 sm:py-12">
    <header class="flex flex-col gap-2">
      <h1 class="text-3xl font-bold tracking-tight sm:text-4xl">Путешествуйте дешевле</h1>
      <p class="text-muted-foreground">
        Klubok ищет билеты не только из вашего города, но и из соседних — и честно считает дорогу до них.
      </p>
    </header>

    <Card>
      <CardHeader>
        <CardTitle>Откуда вы?</CardTitle>
        <CardDescription>Настройки сохранятся в этом браузере — регистрация не нужна.</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <PlaceSearch v-model="homePlace" kind="city" placeholder="Например, Самара" />
        <StepperField v-model="travelers" label="Сколько вас едет" :min="1" :max="9" class="max-w-48" />
      </CardContent>
    </Card>

    <CarSettingsCard :reference-fuel-price="referenceFuelPrice" />

    <TripSearchCard :loading="loading" :disabled="!homePlace || !search.destination" @search="runSearch" />

    <p v-if="error" class="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
      {{ error }}
    </p>
    <TripResults v-if="result" :result="result" :travelers="travelers" :month="searchedMonth" />
  </section>
</template>
