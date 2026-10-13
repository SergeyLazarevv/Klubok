<script setup lang="ts">
import { MinusIcon, PlusIcon } from '@lucide/vue'

const props = withDefaults(defineProps<{ min: number; max: number; step?: number; label: string }>(), { step: 1 })
const model = defineModel<number>({ required: true })

const change = (d: number) => (model.value = Math.min(props.max, Math.max(props.min, model.value + d)))
</script>

<template>
  <div class="flex items-center gap-1" role="group" :aria-label="label">
    <button
      type="button"
      class="flex size-7 items-center justify-center rounded-full text-primary hover:bg-accent disabled:text-muted-foreground disabled:opacity-40"
      :disabled="model <= min"
      :aria-label="`${label}: меньше`"
      @click="change(-step)"
    >
      <MinusIcon class="size-4" />
    </button>
    <span class="min-w-6 text-center text-base font-semibold tabular-nums" aria-live="polite">{{ model }}</span>
    <button
      type="button"
      class="flex size-7 items-center justify-center rounded-full text-primary hover:bg-accent disabled:text-muted-foreground disabled:opacity-40"
      :disabled="model >= max"
      :aria-label="`${label}: больше`"
      @click="change(step)"
    >
      <PlusIcon class="size-4" />
    </button>
  </div>
</template>
