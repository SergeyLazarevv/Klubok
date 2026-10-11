<script setup lang="ts">
import { InfoIcon, PlaneTakeoffIcon, TriangleAlertIcon } from '@lucide/vue'
import type { CarHub } from '@/api/client'
import { hours, km, plural, rub } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

defineProps<{
  hubs: CarHub[]
  loading: boolean
  error?: string
  travelers: number
  tripDays: number
}>()
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="flex items-center gap-2">
        <PlaneTakeoffIcon class="size-5 text-primary" /> Откуда можно улететь
      </CardTitle>
      <CardDescription>
        Расходы на машину туда и обратно на поездку в {{ tripDays }} {{ plural(tripDays, 'день', 'дня', 'дней') }}:
        бензин, парковка, страховка. Билеты добавим на следующем шаге.
      </CardDescription>
    </CardHeader>
    <CardContent>
      <p v-if="error" class="flex items-center gap-2 text-sm text-destructive">
        <TriangleAlertIcon class="size-4" /> {{ error }}
      </p>
      <div v-else-if="loading && !hubs.length" class="flex flex-col gap-3">
        <Skeleton v-for="i in 3" :key="i" class="h-16 w-full" />
      </div>
      <p v-else-if="!hubs.length" class="text-sm text-muted-foreground">
        Ни одного аэропорта в пределах радиуса и времени за рулём — попробуйте увеличить ограничения.
      </p>
      <ul v-else class="divide-y" :class="{ 'opacity-60 transition-opacity': loading }">
        <li v-for="h in hubs" :key="h.place.id" class="flex flex-col gap-1 py-3">
          <div class="flex items-baseline gap-3">
            <span class="w-10 shrink-0 font-mono text-sm font-semibold">{{ h.place.iata }}</span>
            <div class="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-2">
              <span class="font-medium">{{ h.place.city_name ?? h.place.name }}</span>
              <span class="text-xs text-muted-foreground">{{ h.place.name }}</span>
            </div>
            <span class="shrink-0 font-semibold tabular-nums">{{ rub(h.cost.total) }}</span>
          </div>
          <div class="flex flex-col gap-1 pl-13 text-xs text-muted-foreground tabular-nums">
            <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
              <span>
                {{ h.road.approximate ? '≈ ' : '' }}{{ km(h.road.distance_km) }} · {{ hours(h.drive_hours) }} в одну сторону
              </span>
              <span v-if="travelers > 1">{{ rub(h.per_person) }} с человека</span>
            </div>
            <div class="flex flex-wrap items-center gap-x-1">
              <span>бензин {{ rub(h.cost.fuel) }}</span>
              <span>· парковка {{ rub(h.cost.parking) }}{{ h.parking_estimated ? '*' : '' }}</span>
              <span v-if="h.cost.border.amount">· страховка {{ rub(h.cost.border) }}</span>
              <Badge v-if="h.border" variant="outline" class="ml-1 gap-1"><TriangleAlertIcon class="size-3" /> граница</Badge>
            </div>
            <p v-if="h.border?.note" class="flex gap-1 text-amber-700 dark:text-amber-400">
              <InfoIcon class="mt-0.5 size-3 shrink-0" /> {{ h.border.note }}
            </p>
          </div>
        </li>
      </ul>
      <p v-if="hubs.some((h) => h.parking_estimated)" class="mt-3 text-xs text-muted-foreground">
        * Цена парковки ориентировочная — 300 ₽ в сутки.
      </p>
    </CardContent>
  </Card>
</template>
