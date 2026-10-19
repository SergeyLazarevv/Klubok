<script setup lang="ts">
import { computed } from 'vue'
import type { Place, TripsResponse } from '@/api/client'
import { dateRangeLabel, hours, plural, rub } from '@/lib/format'
import { highlights, type Choice, type Highlight } from '@/lib/trip'
import DriveOptionCard from '@/components/DriveOptionCard.vue'
import TripOptionCard from '@/components/TripOptionCard.vue'

const props = defineProps<{ result: TripsResponse; travelers: number; dateFrom: string; dateTo: string }>()

// Сначала — самый дешёвый, самый быстрый, из своего города, на машине; ниже — все остальные варианты
const view = computed(() => highlights(props.result))
const picks = computed(() => view.value.picks)
const homeName = computed(() => props.result.home.name)

type Row =
  | { kind: 'heading'; key: string }
  | { kind: 'choice'; key: string; choice: Choice; tags?: Highlight[]; tradeoff?: (typeof picks.value)[number]['tradeoff'] }
const rows = computed<Row[]>(() => [
  ...picks.value.map((p): Row => ({ kind: 'choice', key: p.choice.key, choice: p.choice, tags: p.tags, tradeoff: p.tradeoff })),
  ...(view.value.rest.length ? [{ kind: 'heading', key: '__rest' } as Row] : []),
  ...view.value.rest.map((c): Row => ({ kind: 'choice', key: c.key, choice: c })),
])

// Заголовок: что выгоднее всего — машина, соседний аэропорт или вылет из своего города
const headline = computed(() => {
  const cheapest = picks.value.find((p) => p.tags.includes('cheapest'))?.choice
  if (!cheapest) return null
  if (cheapest.kind === 'drive') {
    const d = cheapest.drive
    // С остановками по пути — через какие города, а не время напрямую
    const how = d.stops?.length ? `через ${d.stops.map((s) => s.city.name).join(', ')}` : `${hours(d.drive_hours)} в одну сторону`
    if (!props.result.options.length) return `Билетов нет — доехать на машине: ${rub(d.total)}, ${how}`
    return `Дешевле всего на машине — ${rub(d.total)}, ${how}`
  }
  const b = cheapest.option
  const save = b.savings_vs_home?.amount ?? 0
  if (props.result.home_best && save > 0) {
    return `Через ${b.hub.city_name ?? b.hub.name} дешевле на ${rub(save)}, чем из города ${homeName.value}`
  }
  if (props.result.home_best) return `Выгоднее всего лететь из своего города — ${homeName.value}`
  return `Из города ${homeName.value} цен нет — лучший вариант через ${b.hub.city_name ?? b.hub.name}`
})
const cityOf = (p: Place) => p.city_name ?? p.name
</script>

<template>
  <div class="flex flex-col gap-4">
    <header class="flex flex-col gap-1">
      <h2 class="text-2xl font-semibold tracking-tight first-letter:uppercase sm:text-3xl">
        {{ result.destination.name_to ?? result.destination.name }}, {{ dateRangeLabel(dateFrom, dateTo) }}
      </h2>
      <p v-if="headline" class="text-muted-foreground sm:text-lg">{{ headline }}</p>
    </header>

    <p v-if="!picks.length" class="rounded-3xl bg-muted p-5 text-sm text-muted-foreground">
      На эти даты цен нет ни из одного доступного аэропорта. Попробуйте другие даты или длительность.
    </p>

    <!-- Лучшие и остальные — одним списком с ключом варианта: при пересчёте (поменяли остановки) вариант
         может перейти из лучших в остальные, а его карточка с раскрытым планом остаётся той же и не прыгает -->
    <template v-for="row in rows" :key="row.key">
      <h3 v-if="row.kind === 'heading'" class="mt-2 text-sm font-medium text-muted-foreground">Остальные варианты</h3>
      <DriveOptionCard
        v-else-if="row.choice.kind === 'drive'"
        :drive="row.choice.drive"
        :tags="row.tags"
        :tradeoff="row.tradeoff"
        :home="result.home"
        :destination="result.destination"
        :travelers="travelers"
        :date-from="dateFrom"
        :open-key="row.key"
      />
      <TripOptionCard
        v-else
        :option="row.choice.option"
        :tags="row.tags"
        :tradeoff="row.tradeoff"
        :home="result.home"
        :destination="result.destination"
        :travelers="travelers"
        :date-from="dateFrom"
        :open-key="row.key"
      />
    </template>

    <p v-if="result.hidden_flights" class="text-sm text-muted-foreground">
      {{ result.hidden_flights }} {{ plural(result.hidden_flights, 'перелёт скрыт', 'перелёта скрыто', 'перелётов скрыто') }}:
      они не дешевле машины и не заметно быстрее.
    </p>
    <p v-if="result.drive_skipped === 'too_long'" class="text-sm text-muted-foreground">
      На машине целиком не предлагаем: дорога дольше вашего лимита за рулём. Его можно изменить в настройках машины.
    </p>
    <p v-else-if="result.drive_skipped === 'border_not_allowed'" class="text-sm text-muted-foreground">
      На машине целиком не предлагаем: нужно пересечь границу. Включите «Готов ехать за границу» в настройках машины.
    </p>

    <div class="flex flex-col gap-1 text-xs text-muted-foreground">
      <p v-if="result.no_data.length && (result.options.length || !result.drive)">Нет цен на эти даты: {{ result.no_data.map(cityOf).join(', ') }}.</p>
      <p v-if="result.options.some((o) => o.warnings.includes('parking_estimated'))">Цена парковки ориентировочная — 300 ₽ в сутки.</p>
      <p v-if="result.partial" class="text-[#8c4a00]">Часть аэропортов не удалось проверить — результаты могут быть неполными.</p>
      <p>Цены — из недавних поисков на Aviasales и могут измениться. Точная цена — на сайте продавца.</p>
    </div>
  </div>
</template>
