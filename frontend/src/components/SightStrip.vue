<script setup lang="ts">
import { ImageIcon } from '@lucide/vue'
import type { SightItem } from '@/lib/sights'

/** items: undefined — грузятся; failed — не удалось загрузить. */
defineProps<{ items?: SightItem[]; failed?: boolean; empty: string }>()
</script>

<template>
  <div v-if="!items && !failed" class="flex gap-3 overflow-hidden">
    <div v-for="n in 3" :key="n" class="h-56 w-48 shrink-0 animate-pulse rounded-2xl bg-muted" />
  </div>
  <p v-else-if="failed || !items?.length" class="rounded-2xl bg-card p-4 text-sm text-muted-foreground ring-1 ring-border">
    {{ failed ? 'Не удалось загрузить места — попробуйте позже.' : empty }}
  </p>
  <div v-else class="-mx-1 flex snap-x gap-3 overflow-x-auto px-1 pb-2">
    <component
      :is="s.url ? 'a' : 'article'"
      v-for="s in items"
      :key="s.id"
      :href="s.url"
      :target="s.url ? '_blank' : undefined"
      :rel="s.url ? 'noopener' : undefined"
      class="flex w-48 shrink-0 snap-start flex-col overflow-hidden rounded-2xl bg-card ring-1 ring-border sm:w-56"
      :class="s.url && 'transition-shadow hover:shadow-md'"
    >
      <img v-if="s.photoUrl" :src="s.photoUrl" :alt="s.name" loading="lazy" class="h-32 w-full object-cover" />
      <span v-else class="flex h-32 items-center justify-center bg-muted text-muted-foreground"><ImageIcon class="size-6" /></span>
      <div class="flex flex-col gap-1 p-3">
        <span class="line-clamp-2 text-sm leading-snug font-semibold">{{ s.name }}</span>
        <span v-if="s.kind" class="text-xs text-muted-foreground">{{ s.kind }}</span>
        <span v-if="s.description" class="line-clamp-3 text-xs text-muted-foreground">{{ s.description }}</span>
      </div>
    </component>
  </div>
</template>
