<script setup lang="ts">
import { useDark } from '@vueuse/core'

useDark() // .dark на <html> по системной теме
</script>

<template>
  <div class="min-h-dvh bg-background">
    <header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
      <div class="mx-auto flex h-14 max-w-5xl items-center gap-2 px-4">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold">
          <img src="/favicon.svg" alt="" class="size-7" />
          Klubok
        </RouterLink>
      </div>
    </header>
    <main>
      <RouterView />
    </main>
  </div>
</template>
