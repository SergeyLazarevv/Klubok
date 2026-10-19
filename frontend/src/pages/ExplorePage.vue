<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowRightIcon, BedDoubleIcon, CalendarIcon, CarIcon, ChevronDownIcon, ClockIcon, ExternalLinkIcon, FuelIcon,
  MapPinnedIcon, MoonIcon, PlaneIcon, RepeatIcon, RouteIcon, SearchXIcon, SparklesIcon, XIcon,
} from '@lucide/vue'
import { useLocalStorage, watchDebounced } from '@vueuse/core'
import { api, type ExploreItem, type ExploreResponse, type FavoritePick, type FavoriteSnapshot, type Place, type StayPrice, type StopSuggestion, type TripOption, type TripSearchParams } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import { createStyledMap, maplibregl, type MapLibreMap } from '@/lib/map/maplibre'
import { PASTEL, cartoonStyle } from '@/lib/map/cartoonStyle'
import { armsUrl, photoUrl, warmStickerCache, withCities, type City } from '@/lib/map/cities'
import { LOOKUP_LAYER, PRICE_LAYER, addPricePins, setPricePins, withPinFonts } from '@/lib/map/pricePins'
import { type RouteSegment, addRouteLayers, fitRoute, setRoute } from '@/lib/map/routeLine'
import { decodePolyline, greatCircleArc, yandexRouteUrl } from '@/lib/geo'
import {
  cheapestByCity, distanceKm, driveDestinations, durations, estimateDrive, flightDestinations, lookupDestinations, periods, withStay, worthFlying,
  type CityFeature, type Destination, type Mode,
} from '@/lib/explore'
import { daysThere, hours, km, localDay, plural, rub, transfersLabel } from '@/lib/format'
import { drivePick, flightPick, flightSnapshot } from '@/lib/favorites'
import FavoriteButton from '@/components/FavoriteButton.vue'
import CarSettings from '@/components/CarSettings.vue'
import PlaceSearch from '@/components/PlaceSearch.vue'
import WeatherDays from '@/components/WeatherDays.vue'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'

const profile = useProfileStore()
const { homePlace, travelers, hasCar } = storeToRefs(profile)
const route = useRoute()
const router = useRouter()

// Фильтры — в адресе, чтобы подборкой можно было поделиться
const periodList = periods()
const q = route.query
const periodKey = ref(periodList.some((p) => p.key === q.p) ? String(q.p) : periodList[0].key)
const durationKey = ref(durations.some((d) => d.key === q.d) ? String(q.d) : durations[0].key)
const mode = ref<Mode>(q.m === 'car' || q.m === 'fly' ? q.m : 'all')
const period = computed(() => periodList.find((p) => p.key === periodKey.value)!)
const durationChoice = computed(() => durations.find((d) => d.key === durationKey.value)!)
// У «Ближайших выходных» длительность задана самим периодом: суббота — воскресенье
const duration = computed(() =>
  period.value.days ? { min: period.value.days, max: period.value.days, weekends: false } : durationChoice.value,
)
watch([periodKey, durationKey, mode], () =>
  router.replace({ query: { p: periodKey.value, d: durationKey.value, m: mode.value === 'all' ? undefined : mode.value } }),
)

const result = shallowRef<ExploreResponse>()
const loading = ref(false)
const error = ref<string>()
let seq = 0

/** Параметры поиска — одни и те же для подборки и для уточнения цены по городу. */
function searchQuery() {
  return {
    from: homePlace.value!.id,
    date_from: period.value.from,
    date_to: period.value.to,
    min_days: duration.value.min,
    max_days: duration.value.max,
    weekends: duration.value.weekends,
    travelers: travelers.value,
    use_car: hasCar.value,
    radius_km: profile.radiusKm,
    max_drive_hours: profile.maxDriveHours,
    consumption_l100: profile.consumptionL100,
    fuel_type: profile.fuelType,
    fuel_price_rub: profile.fuelPriceRub ?? undefined,
    allow_border: profile.allowBorder,
  }
}

/**
 * Уточнённые цены по городам (по коду города). Подборка «во все города» и поиск по одному городу берут цены
 * из разных выборок кэша Aviasales и иногда расходятся на десятки и сотни рублей. Поэтому при выборе города
 * запрашиваем тот же /trips, что и полный поиск, — карточка, список и страница «Все варианты» совпадают.
 */
const refined = shallowRef(new Map<string, ExploreItem>())
const refining = ref<string>()

async function refine(d: Destination) {
  const item = d.flight
  if (!item || refined.value.has(item.city.iata) || refining.value === d.key || !homePlace.value) return
  const current = seq
  refining.value = d.key
  const { data } = await api.GET('/trips', { params: { query: { ...searchQuery(), to: item.city.id } } })
  if (current !== seq) return // фильтры уже поменялись — уточнение устарело
  if (refining.value === d.key) refining.value = undefined
  // Самый дешёвый из перелётов, которые имеет смысл предлагать, — как в подборке
  const best = data && bestTrip(data)
  if (!best) return
  const next = new Map(refined.value)
  next.set(item.city.iata, { ...item, best, home_best: data!.home_best, drive: data!.drive ?? item.drive })
  refined.value = next
}

/** Лучший осмысленный перелёт из ответа /trips — по тому же правилу, что подборка. */
function bestTrip(data: { options: TripOption[]; drive?: ExploreItem['drive'] }) {
  return data.options
    .filter((o) => worthFlying(o, data.drive))
    .reduce<TripOption | undefined>((a, o) => (!a || o.total.amount < a.total.amount ? o : a), undefined)
}

/**
 * Города за рубежом без цены в подборке: столицы мира с карты. Цену ищем по клику тем же /trips, что и поиск;
 * найденное становится обычным направлением на карте и в списке.
 */
const lookedUp = shallowRef(new Map<string, Destination>())
const lookupEmpty = ref(new Set<string>())

async function lookupPrice(d: Destination) {
  if (!homePlace.value || refining.value === d.key || lookupEmpty.value.has(d.key)) return
  const current = seq
  refining.value = d.key
  const done = (found?: Destination) => {
    if (current !== seq) return
    if (refining.value === d.key) refining.value = undefined
    if (found) lookedUp.value = new Map(lookedUp.value).set(d.key, found)
    else lookupEmpty.value = new Set(lookupEmpty.value).add(d.key)
  }
  const place = await findPlace(d)
  if (!place) return done()
  const { data } = await api.GET('/trips', { params: { query: { ...searchQuery(), to: place.id } } })
  const best = data && bestTrip(data)
  if (!data || !best) return done()
  const flight: ExploreItem = { city: place, best, home_best: data.home_best, drive: data.drive }
  done({ ...d, lookup: false, flight, price: best.total.amount })
}

/** Город карты — из Wikidata, в справочнике backend его ищем по названию и ближайшим координатам. */
// Город справочника рядом с городом карты: с аэропортом или без (town — только на машине)
async function findPlace(d: Destination): Promise<Place | undefined> {
  const { data: places } = await api.GET('/places', { params: { query: { q: d.name, limit: 8 } } })
  return places?.items
    .filter((p) => p.kind !== 'airport')
    .map((p) => ({ p, km: Math.hypot(p.location.lat - d.lat, (p.location.lon - d.lon) * Math.cos((d.lat * Math.PI) / 180)) * 111 }))
    .filter((x) => x.km < 60)
    .sort((a, b) => a.km - b.km)[0]?.p
}

/** Города поездок на машине в справочнике backend — для избранного (по ключу направления). */
const drivePlaces = shallowRef(new Map<string, Place | null>())
async function resolveDrivePlace(d: Destination) {
  if (drivePlaces.value.has(d.key)) return
  drivePlaces.value = new Map(drivePlaces.value).set(d.key, null)
  const place = await findPlace(d)
  if (place) drivePlaces.value = new Map(drivePlaces.value).set(d.key, place)
}

async function load() {
  if (!homePlace.value) return
  const current = ++seq
  loading.value = true
  error.value = undefined
  refined.value = new Map()
  refining.value = undefined
  lookedUp.value = new Map()
  lookupEmpty.value = new Set()
  const { data, error: problem } = await api.GET('/explore', { params: { query: searchQuery() } })
  if (current !== seq) return // фильтры уже поменялись
  loading.value = false
  if (problem) {
    error.value = problem.errors?.[0]?.message ?? problem.detail ?? 'Не удалось загрузить цены'
    result.value = undefined
    return
  }
  result.value = data
}
watch([homePlace, periodKey, durationKey, travelers, hasCar], load)
// Настройки машины меняют соседние аэропорты и дорогу до них — подборку пересчитываем, но не на каждый шаг ползунка.
// Поездки целиком на машине считаются на клиенте и обновляются сразу.
const { radiusKm, maxDriveHours, consumptionL100, fuelType, fuelPriceRub, allowBorder } = storeToRefs(profile)
watchDebounced([radiusKm, maxDriveHours, consumptionL100, fuelType, fuelPriceRub, allowBorder], load, { debounce: 500 })

// Справочник городов карты: гербы, фото и небольшие города для поездок на машине
const cities = shallowRef<CityFeature[]>([])
async function loadCities() {
  const res = await fetch(`${import.meta.env.BASE_URL}data/cities.geojson`)
  const fc = await res.json()
  cities.value = fc.features.map((f: { geometry: { coordinates: [number, number] }; properties: City }) => ({
    city: f.properties,
    lon: f.geometry.coordinates[0],
    lat: f.geometry.coordinates[1],
  }))
}

const fuelPrice = computed(() => result.value?.fuel_price?.amount ?? (profile.fuelPriceRub ? profile.fuelPriceRub * 100 : 0))
const canDrive = computed(() => hasCar.value && fuelPrice.value > 0 && !!homePlace.value)

/**
 * Жильё: средняя цена суток в гостинице 3* по данным Росстата для каждого города карты (GET /stay-prices,
 * один раз на страницу). Учитывать ли его в итоге — выбор пользователя, по умолчанию да.
 */
const stayPrices = shallowRef(new Map<string, StayPrice>())
const includeStay = useLocalStorage('klubok.includeStay', true)
async function loadStayPrices() {
  const { data } = await api.GET('/stay-prices')
  if (data) stayPrices.value = new Map(data.items.map((p) => [p.city_id, p]))
}
const addStay = (list: Destination[]) =>
  withStay(list, stayPrices.value, travelers.value, duration.value.min, includeStay.value)

const flights = computed(() => {
  const list = flightDestinations((result.value?.items ?? []).map((i) => refined.value.get(i.city.iata) ?? i), cities.value)
  const keys = new Set(list.map((d) => d.key))
  const extra = [...lookedUp.value.values()].filter((d) => !keys.has(d.key))
  return addStay(extra.length ? [...list, ...extra] : list)
})
const drives = computed(() =>
  canDrive.value && homePlace.value
    ? addStay(driveDestinations(cities.value, {
        home: homePlace.value.location,
        maxHours: profile.maxDriveHours,
        consumptionL100: profile.consumptionL100,
        fuelPrice: fuelPrice.value,
        allowBorder: profile.allowBorder,
        borders: new Map((result.value?.border_rules ?? []).map((r) => [r.country_code, r])),
      }))
    : [],
)
const activeMode = computed<Mode>(() => (canDrive.value ? mode.value : 'fly'))
// «Всё» — по каждому городу самый дешёвый способ: ближние — машина, дальние — перелёт (с дорогой до аэропорта)
const cheapest = computed(() => cheapestByCity(flights.value, drives.value))
const all = computed(() => ({ all: cheapest.value, fly: flights.value, car: drives.value })[activeMode.value])
// Серые значки — зарубежные города, у которых в текущей подборке нет цены (ни перелёта, ни машины)
const lookups = computed(() =>
  activeMode.value === 'car' ? [] : lookupDestinations(cities.value, new Set(all.value.map((d) => d.key))),
)

// Бюджет на всю поездку; null — без ограничения. Шкала — до самой дорогой поездки в подборке.
const budget = ref<number | null>(null)
watch(activeMode, () => (budget.value = null))
const step = computed(() => (activeMode.value === 'fly' ? 50000 : 10000))
const maxPrice = computed(() => Math.max(step.value, Math.ceil(Math.max(0, ...all.value.map((d) => d.price)) / step.value) * step.value))
const budgetSlider = computed({
  get: () => [budget.value ?? maxPrice.value],
  set: (v: number[] | undefined) => (budget.value = v && v[0] < maxPrice.value ? v[0] : null),
})
const shown = computed(() => (budget.value == null ? all.value : all.value.filter((d) => d.price <= budget.value!)))
const LIST_LIMIT = 60

const selectedKey = ref<string>()
/**
 * Город России без цены (Ростов-на-Дону: аэропорт закрыт, на машине дальше лимита) — по клику на герб
 * карточка объясняет, почему цены нет; жильё показываем и для него.
 */
const unreachable = shallowRef<Destination>()
function explain(c: CityFeature) {
  const home = homePlace.value
  const drive = home && fuelPrice.value > 0
    ? estimateDrive(distanceKm(home.location, c), { home: home.location, maxHours: 0, consumptionL100: profile.consumptionL100, fuelPrice: fuelPrice.value })
    : undefined
  const [d] = addStay([{ key: c.city.id, name: c.city.name, lon: c.lon, lat: c.lat, city: c.city, price: Infinity, by: 'car', unreachable: { drive } }])
  unreachable.value = d
  selectedKey.value = d.key
  nextTick(() => card.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }))
}

/** Почему у города нет цены — по порядку: перелёты, машина. */
function reasons(d: Destination) {
  const out: string[] = []
  const inFlights = flights.value.some((f) => f.key === d.key)
  const inDrives = drives.value.some((f) => f.key === d.key)
  if (activeMode.value === 'fly' && inDrives) return ['Перелёты сюда не выгоднее машины — смотрите вкладку «Машина».']
  if (activeMode.value === 'car' && inFlights) return ['На машине сюда дальше вашего лимита — есть перелёты на вкладке «Самолёт».']
  if (!inFlights) out.push('Перелётов на эти даты нет — ни из вашего, ни из соседних городов. Возможно, аэропорт закрыт или рейсы не продаются.')
  const drive = d.unreachable?.drive
  if (!hasCar.value) out.push('Своя машина в настройках выключена.')
  else if (drive && profile.maxDriveHours > 0 && drive.hours > profile.maxDriveHours)
    out.push(`На машине ≈ ${hours(drive.hours)} в одну сторону (≈ ${km(drive.roadKm)}) — больше вашего лимита ${hours(profile.maxDriveHours)} за рулём. Его можно поменять в настройках машины выше.`)
  else if (drive) out.push(`Небольшой город — в подборку на машине не берём. Дорога ≈ ${hours(drive.hours)}, бензин ≈ ${rub(drive.fuel)}.`)
  return out
}

const selected = computed(() =>
  all.value.find((d) => d.key === selectedKey.value) ??
  lookups.value.find((d) => d.key === selectedKey.value) ??
  (unreachable.value?.key === selectedKey.value ? unreachable.value : undefined),
)
watch(activeMode, () => (selectedKey.value = undefined))
// Город остаётся выбранным при смене дат и длительности — в карточке тогда новая подборка, и её тоже уточняем,
// иначе она покажет вылет из снимков «во все города», а «Все варианты» — другой
watch(selected, (d) => d?.flight && !d.lookup && refine(d))

const card = ref<HTMLElement>()

/**
 * Погода в выбранном городе: у перелёта — на даты рейса, у машины и города без цены — на весь период
 * (даты свободные; длинный период WeatherDays покажет одной сводкой).
 */
const weatherQuery = computed(() => {
  const d = selected.value
  if (!d) return undefined
  const b = d.flight?.best
  if (d.by === 'fly' && b) return { lat: d.lat, lon: d.lon, from: b.depart_at.slice(0, 10), to: b.return_at.slice(0, 10) }
  return { lat: d.lat, lon: d.lon, from: period.value.from, to: period.value.to }
})

function select(d: Destination | undefined, fly = true) {
  selectedKey.value = d?.key
  if (!d) return
  if (d.lookup) {
    lookupPrice(d)
    if (fly && map) map.easeTo({ center: [d.lon, d.lat], zoom: Math.max(map.getZoom(), 5), duration: 700 })
  } else if (d.flight) refine(d)
  else if (d.drive) resolveDrivePlace(d)
  // Карточка — в панели над списком; на телефоне панель под картой, без прокрутки её не видно
  nextTick(() => card.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' }))
}

// --- Карта ---
const el = ref<HTMLElement>()
let map: MapLibreMap | undefined
let homeMarker: maplibregl.Marker | undefined
const mapReady = ref(false)
const PANEL_WIDTH = 432 // ширина панели с отступом, px

function placeHome(p: Place | null) {
  homeMarker?.remove()
  homeMarker = undefined
  if (!map || !p) return
  const dot = document.createElement('div')
  dot.className = 'home-dot'
  dot.title = 'Вы здесь'
  homeMarker = new maplibregl.Marker({ element: dot }).setLngLat([p.location.lon, p.location.lat]).addTo(map)
}

onMounted(() => {
  loadCities()
  loadStayPrices()
  load()
  const style = withPinFonts(withCities(cartoonStyle(PASTEL), PASTEL.title, '#ffffff'))
  const center: [number, number] = homePlace.value ? [homePlace.value.location.lon, homePlace.value.location.lat] : [50.1, 53.2]
  // На компьютере панель лежит поверх карты слева — центр карты и плашки считаем по видимой части
  const wide = window.matchMedia('(min-width: 1024px)').matches
  const m = createStyledMap(el.value!, style, { center, zoom: 4.3 })
  if (wide) m.setPadding({ left: PANEL_WIDTH, top: 0, right: 0, bottom: 0 })
  map = m
  m.once('load', warmStickerCache)
  placeHome(homePlace.value)
  // Слоям нужен разобранный стиль, но не тайлы: style.load приходит сразу, load — после всех тайлов
  const ready = () => {
    addRouteLayers(m, 'cities')
    addPricePins(m, (key) => select([...all.value, ...lookups.value].find((d) => d.key === key), false))
    mapReady.value = true
  }
  if (m.isStyleLoaded()) ready()
  else m.once('style.load', ready)
  m.on('click', 'cities', (e) => {
    if (m.getLayer(PRICE_LAYER) && m.queryRenderedFeatures(e.point, { layers: [PRICE_LAYER, LOOKUP_LAYER] }).length) return
    const id = e.features?.[0]?.properties?.id
    const d = all.value.find((x) => x.key === id) ?? lookups.value.find((x) => x.key === id)
    if (d) return select(d, false)
    const c = cities.value.find((x) => x.city.id === id)
    if (c && !c.city.country) explain(c)
  })
})
onUnmounted(() => map?.remove())

watch(homePlace, (p) => {
  placeHome(p)
  if (p && map) map.easeTo({ center: [p.location.lon, p.location.lat], zoom: 4.3 })
})

watch([shown, lookups, selectedKey, mapReady], () => {
  if (!map || !mapReady.value) return
  setPricePins(
    map,
    shown.value.map((d) => ({ key: d.key, name: d.name, lon: d.lon, lat: d.lat, by: d.by, label: priceLabel(d), price: Math.round(d.price / 100) })),
    lookups.value.map((d) => ({ key: d.key, name: d.name, lon: d.lon, lat: d.lat })),
    selectedKey.value,
  )
})

// --- Маршрут до выбранного города ---
// Дорога — по реальным дорогам (GET /routes/drive, кэш на backend), перелёт — дуга от аэропорта вылета
// до аэропорта прилёта. Камера сразу показывает весь путь — по известным точкам, не дожидаясь дорог.
let routeSeq = 0

const toLonLat = ([lat, lon]: [number, number]): [number, number] => [lon, lat]

async function roadSegment(from: { lat: number; lon: number }, to: { lat: number; lon: number }): Promise<RouteSegment> {
  const { data } = await api.GET('/routes/drive', {
    params: { query: { from_lat: from.lat, from_lon: from.lon, to_lat: to.lat, to_lon: to.lon } },
  })
  const coords = data ? decodePolyline(data.polyline).map(toLonLat) : [[from.lon, from.lat], [to.lon, to.lat]] as [number, number][]
  return { kind: 'road', coords }
}

async function showRoute(d: Destination | undefined) {
  const current = ++routeSeq
  const home = homePlace.value?.location
  if (!map || !mapReady.value || !home || !d || d.lookup) {
    if (map && mapReady.value) setRoute(map, [])
    return
  }
  const dest = { lat: d.lat, lon: d.lon }
  const b = d.flight?.best
  const flightLeg = d.by === 'fly' && b
  const hub = flightLeg ? (b.car ? b.hub.location : home) : undefined
  const arrival = flightLeg ? (b.arrival?.location ?? d.flight!.city.location) : undefined

  // Панель уже учтена отступом карты (setPadding); запас — под плашку над городом у края
  fitRoute(map, [[home.lon, home.lat], [dest.lon, dest.lat], ...(hub ? [[hub.lon, hub.lat] as [number, number]] : [])], 70)

  const tasks: Promise<RouteSegment>[] = []
  if (!flightLeg) tasks.push(roadSegment(home, dest))
  else {
    if (b.car) tasks.push(roadSegment(home, hub!)) // до аэропорта вылета на своей машине
    tasks.push(Promise.resolve({ kind: 'flight', coords: greatCircleArc([hub!.lat, hub!.lon], [arrival!.lat, arrival!.lon]).map(toLonLat) }))
  }
  const segments = await Promise.all(tasks)
  if (current === routeSeq && map) setRoute(map, segments)
}

// Маршрут меняется вместе с выбором и с уточнением цены (вылет мог смениться на другой аэропорт)
watch(
  () => [selected.value?.key, selected.value?.by, selected.value?.flight?.best.hub.iata, mapReady.value] as const,
  () => showRoute(selected.value),
)

// --- Подписи ---
const priceLabel = (d: Destination) => (d.by === 'car' ? '≈ ' : '') + rub(d.price)
const hubName = (p: Place) => p.city_name ?? p.name

/** Вылет из соседнего города — если из своего, про дорогу до аэропорта отдельно не пишем. */
const fromOtherCity = (d: Destination) => {
  const b = d.flight?.best
  return !!b?.car && (b.hub.city_iata ?? b.hub.iata) !== homePlace.value?.iata
}

function subtitle(d: Destination) {
  if (d.drive) return `${hours(d.drive.hours)} за рулём · ${km(d.drive.roadKm)}`
  const b = d.flight!.best
  const parts = [`${localDay(b.depart_at)} – ${localDay(b.return_at)}`, transfersLabel(b.transfers)]
  if (fromOtherCity(d)) parts.push(`из г. ${hubName(b.hub)}`)
  return parts.join(' · ')
}

const monthFmt = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric', timeZone: 'UTC' })
/** «2026-09-01» → «сентябрь 2026 г.» */
const monthLabel = (iso: string) => monthFmt.format(new Date(iso + 'T00:00:00Z'))

const savings = (d: Destination) => Math.max(0, d.flight?.best.savings_vs_home?.amount ?? 0)

/**
 * Полный поиск с теми же параметрами, что уточнение цены в карточке (refine): период, длительность и «только
 * выходные» подборки. Тот же запрос /trips — тот же лучший вариант. Точные даты рейса сюда не годятся: вылет
 * в конце дальнего месяца выходит за горизонт поиска (3 месяца на начало), и поиск молча брал свои даты.
 */
/** Поиск с параметрами подборки; на машине — сразу открыт вариант на машине, с остановками via. */
function searchLink(d: Destination, via?: string[]) {
  const q = searchQuery()
  const to = d.flight?.city.id ?? drivePlaces.value.get(d.key)?.id
  return {
    path: '/',
    query: {
      from: q.from,
      to,
      date_from: q.date_from,
      date_to: q.date_to,
      min: q.min_days,
      max: q.max_days,
      weekends: q.weekends ? '1' : undefined,
      via,
      trip: d.flight ? undefined : 'drive',
    },
  }
}

// Где остановиться по дороге на машине (ADR-0023): города вдоль дороги, нажатие — план поездки с остановкой
const driveStops = ref<StopSuggestion[]>()
watch(
  () => {
    const d = selected.value
    const place = d?.drive && !d.flight ? drivePlaces.value.get(d.key) : undefined
    return place && homePlace.value ? `${homePlace.value.id}|${place.id}|${profile.maxDriveHours}` : undefined
  },
  async (key) => {
    driveStops.value = undefined
    if (!key) return
    const [from, to, hours] = key.split('|') as [string, string, string]
    const { data } = await api.GET('/trips/stops', { params: { query: { from, to, max_drive_hours: Number(hours) } } })
    if (selected.value && drivePlaces.value.get(selected.value.key)?.id === to) driveStops.value = data?.items.slice(0, 4) ?? []
  },
)

/**
 * Избранное из карточки — с параметрами подборки, как у «Все варианты»: звёздочка совпадает с результатами
 * на главной. Конкретный рейс задаёт pick (аэропорт и даты).
 */
const favorite = computed(() => {
  const d = selected.value
  const home = homePlace.value
  if (!d || !home || d.lookup || d.unreachable) return
  const base = searchQuery()
  const common = {
    from: home.id,
    date_from: base.date_from,
    date_to: base.date_to,
    min_days: base.min_days,
    max_days: base.max_days,
    weekends: base.weekends || undefined,
    travelers: base.travelers,
    use_car: base.use_car,
    radius_km: base.radius_km,
    max_drive_hours: base.max_drive_hours,
    consumption_l100: base.consumption_l100,
    fuel_type: base.fuel_type,
    fuel_price_rub: base.fuel_price_rub,
    allow_border: base.allow_border,
  }
  if (d.by === 'fly' && d.flight) {
    const b = d.flight.best
    const params: TripSearchParams = { ...common, to: d.flight.city.id }
    return { home, destination: d.flight.city, params, pick: flightPick(b), snapshot: flightSnapshot(b) }
  }
  const place = drivePlaces.value.get(d.key)
  if (!d.drive || !place) return
  const params: TripSearchParams = { ...common, to: place.id }
  const total = d.drive.total
  const snapshot: FavoriteSnapshot = {
    total: { amount: total, currency: 'RUB' },
    per_person: { amount: Math.round(total / Math.max(1, travelers.value)), currency: 'RUB' },
    drive_hours: d.drive.hours,
    trip_days: duration.value.min,
  }
  return { home, destination: place, params, pick: drivePick as FavoritePick, snapshot }
})

function routeLink(d: Destination) {
  return yandexRouteUrl(homePlace.value!.location, d)
}

const editHome = ref(false)
watch(homePlace, () => (editHome.value = false))

const tabs = computed(() =>
  canDrive.value
    ? [
        { mode: 'all' as const, label: 'Всё', icon: SparklesIcon, count: cheapest.value.length },
        { mode: 'fly' as const, label: 'Самолёт', icon: PlaneIcon, count: flights.value.length },
        { mode: 'car' as const, label: 'Машина', icon: CarIcon, count: drives.value.length },
      ]
    : [{ mode: 'fly' as const, label: 'Самолёт', icon: PlaneIcon, count: flights.value.length }],
)
</script>

<template>
  <div class="relative flex flex-col lg:block lg:h-[calc(100dvh-3.5rem-1px)]">
    <!-- Карта на всю ширину, панель — поверх -->
    <div class="relative h-[55dvh] lg:absolute lg:inset-0 lg:h-auto">
      <div ref="el" class="size-full bg-[#d9f0ff]" />
      <Transition name="fade">
        <div v-if="loading" class="pointer-events-none absolute top-4 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-full bg-background/90 px-4 py-2 text-sm font-medium shadow-lg ring-1 ring-black/5 backdrop-blur lg:left-[calc(50%+210px)]">
          <span class="loader-dot" /> Ищем цены
        </div>
      </Transition>
    </div>

    <aside
      class="relative z-10 -mt-6 flex flex-col gap-5 rounded-t-3xl bg-background px-4 pt-3 pb-8 shadow-[0_-8px_30px_rgb(0_0_0/0.08)]
             lg:absolute lg:top-4 lg:bottom-4 lg:left-4 lg:mt-0 lg:w-[400px] lg:overflow-y-auto lg:rounded-3xl lg:bg-background/85 lg:p-5
             lg:shadow-[0_8px_40px_rgb(0_0_0/0.12)] lg:ring-1 lg:ring-black/5 lg:backdrop-blur-xl [&>*]:shrink-0"
    >
      <div class="mx-auto h-1 w-10 rounded-full bg-border lg:hidden" />

      <header class="flex flex-col gap-1">
        <button
          v-if="homePlace && !editHome"
          type="button"
          class="flex w-fit items-center gap-1 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
          @click="editHome = true"
        >
          Из города {{ homePlace.name }}<ChevronDownIcon class="size-4" />
        </button>
        <h1 class="text-[28px] leading-tight font-semibold tracking-tight">Куда угодно</h1>
        <p class="text-sm text-muted-foreground">Самые дешёвые поездки на выбранные даты — с бензином и соседними аэропортами.</p>
      </header>

      <div v-if="!homePlace || editHome" class="rounded-2xl ring-1 ring-border">
        <PlaceSearch v-model="homePlace" kind="city" placeholder="Ваш город" />
      </div>

      <CarSettings narrow :reference-fuel-price="result?.fuel_price?.amount" />

      <!-- Самолёт / машина -->
      <div class="segmented" :style="{ '--count': tabs.length, '--index': tabs.findIndex((t) => t.mode === activeMode) }">
        <span class="segmented__thumb" />
        <button
          v-for="t in tabs"
          :key="t.mode"
          type="button"
          class="segmented__item"
          :class="t.mode === activeMode && 'is-active'"
          @click="mode = t.mode"
        >
          <component :is="t.icon" v-if="tabs.length < 3" class="size-4" />
          {{ t.label }}
          <span class="tabular-nums opacity-60">{{ t.count }}</span>
        </button>
      </div>

      <div class="flex flex-col gap-2.5">
        <div class="flex flex-wrap gap-2">
          <button v-for="p in periodList" :key="p.key" type="button" class="pill" :class="p.key === periodKey && 'is-active'" @click="periodKey = p.key">
            {{ p.label }}
          </button>
        </div>
        <div v-if="!period.days" class="grid grid-cols-4 gap-2">
          <button v-for="d in durations" :key="d.key" type="button" class="pill pill--cell" :class="d.key === durationKey && 'is-active'" @click="durationKey = d.key">
            {{ d.label }}
          </button>
        </div>
        <label class="flex cursor-pointer items-center justify-between gap-3 pt-1 text-sm">
          <span class="flex items-center gap-2"><BedDoubleIcon class="size-4 text-muted-foreground" /> Учитывать жильё</span>
          <Switch v-model="includeStay" />
        </label>
        <div v-if="all.length > 1" class="flex items-center gap-4 pt-1">
          <span class="w-28 shrink-0 text-sm text-muted-foreground tabular-nums">
            {{ budget == null ? 'Любой бюджет' : `до ${rub(budget)}` }}
          </span>
          <Slider v-model="budgetSlider" :min="step" :max="maxPrice" :step="step" class="flex-1" />
        </div>
      </div>

      <!-- Выбранное направление -->
      <Transition name="card" mode="out-in">
        <article v-if="selected" :key="selected.key" ref="card" class="overflow-hidden rounded-3xl bg-card shadow-[0_4px_24px_rgb(0_0_0/0.08)] ring-1 ring-black/5">
          <div class="relative h-40 bg-muted">
            <img v-if="selected.city?.photo" :src="photoUrl(selected.city.photo)" alt="" class="size-full object-cover" loading="lazy">
            <div class="absolute inset-0 bg-linear-to-t from-black/70 via-black/10 to-transparent" />
            <button type="button" class="absolute top-3 right-3 grid size-8 place-items-center rounded-full bg-black/30 text-white backdrop-blur transition hover:bg-black/50" aria-label="Закрыть" @click="select(undefined)">
              <XIcon class="size-4" />
            </button>
            <div class="absolute right-4 bottom-3 left-4 flex items-end gap-3 text-white">
              <img v-if="selected.city?.arms" :src="armsUrl(selected.city.id)" alt="" class="size-10 object-contain drop-shadow">
              <div class="min-w-0">
                <h2 class="truncate text-xl font-semibold tracking-tight">{{ selected.name }}</h2>
                <p v-if="selected.city?.landmark" class="truncate text-sm text-white/80">{{ selected.city.landmark }}</p>
              </div>
            </div>
          </div>

          <div v-if="selected.unreachable" class="flex flex-col gap-3 p-4 text-sm">
            <p class="flex items-center gap-2 font-medium"><SearchXIcon class="size-4 shrink-0 text-muted-foreground" /> Цены на дорогу нет</p>
            <p v-for="r in reasons(selected)" :key="r" class="text-muted-foreground">{{ r }}</p>
            <div v-if="selected.stay" class="flex items-baseline justify-between gap-2 rounded-2xl bg-muted p-3">
              <span class="flex items-center gap-2"><BedDoubleIcon class="size-4 text-muted-foreground" /> Сутки в гостинице 3*</span>
              <span class="font-semibold tabular-nums">≈ {{ rub(selected.stay.perNight) }}</span>
            </div>
          </div>
          <div v-else-if="selected.lookup" class="flex items-center gap-3 p-4 text-sm">
            <template v-if="lookupEmpty.has(selected.key)">
              <SearchXIcon class="size-5 shrink-0 text-muted-foreground" />
              <span>
                На эти даты цен на перелёт нет — попробуйте другой месяц или длительность.
                <template v-if="selected.city?.cc && hasCar && !profile.allowBorder">
                  На машине тоже не считаем: поездки в другие страны выключены в настройках машины выше.
                </template>
              </span>
            </template>
            <template v-else>
              <span class="loader-dot shrink-0" />
              <span class="text-muted-foreground">Ищем цену на перелёт…</span>
            </template>
          </div>
          <div v-else class="flex flex-col gap-4 p-4">
            <div class="flex flex-wrap items-baseline justify-between gap-x-3">
              <span class="flex items-center gap-1.5 text-sm font-medium" :class="selected.by === 'fly' ? 'text-flight' : 'text-road'">
                <PlaneIcon v-if="selected.by === 'fly'" class="size-4" />
                <CarIcon v-else class="size-4" />
                {{ selected.stay && includeStay ? 'Дорога и жильё' : selected.by === 'fly' ? 'Туда и обратно' : 'Бензин туда и обратно' }}
              </span>
              <span class="text-2xl font-semibold tracking-tight whitespace-nowrap tabular-nums transition-opacity" :class="refining === selected.key && 'animate-pulse opacity-50'">
                {{ priceLabel(selected) }}
              </span>
            </div>

            <dl v-if="selected.flight" class="grid grid-cols-3 gap-2">
              <div class="tile"><CalendarIcon class="tile__icon" /><dt>Даты</dt><dd>{{ localDay(selected.flight.best.depart_at) }} – {{ localDay(selected.flight.best.return_at) }}</dd></div>
              <div class="tile"><ClockIcon class="tile__icon" /><dt>Поездка</dt><dd>{{ selected.flight.best.trip_days }} {{ plural(selected.flight.best.trip_days, 'день', 'дня', 'дней') }}<span class="block text-[11px] font-normal text-muted-foreground">на месте {{ daysThere(selected.flight.best.days_there) }}</span></dd></div>
              <div class="tile"><RepeatIcon class="tile__icon" /><dt>Рейс</dt><dd>{{ transfersLabel(selected.flight.best.transfers) }}</dd></div>
            </dl>
            <dl v-else-if="selected.drive" class="grid grid-cols-3 gap-2">
              <div class="tile"><RouteIcon class="tile__icon" /><dt>Дорога</dt><dd>≈ {{ km(selected.drive.roadKm) }}</dd></div>
              <div class="tile"><ClockIcon class="tile__icon" /><dt>В пути</dt><dd>≈ {{ hours(selected.drive.hours) }}</dd></div>
              <div class="tile"><FuelIcon class="tile__icon" /><dt>Бензин</dt><dd>{{ rub(selected.drive.fuel) }}</dd></div>
            </dl>

            <WeatherDays :query="weatherQuery" />

            <div v-if="selected.stay" class="flex flex-col gap-1.5 rounded-2xl bg-muted p-3 text-sm">
              <div class="flex items-baseline justify-between gap-2">
                <span class="flex items-center gap-2 font-medium">
                  <BedDoubleIcon class="size-4 text-muted-foreground" />
                  Жильё, {{ selected.stay.nights }} {{ plural(selected.stay.nights, 'ночь', 'ночи', 'ночей') }}
                </span>
                <span class="font-semibold tabular-nums" :class="!includeStay && 'text-muted-foreground'">≈ {{ rub(selected.stay.total) }}</span>
              </div>
              <span class="text-xs text-muted-foreground">
                Средняя цена суток в гостинице 3* — {{ rub(selected.stay.perNight) }}.
                Росстат, {{ monthLabel(selected.stay.source.period) }}{{ selected.stay.source.basis !== selected.name ? `, по данным для г. ${selected.stay.source.basis}` : '' }}{{ includeStay ? '' : '. В итог не входит' }}
              </span>
            </div>
            <p v-if="selected.flight?.drive" class="flex items-center gap-2 text-sm text-muted-foreground">
              <CarIcon class="size-4 shrink-0 text-road" />
              На своей машине — {{ rub(selected.flight.drive.total) }}, {{ hours(selected.flight.drive.drive_hours) }} в одну сторону
            </p>
            <p v-if="fromOtherCity(selected)" class="flex gap-2 rounded-2xl bg-[#e6f4ea] p-3 text-sm text-[#0d4f22]">
              <SparklesIcon class="mt-0.5 size-4 shrink-0" />
              <span>
                Вылет из г. {{ hubName(selected.flight!.best.hub) }}, туда {{ hours(selected.flight!.best.car!.drive_hours) }} на машине — бензин и парковка в цене.
                <b v-if="savings(selected)" class="font-semibold">Дешевле, чем из своего города, на {{ rub(savings(selected)) }}.</b>
              </span>
            </p>
            <p v-if="selected.drive?.border" class="flex gap-2 rounded-2xl bg-[#fef3e6] p-3 text-sm text-[#7a3d00]">
              <RouteIcon class="mt-0.5 size-4 shrink-0" />
              <span>
                Граница<template v-if="selected.drive.border.known">: +{{ hours(selected.drive.border.hours) }} на КПП, страховка {{ rub(selected.drive.border.insurance) }} — уже в цене</template>.
                {{ selected.drive.border.note }}
              </span>
            </p>
            <p v-if="selected.drive" class="text-xs text-muted-foreground">
              Оценка по прямой. Точный маршрут — в картах.
            </p>

            <div v-if="selected.drive && driveStops?.length" class="flex flex-col gap-2">
              <span class="flex items-center gap-1.5 text-sm font-medium">
                <MapPinnedIcon class="size-4 text-road" /> Можно остановиться по пути
              </span>
              <div class="flex flex-wrap gap-2">
                <RouterLink
                  v-for="s in driveStops"
                  :key="s.city.id"
                  :to="searchLink(selected, [`${s.city.id}:0`])"
                  class="flex h-9 items-center gap-1.5 rounded-full bg-[#fef3e6] px-3 text-sm text-[#7a3d00] transition-colors hover:bg-[#fde7cc]"
                >
                  <MoonIcon v-if="s.overnight" class="size-3.5" />
                  {{ s.city.name }}
                  <span class="opacity-70">{{ s.detour.duration_min < 10 ? 'без крюка' : `+${hours(s.detour.duration_min / 60)}` }}</span>
                </RouterLink>
              </div>
            </div>

            <div class="flex gap-2">
              <template v-if="selected.flight">
                <RouterLink :to="searchLink(selected)" class="btn btn--primary flex-1">
                  Все варианты <ArrowRightIcon class="size-4" />
                </RouterLink>
                <a v-if="selected.flight.best.booking_url" :href="selected.flight.best.booking_url" target="_blank" rel="noopener" class="btn btn--ghost">
                  Билеты <ExternalLinkIcon class="size-4" />
                </a>
              </template>
              <template v-else-if="drivePlaces.get(selected.key)">
                <RouterLink :to="searchLink(selected)" class="btn btn--primary flex-1">
                  План поездки <ArrowRightIcon class="size-4" />
                </RouterLink>
                <a :href="routeLink(selected)" target="_blank" rel="noopener" class="btn btn--ghost">
                  Маршрут <ExternalLinkIcon class="size-4" />
                </a>
              </template>
              <a v-else :href="routeLink(selected)" target="_blank" rel="noopener" class="btn btn--primary flex-1">
                Маршрут в Яндекс Картах <ExternalLinkIcon class="size-4" />
              </a>
              <FavoriteButton
                v-if="favorite"
                :pick="favorite.pick"
                :snapshot="favorite.snapshot"
                :home="favorite.home"
                :destination="favorite.destination"
                :params="favorite.params"
                class="size-11!"
              />
            </div>
          </div>
          <div v-if="selected.lookup || selected.unreachable" class="px-4 pb-4">
            <WeatherDays :query="weatherQuery" />
          </div>
        </article>
      </Transition>

      <button
        v-if="activeMode === 'fly' && canDrive && !loading && result?.drive_better"
        type="button"
        class="flex items-center gap-2 rounded-2xl bg-[#fef3e6] px-3 py-2.5 text-left text-sm text-[#7a3d00] transition-colors hover:bg-[#fde7cc]"
        @click="mode = 'car'"
      >
        <CarIcon class="size-4 shrink-0" />
        <span class="flex-1">
          Ещё {{ result.drive_better }} {{ plural(result.drive_better, 'город', 'города', 'городов') }} — дешевле и не дольше на своей машине
        </span>
        <ArrowRightIcon class="size-4 shrink-0" />
      </button>

      <p v-if="error" class="rounded-2xl bg-[#fce8e6] p-3 text-sm text-destructive">{{ error }}</p>
      <p v-else-if="result?.partial" class="text-xs text-muted-foreground">Часть цен не загрузилась — список может быть неполным.</p>

      <!-- Список -->
      <section v-if="homePlace" class="flex flex-col gap-1">
        <div class="flex items-baseline justify-between px-1 pb-1">
          <h2 class="text-base font-semibold">Дешевле всего</h2>
          <span v-if="!loading || activeMode === 'car'" class="text-xs text-muted-foreground tabular-nums">
            {{ shown.length }} {{ plural(shown.length, 'направление', 'направления', 'направлений') }}
          </span>
        </div>

        <template v-if="loading && activeMode === 'fly'">
          <div v-for="i in 6" :key="i" class="flex items-center gap-3 px-2 py-2.5">
            <div class="skeleton size-11 rounded-xl" />
            <div class="flex flex-1 flex-col gap-2"><div class="skeleton h-3.5 w-28" /><div class="skeleton h-3 w-40" /></div>
            <div class="skeleton h-4 w-16" />
          </div>
        </template>

        <div v-else-if="!shown.length && !error" class="flex flex-col items-center gap-2 rounded-3xl bg-muted px-6 py-8 text-center">
          <SearchXIcon class="size-6 text-muted-foreground" />
          <p class="text-sm font-medium">На эти даты цен пока нет</p>
          <p class="text-xs text-muted-foreground">
            Цены берём из поисков других путешественников — на узкие и далёкие даты их бывает мало. Попробуйте другой месяц или длительность.
          </p>
        </div>

        <TransitionGroup v-else :key="`${activeMode}-${periodKey}-${durationKey}`" tag="ul" name="row" appear class="flex flex-col">
          <li v-for="(d, i) in shown.slice(0, LIST_LIMIT)" :key="d.key" :style="{ '--i': Math.min(i, 15) }">
            <button type="button" class="row" :class="d.key === selectedKey && 'is-active'" @click="select(d)">
              <span class="grid size-11 shrink-0 place-items-center rounded-xl bg-muted">
                <img v-if="d.city?.arms" :src="armsUrl(d.city.id)" alt="" class="size-8 object-contain" loading="lazy">
                <PlaneIcon v-else-if="d.by === 'fly'" class="size-4 text-flight" />
                <CarIcon v-else class="size-4 text-road" />
              </span>
              <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                <span class="flex items-center gap-2">
                  <span class="truncate font-medium">{{ d.name }}</span>
                  <span v-if="i === 0" class="badge">дешевле всего</span>
                  <span v-else-if="savings(d)" class="badge badge--green">−{{ rub(savings(d)) }}</span>
                </span>
                <span class="truncate text-xs text-muted-foreground">{{ subtitle(d) }}</span>
              </span>
              <span class="shrink-0 font-semibold tabular-nums">{{ priceLabel(d) }}</span>
            </button>
          </li>
        </TransitionGroup>
      </section>
    </aside>
  </div>
</template>

<style scoped>
/* Переключатель «Самолёт / На машине» с бегущей подложкой */
.segmented {
  position: relative;
  display: grid;
  grid-template-columns: repeat(var(--count), 1fr);
  padding: 4px;
  border-radius: 999px;
  background: var(--muted);
}
.segmented__thumb {
  position: absolute;
  inset: 4px auto 4px 4px;
  width: calc((100% - 8px) / var(--count));
  border-radius: 999px;
  background: var(--background);
  box-shadow: 0 1px 3px rgb(0 0 0 / 0.12);
  transform: translateX(calc(100% * var(--index)));
  transition: transform 0.35s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.segmented__item {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  white-space: nowrap;
  height: 40px;
  border-radius: 999px;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--muted-foreground);
  transition: color 0.2s;
}
.segmented__item.is-active {
  color: var(--foreground);
}

/* Фильтры помещаются целиком: периоды переносятся на новую строку, длительности — ровная сетка */
.pill {
  flex: none;
  height: 34px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  box-shadow: inset 0 0 0 1px var(--border);
  transition: background-color 0.2s, color 0.2s, box-shadow 0.2s, transform 0.15s;
}
/* Ячейка сетки длительностей: ширина — четверть ряда, текст по центру без боковых отступов */
.pill--cell {
  padding: 0 2px;
  font-size: 12px;
  text-align: center;
}
.pill:hover {
  background: var(--muted);
}
.pill:active {
  transform: scale(0.96);
}
.pill.is-active {
  background: var(--foreground);
  color: var(--background);
  box-shadow: none;
}

.row {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 12px;
  padding: 8px;
  border-radius: 16px;
  text-align: left;
  transition: background-color 0.2s;
}
.row:hover {
  background: var(--muted);
}
.row.is-active {
  background: var(--accent);
}

.badge {
  flex: none;
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--accent);
  color: var(--accent-foreground);
  font-size: 11px;
  font-weight: 500;
}
.badge--green {
  background: #e6f4ea;
  color: #0d4f22;
}

.tile {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px;
  border-radius: 16px;
  background: var(--muted);
}
.tile__icon {
  width: 16px;
  height: 16px;
  margin-bottom: 4px;
  color: var(--muted-foreground);
}
.tile dt {
  font-size: 11px;
  color: var(--muted-foreground);
}
.tile dd {
  font-size: 13px;
  font-weight: 600;
  line-height: 1.25;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 44px;
  padding: 0 18px;
  border-radius: 999px;
  font-size: 14px;
  font-weight: 500;
  transition: background-color 0.2s, transform 0.15s;
}
.btn:active {
  transform: scale(0.97);
}
.btn--primary {
  background: var(--cta);
  color: var(--cta-foreground);
}
.btn--primary:hover {
  background: var(--cta-hover);
}
.btn--ghost {
  box-shadow: inset 0 0 0 1px var(--border);
}
.btn--ghost:hover {
  background: var(--muted);
}

/* .skeleton — глобальный, в style.css */

.loader-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--primary);
  animation: pulse 1s ease-in-out infinite alternate;
}
@keyframes pulse {
  from {
    opacity: 0.3;
    transform: scale(0.7);
  }
}

/* Строки списка появляются каскадом */
.row-enter-active {
  transition: opacity 0.35s ease, transform 0.35s cubic-bezier(0.2, 0.8, 0.2, 1);
  transition-delay: calc(var(--i) * 25ms);
}
.row-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.card-enter-active,
.card-leave-active {
  transition: opacity 0.25s ease, transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.card-enter-from {
  opacity: 0;
  transform: translateY(12px) scale(0.98);
}
.card-leave-to {
  opacity: 0;
  transform: scale(0.98);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* На телефоне низ карты закрыт краем панели — поднимаем подписи и масштаб */
@media (max-width: 1023px) {
  :deep(.maplibregl-ctrl-bottom-left),
  :deep(.maplibregl-ctrl-bottom-right) {
    margin-bottom: 24px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .row-enter-active,
  .card-enter-active,
  .card-leave-active,
  .segmented__thumb,
  .loader-dot {
    transition: none;
    animation: none;
  }
}
</style>

<style>
/* «Вы здесь» — точка с мягкой пульсацией */
.home-dot {
  width: 18px;
  height: 18px;
  border: 3px solid #fff;
  border-radius: 999px;
  background: var(--primary);
  box-shadow: 0 2px 8px rgb(11 87 208 / 0.4);
}
.home-dot::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 999px;
  background: var(--primary);
  opacity: 0.35;
  animation: home-pulse 2.4s ease-out infinite;
  z-index: -1;
}
@keyframes home-pulse {
  to {
    transform: scale(3);
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-dot::after {
    animation: none;
  }
}
</style>
