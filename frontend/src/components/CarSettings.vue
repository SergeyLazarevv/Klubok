<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { ChevronDownIcon } from '@lucide/vue'
import type { FuelType } from '@/api/client'
import { useProfileStore } from '@/stores/profile'
import { hours, rub } from '@/lib/format'
import CompactStepper from '@/components/CompactStepper.vue'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

// narrow — узкая панель (карта «Куда угодно»): настройки в одну колонку
defineProps<{ referenceFuelPrice?: number; narrow?: boolean }>()

const { hasCar, radiusKm, maxDriveHours, consumptionL100, fuelType, fuelPriceRub, allowBorder } =
  storeToRefs(useProfileStore())
const open = ref(false)

const fuels: { value: FuelType; label: string }[] = [
  { value: 'ai92', label: 'АИ-92' },
  { value: 'ai95', label: 'АИ-95' },
  { value: 'dt', label: 'ДТ' },
  { value: 'lpg', label: 'Газ' },
]
const fuelLabel = computed(() => fuels.find((f) => f.value === fuelType.value)?.label)

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
  <div class="flex flex-col gap-2">
    <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
      <label class="flex cursor-pointer items-center gap-2 font-medium">
        <Switch v-model="hasCar" />
        Своя машина — ищем и в соседних аэропортах
      </label>
      <button
        v-if="hasCar"
        type="button"
        class="flex items-center gap-1 rounded-md px-1 text-left text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
        :aria-expanded="open"
        @click="open = !open"
      >
        до {{ hours(maxDriveHours) }} за рулём · аэропорты до {{ radiusKm }} км · {{ consumptionL100 }} л/100 км · {{ fuelLabel }}
        <ChevronDownIcon class="size-4 transition-transform" :class="open && 'rotate-180'" />
      </button>
    </div>

    <div v-if="hasCar && open" class="grid gap-x-8 gap-y-5 rounded-xl bg-muted p-4" :class="!narrow && 'sm:grid-cols-2 lg:grid-cols-3'">
      <div class="flex flex-col gap-3">
        <div class="flex items-center justify-between text-sm">
          <Label>За рулём в одну сторону</Label>
          <span class="font-semibold tabular-nums">до {{ hours(maxDriveHours) }}</span>
        </div>
        <Slider v-model="driveHours" :min="1" :max="16" :step="0.5" />
        <span class="text-xs text-muted-foreground">Дальше не предлагаем ни поездку на машине, ни дорогу до аэропорта.</span>
      </div>

      <div class="flex flex-col gap-3">
        <div class="flex items-center justify-between text-sm">
          <Label>Соседние аэропорты</Label>
          <span class="font-semibold tabular-nums">до {{ radiusKm }} км</span>
        </div>
        <Slider v-model="radius" :min="25" :max="1000" :step="25" />
        <span class="text-xs text-muted-foreground">Где искать вылет, если из своего города дорого. По прямой от дома.</span>
      </div>

      <div class="flex items-center justify-between gap-3 text-sm">
        <Label>Расход, л/100 км</Label>
        <CompactStepper v-model="consumptionL100" label="Расход, л/100 км" :min="2" :max="30" :step="0.5" />
      </div>

      <div class="grid gap-1.5">
        <Label class="text-muted-foreground">Топливо</Label>
        <ToggleGroup v-model="fuelType" type="single" variant="outline" class="w-full">
          <ToggleGroupItem v-for="f in fuels" :key="f.value" :value="f.value" class="flex-1">
            {{ f.label }}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div class="grid gap-1.5">
        <Label for="fuel-price" class="text-muted-foreground">Цена топлива, ₽/л</Label>
        <Input
          id="fuel-price"
          v-model="fuelPriceInput"
          inputmode="decimal"
          :placeholder="referenceFuelPrice ? `${rub(referenceFuelPrice)} — средняя` : 'как в справочнике'"
        />
      </div>

      <label class="flex items-center justify-between gap-4">
        <span class="flex flex-col gap-0.5">
          <span class="text-sm font-medium">Готов ехать за границу</span>
          <span class="text-xs text-muted-foreground">Например, в Уральск или Минск: страховка, очереди на КПП.</span>
        </span>
        <Switch v-model="allowBorder" />
      </label>
    </div>
  </div>
</template>
