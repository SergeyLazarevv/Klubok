<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { SearchIcon } from '@lucide/vue'
import { monthLabel, monthShort } from '@/lib/format'
import { upcomingMonths, useSearchStore } from '@/stores/search'
import PlaceSearch from '@/components/PlaceSearch.vue'
import StepperField from '@/components/StepperField.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

defineProps<{ loading: boolean; disabled: boolean }>()
const emit = defineEmits<{ search: [] }>()

const { destination, month, minDays, maxDays } = storeToRefs(useSearchStore())
const months = upcomingMonths()
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>Куда и когда?</CardTitle>
    </CardHeader>
    <CardContent class="flex flex-col gap-5">
      <PlaceSearch v-model="destination" placeholder="Например, Алматы" />

      <div class="grid gap-2">
        <Label class="text-muted-foreground">Месяц вылета</Label>
        <ToggleGroup v-model="month" type="single" variant="outline" class="grid w-full grid-cols-3 sm:grid-cols-6">
          <ToggleGroupItem v-for="m in months" :key="m" :value="m" :title="monthLabel(m)">
            {{ monthShort(m) }}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <StepperField v-model="minDays" label="Дней в поездке: от" :min="1" :max="maxDays" />
        <StepperField v-model="maxDays" label="до" :min="minDays" :max="60" />
      </div>

      <Button size="lg" class="w-full" :disabled="disabled || loading" @click="emit('search')">
        <SearchIcon /> {{ loading ? 'Ищем…' : 'Найти самые дешёвые варианты' }}
      </Button>
    </CardContent>
  </Card>
</template>
