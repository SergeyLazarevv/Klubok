<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { CircleAlertIcon, XIcon } from '@lucide/vue'
import type { AuthProvider } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useFavoritesStore } from '@/stores/favorites'

const auth = useAuthStore()
const favorites = useFavoritesStore()
const route = useRoute()
const router = useRouter()

// После входа — на ту же страницу, без отметки о неудачном входе
const returnTo = computed(() => {
  const { login: _, ...query } = route.query
  return router.resolve({ path: route.path, query, hash: route.hash }).fullPath
})

const buttons: Record<AuthProvider, { label: string; class: string; mark: string }> = {
  yandex: { label: 'Войти с Яндекс ID', class: 'bg-[#000] text-white hover:bg-[#1f1f1f] dark:bg-white dark:text-black dark:hover:bg-neutral-200', mark: 'Я' },
  vk: { label: 'Войти через VK ID', class: 'bg-[#0077ff] text-white hover:bg-[#0069e0]', mark: 'VK' },
}

function onOpenChange(open: boolean) {
  auth.loginOpen = open
  if (!open) auth.loginFailed = false
}
</script>

<template>
  <DialogRoot :open="auth.loginOpen" @update:open="onOpenChange">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-foreground/30 data-[state=open]:animate-in data-[state=open]:fade-in-0" />
      <!-- На телефоне — лист снизу, у большого пальца; шире — диалог по центру -->
      <DialogContent
        class="fixed inset-x-0 bottom-0 z-50 flex flex-col gap-5 rounded-t-2xl border bg-card px-5 pt-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] shadow-xl outline-none data-[state=open]:animate-in data-[state=open]:slide-in-from-bottom sm:inset-x-auto sm:top-1/2 sm:bottom-auto sm:left-1/2 sm:w-[24rem] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl sm:pb-5 sm:data-[state=open]:slide-in-from-bottom-0 sm:data-[state=open]:zoom-in-95"
      >
        <div class="flex items-start gap-3">
          <div class="flex min-w-0 flex-1 flex-col gap-1">
            <DialogTitle class="text-lg font-semibold">Вход в Клубок</DialogTitle>
            <DialogDescription class="text-sm text-muted-foreground">
              Избранное и путевые листы будут с вами на любом устройстве.
              <template v-if="favorites.items.length">Уже сохранённые маршруты останутся.</template>
            </DialogDescription>
          </div>
          <DialogClose class="-mt-1 -mr-2 flex size-11 shrink-0 items-center justify-center rounded-full hover:bg-accent" aria-label="Закрыть">
            <XIcon class="size-5" />
          </DialogClose>
        </div>

        <p v-if="auth.loginFailed" role="alert" class="flex items-start gap-2 rounded-xl bg-destructive/10 px-3 py-2.5 text-sm text-destructive">
          <CircleAlertIcon class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          Не получилось войти. Попробуйте ещё раз или другим способом.
        </p>

        <div v-if="auth.providers.length" class="flex flex-col gap-2.5">
          <a
            v-for="p in auth.providers"
            :key="p"
            :href="auth.loginUrl(p, returnTo)"
            class="flex h-12 items-center justify-center gap-2.5 rounded-xl text-base font-medium transition-colors"
            :class="buttons[p].class"
          >
            <span class="flex h-6 min-w-6 items-center justify-center rounded-md bg-white/15 px-1 text-sm font-semibold" aria-hidden="true">
              {{ buttons[p].mark }}
            </span>
            {{ buttons[p].label }}
          </a>
        </div>
        <p v-else class="rounded-xl bg-muted px-3 py-2.5 text-sm text-muted-foreground">
          Вход пока не настроен на сервере.
        </p>

        <p class="text-xs text-muted-foreground">
          Пароль не нужен: подтверждаете вход у Яндекса или VK, мы получаем только имя, фото и почту.
        </p>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
