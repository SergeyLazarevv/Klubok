<script setup lang="ts">
import { computed } from 'vue'
import { CarIcon, ChevronDownIcon, ExternalLinkIcon, InfoIcon } from '@lucide/vue'
import type { DriveTrip, Favorite, Place, TripSearchParams } from '@/api/client'
import { hours, km, rub } from '@/lib/format'
import { yandexRouteUrl } from '@/lib/geo'
import type { Highlight } from '@/lib/trip'
import TripPlan from '@/components/TripPlan.vue'
import FavoriteButton from '@/components/FavoriteButton.vue'
import WeatherChip from '@/components/WeatherChip.vue'
import WeatherDays from '@/components/WeatherDays.vue'
import { addDays, todayISO } from '@/stores/search'
import TripScreen from '@/components/TripScreen.vue'
import { drivePick, driveSnapshot } from '@/lib/favorites'
import { useTripOpen } from '@/lib/tripOpen'

const props = defineProps<{
  drive: DriveTrip
  home: Place
  destination: Place
  travelers: number
  dateFrom: string
  tags?: Highlight[]
  tradeoff?: { slowerHours?: number; pricierKop?: number }
  searchParams?: TripSearchParams // для избранного: параметры поиска, в котором найден вариант
  favorite?: Favorite // открыто из избранного: путевой лист этой записи
  openKey?: string // ключ варианта в адресе (?trip=), чтобы на телефоне открыть подробности отдельным экраном
}>()
const { open, screen } = useTripOpen(() => props.openKey)
const destName = computed(() => props.destination.city_name ?? props.destination.name)
const routeUrl = computed(() => yandexRouteUrl(props.home.location, props.destination.location))

// Даты у машины свободные — погода на те же дни, что в плане поездки (TripPlan): с начала периода, но не раньше
// завтрашнего дня, на trip_days
const weather = computed(() => {
  const tomorrow = addDays(todayISO(), 1)
  const from = props.dateFrom > tomorrow ? props.dateFrom : tomorrow
  return {
    lat: props.destination.location.lat,
    lon: props.destination.location.lon,
    from,
    to: addDays(from, Math.max(props.drive.trip_days - 1, 1)),
  }
})

const tagLabels: Record<Highlight, string> = {
  cheapest: 'Самый дешёвый',
  fastest: 'Самый быстрый',
  home: 'Из вашего города',
}
</script>

<template>
  <article class="overflow-hidden rounded-3xl bg-card ring-1 ring-border" :class="open && !screen && 'ring-primary/40'">
    <div class="flex flex-col gap-3 p-4 sm:p-5">
      <div class="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <div class="flex min-w-0 flex-col gap-1.5">
          <div v-if="tags?.length" class="flex flex-wrap gap-1.5">
            <span
              v-for="t in tags"
              :key="t"
              class="rounded-full px-2.5 py-0.5 text-xs font-medium"
              :class="t === 'cheapest' ? 'bg-[#c4eed0] text-[#072711]' : 'bg-accent text-accent-foreground'"
            >
              {{ tagLabels[t] }}
            </span>
          </div>
          <h3 class="text-xl font-semibold tracking-tight">На своей машине</h3>
          <span class="text-sm text-muted-foreground">без самолёта · в любые даты · туда и обратно</span>
        </div>
        <div class="flex flex-col items-start sm:items-end">
          <span class="text-3xl font-semibold tracking-tight tabular-nums">{{ rub(drive.total) }}</span>
          <span class="text-xs text-muted-foreground">
            бензин и страховка
          </span>
          <span v-if="drive.savings_vs_home && drive.savings_vs_home.amount > 0" class="text-sm font-medium text-success tabular-nums">
            −{{ rub(drive.savings_vs_home.amount) }} к перелёту из дома
          </span>
          <span v-if="tradeoff?.slowerHours" class="text-sm text-muted-foreground">дольше на ~{{ hours(tradeoff.slowerHours) }}</span>
          <span v-if="tradeoff?.pricierKop" class="text-sm text-muted-foreground tabular-nums">
            дороже на {{ rub(tradeoff.pricierKop) }}{{ tradeoff.slowerHours ? '' : ', зато быстрее' }}
          </span>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm">
        <span class="flex items-center gap-1.5">
          <CarIcon class="size-4 text-road" /> {{ home.name }} → {{ destination.city_name ?? destination.name }} ·
          {{ km(drive.road.distance_km) }} · {{ hours(drive.drive_hours) }} в одну сторону
        </span>
        <span v-if="drive.border" class="flex items-center gap-1 rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-900">
          <InfoIcon class="size-3" /> граница, страховка {{ rub(drive.border.insurance) }}
        </span>
        <WeatherChip :query="weather" />
      </div>

      <div class="flex flex-wrap items-center gap-2 pt-1">
        <button
          type="button"
          class="flex h-10 items-center gap-1.5 rounded-full px-4 text-sm font-medium ring-1 ring-input hover:bg-muted"
          :aria-expanded="open"
          @click="open = !open"
        >
          <template v-if="open && !screen">Свернуть</template>
          <template v-else>Подробнее<span class="max-sm:hidden">: дорога, что по пути, где остановиться</span></template>
          <ChevronDownIcon class="size-4 transition-transform max-sm:-rotate-90" :class="open && !screen && 'rotate-180'" />
        </button>
        <FavoriteButton
          :pick="drivePick"
          :snapshot="driveSnapshot(drive)"
          :home="home"
          :destination="destination"
          :params="searchParams"
        />
      </div>
    </div>

    <TripScreen
      v-if="screen"
      :title="`На машине → ${destName}`"
      :subtitle="`${km(drive.road.distance_km)} · ${hours(drive.drive_hours)} в одну сторону`"
      @close="open = false"
    >
      <div class="flex flex-col gap-5 bg-[#f8fafd] p-4">
        <WeatherDays :query="weather" :title="`Погода в г. ${destName}`" />
        <TripPlan :home="home" :drive="drive" :destination="destination" :travelers="travelers" :date-from="dateFrom" :search-params="searchParams" :favorite="favorite" />
      </div>
      <template #footer>
        <div class="flex min-w-0 flex-1 flex-col">
          <span class="text-xl leading-tight font-semibold tracking-tight tabular-nums">{{ rub(drive.total) }}</span>
          <span class="truncate text-xs text-muted-foreground">бензин и страховка на всех</span>
        </div>
        <a
          :href="routeUrl"
          target="_blank"
          rel="noopener"
          class="flex h-11 items-center gap-1.5 rounded-full bg-cta px-5 text-sm font-medium text-cta-foreground transition-colors hover:bg-cta-hover active:bg-cta-hover"
        >
          Маршрут <ExternalLinkIcon class="size-3.5" />
        </a>
        <FavoriteButton
          :pick="drivePick"
          :snapshot="driveSnapshot(drive)"
          :home="home"
          :destination="destination"
          :params="searchParams"
          class="size-11!"
        />
      </template>
    </TripScreen>
    <div v-else-if="open" class="flex flex-col gap-5 border-t bg-[#f8fafd] p-4 sm:p-5">
      <WeatherDays :query="weather" :title="`Погода в г. ${destName}`" />
      <TripPlan :home="home" :drive="drive" :destination="destination" :travelers="travelers" :date-from="dateFrom" :search-params="searchParams" :favorite="favorite" />
    </div>
  </article>
</template>
