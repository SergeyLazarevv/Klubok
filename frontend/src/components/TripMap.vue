<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from 'vue'
import { Maximize2Icon } from '@lucide/vue'
import type { RoadRoute } from '@/api/client'
import { greatCircleKm, type LatLon } from '@/lib/geo'
import { km } from '@/lib/format'
import type { MapLibreMap } from '@/lib/map/maplibre'
import { FIT_PADDING, bounds, createTripMap, onCityClick, setCities, setStops, setTripPoints, setTripRoute, tripBounds } from '@/lib/map/tripLayers'
import { stopPickerKey } from '@/lib/stopPicker'
import { arrivalOf, at, byCar, loadRoad, roadEnd, roadStops, type Trip } from '@/lib/tripMap'
import TripMapDialog from '@/components/TripMapDialog.vue'

const props = defineProps<Trip>()
/** Дорога, как она нарисована на карте, — с заездами в места по пути. */
const emit = defineEmits<{ road: [route: RoadRoute] }>()

const el = useTemplateRef<HTMLDivElement>('mapEl')
const insetEl = useTemplateRef<HTMLDivElement>('inset')
const road = ref<RoadRoute>()
const showInset = ref(false)
const expanded = ref(false)
const pick = ref<string>() // город, нажатый на маленькой карте, — его карточка на большой
let map: MapLibreMap | undefined
let inset: MapLibreMap | undefined
let ready: Promise<void> | undefined

// Города для остановки (ADR-0023) — и здесь: нажатие открывает большую карту с карточкой города
const picker = inject(stopPickerKey, undefined)

// Дорога короче этой доли перелёта на общей карте сливается в точку — показываем её на врезке
const insetRatio = 0.15

// 0 — без перелёта, только дорога
const flightKm = computed(() => (props.option ? greatCircleKm(at(props.option.hub), at(arrivalOf(props))) : 0))

async function renderMarks() {
  await ready
  if (!map) return
  setStops(map, props.stops)
  setCities(map, picker?.suggestions.value, picker?.chosen.value, props.stops)
}
watch(() => [props.stops, picker?.suggestions.value, picker?.chosen.value], renderMarks, { deep: true })

// Дорогу перерисовываем, когда меняется набор мест по пути; последний запрос побеждает
let roadVersion = 0
async function renderRoad() {
  const version = ++roadVersion
  const [drawn] = await Promise.all([loadRoad(props), ready])
  if (!map || version !== roadVersion) return
  setTripRoute(map, props, drawn?.line)
  // Первый раз — масштаб по всей дороге: она бывает заметно в стороне от прямой
  if (version === 1 && drawn) map.fitBounds(tripBounds(props, drawn.line), { padding: FIT_PADDING, duration: 0 })
  if (!drawn) return
  road.value = drawn.route
  emit('road', drawn.route)
  return drawn
}
const viaKey = computed(() => roadStops(props).join(';'))
watch(viaKey, () => void renderRoad())

onMounted(async () => {
  const created = createTripMap(el.value!, props, { compact: true })
  map = created.map
  ready = created.ready.then(() => {
    if (!map) return
    setTripPoints(map, props)
    onCityClick(map, (id) => {
      pick.value = id
      expanded.value = true
    })
  })
  void renderMarks()
  const drawn = await renderRoad()
  if (!map || !drawn) return

  if (!props.option || greatCircleKm(at(props.home), at(props.option.hub)) > flightKm.value * insetRatio) return
  showInset.value = true
  await nextTick()
  if (!map || !insetEl.value) return
  const line: LatLon[] = drawn.line
  const small = createTripMap(insetEl.value, { ...props, option: undefined, destination: roadEnd(props) }, { static: true })
  inset = small.map
  inset.fitBounds(bounds(line), { padding: 14, animate: false })
  await small.ready
  if (!inset) return
  setTripPoints(inset, { ...props, option: undefined, destination: roadEnd(props) })
  setTripRoute(inset, { ...props, option: undefined }, line)
})

watch(expanded, (open) => open || (pick.value = undefined))

onBeforeUnmount(() => {
  inset?.remove()
  map?.remove()
  map = inset = undefined
})
</script>

<template>
  <figure class="flex flex-col gap-2">
    <div class="relative">
      <div ref="mapEl" class="trip-map isolate h-56 overflow-hidden rounded-2xl ring-1 ring-border sm:h-64" />
      <button
        type="button"
        class="absolute top-2 right-2 z-10 flex size-10 items-center justify-center rounded-full bg-card text-foreground shadow-md ring-1 ring-black/5 hover:bg-accent"
        aria-label="Развернуть карту"
        @click="expanded = true"
      >
        <Maximize2Icon class="size-4" />
      </button>
      <div
        v-if="showInset"
        class="absolute bottom-2 left-2 z-10 flex size-28 flex-col overflow-hidden rounded-xl border-2 border-card bg-card shadow-md sm:size-32"
      >
        <div ref="inset" class="trip-map min-h-0 flex-1" />
        <span class="bg-card px-1.5 py-0.5 text-center text-[11px] leading-tight text-muted-foreground">дорога до аэропорта</span>
      </div>
    </div>
    <figcaption class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
      <span v-if="byCar(props)" class="flex items-center gap-1.5">
        <span class="h-1 w-5 rounded-full bg-road" />
        на машине{{ road ? ` · ${km(road.road.distance_km)}` : '' }}{{ road?.road.approximate ? ' (по прямой)' : '' }}
      </span>
      <span v-if="option" class="flex items-center gap-1.5">
        <span class="w-5 border-t-2 border-dashed border-flight" />
        перелёт · ~{{ km(flightKm) }}
      </span>
      <button type="button" class="ml-auto font-medium text-primary hover:underline" @click="expanded = true">
        {{ picker?.suggestions.value?.length ? 'Города, заправки и кафе →' : 'Заправки, кафе и места →' }}
      </button>
    </figcaption>
    <TripMapDialog
      v-model:open="expanded"
      :home="home"
      :option="option"
      :drive="drive"
      :destination="destination"
      :stops="stops"
      :pick="pick"
    />
  </figure>
</template>
