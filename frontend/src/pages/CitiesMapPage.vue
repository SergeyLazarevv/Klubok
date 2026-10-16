<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import '@fontsource/nunito/700.css'
import '@fontsource/nunito/900.css'
import { createStyledMap, maplibregl, type MapLibreMap } from '@/lib/map/maplibre'
import { PASTEL, cartoonStyle } from '@/lib/map/cartoonStyle'
import { armsUrl, photoUrl, warmStickerCache, withCities, type City } from '@/lib/map/cities'

// Издалека: европейская часть России с Самарой посередине, чтобы сразу были видны гербы городов
const SAMARA: [number, number] = [50.1, 53.2]

const el = ref<HTMLElement>()
let map: MapLibreMap | undefined

const escape = (s: string) => s.replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`)

function card(c: City) {
  const sub = c.country ? `<span class="city-card-country">${escape(c.country)}</span>` : ''
  const landmark = c.landmark ? `<p>${escape(c.landmark)}</p>` : ''
  // Фото грузится только по клику и лениво — карта от него не тормозит
  const photo = c.photo ? `<img class="city-card-photo" src="${photoUrl(c.photo)}" alt="" loading="lazy">` : ''
  const arms = c.arms ? `<img class="city-card-arms" src="${armsUrl(c.id)}" alt="">` : ''
  return `${photo}<div class="city-card-body">${arms}<div><b>${escape(c.name)}</b>${sub}${landmark}</div></div>`
}

onMounted(() => {
  const style = withCities(cartoonStyle(PASTEL), PASTEL.title, '#ffffff')
  const m = createStyledMap(el.value!, style, { center: SAMARA, zoom: 4.6 })
  map = m
  m.once('load', warmStickerCache)

  m.on('click', 'cities', (e) => {
    const f = e.features?.[0]
    if (!f || f.geometry.type !== 'Point') return
    new maplibregl.Popup({ className: 'city-card', offset: 26, maxWidth: '280px' })
      .setLngLat(f.geometry.coordinates as [number, number])
      .setHTML(card(f.properties as City))
      .addTo(m)
  })
  m.on('mouseenter', 'cities', () => (m.getCanvas().style.cursor = 'pointer'))
  m.on('mouseleave', 'cities', () => (m.getCanvas().style.cursor = ''))
})

onUnmounted(() => map?.remove())
</script>

<template>
  <!-- Вокруг глобуса — светлое небо -->
  <div class="h-[calc(100dvh-3.5rem-1px)] w-full bg-gradient-to-b from-[#d9f0ff] to-[#fdf6e9]">
    <div ref="el" class="h-full w-full" />
  </div>
</template>

<style>
.city-card .maplibregl-popup-content {
  overflow: hidden;
  padding: 0;
  border-radius: 18px;
  background: #fffdf8;
  box-shadow: 0 6px 24px rgb(90 70 60 / 0.18);
  font-family: 'Nunito', sans-serif;
  color: #5b4a63;
}
.city-card .maplibregl-popup-close-button {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  background: rgb(255 255 255 / 0.85);
  font-size: 18px;
}
.city-card-photo {
  display: block;
  width: 100%;
  height: 150px;
  object-fit: cover;
  background: #f1e8f7;
}
.city-card-body {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px 14px;
}
.city-card-arms {
  width: 44px;
  height: 44px;
  object-fit: contain;
  flex: none;
}
.city-card b {
  display: block;
  font-size: 17px;
  font-weight: 900;
  color: #6f5a8e;
}
.city-card-country {
  font-size: 12px;
  font-weight: 700;
  color: #a0809a;
}
.city-card p {
  margin: 2px 0 0;
  font-size: 13px;
  font-weight: 700;
}
</style>
