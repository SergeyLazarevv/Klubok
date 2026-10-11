<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { watchDebounced } from '@vueuse/core'
import { api, type CarHub } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import CarHubsCard from '@/components/CarHubsCard.vue'
import CarSettingsCard from '@/components/CarSettingsCard.vue'
import PlaceSearch from '@/components/PlaceSearch.vue'
import StepperField from '@/components/StepperField.vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const profile = useProfileStore()
const { homePlace, travelers, tripDays, hasCar } = storeToRefs(profile)

const hubs = ref<CarHub[]>([])
const referenceFuelPrice = ref<number>()
const loading = ref(false)
const error = ref<string>()

const query = computed(() =>
  homePlace.value
    ? {
        from: homePlace.value.id,
        radius_km: profile.radiusKm,
        max_drive_hours: profile.maxDriveHours,
        days: tripDays.value,
        travelers: travelers.value,
        consumption_l100: profile.consumptionL100,
        fuel_type: profile.fuelType,
        fuel_price_rub: profile.fuelPriceRub ?? undefined,
        allow_border: profile.allowBorder,
      }
    : null,
)

let seq = 0

async function loadHubs() {
  const current = ++seq
  if (!query.value || !hasCar.value) {
    hubs.value = []
    return
  }
  loading.value = true
  const { data, error: problem } = await api.GET('/car-hubs', { params: { query: query.value } })
  if (current !== seq) return // пришёл ответ на устаревший запрос
  loading.value = false
  error.value = problem?.errors?.[0]?.message ?? problem?.detail ?? (problem ? 'Не удалось посчитать' : undefined)
  hubs.value = data?.items ?? []
  if (data?.fuel_price_source === 'reference') referenceFuelPrice.value = data.fuel_price.amount
}

watchDebounced([query, hasCar], loadHubs, { debounce: 300, immediate: true, deep: true })
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
        <div class="grid grid-cols-2 gap-4">
          <StepperField v-model="travelers" label="Сколько вас" :min="1" :max="9" />
          <StepperField v-model="tripDays" label="Дней в поездке" :min="1" :max="60" />
        </div>
      </CardContent>
    </Card>

    <CarSettingsCard :reference-fuel-price="referenceFuelPrice" />

    <CarHubsCard
      v-if="homePlace && hasCar"
      :hubs="hubs"
      :loading="loading"
      :error="error"
      :travelers="travelers"
      :trip-days="tripDays"
    />
  </section>
</template>
