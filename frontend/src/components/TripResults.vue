<script setup lang="ts">
import { computed } from 'vue'
import { CarIcon, ExternalLinkIcon, InfoIcon, PlaneIcon, TriangleAlertIcon } from '@lucide/vue'
import type { TripOption, TripsResponse } from '@/api/client'
import { hours, inMonth, km, localDay, plural, rub, transfersLabel } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const props = defineProps<{ result: TripsResponse; travelers: number; month: string }>()

const best = computed(() => props.result.options[0])
const homeName = computed(() => props.result.home.name)

// Заголовок: насколько выгоднее лучший вариант, чем вылет из своего города
const headline = computed(() => {
  const b = best.value
  if (!b) return null
  const save = b.savings_vs_home?.amount ?? 0
  if (props.result.home_best && save > 0) {
    return `Через ${b.hub.city_name ?? b.hub.name} дешевле на ${rub(save)}, чем из города ${homeName.value}`
  }
  if (props.result.home_best) return `Выгоднее всего лететь из своего города — ${homeName.value}`
  return `Из города ${homeName.value} цен нет — лучший вариант через ${b.hub.city_name ?? b.hub.name}`
})

const hubName = (o: TripOption) => o.hub.city_name ?? o.hub.name
const warningText: Record<string, string> = {
  stale_price: 'цене больше трёх дней — проверьте на сайте',
  road_approximate: 'дорога посчитана приблизительно',
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-xl first-letter:uppercase">
        {{ result.destination.name_to ?? result.destination.name }} {{ inMonth(month) }}
      </CardTitle>
      <CardDescription v-if="headline" class="text-base font-medium text-foreground">{{ headline }}</CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-4 px-3 sm:px-6">
      <p v-if="!result.options.length" class="text-sm text-muted-foreground">
        На этот месяц цен нет ни из одного доступного аэропорта. Попробуйте другой месяц или длительность.
      </p>

      <article
        v-for="(o, i) in result.options"
        :key="o.hub.id"
        class="flex flex-col gap-3 rounded-xl border p-3 sm:p-4"
        :class="i === 0 ? 'border-primary/50 bg-primary/5' : ''"
      >
        <header class="flex items-start justify-between gap-3">
          <div class="flex min-w-0 flex-col gap-1">
            <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
              <span class="text-lg leading-tight font-semibold">{{ hubName(o) }}</span>
              <span class="font-mono text-xs text-muted-foreground">{{ o.hub.iata }}</span>
              <Badge v-if="i === 0">Дешевле всего</Badge>
              <Badge v-if="o.car?.border" variant="outline" class="gap-1">
                <TriangleAlertIcon class="size-3" /> граница
              </Badge>
            </div>
            <div class="text-sm text-muted-foreground">
              {{ localDay(o.depart_at) }} → {{ localDay(o.return_at) }}
              <span class="whitespace-nowrap">· {{ o.trip_days }} {{ plural(o.trip_days, 'день', 'дня', 'дней') }}</span>
            </div>
          </div>
          <div class="flex shrink-0 flex-col items-end">
            <span class="text-xl font-bold whitespace-nowrap tabular-nums">{{ rub(o.total) }}</span>
            <span v-if="travelers > 1" class="text-xs whitespace-nowrap text-muted-foreground tabular-nums">{{ rub(o.per_person) }} с чел.</span>
            <span
              v-if="o.savings_vs_home && o.savings_vs_home.amount !== 0"
              class="text-xs font-medium tabular-nums"
              :class="o.savings_vs_home.amount > 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-muted-foreground'"
            >
              {{ o.savings_vs_home.amount > 0 ? 'выгоднее на' : 'дороже на' }} {{ rub(Math.abs(o.savings_vs_home.amount)) }}
            </span>
          </div>
        </header>

        <dl class="grid gap-1.5 text-sm">
          <div class="flex items-start justify-between gap-3">
            <dt class="flex flex-col text-muted-foreground">
              <span class="flex items-center gap-2"><PlaneIcon class="size-4" /> Билеты туда и обратно</span>
              <span class="pl-6 text-xs">
                туда {{ transfersLabel(o.transfers) }}, обратно {{ transfersLabel(o.return_transfers) }} · {{ o.airline }}
              </span>
            </dt>
            <dd class="flex shrink-0 flex-col items-end tabular-nums">
              <span>{{ rub(o.tickets) }}</span>
              <span v-if="travelers > 1" class="text-xs text-muted-foreground">{{ travelers }} × {{ rub(o.ticket_per_person) }}</span>
            </dd>
          </div>
          <div v-if="o.car" class="flex items-start justify-between gap-3">
            <dt class="flex flex-col text-muted-foreground">
              <span class="flex items-center gap-2"><CarIcon class="size-4" /> Машина туда и обратно</span>
              <span class="pl-6 text-xs">
                {{ km(o.car.road.distance_km) }}, {{ hours(o.car.drive_hours) }} в одну сторону ·
                бензин {{ rub(o.car.cost.fuel) }} · парковка {{ rub(o.car.cost.parking) }}{{ o.warnings.includes('parking_estimated') ? '*' : '' }}
                <template v-if="o.car.cost.border.amount"> · страховка {{ rub(o.car.cost.border) }}</template>
              </span>
            </dt>
            <dd class="shrink-0 tabular-nums">{{ rub(o.car.cost.total) }}</dd>
          </div>
        </dl>

        <p v-if="o.car?.border?.note" class="flex gap-1.5 text-xs text-amber-700 dark:text-amber-400">
          <InfoIcon class="mt-0.5 size-3 shrink-0" /> {{ o.car.border.note }}
        </p>
        <p
          v-for="w in o.warnings.filter((w) => warningText[w])"
          :key="w"
          class="flex gap-1.5 text-xs text-muted-foreground"
        >
          <InfoIcon class="mt-0.5 size-3 shrink-0" /> {{ warningText[w] }}
        </p>

        <footer class="flex flex-wrap items-center justify-between gap-2">
          <span v-if="o.found_on" class="text-xs text-muted-foreground">Цена найдена {{ localDay(o.found_on) }}</span>
          <Button v-if="o.booking_url" as="a" :href="o.booking_url" target="_blank" rel="noopener" variant="outline" size="sm">
            Билеты на Aviasales <ExternalLinkIcon />
          </Button>
        </footer>
      </article>

      <p v-if="result.no_data.length" class="text-xs text-muted-foreground">
        Нет цен на этот месяц: {{ result.no_data.map((p) => p.city_name ?? p.name).join(', ') }}.
      </p>
      <p v-if="result.options.some((o) => o.warnings.includes('parking_estimated'))" class="text-xs text-muted-foreground">
        * Цена парковки ориентировочная — 300 ₽ в сутки.
      </p>
      <p v-if="result.partial" class="text-xs text-amber-700 dark:text-amber-400">
        Часть аэропортов не удалось проверить — результаты могут быть неполными.
      </p>
      <p class="text-xs text-muted-foreground">
        Цены — из недавних поисков на Aviasales и могут измениться. Точная цена — на сайте продавца.
      </p>
    </CardContent>
  </Card>
</template>
