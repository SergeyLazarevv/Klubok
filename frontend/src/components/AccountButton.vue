<script setup lang="ts">
import { computed } from 'vue'
import { UserRoundIcon } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const initials = computed(() =>
  (auth.user?.name ?? '')
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join(''),
)
</script>

<template>
  <!-- Пока не знаем, вошёл ли пользователь, держим место, чтобы шапка не прыгала -->
  <div v-if="!auth.loaded" class="size-11" aria-hidden="true" />

  <RouterLink
    v-else-if="auth.user"
    to="/account"
    class="flex size-11 items-center justify-center rounded-full hover:bg-muted"
    active-class="bg-muted"
    :aria-label="`Личный кабинет: ${auth.user.name}`"
  >
    <img
      v-if="auth.user.avatar_url"
      :src="auth.user.avatar_url"
      alt=""
      class="size-8 rounded-full object-cover ring-1 ring-border"
      referrerpolicy="no-referrer"
    />
    <span v-else class="flex size-8 items-center justify-center rounded-full bg-accent text-xs font-semibold text-accent-foreground">
      {{ initials || '?' }}
    </span>
  </RouterLink>

  <button
    v-else
    type="button"
    class="flex h-11 items-center gap-1.5 rounded-full pr-3.5 pl-3 text-sm font-medium hover:bg-muted"
    @click="auth.loginOpen = true"
  >
    <span class="flex size-7 items-center justify-center rounded-full bg-muted">
      <UserRoundIcon class="size-4" :stroke-width="1.75" aria-hidden="true" />
    </span>
    Войти
  </button>
</template>
