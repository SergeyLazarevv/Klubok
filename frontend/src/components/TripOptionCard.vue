<script setup lang="ts">
import { computed } from 'vue'
import { CarIcon, ChevronDownIcon, ExternalLinkIcon, PlaneTakeoffIcon } from '@lucide/vue'
import type { Favorite, Place, TripOption, TripSearchParams } from '@/api/client'
import { hours, km, localDay, plural, rub, transfersLabel } from '@/lib/format'
import { doorToDoorHours, type Highlight } from '@/lib/trip'
import TripPlan from '@/components/TripPlan.vue'
import FavoriteButton from '@/components/FavoriteButton.vue'
import WeatherChip from '@/components/WeatherChip.vue'
import WeatherDays from '@/components/WeatherDays.vue'
import { flightPick, flightSnapshot } from '@/lib/favorites'

const props = defineProps<{
  option: TripOption
  home: Place
  destination: Place
  travelers: number
  dateFrom: string
  tags?: Highlight[]
  tradeoff?: { slowerHours?: number; pricierKop?: number }
  searchParams?: TripSearchParams // для избранного: параметры поиска, в котором найден вариант
  favorite?: Favorite // открыто из избранного: путевой лист этой записи
}>()
const open = defineModel<boolean>('open', { default: false })

const tagLabels: Record<Highlight, string> = {
  cheapest: 'Самый дешёвый',
  fastest: 'Самый быстрый',
  home: 'Из вашего города',
}
const cityOf = (p: Place) => p.city_name ?? p.name
const arrival = computed(() => props.option.arrival ?? props.destination)
const door = computed(() => doorToDoorHours(props.option))
const fromHome = computed(() => (props.option.hub.city_iata ?? props.option.hub.iata) === props.home.iata)
// Погода в месте прилёта на дни поездки — по местным датам рейсов
const weather = computed(() => ({
  lat: arrival.value.location.lat,
  lon: arrival.value.location.lon,
  from: props.option.depart_at.slice(0, 10),
  to: props.option.return_at.slice(0, 10),
}))
</script>

<template>
  <article class="overflow-hidden rounded-3xl bg-card ring-1 ring-border" :class="open && 'ring-primary/40'">
    <div class="flex flex-col gap-3 p-4 sm:p-5">
      <div class="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <div class="flex min-w-0 flex-col gap-1.5">
          <div v-if="tags?.length" class="flex flex-wrap gap-1.5">
            <span
              v-for="t in tags"
              :key="t"
              class="rounded-full px-2.5 py-0.5 text-xs font-medium"
              :class="t === 'cheapest' ? 'bg-[#c4eed0] text-[#072711]' : t === 'fastest' ? 'bg-accent text-accent-foreground' : 'bg-muted text-muted-foreground'"
            >
              {{ tagLabels[t] }}
            </span>
          </div>
          <h3 class="text-xl font-semibold tracking-tight">
            {{ fromHome ? 'Вылет из своего города' : `Через ${cityOf(option.hub)}` }}
            <span class="text-sm font-normal text-muted-foreground">{{ option.hub.name }}</span>
          </h3>
          <span class="text-sm text-muted-foreground">
            {{ localDay(option.depart_at) }} – {{ localDay(option.return_at) }} · {{ option.trip_days }}
            {{ plural(option.trip_days, 'день', 'дня', 'дней') }}
          </span>
        </div>
        <div class="flex flex-col items-end">
          <span class="text-3xl font-semibold tracking-tight tabular-nums">{{ rub(option.total) }}</span>
          <span class="text-xs text-muted-foreground">
            дорога и билеты на всех
          </span>
          <span v-if="option.savings_vs_home && option.savings_vs_home.amount > 0" class="text-sm font-medium text-success tabular-nums">
            −{{ rub(option.savings_vs_home.amount) }} к вылету из дома
          </span>
          <span v-if="tradeoff?.slowerHours" class="text-sm text-muted-foreground">дольше на ~{{ hours(tradeoff.slowerHours) }}</span>
          <span v-if="tradeoff?.pricierKop" class="text-sm text-muted-foreground tabular-nums">
            дороже на {{ rub(tradeoff.pricierKop) }}, зато быстрее
          </span>
        </div>
      </div>

      <!-- Маршрут в одну строку: дорога → перелёт, время от двери до места -->
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm">
        <span v-if="option.car" class="flex items-center gap-1.5">
          <CarIcon class="size-4 text-road" /> {{ km(option.car.road.distance_km) }} · {{ hours(option.car.drive_hours) }}
        </span>
        <span class="flex items-center gap-1.5">
          <PlaneTakeoffIcon class="size-4 text-flight" />
          {{ option.hub.iata }} → {{ arrival.iata }} ·
          <span :class="option.transfers === 0 && 'text-success'">{{ transfersLabel(option.transfers) }}</span>
          <template v-if="option.duration_to_min"> · {{ hours(option.duration_to_min / 60) }}</template>
        </span>
        <span v-if="door" class="rounded-full bg-muted px-2.5 py-0.5 text-xs text-muted-foreground">
          от двери до места ~{{ hours(door) }}
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
          {{ open ? 'Свернуть' : 'Подробнее: дорога, перелёт, что по пути' }}
          <ChevronDownIcon class="size-4 transition-transform" :class="open && 'rotate-180'" />
        </button>
        <a
          v-if="option.booking_url"
          :href="option.booking_url"
          target="_blank"
          rel="noopener"
          class="flex h-10 items-center gap-1.5 rounded-full bg-cta px-5 text-sm font-medium text-cta-foreground transition-colors hover:bg-cta-hover"
        >
          Купить билеты <ExternalLinkIcon class="size-3.5" />
        </a>
        <FavoriteButton
          :pick="flightPick(option)"
          :snapshot="flightSnapshot(option)"
          :home="home"
          :destination="destination"
          :params="searchParams"
        />
        <span v-if="option.found_on" class="ml-auto text-xs text-muted-foreground">цена найдена {{ localDay(option.found_on) }}</span>
      </div>
    </div>

    <div v-if="open" class="flex flex-col gap-5 border-t bg-[#f8fafd] p-4 sm:p-5">
      <WeatherDays :query="weather" :title="`Погода в г. ${cityOf(destination)}`" />
      <TripPlan :home="home" :option="option" :destination="destination" :travelers="travelers" :date-from="dateFrom" :search-params="searchParams" :favorite="favorite" />
    </div>
  </article>
</template>
