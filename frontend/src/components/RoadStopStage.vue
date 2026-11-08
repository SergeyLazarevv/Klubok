<script setup lang="ts">
import { ExternalLinkIcon, MapPinnedIcon, XIcon } from '@lucide/vue'
import type { DriveStop } from '@/api/client'
import { plural, rub } from '@/lib/format'
import { armsUrl, photoUrl } from '@/lib/map/cities'
import PlanStage from '@/components/PlanStage.vue'
import SheetStage from '@/components/SheetStage.vue'

/**
 * Город-остановка по дороге туда или обратно (ADR-0023): заезд или ночёвка, ссылки на жильё на дату, когда
 * до него доезжаем, и свои пункты путевого листа у этого города.
 */
const props = defineProps<{
  stop: DriveStop
  editable: boolean
  busy?: string // id города, у которого идёт пересчёт
  error?: string // текст ошибки пересчёта у этого города
  stayUrls: { sutochno: string; avito: string }
  ownStay: boolean // своё жильё выбрано и с ценой — оно в итоге вместо оценки
  prompt: boolean // вернулись с сайта жилья — сразу предлагаем вставить ссылку
  last?: boolean
}>()
const emit = defineEmits<{ nights: [number]; remove: []; leave: []; prompted: [] }>()

const nightOptions = [
  { n: 0, label: 'Заезд' },
  { n: 1, label: '1 ночь' },
  { n: 2, label: '2 ночи' },
]
const subtitle = () =>
  (props.stop.nights ? `${props.stop.nights} ${plural(props.stop.nights, 'ночь', 'ночи', 'ночей')}` : 'заезд на несколько часов') +
  (props.stop.back ? ' · на обратном пути' : '')
</script>

<template>
  <PlanStage :icon="MapPinnedIcon" :title="stop.city.name" :subtitle="subtitle()" :last="last">
    <article class="plan-card">
      <div class="flex items-start gap-3">
        <span class="relative size-16 shrink-0 overflow-hidden rounded-xl bg-muted">
          <img v-if="stop.city.photo" :src="photoUrl(stop.city.photo)" alt="" loading="lazy" class="size-full object-cover" />
          <img
            v-if="stop.city.arms"
            :src="armsUrl(stop.city.id)"
            alt=""
            class="absolute object-contain drop-shadow"
            :class="stop.city.photo ? 'right-1 bottom-1 size-6' : 'inset-2 size-12'"
          />
        </span>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <span class="font-semibold">{{ stop.nights ? 'Ночёвка в пути' : 'Заезд по пути' }}{{ stop.back ? ' домой' : '' }}</span>
          <span v-if="stop.city.landmark" class="line-clamp-2 text-sm text-muted-foreground">{{ stop.city.landmark }}</span>
        </div>
      </div>
      <div v-if="editable" class="flex items-center gap-2">
        <div class="flex flex-1 rounded-full bg-muted p-1" role="radiogroup" :aria-label="`Сколько быть в г. ${stop.city.name}`">
          <button
            v-for="o in nightOptions"
            :key="o.n"
            type="button"
            role="radio"
            class="h-9 flex-1 rounded-full text-sm font-medium transition-colors disabled:opacity-60"
            :class="stop.nights === o.n ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
            :aria-checked="stop.nights === o.n"
            :disabled="!!busy"
            @click="stop.nights !== o.n && emit('nights', o.n)"
          >
            {{ o.label }}
          </button>
        </div>
        <button
          type="button"
          class="flex size-11 shrink-0 items-center justify-center rounded-full text-muted-foreground ring-1 ring-input hover:bg-muted disabled:opacity-60"
          :aria-label="`Убрать остановку в г. ${stop.city.name}`"
          title="Убрать остановку"
          :disabled="!!busy"
          @click="emit('remove')"
        >
          <XIcon class="size-4" />
        </button>
      </div>
      <p class="text-xs text-muted-foreground">
        <template v-if="stop.nights && ownStay">В итоге — цена выбранного жилья ниже, а не оценка.</template>
        <template v-else-if="stop.nights && stop.stay_per_night">
          Гостиница 3* ≈ <b class="font-semibold text-foreground tabular-nums">{{ rub(stop.stay!) }}</b> (Росстат), уже в итоге.
        </template>
        <template v-else-if="stop.nights">Цены на гостиницу здесь нет — ночёвка в итог не вошла.</template>
        <template v-else>Без ночёвки: в итоге только бензин на заезд.</template>
      </p>
      <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      <div v-if="stop.nights" class="flex flex-wrap gap-2">
        <a
          :href="stayUrls.sutochno"
          @click="emit('leave')"
          target="_blank"
          rel="noopener"
          class="flex h-10 items-center gap-1.5 rounded-full px-4 text-sm font-medium text-primary ring-1 ring-input hover:bg-accent"
        >
          Жильё на Суточно.ру <ExternalLinkIcon class="size-3.5" />
        </a>
        <a
          :href="stayUrls.avito"
          @click="emit('leave')"
          target="_blank"
          rel="noopener"
          class="flex h-10 items-center gap-1.5 rounded-full px-4 text-sm font-medium text-primary ring-1 ring-input hover:bg-accent"
        >
          Посуточно на Авито <ExternalLinkIcon class="size-3.5" />
        </a>
      </div>
    </article>
    <!-- Жильё и дела в этом городе — рядом с ним, а не у места назначения -->
    <SheetStage
      v-if="stop.nights"
      stage="stay"
      :city="stop.city.id"
      add-label="Выбрали жильё? Добавьте ссылку на объявление"
      placeholder="Ссылка на объявление или отель"
      :nights="stop.nights"
      :prompt="prompt"
      more-label="Ещё вариант жилья"
      @prompted="emit('prompted')"
    />
    <SheetStage
      stage="see"
      :city="stop.city.id"
      :add-label="`Дело или место в г. ${stop.city.name}`"
      placeholder="Например: пообедать в центре, музей, набережная"
      :more-label="`Ещё дело в г. ${stop.city.name}`"
    />
  </PlanStage>
</template>

<style scoped>
@reference '../style.css';

.plan-card {
  @apply flex flex-col gap-2.5 rounded-2xl bg-card p-4 ring-1 ring-border;
}
</style>
