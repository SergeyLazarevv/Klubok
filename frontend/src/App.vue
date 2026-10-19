<script setup lang="ts">
import { onMounted } from 'vue'
import { CompassIcon, MapIcon, SearchIcon, StarIcon } from '@lucide/vue'
import { useFavoritesStore } from '@/stores/favorites'
import logoUrl from '@/assets/logo.svg?inline'

const favorites = useFavoritesStore()
onMounted(favorites.load)

// /map2 — рабочая карта для экспериментов, в меню её нет
const tabs = [
  { to: '/', label: 'Поиск', icon: SearchIcon },
  { to: '/explore', label: 'Куда угодно', icon: CompassIcon },
  { to: '/map', label: 'Карта', icon: MapIcon },
  { to: '/favorites', label: 'Избранное', icon: StarIcon },
]
</script>

<template>
  <div class="min-h-dvh bg-background">
    <header class="border-b pt-[env(safe-area-inset-top)]">
      <div class="mx-auto flex h-14 max-w-6xl items-center px-4">
        <RouterLink to="/" class="flex items-center gap-2 text-lg font-bold tracking-tight">
          <img :src="logoUrl" alt="" class="size-9" width="36" height="36" />
          <span class="uppercase tracking-wide text-[#1f3a5f] dark:text-[#9bb7d4]">Клубок</span>
        </RouterLink>
        <!-- На планшете и шире — меню в шапке, на телефоне — панель вкладок внизу -->
        <nav class="ml-auto flex items-center max-sm:hidden" aria-label="Разделы">
          <RouterLink
            v-for="t in tabs.slice(1)"
            :key="t.to"
            :to="t.to"
            class="flex h-10 items-center gap-1.5 rounded-full px-3 text-sm font-medium hover:bg-muted"
            active-class="bg-muted"
          >
            <component
              :is="t.icon"
              class="size-4"
              :class="t.to === '/favorites' && favorites.items.length > 0 && 'fill-amber-400 text-amber-500'"
            />
            {{ t.label }}
            <span
              v-if="t.to === '/favorites' && favorites.items.length"
              class="rounded-full bg-amber-100 px-1.5 text-xs tabular-nums text-amber-900"
            >
              {{ favorites.items.length }}
            </span>
          </RouterLink>
        </nav>
      </div>
    </header>
    <main class="pb-(--tabbar-h)">
      <RouterView />
    </main>

    <nav
      class="fixed inset-x-0 bottom-0 z-40 grid h-(--tabbar-h) grid-cols-4 border-t bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur sm:hidden"
      aria-label="Разделы"
    >
      <RouterLink
        v-for="t in tabs"
        :key="t.to"
        v-slot="{ href, navigate, isExactActive, isActive }"
        :to="t.to"
        custom
      >
        <a
          :href="href"
          class="tab"
          :class="(t.to === '/' ? isExactActive : isActive) && 'is-active'"
          :aria-current="(t.to === '/' ? isExactActive : isActive) ? 'page' : undefined"
          @click="navigate"
        >
          <span class="tab__icon">
            <component :is="t.icon" class="size-5" />
            <span v-if="t.to === '/favorites' && favorites.items.length" class="tab__badge">{{ favorites.items.length }}</span>
          </span>
          {{ t.label }}
        </a>
      </RouterLink>
    </nav>
  </div>
</template>

<style scoped>
@reference './style.css';

.tab {
  @apply flex flex-col items-center justify-center gap-1 text-[11px] font-medium text-muted-foreground transition-colors;
  -webkit-tap-highlight-color: transparent;
}
.tab__icon {
  @apply relative flex h-8 w-14 items-center justify-center rounded-full transition-colors;
}
.tab.is-active {
  @apply text-foreground;
}
.tab.is-active .tab__icon {
  @apply bg-accent text-accent-foreground;
}
.tab:active .tab__icon {
  @apply bg-muted;
}
.tab__badge {
  @apply absolute top-0 right-2 min-w-4 rounded-full bg-amber-400 px-1 text-center text-[10px] leading-4 font-semibold tabular-nums text-amber-950;
}
</style>
