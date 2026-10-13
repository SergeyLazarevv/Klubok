<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ icon: Component; title: string; subtitle?: string; last?: boolean }>()
</script>

<template>
  <!-- Этап поездки: слева значок на общей линии, название и даты, справа карточки этапа -->
  <li class="relative grid grid-cols-[2.5rem_minmax(0,1fr)] gap-x-3 pb-6 sm:grid-cols-[2.5rem_9rem_minmax(0,1fr)] sm:gap-x-4">
    <span v-if="!last" class="absolute top-10 bottom-0 left-5 w-px -translate-x-1/2 bg-border" aria-hidden="true" />
    <span class="relative flex size-10 items-center justify-center rounded-full bg-foreground text-background">
      <component :is="icon" class="size-[1.15rem]" />
    </span>
    <div class="flex min-w-0 flex-col justify-center sm:justify-start sm:pt-1.5">
      <span class="text-sm font-semibold">{{ title }}</span>
      <span v-if="subtitle" class="text-xs text-muted-foreground">{{ subtitle }}</span>
    </div>
    <div class="col-start-2 mt-2.5 flex min-w-0 flex-col gap-3 sm:col-start-3 sm:mt-0">
      <slot />
    </div>
  </li>
</template>
