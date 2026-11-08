<script setup lang="ts">
import { ref, watch } from 'vue'
import { ChevronRightIcon, FlameIcon, PlaneTakeoffIcon, TreePalmIcon } from '@lucide/vue'
import { api, type HotToursResponse } from '@/api/client'
import { km, localDay, plural, rub } from '@/lib/format'

/**
 * Горящие туры из ближайшего к дому города вылета (ADR-0027). Пока без цен: карточка ведёт на поиск
 * туров Level.Travel по направлению. Нет туров (город далеко от городов вылета) — блок не показываем.
 */
const props = defineProps<{ from: string; travelers: number }>()

const data = ref<HotToursResponse>()
const loading = ref(false)

watch(
  () => [props.from, props.travelers] as const,
  async ([from, travelers]) => {
    loading.value = true
    const { data: res } = await api
      .GET('/tours/hot', { params: { query: { from, travelers } } })
      .catch(() => ({ data: undefined }))
    // Пока грузилось, город могли сменить — старый ответ не показываем
    if (from !== props.from) return
    data.value = res
    loading.value = false
  },
  { immediate: true },
)
</script>

<template>
  <section v-if="loading || data?.items.length" class="flex flex-col gap-3" aria-labelledby="hot-tours-title">
    <header class="flex items-end justify-between gap-3">
      <div class="flex flex-col gap-0.5">
        <h2 id="hot-tours-title" class="flex items-center gap-2 text-lg font-semibold sm:text-xl">
          <FlameIcon class="size-5 text-orange-500" />
          Горящие туры<template v-if="data"> {{ data.departure.name_from }}</template>
        </h2>
        <p v-if="data" class="text-sm text-muted-foreground">
          Вылет через несколько дней
          <template v-if="data.departure.distance_km >= 50"> · до аэропорта ≈ {{ km(data.departure.distance_km) }}</template>
        </p>
      </div>
      <a
        v-if="data"
        :href="data.all_url"
        target="_blank"
        rel="noopener sponsored"
        class="flex shrink-0 items-center gap-0.5 text-sm font-medium text-primary hover:underline"
      >
        Все<span class="max-sm:hidden">&nbsp;горящие туры</span>
        <ChevronRightIcon class="size-4" />
      </a>
    </header>

    <div v-if="!data" class="flex gap-3 overflow-hidden">
      <div v-for="n in 4" :key="n" class="h-32 w-44 shrink-0 animate-pulse rounded-2xl bg-muted" />
    </div>
    <div v-else class="-mx-1 flex snap-x gap-3 overflow-x-auto px-1 pt-1 pb-2">
      <a
        v-for="t in data.items"
        :key="t.country"
        :href="t.url"
        target="_blank"
        rel="noopener sponsored"
        class="flex w-44 shrink-0 snap-start flex-col gap-2 rounded-2xl bg-card p-3 ring-1 ring-border transition-shadow hover:shadow-md sm:w-52"
      >
        <span class="flex size-9 items-center justify-center rounded-full bg-orange-500/10 text-orange-500">
          <TreePalmIcon class="size-5" />
        </span>
        <span class="font-semibold leading-snug">{{ t.name }}</span>
        <span class="flex items-center gap-1 text-xs text-muted-foreground">
          <PlaneTakeoffIcon class="size-3.5" />
          {{ localDay(t.departure_date) }} · {{ t.nights }} {{ plural(t.nights, 'ночь', 'ночи', 'ночей') }}
        </span>
        <span v-if="t.price" class="mt-auto text-sm font-semibold">от {{ rub(t.price) }}</span>
        <span v-else class="mt-auto text-sm font-medium text-primary">Смотреть туры</span>
      </a>
    </div>
    <p v-if="data" class="text-xs text-muted-foreground">Туры ищет Level.Travel — цены и наличие на сайте.</p>
  </section>
</template>
