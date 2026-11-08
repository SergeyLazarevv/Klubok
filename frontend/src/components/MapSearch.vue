<script setup lang="ts">
import { computed, onUnmounted, ref, useTemplateRef, watch } from 'vue'
import { onClickOutside, useDebounceFn } from '@vueuse/core'
import {
  Building2Icon,
  CrosshairIcon,
  HistoryIcon,
  HouseIcon,
  LandmarkIcon,
  LoaderCircleIcon,
  MapPinIcon,
  MountainIcon,
  SearchIcon,
  SignpostIcon,
  XIcon,
} from '@lucide/vue'
import { api, type MapSearchItem } from '@/api/client'
import { maplibregl, type MapLibreMap } from '@/lib/map/maplibre'
import { loadHistory, parseCoords, remember, zoomFor } from '@/lib/map/search'

// Поиск по карте в духе Google Карт: поле сверху по центру, результаты в той же карточке
const props = defineProps<{ map?: MapLibreMap }>()

const root = useTemplateRef('root')
const input = useTemplateRef('input')

const term = ref('')
const items = ref<MapSearchItem[]>([])
const history = ref(loadHistory())
const open = ref(false)
const loading = ref(false)
const failed = ref(false)
const active = ref(-1)
let picked = '' // подпись выбранного места: искать по ней заново не нужно
let seq = 0

const showHistory = computed(() => term.value.trim().length < 2 && history.value.length > 0)
const list = computed(() => (showHistory.value ? history.value : items.value))
const searched = computed(() => term.value.trim().length >= 2 && !loading.value)

const icons = {
  city: Building2Icon,
  village: HouseIcon,
  sight: LandmarkIcon,
  nature: MountainIcon,
  street: SignpostIcon,
  house: MapPinIcon,
  other: MapPinIcon,
}
const iconFor = (it: MapSearchItem) =>
  showHistory.value ? HistoryIcon : it.subtitle === 'Координаты' ? CrosshairIcon : icons[it.kind]

const load = useDebounceFn(async (q: string) => {
  const current = ++seq
  const m = props.map
  const c = m?.getCenter()
  const { data, error } = await api.GET('/map/search', {
    params: { query: { q, lat: c?.lat, lon: c?.lng, zoom: m ? Math.min(18, Math.round(m.getZoom())) : undefined } },
  })
  if (current !== seq) return // пользователь уже ввёл другой текст
  failed.value = !!error
  items.value = data?.items ?? []
  active.value = -1
  loading.value = false
}, 250)

watch(term, (raw) => {
  const q = raw.trim()
  if (raw === picked) return
  picked = ''
  open.value = true
  const coords = parseCoords(q)
  if (coords || q.length < 2) {
    seq++ // ответ на прошлый запрос уже не нужен
    items.value = coords ? [coords] : []
    loading.value = failed.value = false
    active.value = coords ? 0 : -1
    return
  }
  loading.value = true
  load(q)
})

let marker: maplibregl.Marker | undefined

function pinElement() {
  // Позицию маркера MapLibre задаёт через transform — анимация на вложенном элементе
  const el = document.createElement('div')
  el.innerHTML = '<div class="map-search-pin"></div>'
  return el
}

function pick(it: MapSearchItem) {
  const m = props.map
  picked = term.value = it.name
  open.value = false
  input.value?.blur() // на телефоне прячем клавиатуру — карта нужна целиком
  if (it.subtitle !== 'Координаты') history.value = remember(it)
  if (!m) return

  const at: [number, number] = [it.location.lon, it.location.lat]
  marker ??= new maplibregl.Marker({ element: pinElement(), anchor: 'bottom' })
  marker.setLngLat(at).addTo(m)

  // Сверху поле поиска — отступаем, чтобы место не пряталось под ним
  const padding = { top: 96, bottom: 48, left: 48, right: 48 }
  const [w, s, e, n] = it.bbox ?? []
  const big = it.bbox && (e! - w! > 0.004 || n! - s! > 0.004)
  if (big) m.fitBounds([[w!, s!], [e!, n!]], { padding, maxZoom: 16, pitch: 0, duration: 1600 })
  else m.flyTo({ center: at, zoom: zoomFor[it.kind], offset: [0, 24], duration: 1600, essential: true })
}

function clear() {
  term.value = picked = ''
  items.value = []
  marker?.remove()
  input.value?.focus()
}

function onKey(e: KeyboardEvent) {
  const n = list.value.length
  if (e.key === 'ArrowDown' && n) {
    e.preventDefault()
    open.value = true
    active.value = (active.value + 1) % n
  } else if (e.key === 'ArrowUp' && n) {
    e.preventDefault()
    active.value = (active.value - 1 + n) % n
  } else if (e.key === 'Enter') {
    // Как в Google: Enter без выбора — первый результат
    const it = list.value[Math.max(0, active.value)]
    if (it && open.value) pick(it)
  } else if (e.key === 'Escape') {
    open.value = false
    input.value?.blur()
  }
}

onClickOutside(root, () => (open.value = false))
onUnmounted(() => marker?.remove())

const expanded = computed(() => open.value && (list.value.length > 0 || searched.value || failed.value))
</script>

<template>
  <div
    ref="root"
    class="pointer-events-auto absolute inset-x-3 top-3 z-20 sm:left-1/2 sm:right-auto sm:w-[min(36rem,calc(100%-8rem))] sm:-translate-x-1/2"
  >
    <div
      class="overflow-hidden rounded-3xl bg-background shadow-[0_2px_6px_rgb(0_0_0/0.12),0_8px_24px_rgb(0_0_0/0.08)] transition-shadow focus-within:shadow-[0_4px_10px_rgb(0_0_0/0.16),0_12px_32px_rgb(0_0_0/0.1)]"
    >
      <div class="flex h-12 items-center gap-1 pl-4 pr-1.5">
        <LoaderCircleIcon v-if="loading" class="size-5 shrink-0 animate-spin text-muted-foreground" />
        <SearchIcon v-else class="size-5 shrink-0 text-muted-foreground" />
        <input
          ref="input"
          v-model="term"
          type="search"
          enterkeyhint="search"
          autocomplete="off"
          spellcheck="false"
          placeholder="Город, село, место или координаты"
          aria-label="Поиск по карте"
          role="combobox"
          :aria-expanded="expanded"
          aria-controls="map-search-list"
          class="h-full min-w-0 flex-1 bg-transparent px-2 text-base outline-none placeholder:text-muted-foreground [&::-webkit-search-cancel-button]:hidden"
          @focus="open = true"
          @keydown="onKey"
        />
        <button
          v-if="term"
          type="button"
          class="grid size-9 shrink-0 place-items-center rounded-full text-muted-foreground hover:bg-muted"
          aria-label="Очистить"
          @click="clear"
        >
          <XIcon class="size-5" />
        </button>
      </div>

      <div v-if="expanded" class="border-t">
        <ul id="map-search-list" role="listbox" class="max-h-[min(24rem,55dvh)] overflow-y-auto overscroll-contain py-1.5">
          <li
            v-for="(it, i) in list"
            :key="`${it.name}|${it.subtitle}|${i}`"
            role="option"
            :aria-selected="i === active"
            class="flex cursor-pointer items-center gap-3 px-4 py-2.5 hover:bg-muted"
            :class="i === active && 'bg-muted'"
            @mousedown.prevent
            @click="pick(it)"
            @mousemove="active = i"
          >
            <span class="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-muted-foreground">
              <component :is="iconFor(it)" class="size-4" />
            </span>
            <span class="min-w-0">
              <span class="block truncate font-medium">{{ it.name }}</span>
              <span v-if="it.subtitle" class="block truncate text-sm text-muted-foreground">{{ it.subtitle }}</span>
            </span>
          </li>
        </ul>
        <p v-if="!list.length && failed" class="px-4 pb-3 text-sm text-muted-foreground">
          Поиск временно недоступен, попробуйте позже
        </p>
        <p v-else-if="!list.length && searched" class="px-4 pb-3 text-sm text-muted-foreground">
          Ничего не нашли. Можно ввести координаты: 53.19, 50.10
        </p>
      </div>
    </div>
  </div>
</template>

<style>
/* На телефоне поле на всю ширину — кнопки масштаба уходят под него */
@media (width < 40rem) {
  .map-with-search .maplibregl-ctrl-top-right {
    top: 3.75rem;
  }
}

.map-search-pin {
  width: 30px;
  height: 40px;
  background: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 30 40'%3E%3Cpath d='M15 1C7.3 1 1 7.1 1 14.7 1 25 15 39 15 39s14-14 14-24.3C29 7.1 22.7 1 15 1z' fill='%23e8657a' stroke='%23fff' stroke-width='2'/%3E%3Ccircle cx='15' cy='14.5' r='5' fill='%23fff'/%3E%3C/svg%3E")
    center / contain no-repeat;
  filter: drop-shadow(0 3px 4px rgb(0 0 0 / 0.25));
  animation: map-search-drop 0.35s ease-out;
}
@keyframes map-search-drop {
  from {
    transform: translateY(-14px);
    opacity: 0;
  }
}
</style>
