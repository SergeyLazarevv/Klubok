<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, useTemplateRef, watch } from 'vue'
import type L from 'leaflet'
import { useResizeObserver } from '@vueuse/core'
import { LoaderCircleIcon } from '@lucide/vue'
import { api, type Poi } from '@/api/client'
import { byCar, createMap, drawRoad, drawTrip, roadEnd, type Trip } from '@/lib/tripMap'
import { categories, poiCluster, poiMarker, type PoiCategory } from '@/lib/poiLayer'

const props = defineProps<Trip>()

const el = useTemplateRef<HTMLDivElement>('map')
const loading = ref(true)
const failed = ref(false)
const counts = reactive<Record<PoiCategory, number>>({ fuel: 0, food: 0, attraction: 0 })
const visible = reactive<Record<PoiCategory, boolean>>({ fuel: true, food: true, attraction: true })
let map: L.Map | undefined
const clusters = new Map<PoiCategory, L.MarkerClusterGroup>()

// Карта открывается с анимацией — Leaflet должен пересчитать размер, когда контейнер его получит
useResizeObserver(el, () => map?.invalidateSize())

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
  map = createMap(el.value!, { scrollWheelZoom: true })
  map.fitBounds(drawTrip(map, props), { padding: [40, 40] })
  void drawRoad(map, props)

  const pois = await loadPois()
  loading.value = false
  if (!map) return
  for (const c of categories) {
    const items = pois.filter((p) => p.category === c.id)
    counts[c.id] = items.length
    const cluster = poiCluster(c.id).addLayers(items.map(poiMarker))
    clusters.set(c.id, cluster)
    if (visible[c.id]) cluster.addTo(map)
  }
})

watch(visible, () => {
  if (!map) return
  for (const [id, cluster] of clusters) {
    if (visible[id]) cluster.addTo(map)
    else cluster.remove()
  }
})

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
      <span v-if="loading" class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <LoaderCircleIcon class="size-3.5 animate-spin" /> Ищем места на OpenStreetMap…
      </span>
      <span v-else-if="failed" class="text-xs text-amber-700">
        Часть мест не загрузилась — OpenStreetMap сейчас перегружен, попробуйте позже.
      </span>
    </div>
    <div ref="map" class="trip-map isolate min-h-0 flex-1" />
  </div>
</template>
