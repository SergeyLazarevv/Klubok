<script setup lang="ts">
import { Label } from '@/components/ui/label'
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '@/components/ui/number-field'

defineProps<{ label: string; min: number; max: number; step?: number; suffix?: string }>()
const model = defineModel<number>({ required: true })
</script>

<template>
  <NumberField v-model="model" :min="min" :max="max" :step="step ?? 1" locale="ru-RU">
    <Label class="text-muted-foreground">{{ label }}</Label>
    <NumberFieldContent>
      <NumberFieldDecrement />
      <NumberFieldInput />
      <NumberFieldIncrement />
    </NumberFieldContent>
    <span v-if="suffix" class="sr-only">{{ suffix }}</span>
  </NumberField>
</template>
