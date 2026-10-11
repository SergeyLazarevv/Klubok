<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { CarIcon } from '@lucide/vue'
import type { FuelType } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import { hours, rub } from '@/lib/format'
import StepperField from '@/components/StepperField.vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

defineProps<{ referenceFuelPrice?: number }>()

const { hasCar, radiusKm, maxDriveHours, consumptionL100, fuelType, fuelPriceRub, allowBorder } =
  storeToRefs(useProfileStore())

const fuels: { value: FuelType; label: string }[] = [
  { value: 'ai92', label: 'АИ-92' },
  { value: 'ai95', label: 'АИ-95' },
  { value: 'dt', label: 'ДТ' },
  { value: 'lpg', label: 'Газ' },
]

// Slider в reka-ui работает с массивом значений
const sliderOf = (r: typeof radiusKm) =>
  computed({ get: () => [r.value], set: (v: number[] | undefined) => (r.value = v?.[0] ?? r.value) })
const radius = sliderOf(radiusKm)
const driveHours = sliderOf(maxDriveHours)

const fuelPriceInput = computed({
  get: () => fuelPriceRub.value ?? '',
  set: (v: string | number) => {
    const n = Number(String(v).replace(',', '.'))
    fuelPriceRub.value = v === '' || !Number.isFinite(n) || n <= 0 ? null : n
  },
})
</script>

<template>
  <Card>
    <CardHeader class="flex flex-row items-start justify-between gap-4">
      <div class="flex flex-col gap-1.5">
        <CardTitle class="flex items-center gap-2"><CarIcon class="size-5" /> Своя машина</CardTitle>
        <CardDescription>Посчитаем бензин, парковку у аэропорта и страховку за границей.</CardDescription>
      </div>
      <Switch v-model="hasCar" aria-label="Есть машина" />
    </CardHeader>

    <CardContent v-if="hasCar" class="flex flex-col gap-6">
      <div class="flex flex-col gap-3">
        <div class="flex items-center justify-between text-sm">
          <Label>Радиус поиска</Label>
          <span class="font-medium tabular-nums">до {{ radiusKm }} км</span>
        </div>
        <Slider v-model="radius" :min="25" :max="1000" :step="25" />
      </div>

      <div class="flex flex-col gap-3">
        <div class="flex items-center justify-between text-sm">
          <Label>За рулём в одну сторону</Label>
          <span class="font-medium tabular-nums">не больше {{ hours(maxDriveHours) }}</span>
        </div>
        <Slider v-model="driveHours" :min="1" :max="16" :step="0.5" />
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <StepperField v-model="consumptionL100" label="Расход, л/100 км" :min="2" :max="30" :step="0.5" />
        <div class="grid gap-1.5">
          <Label for="fuel-price" class="text-muted-foreground">Цена топлива, ₽/л</Label>
          <Input
            id="fuel-price"
            v-model="fuelPriceInput"
            inputmode="decimal"
            :placeholder="referenceFuelPrice ? `${rub(referenceFuelPrice)} — средняя` : 'как в справочнике'"
          />
        </div>
      </div>

      <div class="grid gap-2">
        <Label class="text-muted-foreground">Топливо</Label>
        <ToggleGroup v-model="fuelType" type="single" variant="outline" class="w-full">
          <ToggleGroupItem v-for="f in fuels" :key="f.value" :value="f.value" class="flex-1">
            {{ f.label }}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <label class="flex items-center justify-between gap-4">
        <span class="flex flex-col gap-0.5">
          <span class="text-sm font-medium">Готов ехать за границу на машине</span>
          <span class="text-xs text-muted-foreground">Например, в Уральск или Минск. Нужна страховка, бывают очереди на КПП.</span>
        </span>
        <Switch v-model="allowBorder" />
      </label>
    </CardContent>
  </Card>
</template>
