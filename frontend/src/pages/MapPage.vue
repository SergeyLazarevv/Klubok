<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { createStyledMap, type MapLibreMap } from '@/lib/map/maplibre'
import { cartoonStyle } from '@/lib/map/cartoonStyle'

// Самара: площадь Куйбышева, Волга видна слева
const SAMARA: [number, number] = [50.098, 53.196]

const el = ref<HTMLElement>()
let map: MapLibreMap | undefined

onMounted(() => {
  map = createStyledMap(el.value!, cartoonStyle(), { center: SAMARA, zoom: 15.2, pitch: 60, bearing: -25 })
})

onUnmounted(() => map?.remove())
</script>

<template>
  <div ref="el" class="h-[calc(100dvh-3.5rem-1px)] w-full" />
</template>
