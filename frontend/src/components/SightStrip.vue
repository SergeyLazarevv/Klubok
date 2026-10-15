<script setup lang="ts">
import { CheckIcon, ExternalLinkIcon, ImageIcon, PlusIcon } from '@lucide/vue'
import type { SightItem } from '@/lib/sights'

/**
 * items: undefined — грузятся; failed — не удалось загрузить.
 * added — когда задан, карточки выбираются в путевой лист: нажатие добавляет место или убирает его;
 * в added — названия уже добавленных. Ссылка на статью тогда — отдельной иконкой.
 */
defineProps<{ items?: SightItem[]; failed?: boolean; empty: string; added?: string[] }>()
const emit = defineEmits<{ toggle: [item: SightItem] }>()
</script>

<template>
  <div v-if="!items && !failed" class="flex gap-3 overflow-hidden">
    <div v-for="n in 3" :key="n" class="h-56 w-48 shrink-0 animate-pulse rounded-2xl bg-muted" />
  </div>
  <p v-else-if="failed || !items?.length" class="rounded-2xl bg-card p-4 text-sm text-muted-foreground ring-1 ring-border">
    {{ failed ? 'Не удалось загрузить места — попробуйте позже.' : empty }}
  </p>
  <div v-else class="-mx-1 flex snap-x gap-3 overflow-x-auto px-1 pt-1 pb-2">
    <component
      :is="added ? 'div' : s.url ? 'a' : 'article'"
      v-for="s in items"
      :key="s.id"
      :role="added ? 'button' : undefined"
      :tabindex="added ? 0 : undefined"
      :href="added ? undefined : s.url"
      :target="!added && s.url ? '_blank' : undefined"
      :rel="!added && s.url ? 'noopener' : undefined"
      :aria-pressed="added ? added.includes(s.name) : undefined"
      class="flex w-48 shrink-0 snap-start flex-col overflow-hidden rounded-2xl bg-card text-left ring-1 transition-shadow sm:w-56"
      :class="[
        added?.includes(s.name) ? 'ring-2 ring-primary' : 'ring-border',
        (added || s.url) && 'hover:shadow-md',
        added && 'cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-primary',
      ]"
      @click="added && emit('toggle', s)"
      @keydown.enter.space.self.prevent="added && emit('toggle', s)"
    >
      <div class="relative w-full">
        <img v-if="s.photoUrl" :src="s.photoUrl" :alt="s.name" loading="lazy" class="h-32 w-full object-cover" />
        <span v-else class="flex h-32 items-center justify-center bg-muted text-muted-foreground"><ImageIcon class="size-6" /></span>
        <span
          v-if="added"
          class="absolute top-2 right-2 flex h-7 items-center gap-1 rounded-full px-2.5 text-xs font-medium shadow-sm"
          :class="added.includes(s.name) ? 'bg-primary text-primary-foreground' : 'bg-card/95 text-foreground'"
        >
          <template v-if="added.includes(s.name)"><CheckIcon class="size-3.5" /> в списке</template>
          <template v-else><PlusIcon class="size-3.5" /> в список</template>
        </span>
      </div>
      <div class="flex flex-col gap-1 p-3">
        <span class="flex items-start justify-between gap-2">
          <span class="line-clamp-2 text-sm leading-snug font-semibold">{{ s.name }}</span>
          <a
            v-if="added && s.url"
            :href="s.url"
            target="_blank"
            rel="noopener"
            class="-m-1 shrink-0 rounded-full p-1 text-muted-foreground hover:bg-muted hover:text-primary"
            :aria-label="`Подробнее о месте «${s.name}»`"
            title="Подробнее"
            @click.stop
          >
            <ExternalLinkIcon class="size-3.5" />
          </a>
        </span>
        <span v-if="s.kind" class="text-xs text-muted-foreground">{{ s.kind }}</span>
        <span v-if="s.description" class="line-clamp-3 text-xs text-muted-foreground">{{ s.description }}</span>
      </div>
    </component>
  </div>
</template>
