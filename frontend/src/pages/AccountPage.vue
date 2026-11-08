<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ChevronRightIcon, LogOutIcon, StarIcon, UserRoundIcon } from '@lucide/vue'
import type { AuthProvider } from '@/api/client'
import { plural } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { useFavoritesStore } from '@/stores/favorites'
import { useProfileStore } from '@/stores/profile'
import PlaceSearch from '@/components/PlaceSearch.vue'

const auth = useAuthStore()
const favorites = useFavoritesStore()
const profile = useProfileStore()
const { homePlace } = storeToRefs(profile)
const router = useRouter()

const providerName: Record<AuthProvider, string> = { yandex: 'Яндекс ID', vk: 'VK ID' }

const leaving = ref(false)
async function logout() {
  leaving.value = true
  try {
    await auth.logout()
    router.push('/')
  } finally {
    leaving.value = false
  }
}
</script>

<template>
  <section class="mx-auto flex max-w-xl flex-col gap-6 px-4 py-8 sm:py-12">
    <header class="flex flex-col gap-2">
      <span class="eyebrow">Личный кабинет</span>
      <h1 class="display">{{ auth.user ? auth.user.name : 'Вы не вошли' }}</h1>
    </header>

    <template v-if="auth.user">
      <div class="flex items-center gap-4 rounded-xl border bg-card p-4">
        <img
          v-if="auth.user.avatar_url"
          :src="auth.user.avatar_url"
          alt=""
          class="size-14 rounded-full object-cover ring-1 ring-border"
          referrerpolicy="no-referrer"
        />
        <span v-else class="flex size-14 items-center justify-center rounded-full bg-accent text-accent-foreground">
          <UserRoundIcon class="size-6" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <div class="flex min-w-0 flex-col gap-0.5">
          <span class="truncate font-semibold">{{ auth.user.name }}</span>
          <span v-if="auth.user.email" class="truncate text-sm text-muted-foreground">{{ auth.user.email }}</span>
          <span class="text-sm text-muted-foreground">
            Вход через {{ auth.user.providers.map((p) => providerName[p]).join(' и ') }}
          </span>
        </div>
      </div>

      <section class="flex flex-col gap-3 rounded-xl border bg-card p-4">
        <div>
          <h2 class="font-semibold">Ваш город</h2>
          <p class="mt-1 text-sm text-muted-foreground">Он будет подставляться в поиск и подборку «Куда угодно».</p>
        </div>
        <PlaceSearch v-model="homePlace" kind="city" placeholder="Выберите город" />
        <p v-if="auth.homePlaceSaving" class="text-sm text-muted-foreground" aria-live="polite">Сохраняем город…</p>
        <div v-else-if="auth.homePlaceSaveError" class="flex items-center justify-between gap-3 text-sm text-destructive">
          <p role="alert">Не удалось сохранить город в аккаунте.</p>
          <button v-if="homePlace" type="button" class="shrink-0 underline" @click="auth.saveHomePlace(homePlace)">Повторить</button>
        </div>
      </section>

      <RouterLink to="/favorites" class="flex h-14 items-center gap-3 rounded-xl border bg-card px-4 hover:bg-muted">
        <StarIcon class="size-5 text-amber-500" :stroke-width="1.75" aria-hidden="true" />
        <span class="flex-1 font-medium">Избранное</span>
        <span v-if="favorites.items.length" class="text-sm tabular-nums text-muted-foreground">
          {{ favorites.items.length }} {{ plural(favorites.items.length, 'маршрут', 'маршрута', 'маршрутов') }}
        </span>
        <ChevronRightIcon class="size-4 text-muted-foreground" aria-hidden="true" />
      </RouterLink>

      <button
        type="button"
        class="flex h-12 items-center justify-center gap-2 rounded-xl border bg-card font-medium text-destructive hover:bg-destructive/10 disabled:opacity-50"
        :disabled="leaving"
        @click="logout"
      >
        <LogOutIcon class="size-4" aria-hidden="true" /> Выйти
      </button>
      <p class="text-sm text-muted-foreground">
        После выхода маршруты останутся в аккаунте и вернутся при следующем входе.
      </p>
    </template>

    <template v-else-if="auth.loaded">
      <p class="text-muted-foreground">
        Войдите через Яндекс или VK, чтобы избранное и путевые листы были с вами на любом устройстве.
        Маршруты, которые вы уже сохранили, никуда не денутся.
      </p>
      <button
        type="button"
        class="flex h-12 items-center justify-center rounded-xl bg-primary font-medium text-primary-foreground hover:bg-primary/90"
        @click="auth.loginOpen = true"
      >
        Войти
      </button>
    </template>
  </section>
</template>

<style scoped>
/* Как заголовок «Избранного», но мельче: здесь имя, а не раздел */
.display {
  font-size: clamp(32px, 5vw, 44px);
  font-weight: 650;
  line-height: 1.05;
  letter-spacing: -0.035em;
  font-feature-settings: 'cv11', 'ss01';
}
.eyebrow {
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted-foreground);
}
</style>
