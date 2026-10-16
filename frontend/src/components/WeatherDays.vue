<script setup lang="ts">
import { computed } from 'vue'
import { localDayWeek } from '@/lib/format'
import { summarize, temp, useWeather, weatherKind, wetChance, type WeatherQuery } from '@/lib/weather'

/**
 * Погода по дням поездки: плитки «день — значок — днём/ночью — осадки». Длинный период (больше двух недель) —
 * одной сводкой: по дням на месяц вперёд это шум. Пока грузится — скелетон, нет данных — ничего.
 */
const props = defineProps<{ query?: WeatherQuery; title?: string }>()
const weather = useWeather(() => props.query)
const days = computed(() => weather.value?.days ?? [])
const s = computed(() => summarize(days.value))
const long = computed(() => days.value.length > 14)
const loading = computed(() => !!props.query && !weather.value)
</script>

<template>
  <section v-if="loading || s" class="flex flex-col gap-2">
    <div class="flex items-baseline justify-between gap-2">
      <h4 class="text-sm font-medium">{{ title ?? 'Погода' }}</h4>
      <span v-if="s" class="text-xs text-muted-foreground" :title="s.hint">
        {{ s.source }}<template v-if="weather?.stale"> · из кэша</template>
      </span>
    </div>

    <div v-if="loading" class="flex gap-2 overflow-hidden">
      <div v-for="i in 4" :key="i" class="h-[92px] w-[72px] shrink-0 animate-pulse rounded-2xl bg-muted motion-reduce:animate-none" />
    </div>

    <div v-else-if="s && long" class="flex items-center gap-3 rounded-2xl bg-muted p-3 text-sm">
      <component :is="s.kind.icon" class="size-6 shrink-0" :class="s.kind.tone" />
      <span>
        <b class="font-semibold tabular-nums">{{ s.range }}</b>, {{ s.wet }}
        <span class="block text-xs text-muted-foreground">{{ s.kind.label }} чаще всего · {{ s.hint }}</span>
      </span>
    </div>

    <ul v-else class="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
      <li
        v-for="d in days"
        :key="d.date"
        class="flex w-[72px] shrink-0 flex-col items-center gap-1 rounded-2xl bg-muted px-1.5 py-2 text-center"
        :class="d.source === 'climate' && 'bg-muted/60'"
        :title="`${weatherKind(d.code).label}${d.source === 'climate' ? ' — обычно в этот день' : ''}`"
      >
        <span class="text-[11px] leading-tight text-muted-foreground">{{ localDayWeek(d.date) }}</span>
        <component :is="weatherKind(d.code).icon" class="size-5" :class="weatherKind(d.code).tone" />
        <span class="text-sm leading-none font-semibold tabular-nums">{{ temp(d.t_max) }}</span>
        <span class="text-xs leading-none text-muted-foreground tabular-nums">{{ temp(d.t_min) }}</span>
        <span v-if="wetChance(d) >= 0.15" class="text-[11px] leading-none text-sky-600 tabular-nums">
          {{ Math.round(wetChance(d) * 100) }}%
        </span>
      </li>
    </ul>

    <p v-if="s" class="text-[11px] text-muted-foreground">
      <template v-if="s.source === 'обычно в эти дни'">До поездки больше двух недель — это не прогноз, а обычная погода в эти даты по 5 прошлым годам. </template>
      <template v-else-if="s.source !== 'прогноз' && !long">Бледные дни — не прогноз, а обычная погода по 5 прошлым годам. </template>
      Погода: <a href="https://open-meteo.com" target="_blank" rel="noopener" class="underline hover:text-foreground">Open-Meteo</a>
    </p>
  </section>
</template>
