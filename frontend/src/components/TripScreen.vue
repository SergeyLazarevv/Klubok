<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useScrollLock } from '@vueuse/core'
import { ArrowLeftIcon } from '@lucide/vue'

/** Подробности варианта на телефоне: экран поверх списка с шапкой «назад» и закреплённой панелью цены и действий. */
defineProps<{ title: string; subtitle?: string }>()
const emit = defineEmits<{ close: [] }>()

// Список под экраном не прокручивается и остаётся на том же месте
const locked = useScrollLock(document.body)
onMounted(() => (locked.value = true))
onUnmounted(() => (locked.value = false))
</script>

<template>
  <Teleport to="body">
    <div class="trip-screen fixed inset-0 z-50 flex flex-col bg-background" role="dialog" aria-modal="true" :aria-label="title">
      <header class="flex items-center gap-1 border-b bg-background px-1 pt-[env(safe-area-inset-top)]">
        <button
          type="button"
          class="flex size-11 shrink-0 items-center justify-center rounded-full hover:bg-muted active:bg-muted"
          aria-label="Назад к вариантам"
          @click="emit('close')"
        >
          <ArrowLeftIcon class="size-5" />
        </button>
        <div class="flex min-w-0 flex-col py-2 pr-3">
          <span class="truncate font-semibold">{{ title }}</span>
          <span v-if="subtitle" class="truncate text-xs text-muted-foreground">{{ subtitle }}</span>
        </div>
      </header>
      <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
        <slot />
      </div>
      <footer
        v-if="$slots.footer"
        class="flex items-center gap-2 border-t bg-background px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] shadow-[0_-4px_16px_rgb(0_0_0/0.06)]"
      >
        <slot name="footer" />
      </footer>
    </div>
  </Teleport>
</template>

<style scoped>
.trip-screen {
  animation: trip-screen-in 220ms cubic-bezier(0.2, 0.8, 0.2, 1);
}
@keyframes trip-screen-in {
  from {
    transform: translateX(24px);
    opacity: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .trip-screen {
    animation: none;
  }
}
</style>
