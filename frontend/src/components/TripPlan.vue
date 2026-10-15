<script setup lang="ts">
import { computed, onUnmounted, provide, ref, watch } from 'vue'
import {
  BedDoubleIcon,
  CarIcon,
  ClipboardListIcon,
  ExternalLinkIcon,
  HomeIcon,
  HouseIcon,
  InfoIcon,
  KeyRoundIcon,
  LandmarkIcon,
  MapIcon,
  MapPinIcon,
  PlaneLandingIcon,
  PlaneTakeoffIcon,
} from '@lucide/vue'
import { api, type DriveTrip, type Favorite, type Guide, type Place, type Poi, type RoadRoute, type TripItemStage, type TripOption, type TripSearchParams } from '@/api/client'
import { hours, km, localDay, localDayWeek, localTime, plural, rub, transfersLabel } from '@/lib/format'
import { greatCircleKm } from '@/lib/geo'
import { fromListing, fromPoi, notInGuide, type SightItem } from '@/lib/sights'
import { drivePick, driveSnapshot, favoriteTitle, flightPick, flightSnapshot } from '@/lib/favorites'
import { tripSheetKey, useTripSheet } from '@/lib/tripSheet'
import type { Stop } from '@/lib/tripMap'
import { useSearchStore } from '@/stores/search'
import PlanStage from '@/components/PlanStage.vue'
import SheetStage from '@/components/SheetStage.vue'
import SightStrip from '@/components/SightStrip.vue'
import TripMap from '@/components/TripMap.vue'
import TripMapDialog from '@/components/TripMapDialog.vue'

/** option — поездка с перелётом, drive — целиком на машине (тогда даты свободные, от начала выбранного диапазона). */
const props = defineProps<{
  home: Place
  destination: Place
  travelers: number
  dateFrom: string
  option?: TripOption
  drive?: DriveTrip
  searchParams?: TripSearchParams // параметры поиска, в котором найден вариант; по умолчанию — последнего поиска
  favorite?: Favorite // поездка из избранного: путевой лист берём у неё, даже если вариант сменился
}>()

const arrival = computed(() => props.option?.arrival ?? props.destination)
const cityOf = (p: Place) => p.city_name ?? p.name
const isoDay = (d: Date) => d.toISOString().slice(0, 10)
const tripDays = computed(() => props.option?.trip_days ?? props.drive?.trip_days ?? 1)
// У машины дат нет — считаем от начала выбранного диапазона, но не раньше завтрашнего дня
const driveStart = computed(() => {
  const chosen = new Date(props.dateFrom + 'T00:00:00Z')
  const tomorrow = new Date()
  tomorrow.setUTCHours(0, 0, 0, 0)
  tomorrow.setUTCDate(tomorrow.getUTCDate() + 1)
  return chosen > tomorrow ? chosen : tomorrow
})
const checkIn = computed(() => props.option?.depart_at.slice(0, 10) ?? isoDay(driveStart.value))
const checkOut = computed(() => {
  if (props.option) return props.option.return_at.slice(0, 10)
  const end = new Date(driveStart.value)
  end.setUTCDate(end.getUTCDate() + Math.max(tripDays.value - 1, 1))
  return isoDay(end)
})
const nights = computed(() => Math.max(tripDays.value - 1, 1))
const departLabel = computed(() => (props.option ? localDayWeek(props.option.depart_at) : 'в удобный день'))
const returnLabel = computed(() => (props.option ? localDayWeek(props.option.return_at) : 'через ' + tripDays.value + ' ' + plural(tripDays.value, 'день', 'дня', 'дней')))

// Дорога: до аэропорта вылета или, без перелёта, прямо до места
const car = computed(() => props.option?.car ?? (props.drive && { ...props.drive, border: props.drive.border }))
const roadTo = computed(() => props.option?.hub ?? props.destination)
const warnings = computed<string[]>(() => props.option?.warnings ?? props.drive?.warnings ?? [])
const mapOpen = ref(false)

const warningText: Record<string, string> = {
  stale_price: 'Цене больше трёх дней — проверьте на сайте',
  road_approximate: 'Дорога посчитана приблизительно',
}

// Путевой лист: пункты пользователя на этапах поездки. Первый пункт сохраняет поездку в избранное.
const search = useSearchStore()
const sheetParams = computed(() => props.searchParams ?? props.favorite?.params ?? search.lastQuery ?? undefined)
const sheet = useTripSheet({
  favorite: computed(() => props.favorite),
  params: sheetParams,
  pick: computed(() => (props.option ? flightPick(props.option) : drivePick)),
  snapshot: computed(() => (props.option ? flightSnapshot(props.option) : driveSnapshot(props.drive!))),
  title: computed(() =>
    sheetParams.value ? favoriteTitle(props.home, props.destination, sheetParams.value.date_from, sheetParams.value.date_to) : '',
  ),
})
provide(tripSheetKey, sheet)
const tripTotal = computed(() => props.option?.total.amount ?? props.drive?.total.amount ?? 0)
const doneCount = computed(() => sheet.items.value.filter((i) => i.done).length)

// Места из лент «По пути» и «Что посмотреть»: нажатие добавляет место в список этапа, повторное — убирает
const addedNames = (stage: TripItemStage) => sheet.byStage(stage).map((i) => i.title)
function toggleSight(stage: TripItemStage, s: SightItem) {
  const existing = sheet.byStage(stage).find((i) => i.title === s.name)
  if (existing) return sheet.remove(existing)
  const location = s.location && { lat: s.location[0], lon: s.location[1] }
  return sheet.add({ stage, kind: 'place', title: s.name, url: s.url, location })
}

// Места из листа на карте: через места по пути прокладываем дорогу, места в городе просто отмечаем
const stops = computed<Stop[]>(() =>
  sheet.items.value
    .filter((i) => i.location)
    .map((i) => ({ id: i.id, title: i.title, at: [i.location!.lat, i.location!.lon], onRoad: i.stage === 'road' && byCar.value })),
)
const roadStopCount = computed(() => stops.value.filter((s) => s.onRoad).length)
// Во что обходятся заезды: разница с прямой дорогой; бензин — по той же цене за км, что в расчёте машины
const mapRoad = ref<RoadRoute>()
const detour = computed(() => {
  const c = car.value
  if (!roadStopCount.value || !mapRoad.value || !c) return undefined
  const km = mapRoad.value.road.distance_km - c.road.distance_km
  const min = mapRoad.value.road.duration_min - c.road.duration_min
  const fuelPerKm = c.road.distance_km ? c.cost.fuel.amount / (2 * c.road.distance_km) : 0
  return { km: Math.max(km, 0), min: Math.max(min, 0), fuel: Math.max(km, 0) * fuelPerKm, approximate: mapRoad.value.road.approximate }
})

// Ушёл выбирать жильё на чужой сайт и вернулся — сразу предлагаем вставить ссылку на выбранное объявление
const awaitingStay = ref(false)
const stayPrompt = ref(false)
function onReturn() {
  if (document.visibilityState === 'visible' && awaitingStay.value) {
    awaitingStay.value = false
    stayPrompt.value = true
  }
}
document.addEventListener('visibilitychange', onReturn)
onUnmounted(() => document.removeEventListener('visibilitychange', onReturn))
const leaveForStay = () => {
  if (sheet.enabled.value) awaitingStay.value = true
}

// Проживание. Отели: переход на поиск Островка с городом и датами поездки. Кнопка доступна сразу — сервер
// перенаправит; заранее просим ссылку, чтобы сервер успел найти город на Островке к нажатию.
const hotelsQuery = computed(() => ({
  place: props.destination.id,
  check_in: checkIn.value,
  check_out: checkOut.value,
  travelers: props.travelers,
}))
const goUrl = (query: Record<string, string | number>) =>
  `/api/v1/hotels/go?${new URLSearchParams(Object.entries(query).map(([k, v]) => [k, String(v)]))}`
const hotelsUrl = computed(() => goUrl(hotelsQuery.value))
watch(hotelsQuery, (query) => void api.GET('/hotels/link', { params: { query } }), { immediate: true })
// Суточно.ру — квартиры, дома и гостевые дома с датами и гостями; работает в России и странах СНГ, Турции, ОАЭ…
const sutochnoCountries = ['RU', 'AM', 'AZ', 'BY', 'EG', 'GE', 'KZ', 'KG', 'TH', 'TR', 'AE', 'UZ']
const sutochnoUrl = computed(() =>
  sutochnoCountries.includes(props.destination.country_code) ? goUrl({ ...hotelsQuery.value, provider: 'sutochno' }) : undefined,
)
// Квартиры посуточно на Авито — только в России; даты на Авито выбирают сами
const rentalsUrl = computed(() => (props.destination.country_code === 'RU' ? goUrl({ ...hotelsQuery.value, provider: 'avito' }) : undefined))

const sights = ref<Poi[]>()
const sightsFailed = ref(false)
const roadSights = ref<Poi[]>()
const roadSightsFailed = ref(false)

// Достопримечательности для ленты: сначала с фото, потом с описанием
const pickSights = (items: Poi[]) => {
  const score = (p: Poi) => (p.photo ? 2 : 0) + (p.description || p.details_pending ? 1 : 0)
  return items
    .filter((p) => p.category === 'attraction' && p.name)
    .sort((a, b) => score(b) - score(a))
    .slice(0, 8)
}

// Фото и описание показанных мест, которые ещё не загружались, — подтягиваем сразу (сервер их сохранит)
async function withDetails(list: Poi[], update: (p: Poi) => void) {
  await Promise.all(
    list
      .filter((p) => p.details_pending)
      .map(async (p) => {
        const [type, id] = p.id.split('/') as ['node' | 'way' | 'relation', string]
        const { data } = await api.GET('/pois/{type}/{id}', { params: { path: { type, id: Number(id) } } })
        if (data) update(data)
      }),
  )
}
const replaceIn = (target: typeof sights) => (p: Poi) => {
  if (target.value) target.value = target.value.map((s) => (s.id === p.id ? p : s))
}

// «По пути» — между городами, а не в них: места ближе этого к дому и аэропорту не показываем
const cityRadiusKm = 15

// Что посмотреть на месте: сначала путеводитель (Викигид — места, отобранные людьми), потом OSM без повторов.
// undefined — грузится, null — статьи о городе нет.
const guide = ref<Guide | null>()
const guideItems = computed(() =>
  (guide.value?.listings ?? [])
    .map(fromListing)
    .filter((_, i) => ['see', 'do'].includes(guide.value!.listings[i]!.type))
    .sort((a, b) => Number(!!b.photoUrl) - Number(!!a.photoUrl))
    .slice(0, 12),
)
watch(
  () => props.destination.id,
  async (id) => {
    guide.value = undefined
    sights.value = undefined
    sightsFailed.value = false
    const [g, osm] = await Promise.all([
      api.GET('/places/{id}/guide', { params: { path: { id } } }),
      api.GET('/places/{id}/pois', { params: { path: { id } } }),
    ])
    guide.value = g.data ?? null
    sightsFailed.value = !!osm.error
    sights.value = pickSights(notInGuide(osm.data?.items ?? [], guide.value?.listings ?? []))
    void withDetails(sights.value, replaceIn(sights))
  },
  { immediate: true },
)

// По пути: достопримечательности до 5 км от дороги до аэропорта — раз едем рядом, стоит заехать
const byCar = computed(() => (props.option ? !!props.option.car && props.home.id !== props.option.hub.id : !!props.drive))
watch(
  () => [props.home.id, roadTo.value.id, byCar.value] as const,
  async ([from, to, car]) => {
    roadSights.value = undefined
    roadSightsFailed.value = false
    if (!car) return
    const { data, error } = await api.GET('/routes/road/pois', { params: { query: { from, to } } })
    roadSightsFailed.value = !!error
    const ends = [props.home, roadTo.value].map((p): [number, number] => [p.location.lat, p.location.lon])
    const between = (data?.items ?? []).filter((p) =>
      ends.every((e) => greatCircleKm(e, [p.location.lat, p.location.lon]) > cityRadiusKm),
    )
    roadSights.value = pickSights(between)
    void withDetails(roadSights.value, replaceIn(roadSights))
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex flex-col gap-6">
    <TripMap :home="home" :option="option" :drive="drive" :destination="destination" :stops="stops" @road="mapRoad = $event" />

    <!-- Лента поездки по этапам -->
    <ol class="flex flex-col">
      <PlanStage v-if="sheet.enabled.value" :icon="ClipboardListIcon" title="Сборы" subtitle="до выезда">
        <SheetStage stage="prep" add-label="Добавить дело" placeholder="Например: оформить страховку, забронировать парковку" />
      </PlanStage>

      <PlanStage v-if="car" :icon="CarIcon" title="Дорога" :subtitle="departLabel">
        <article class="plan-card">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 flex-col gap-0.5">
              <span class="font-semibold">{{ home.name }} → {{ roadTo.name }}</span>
              <span class="text-sm text-muted-foreground">
                {{ km(car.road.distance_km) }} · {{ hours(car.drive_hours) }} за рулём{{ option ? '' : ' в одну сторону' }}
              </span>
            </div>
            <span class="shrink-0 text-lg font-semibold tabular-nums">{{ rub(car.cost.total) }}</span>
          </div>
          <div class="flex flex-wrap gap-1.5 text-xs">
            <span class="chip">бензин туда и обратно {{ rub(car.cost.fuel) }}</span>
            <span v-if="option" class="chip">парковка {{ rub(car.cost.parking) }}{{ warnings.includes('parking_estimated') ? ' — ориентировочно' : '' }}</span>
            <span v-if="car.cost.border.amount" class="chip">страховка {{ rub(car.cost.border) }}</span>
          </div>
          <p v-if="detour" class="flex gap-1.5 text-sm">
            <MapPinIcon class="mt-0.5 size-4 shrink-0 text-primary" />
            <span>
              Маршрут проложен через {{ roadStopCount }} {{ plural(roadStopCount, 'место', 'места', 'мест') }}:
              <template v-if="detour.km >= 1">
                <span class="font-medium">+{{ km(detour.km) }}, +{{ hours(detour.min / 60) }} в пути</span>
                <span class="text-muted-foreground"> · ≈ +{{ rub(detour.fuel) }} бензина{{ detour.approximate ? ', оценка по прямой' : '' }}</span>
              </template>
              <span v-else class="text-muted-foreground">крюка почти нет{{ detour.min >= 1 ? `, +${hours(detour.min / 60)} на заезд` : '' }}</span>
            </span>
          </p>
          <p v-if="car.border?.note" class="flex gap-1.5 text-xs text-[#8c4a00]">
            <InfoIcon class="mt-0.5 size-3.5 shrink-0" /> {{ car.border.note }}
          </p>
        </article>
        <template v-if="byCar">
          <span class="mt-1 text-sm font-semibold">По пути <span class="font-normal text-muted-foreground">— до 5 км от трассы</span></span>
          <SightStrip
            :items="roadSights?.map(fromPoi)"
            :failed="roadSightsFailed"
            empty="Вдоль дороги не нашлось отмеченных достопримечательностей."
            :added="sheet.enabled.value ? addedNames('road') : undefined"
            @toggle="toggleSight('road', $event)"
          />
        </template>
        <SheetStage stage="road" add-label="Заметка в дорогу" placeholder="Например: заехать на озеро, заправиться в Тольятти" />
      </PlanStage>

      <PlanStage v-if="option" :icon="PlaneTakeoffIcon" title="Вылет" :subtitle="localDayWeek(option.depart_at)">
        <article class="plan-card">
          <div class="flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span class="rounded-full bg-muted px-2.5 py-0.5 font-mono font-medium text-foreground">{{ option.airline }}</span>
            <span>туда и обратно {{ rub(option.tickets) }}</span>
          </div>
          <div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3">
            <div class="flex flex-col">
              <span class="text-2xl font-semibold">{{ option.hub.iata }}</span>
              <span class="text-xs text-muted-foreground">{{ localTime(option.depart_at) }} · {{ cityOf(option.hub) }}</span>
            </div>
            <div class="flex flex-col items-center gap-1 text-xs text-muted-foreground">
              <span v-if="option.duration_to_min">{{ hours(option.duration_to_min / 60) }}</span>
              <span class="relative flex h-1.5 w-full items-center rounded-full bg-accent">
                <PlaneTakeoffIcon class="absolute left-1/2 size-5 -translate-x-1/2 rounded-full bg-card p-0.5 text-flight" />
              </span>
              <span :class="option.transfers === 0 && 'font-medium text-success'">{{ transfersLabel(option.transfers) }}</span>
            </div>
            <div class="flex flex-col items-end">
              <span class="text-2xl font-semibold">{{ arrival.iata }}</span>
              <span class="text-xs text-muted-foreground">{{ cityOf(arrival) }}</span>
            </div>
          </div>
        </article>
      </PlanStage>

      <PlanStage
        :icon="BedDoubleIcon"
        title="Проживание"
        :subtitle="`${option ? '' : 'например, '}${localDay(checkIn)} – ${localDay(checkOut)} · ${nights} ${plural(nights, 'ночь', 'ночи', 'ночей')}`"
      >
        <div class="flex flex-col divide-y divide-border rounded-2xl bg-card ring-1 ring-border">
          <article class="grid grid-cols-[3rem_minmax(0,1fr)] items-center gap-3 p-4 sm:flex">
            <span class="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-accent text-accent-foreground">
              <BedDoubleIcon class="size-6" />
            </span>
            <div class="flex min-w-0 flex-1 flex-col gap-0.5">
              <span class="font-semibold">Отели — {{ cityOf(destination) }}</span>
              <span class="text-sm text-muted-foreground">
                {{ nights }} {{ plural(nights, 'ночь', 'ночи', 'ночей') }} · {{ travelers }}
                {{ plural(travelers, 'гость', 'гостя', 'гостей') }} — цены и наличие на Островке
              </span>
            </div>
            <a
              :href="hotelsUrl"
              target="_blank"
              rel="noopener"
              class="flex h-10 shrink-0 items-center justify-center gap-1.5 rounded-full px-4 text-sm font-medium text-primary ring-1 ring-input hover:bg-accent max-sm:col-span-2"
              @click="leaveForStay"
            >
              Найти отель <ExternalLinkIcon class="size-3.5" />
            </a>
          </article>
          <article v-if="sutochnoUrl" class="grid grid-cols-[3rem_minmax(0,1fr)] items-center gap-3 p-4 sm:flex">
            <span class="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
              <HomeIcon class="size-6" />
            </span>
            <div class="flex min-w-0 flex-1 flex-col gap-0.5">
              <span class="font-semibold">Квартиры, дома и гостевые дома</span>
              <span class="text-sm text-muted-foreground">
                На Суточно.ру — сразу с вашими датами и {{ travelers }} {{ plural(travelers, 'гостем', 'гостями', 'гостями') }}
              </span>
            </div>
            <a
              :href="sutochnoUrl"
              target="_blank"
              rel="noopener"
              class="flex h-10 shrink-0 items-center justify-center gap-1.5 rounded-full px-4 text-sm font-medium text-primary ring-1 ring-input hover:bg-accent max-sm:col-span-2"
              @click="leaveForStay"
            >
              Найти на Суточно.ру <ExternalLinkIcon class="size-3.5" />
            </a>
          </article>
          <article v-if="rentalsUrl" class="grid grid-cols-[3rem_minmax(0,1fr)] items-center gap-3 p-4 sm:flex">
            <span class="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
              <KeyRoundIcon class="size-6" />
            </span>
            <div class="flex min-w-0 flex-1 flex-col gap-0.5">
              <span class="font-semibold">Квартиры посуточно</span>
              <span class="text-sm text-muted-foreground">Частное жильё на Авито — даты и гостей укажите на сайте</span>
            </div>
            <a
              :href="rentalsUrl"
              target="_blank"
              rel="noopener"
              class="flex h-10 shrink-0 items-center justify-center gap-1.5 rounded-full px-4 text-sm font-medium text-primary ring-1 ring-input hover:bg-accent max-sm:col-span-2"
              @click="leaveForStay"
            >
              Смотреть на Авито <ExternalLinkIcon class="size-3.5" />
            </a>
          </article>
        </div>
        <SheetStage
          stage="stay"
          add-label="Выбрали жильё? Добавьте ссылку на объявление"
          placeholder="Ссылка на объявление или отель"
          :nights="nights"
          :prompt="stayPrompt"
          @prompted="stayPrompt = false"
        />
      </PlanStage>

      <PlanStage :icon="LandmarkIcon" title="Что посмотреть" :subtitle="`${tripDays} ${plural(tripDays, 'день', 'дня', 'дней')} на месте`">
        <span v-if="sheet.enabled.value" class="text-sm font-semibold">Мой список</span>
        <SheetStage
          stage="see"
          add-label="Добавить своё место"
          placeholder="Например: прогулка по набережной Казанки"
          hint="Нажмите на место ниже — оно попадёт в список. Или впишите своё."
        />
        <template v-if="guide">
          <p v-if="guide.overview" class="line-clamp-4 text-sm text-muted-foreground">{{ guide.overview }}</p>
          <SightStrip
            v-if="guideItems.length"
            :items="guideItems"
            empty=""
            :added="sheet.enabled.value ? addedNames('see') : undefined"
            @toggle="toggleSight('see', $event)"
          />
          <span class="text-xs text-muted-foreground">
            По материалам <a :href="guide.url" target="_blank" rel="noopener" class="text-primary hover:underline">Викигида</a>
            · {{ guide.license }}
          </span>
          <span class="mt-1 text-sm font-semibold">Ещё рядом <span class="font-normal text-muted-foreground">— по данным OpenStreetMap</span></span>
        </template>
        <SightStrip
          :items="guide === undefined ? undefined : sights?.map(fromPoi)"
          :failed="sightsFailed"
          :empty="guide ? 'Других отмеченных мест рядом нет.' : 'Рядом не нашлось отмеченных достопримечательностей.'"
          :added="sheet.enabled.value ? addedNames('see') : undefined"
          @toggle="toggleSight('see', $event)"
        />
        <button
          type="button"
          class="flex w-fit items-center gap-1.5 rounded-full px-4 py-2 text-sm font-medium text-primary ring-1 ring-input hover:bg-accent"
          @click="mapOpen = true"
        >
          <MapIcon class="size-4" /> Все места, кафе и заправки на карте
        </button>
        <SheetStage stage="place" add-label="Аренда машины, трансфер или другая услуга" placeholder="Ссылка на аренду или сервис — или просто текст" />
      </PlanStage>

      <PlanStage v-if="option" :icon="PlaneLandingIcon" title="Возвращение" :subtitle="localDayWeek(option.return_at)" :last="!option.car">
        <article class="plan-card">
          <div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3">
            <div class="flex flex-col">
              <span class="text-2xl font-semibold">{{ arrival.iata }}</span>
              <span class="text-xs text-muted-foreground">{{ localTime(option.return_at) }} · {{ cityOf(arrival) }}</span>
            </div>
            <div class="flex flex-col items-center gap-1 text-xs text-muted-foreground">
              <span v-if="option.duration_back_min">{{ hours(option.duration_back_min / 60) }}</span>
              <span class="relative flex h-1.5 w-full items-center rounded-full bg-accent">
                <PlaneLandingIcon class="absolute left-1/2 size-5 -translate-x-1/2 rounded-full bg-card p-0.5 text-flight" />
              </span>
              <span :class="option.return_transfers === 0 && 'font-medium text-success'">{{ transfersLabel(option.return_transfers) }}</span>
            </div>
            <div class="flex flex-col items-end">
              <span class="text-2xl font-semibold">{{ option.hub.iata }}</span>
              <span class="text-xs text-muted-foreground">{{ cityOf(option.hub) }}</span>
            </div>
          </div>
        </article>
      </PlanStage>

      <PlanStage v-if="car" :icon="HouseIcon" title="Домой" :subtitle="returnLabel" last>
        <article class="plan-card">
          <span class="font-semibold">{{ roadTo.name }} → {{ home.name }}</span>
          <span class="text-sm text-muted-foreground">
            тот же путь, {{ km(car.road.distance_km) }} · {{ hours(car.drive_hours) }} — уже учтено в стоимости машины
          </span>
        </article>
      </PlanStage>
    </ol>

    <section v-if="sheet.items.value.length" class="flex flex-col gap-1 rounded-2xl bg-card p-4 ring-1 ring-border">
      <div class="flex items-baseline justify-between gap-3">
        <span class="font-semibold">Путевой лист</span>
        <span class="text-sm text-muted-foreground">сделано {{ doneCount }} из {{ sheet.items.value.length }}</span>
      </div>
      <div v-if="sheet.extra.value" class="flex items-baseline justify-between gap-3">
        <span class="text-sm text-muted-foreground">
          {{ option ? 'билеты и дорога' : 'дорога' }} {{ rub(tripTotal) }} + ваши добавления {{ rub(sheet.extra.value) }}
        </span>
        <span class="text-lg font-semibold tabular-nums">{{ rub(tripTotal + sheet.extra.value) }}</span>
      </div>
      <p v-if="sheet.unchosen.value" class="text-xs text-muted-foreground">
        {{ sheet.unchosen.value }} {{ plural(sheet.unchosen.value, 'вариант', 'варианта', 'вариантов') }} без отметки «Выбрано» в итог не
        {{ sheet.unchosen.value === 1 ? 'входит' : 'входят' }}.
      </p>
      <p v-if="sheet.justSaved.value" class="text-sm text-muted-foreground">
        Поездка сохранена в <RouterLink to="/favorites" class="font-medium text-primary hover:underline">избранное</RouterLink> — путевой лист там же.
      </p>
    </section>

    <p v-for="w in warnings.filter((w) => warningText[w])" :key="w" class="flex gap-1.5 text-xs text-muted-foreground">
      <InfoIcon class="mt-0.5 size-3.5 shrink-0" /> {{ warningText[w] }}
    </p>

    <TripMapDialog v-model:open="mapOpen" :home="home" :option="option" :drive="drive" :destination="destination" :stops="stops" />
  </div>
</template>

<style scoped>
@reference '../style.css';

.plan-card {
  @apply flex flex-col gap-2.5 rounded-2xl bg-card p-4 ring-1 ring-border;
}
.chip {
  @apply rounded-full bg-muted px-2.5 py-1 text-muted-foreground;
}
</style>
