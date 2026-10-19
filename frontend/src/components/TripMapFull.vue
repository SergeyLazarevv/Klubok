<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, reactive, ref, useTemplateRef, watch } from 'vue'
import { CheckIcon, LoaderCircleIcon, MapPinnedIcon, MoonIcon, PlusIcon, XIcon } from '@lucide/vue'
import { api, type Poi, type StopSuggestion } from '@/api/client'
import { hours, rub } from '@/lib/format'
import type { MapLibreMap } from '@/lib/map/maplibre'
import {
  FIT_PADDING, createTripMap, onCityClick, setCities, setOfferedVisible, setPoiVisible, setPois, setStops, setTripPoints, setTripRoute, tripBounds,
} from '@/lib/map/tripLayers'
import { categories, type PoiCategory } from '@/lib/poiLayer'
import { detourLabel, stopPickerKey } from '@/lib/stopPicker'
import { byCar, loadRoad, roadEnd, roadStops, type Trip } from '@/lib/tripMap'

const props = defineProps<Trip & { pick?: string }>()

const el = useTemplateRef<HTMLDivElement>('mapEl')
const loading = ref(true)
const failed = ref(false)
const counts = reactive<Record<PoiCategory, number>>({ fuel: 0, food: 0, attraction: 0 })
const visible = reactive<Record<PoiCategory, boolean>>({ fuel: true, food: true, attraction: true })
let map: MapLibreMap | undefined
let ready: Promise<void> | undefined

// Города для остановки (ADR-0023): точки с плашкой «Казань · +15 мин» вдоль дороги, нажатие — карточка снизу
const picker = inject(stopPickerKey, undefined)
const showCities = ref(true)
const picked = ref<StopSuggestion>()
const cityCount = computed(() => picker?.suggestions.value?.length ?? 0)
const pickById = (id?: string) => (picked.value = picker?.suggestions.value?.find((s) => s.city.id === id))

async function renderMarks() {
  await ready
  if (!map) return
  setStops(map, props.stops)
  setCities(map, picker?.suggestions.value, picker?.chosen.value, props.stops)
  setOfferedVisible(map, showCities.value)
}
watch(() => [props.stops, picker?.suggestions.value, picker?.chosen.value, showCities.value], renderMarks, { deep: true })
// Нажали город на маленькой карте — карточка открыта сразу; предложения могли догрузиться позже
watch(() => [props.pick, picker?.suggestions.value], () => props.pick && pickById(props.pick), { immediate: true })

// Поменялись места по пути — перерисовываем дорогу через них; последний запрос побеждает
let roadVersion = 0
async function renderRoad() {
  const version = ++roadVersion
  const [drawn] = await Promise.all([loadRoad(props), ready])
  if (!map || version !== roadVersion) return
  setTripRoute(map, props, drawn?.line)
  if (version === 1 && drawn) map.fitBounds(tripBounds(props, drawn.line), { padding: FIT_PADDING, duration: 0 })
}
watch(() => roadStops(props).join(';'), () => void renderRoad())

async function loadPois(): Promise<Poi[]> {
  const requests = [api.GET('/places/{id}/pois', { params: { path: { id: props.destination.id } } })]
  if (byCar(props)) {
    requests.push(api.GET('/routes/road/pois', { params: { query: { from: props.home.id, to: roadEnd(props).id } } }))
  }
  const results = await Promise.all(requests)
  failed.value = results.some((r) => r.error)
  const byId = new Map<string, Poi>()
  for (const r of results) for (const p of r.data?.items ?? []) byId.set(p.id, p)
  return [...byId.values()]
}

onMounted(async () => {
  const created = createTripMap(el.value!, props, { pois: true })
  map = created.map
  ready = created.ready.then(() => {
    if (!map) return
    setTripPoints(map, props)
    onCityClick(map, pickById)
  })
  void renderMarks()
  void renderRoad()

  const [pois] = await Promise.all([loadPois(), ready])
  loading.value = false
  if (!map) return
  for (const c of categories) counts[c.id] = pois.filter((p) => p.category === c.id).length
  setPois(map, pois)
})

watch(visible, () => {
  if (map) for (const c of categories) setPoiVisible(map, c.id, visible[c.id])
})

// Карта открывается с анимацией — размер контейнера меняется, MapLibre следит за ним сам
onBeforeUnmount(() => {
  map?.remove()
  map = undefined
})

</script>

<template>
  <div class="flex flex-col">
    <div class="flex flex-wrap items-center gap-2 border-b px-4 py-2">
      <button
        v-for="c in categories"
        :key="c.id"
        type="button"
        :aria-pressed="visible[c.id]"
        class="flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm transition-colors"
        :class="visible[c.id] ? `poi-chip-${c.id} border-transparent` : 'bg-card text-muted-foreground hover:bg-accent'"
        @click="visible[c.id] = !visible[c.id]"
      >
        <component :is="c.icon" class="size-3.5" />
        {{ c.label }}
        <span v-if="!loading" class="tabular-nums opacity-75">{{ counts[c.id] }}</span>
      </button>
      <button
        v-if="cityCount"
        type="button"
        :aria-pressed="showCities"
        class="flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm transition-colors"
        :class="showCities ? 'border-transparent bg-[#fde7cc] text-[#7a3d00]' : 'bg-card text-muted-foreground hover:bg-accent'"
        @click="showCities = !showCities"
      >
        <MapPinnedIcon class="size-3.5" /> Остановиться в городе
        <span class="tabular-nums opacity-75">{{ cityCount }}</span>
      </button>
      <span v-if="loading" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <LoaderCircleIcon class="size-3.5 animate-spin" /> Ищем места на OpenStreetMap…
      </span>
      <span v-else-if="failed" class="text-xs text-amber-700">
        Часть мест не загрузилась — OpenStreetMap сейчас перегружен, попробуйте позже.
      </span>
    </div>
    <div class="relative min-h-0 flex-1">
      <div ref="mapEl" class="trip-map isolate size-full" />
      <!-- Карточка выбранного на карте города -->
      <article
        v-if="picker && picked"
        class="absolute inset-x-3 bottom-[calc(0.75rem+env(safe-area-inset-bottom))] z-[1000] flex flex-col gap-3 rounded-2xl bg-card p-4 shadow-xl ring-1 ring-black/5 sm:right-auto sm:w-96"
      >
        <div class="flex items-start gap-3">
          <span class="flex size-10 shrink-0 items-center justify-center rounded-full bg-[#fef3e6] text-road">
            <MapPinnedIcon class="size-5" />
          </span>
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="font-semibold">{{ picked.city.name }}</span>
            <span v-if="picked.city.landmark" class="line-clamp-1 text-sm text-muted-foreground">{{ picked.city.landmark }}</span>
            <span class="text-sm">
              {{ detourLabel(picked) }}{{ picked.detour.duration_min < 10 ? '' : ' к дороге' }} · {{ hours(picked.drive_to_hours) }} от дома
            </span>
            <span v-if="picked.stay_per_night" class="text-xs text-muted-foreground">гостиница ≈ {{ rub(picked.stay_per_night) }} / чел.</span>
            <span v-if="picked.overnight" class="flex items-center gap-1 text-xs font-medium text-[#0d4f22]">
              <MoonIcon class="size-3" /> удобно заночевать: оба дня за рулём в вашем лимите
            </span>
          </div>
          <button
            type="button"
            class="-mt-2 -mr-2 flex size-11 shrink-0 items-center justify-center rounded-full hover:bg-muted"
            aria-label="Закрыть"
            @click="picked = undefined"
          >
            <XIcon class="size-4" />
          </button>
        </div>
        <button
          type="button"
          class="flex h-11 items-center justify-center gap-1.5 rounded-full text-sm font-semibold transition-colors disabled:opacity-60"
          :class="picker.chosen.value.includes(picked.city.id) ? 'text-foreground ring-1 ring-input hover:bg-muted' : 'bg-cta text-cta-foreground hover:bg-cta-hover'"
          :disabled="!!picker.busy.value"
          @click="picker.toggle(picked)"
        >
          <LoaderCircleIcon v-if="picker.busy.value === picked.city.id" class="size-4 animate-spin" />
          <CheckIcon v-else-if="picker.chosen.value.includes(picked.city.id)" class="size-4" />
          <PlusIcon v-else class="size-4" />
          {{ picker.chosen.value.includes(picked.city.id) ? 'В маршруте — убрать' : 'Заехать' }}
        </button>
        <p v-if="picker.error.value === picked.city.id" class="text-sm text-destructive" role="alert">
          Не получилось пересчитать поездку — попробуйте ещё раз.
        </p>
      </article>
    </div>
  </div>
</template>
