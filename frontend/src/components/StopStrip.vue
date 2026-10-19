<script setup lang="ts">
import { CheckIcon, LoaderCircleIcon, MoonIcon, PlusIcon } from '@lucide/vue'
import type { StopSuggestion } from '@/api/client'
import { hours, rub } from '@/lib/format'
import { armsUrl, photoUrl } from '@/lib/map/cities'
import { detourLabel } from '@/lib/stopPicker'

/** Города, где можно остановиться по дороге: лента карточек, «Заехать» добавляет город в маршрут. */
defineProps<{ items: StopSuggestion[]; chosen: string[]; busy?: string }>()
const emit = defineEmits<{ toggle: [StopSuggestion] }>()
</script>

<template>
  <div class="-mx-1 flex snap-x gap-3 overflow-x-auto px-1 pt-1 pb-2">
    <article
      v-for="s in items"
      :key="s.city.id"
      class="flex w-48 shrink-0 snap-start flex-col overflow-hidden rounded-2xl bg-card ring-1 sm:w-56"
      :class="chosen.includes(s.city.id) ? 'ring-2 ring-primary' : 'ring-border'"
    >
      <div class="relative h-28 bg-muted">
        <img v-if="s.city.photo" :src="photoUrl(s.city.photo)" :alt="s.city.name" loading="lazy" class="size-full object-cover" />
        <div class="absolute inset-0 bg-linear-to-t from-black/60 via-transparent to-transparent" />
        <img
          v-if="s.city.arms"
          :src="armsUrl(s.city.id)"
          alt=""
          loading="lazy"
          class="absolute bottom-2 left-2 size-9 object-contain drop-shadow"
        />
        <span
          v-if="s.overnight"
          class="absolute top-2 left-2 flex items-center gap-1 rounded-full bg-card/95 px-2 py-0.5 text-[11px] font-medium shadow-sm"
        >
          <MoonIcon class="size-3" /> удобно заночевать
        </span>
      </div>
      <div class="flex flex-1 flex-col gap-1 p-3">
        <span class="font-semibold">{{ s.city.name }}</span>
        <span v-if="s.city.landmark" class="line-clamp-1 text-xs text-muted-foreground">{{ s.city.landmark }}</span>
        <span class="text-xs">
          {{ detourLabel(s) }}{{ s.detour.duration_min < 10 ? '' : ' к дороге' }} · через {{ hours(s.drive_to_hours) }} от дома
        </span>
        <span v-if="s.stay_per_night" class="text-xs text-muted-foreground">гостиница ≈ {{ rub(s.stay_per_night) }} / чел.</span>
        <button
          type="button"
          class="mt-auto flex h-10 items-center justify-center gap-1.5 rounded-full text-sm font-medium transition-colors disabled:opacity-60"
          :class="chosen.includes(s.city.id) ? 'bg-primary text-primary-foreground' : 'text-primary ring-1 ring-input hover:bg-accent active:bg-accent'"
          :aria-pressed="chosen.includes(s.city.id)"
          :disabled="!!busy"
          @click="emit('toggle', s)"
        >
          <LoaderCircleIcon v-if="busy === s.city.id" class="size-4 animate-spin" />
          <CheckIcon v-else-if="chosen.includes(s.city.id)" class="size-4" />
          <PlusIcon v-else class="size-4" />
          {{ chosen.includes(s.city.id) ? 'В маршруте' : 'Заехать' }}
        </button>
      </div>
    </article>
  </div>
</template>
