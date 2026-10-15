<script setup lang="ts">
import { CarIcon, ChevronDownIcon, InfoIcon } from '@lucide/vue'
import type { DriveTrip, Favorite, Place, TripSearchParams } from '@/api/client'
import { hours, km, rub } from '@/lib/format'
import type { Highlight } from '@/lib/trip'
import TripPlan from '@/components/TripPlan.vue'
import FavoriteButton from '@/components/FavoriteButton.vue'
import { drivePick, driveSnapshot } from '@/lib/favorites'

defineProps<{
  drive: DriveTrip
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
              :class="t === 'cheapest' ? 'bg-[#c4eed0] text-[#072711]' : 'bg-accent text-accent-foreground'"
            >
              {{ tagLabels[t] }}
            </span>
          </div>
          <h3 class="text-xl font-semibold tracking-tight">На своей машине</h3>
          <span class="text-sm text-muted-foreground">без самолёта · в любые даты · туда и обратно</span>
        </div>
        <div class="flex flex-col items-end">
          <span class="text-3xl font-semibold tracking-tight tabular-nums">{{ rub(drive.total) }}</span>
          <span class="text-xs text-muted-foreground">
            {{ travelers > 1 ? `${rub(drive.per_person)} с человека` : 'бензин и страховка' }}
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
      </div>

      <div class="flex flex-wrap items-center gap-2 pt-1">
        <button
          type="button"
          class="flex h-10 items-center gap-1.5 rounded-full px-4 text-sm font-medium ring-1 ring-input hover:bg-muted"
          :aria-expanded="open"
          @click="open = !open"
        >
          {{ open ? 'Свернуть' : 'Подробнее: дорога, что по пути, где остановиться' }}
          <ChevronDownIcon class="size-4 transition-transform" :class="open && 'rotate-180'" />
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

    <div v-if="open" class="border-t bg-[#f8fafd] p-4 sm:p-5">
      <TripPlan :home="home" :drive="drive" :destination="destination" :travelers="travelers" :date-from="dateFrom" :search-params="searchParams" :favorite="favorite" />
    </div>
  </article>
</template>
