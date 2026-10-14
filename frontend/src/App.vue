<script setup lang="ts">
import { onMounted } from 'vue'
import { StarIcon } from '@lucide/vue'
import { useFavoritesStore } from '@/stores/favorites'
import logoUrl from '@/assets/logo.svg?inline'

const favorites = useFavoritesStore()
onMounted(favorites.load)
</script>

<template>
  <div class="min-h-dvh bg-background">
    <header class="border-b">
      <div class="mx-auto flex h-14 max-w-6xl items-center px-4">
        <RouterLink to="/" class="flex items-center gap-2 text-lg font-bold tracking-tight">
          <img :src="logoUrl" alt="" class="size-9" width="36" height="36" />
          <span class="uppercase tracking-wide text-[#1f3a5f] dark:text-[#9bb7d4]">Клубок</span>
        </RouterLink>
        <RouterLink
          to="/favorites"
          class="ml-auto flex h-10 items-center gap-1.5 rounded-full px-3 text-sm font-medium hover:bg-muted"
          active-class="bg-muted"
        >
          <StarIcon class="size-4" :class="favorites.items.length > 0 && 'fill-amber-400 text-amber-500'" />
          Избранное
          <span v-if="favorites.items.length" class="rounded-full bg-amber-100 px-1.5 text-xs tabular-nums text-amber-900">
            {{ favorites.items.length }}
          </span>
        </RouterLink>
      </div>
    </header>
    <main>
      <RouterView />
    </main>
  </div>
</template>
