<script setup lang="ts">
import { computed, ref } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { CarIcon, PlaneTakeoffIcon, TriangleAlertIcon } from '@lucide/vue'
import { api, type Hub } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import PlaceSearch from '@/components/PlaceSearch.vue'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Slider } from '@/components/ui/slider'

const { homePlace, radiusKm } = storeToRefs(useProfileStore())

const hubs = ref<Hub[]>([])
const loading = ref(false)

// Slider в reka-ui работает с массивом значений
const radius = computed({
  get: () => [radiusKm.value],
  set: (v: number[] | undefined) => (radiusKm.value = v?.[0] ?? radiusKm.value),
})

let seq = 0

async function loadHubs() {
  const current = ++seq
  if (!homePlace.value) {
    hubs.value = []
    return
  }
  loading.value = true
  const { data } = await api.GET('/places/{id}/hubs', {
    params: { path: { id: homePlace.value.id }, query: { radius_km: radiusKm.value } },
  })
  if (current !== seq) return // пришёл ответ на устаревший запрос
  hubs.value = data?.items ?? []
  loading.value = false
}

watchDebounced([homePlace, radiusKm], loadHubs, { debounce: 250, immediate: true })

const fmtKm = (km: number) => `${Math.round(km)} км`
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
        <CardDescription>Город сохранится в этом браузере — регистрация не нужна.</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-6">
        <PlaceSearch v-model="homePlace" kind="city" placeholder="Например, Самара" />

        <div class="flex flex-col gap-3">
          <div class="flex items-center justify-between">
            <Label class="flex items-center gap-2"><CarIcon class="size-4" /> Готов ехать на машине</Label>
            <span class="text-sm font-medium tabular-nums">до {{ radiusKm }} км</span>
          </div>
          <Slider v-model="radius" :min="0" :max="1000" :step="25" />
        </div>
      </CardContent>
    </Card>

    <Card v-if="homePlace">
      <CardHeader>
        <CardTitle class="flex items-center gap-2">
          <PlaneTakeoffIcon class="size-5 text-primary" /> Аэропорты рядом
        </CardTitle>
        <CardDescription>
          В радиусе {{ radiusKm }} км от города {{ homePlace.name }}. Пока расстояние по прямой — дорогу и бензин
          посчитаем на следующем шаге.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="loading && !hubs.length" class="flex flex-col gap-3">
          <Skeleton v-for="i in 3" :key="i" class="h-12 w-full" />
        </div>
        <p v-else-if="!hubs.length" class="text-sm text-muted-foreground">
          В этом радиусе аэропортов нет — увеличьте расстояние.
        </p>
        <ul v-else class="divide-y">
          <li v-for="h in hubs" :key="h.place.id" class="flex items-center gap-3 py-3">
            <span class="w-12 font-mono text-sm font-semibold">{{ h.place.iata }}</span>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium">{{ h.place.name }}</div>
              <div class="text-xs text-muted-foreground">{{ h.place.city_name ?? h.place.city_iata }} · {{ h.place.country_code }}</div>
            </div>
            <Badge v-if="h.crosses_border" variant="outline" class="gap-1">
              <TriangleAlertIcon class="size-3" /> граница
            </Badge>
            <span class="w-16 text-right text-sm tabular-nums text-muted-foreground">{{ fmtKm(h.straight_km) }}</span>
          </li>
        </ul>
      </CardContent>
    </Card>
  </section>
</template>
