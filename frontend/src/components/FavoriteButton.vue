<script setup lang="ts">
import { computed } from 'vue'
import { StarIcon } from '@lucide/vue'
import type { FavoritePick, FavoriteSnapshot, Place, TripSearchParams } from '@/api/client'
import { favoriteTitle } from '@/lib/favorites'
import { useFavoritesStore } from '@/stores/favorites'
import { useSearchStore } from '@/stores/search'

const props = defineProps<{
  pick: FavoritePick
  snapshot: FavoriteSnapshot
  home: Place
  destination: Place
  params?: TripSearchParams // по умолчанию — параметры последнего поиска
}>()

const favorites = useFavoritesStore()
const search = useSearchStore()
const params = computed(() => props.params ?? search.lastQuery)
const saved = computed(() => (params.value ? favorites.find(params.value, props.pick) : undefined))

async function toggle() {
  if (!params.value) return
  if (saved.value) await favorites.remove(saved.value)
  else await favorites.add(favoriteTitle(props.home, props.destination, params.value.date_from, params.value.date_to), params.value, props.pick, props.snapshot)
}
</script>

<template>
  <button
    v-if="params"
    type="button"
    class="flex size-10 shrink-0 items-center justify-center rounded-full ring-1 ring-input transition-colors hover:bg-muted"
    :class="saved && 'bg-amber-50 ring-amber-300 hover:bg-amber-100'"
    :aria-pressed="!!saved"
    :aria-label="saved ? 'Убрать из избранного' : 'Добавить в избранное'"
    :title="saved ? 'В избранном — нажмите, чтобы убрать' : 'Сохранить в избранное'"
    @click="toggle"
  >
    <StarIcon class="size-5" :class="saved ? 'fill-amber-400 text-amber-500' : 'text-muted-foreground'" />
  </button>
</template>
