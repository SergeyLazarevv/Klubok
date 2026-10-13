<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef } from 'vue'
import type L from 'leaflet'
import { Maximize2Icon } from '@lucide/vue'
import type { RoadRoute } from '@/api/client'
import { greatCircleKm } from '@/lib/geo'
import { km } from '@/lib/format'
import { arrivalOf, at, byCar, createMap, drawRoad, drawTrip, marker, roadEnd, roadLine, type Trip } from '@/lib/tripMap'
import TripMapDialog from '@/components/TripMapDialog.vue'

const props = defineProps<Trip>()

const el = useTemplateRef<HTMLDivElement>('map')
const insetEl = useTemplateRef<HTMLDivElement>('inset')
const road = ref<RoadRoute>()
const showInset = ref(false)
const expanded = ref(false)
let map: L.Map | undefined
let inset: L.Map | undefined

// Дорога короче этой доли перелёта на общей карте сливается в точку — показываем её на врезке
const insetRatio = 0.15

// 0 — без перелёта, только дорога
const flightKm = computed(() => (props.option ? greatCircleKm(at(props.option.hub), at(arrivalOf(props))) : 0))

onMounted(async () => {
  map = createMap(el.value!)
  map.fitBounds(drawTrip(map, props), { padding: [32, 32] })

  const drawn = await drawRoad(map, props)
  if (!map || !drawn) return
  road.value = drawn.route

  if (!props.option || greatCircleKm(at(props.home), at(props.option.hub)) > flightKm.value * insetRatio) return
  showInset.value = true
  await nextTick()
  if (!map || !insetEl.value) return
  inset = createMap(insetEl.value, {
    attributionControl: false,
    zoomControl: false,
    dragging: false,
    touchZoom: false,
    doubleClickZoom: false,
    boxZoom: false,
    keyboard: false,
  })
  const insetRoad = roadLine(drawn.line, drawn.route.road.approximate).addTo(inset)
  marker(props.home, 'trip-map-home').addTo(inset)
  marker(roadEnd(props), 'trip-map-hub').addTo(inset)
  inset.fitBounds(insetRoad.getBounds(), { padding: [14, 14] })
})

onBeforeUnmount(() => {
  inset?.remove()
  map?.remove()
  map = inset = undefined
})
</script>

<template>
  <figure class="flex flex-col gap-2">
    <div class="relative">
      <div ref="map" class="trip-map isolate h-56 overflow-hidden rounded-lg border sm:h-64" />
      <button
        type="button"
        class="absolute top-2 right-2 z-10 flex size-8 items-center justify-center rounded-md border bg-card text-foreground shadow-sm hover:bg-accent"
        aria-label="Развернуть карту"
        @click="expanded = true"
      >
        <Maximize2Icon class="size-4" />
      </button>
      <div
        v-if="showInset"
        class="absolute bottom-2 left-2 z-10 flex size-28 flex-col overflow-hidden rounded-md border-2 border-card bg-card shadow-md sm:size-32"
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
        Заправки, кафе и места →
      </button>
    </figcaption>
    <TripMapDialog v-model:open="expanded" :home="home" :option="option" :drive="drive" :destination="destination" />
  </figure>
</template>
