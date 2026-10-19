<script setup lang="ts">
import { computed } from 'vue'
import { summarize, useWeather, type WeatherQuery } from '@/lib/weather'

/** Погода на даты поездки одной плашкой: значок, «+4…+10°», осадки. Пока грузится или нет данных — ничего. */
const props = defineProps<{ query?: WeatherQuery }>()
const { weather } = useWeather(() => props.query)
const s = computed(() => weather.value && summarize(weather.value.days))
</script>

<template>
  <Transition name="weather-fade">
    <span
      v-if="s"
      class="flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-0.5 text-xs text-muted-foreground"
      :title="`${s.kind.label}. ${s.hint}${weather?.stale ? '. Источник недоступен — данные из кэша' : ''}`"
    >
      <component :is="s.kind.icon" class="size-3.5" :class="s.kind.tone" />
      <span class="font-medium text-foreground tabular-nums">{{ s.range }}</span>
      · {{ s.wet }}
      <span v-if="s.source !== 'прогноз'" class="opacity-70">· {{ s.source }}</span>
    </span>
  </Transition>
</template>

<style scoped>
.weather-fade-enter-active {
  transition: opacity 0.25s ease;
}
.weather-fade-enter-from {
  opacity: 0;
}
@media (prefers-reduced-motion: reduce) {
  .weather-fade-enter-active {
    transition: none;
  }
}
</style>
