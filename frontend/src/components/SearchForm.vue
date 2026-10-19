<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { plural } from '@/lib/format'
import { useProfileStore } from '@/stores/profile'
import { useSearchStore } from '@/stores/search'
import CompactStepper from '@/components/CompactStepper.vue'
import DateRangeField from '@/components/DateRangeField.vue'
import PlaceSearch from '@/components/PlaceSearch.vue'

defineProps<{ loading: boolean; disabled: boolean }>()
const emit = defineEmits<{ search: [] }>()

const { homePlace, travelers } = storeToRefs(useProfileStore())
const { destination, dateFrom, dateTo, minDays, maxDays } = storeToRefs(useSearchStore())
</script>

<template>
  <!-- Поля склеены в одну строку, на телефоне — в столбик; тонкие линии между ячейками — это фон сетки -->
  <form class="flex flex-col gap-2 lg:flex-row" @submit.prevent="emit('search')">
    <div
      class="grid flex-1 grid-cols-2 gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1.3fr)_minmax(0,1.1fr)_auto_auto]"
    >
      <div class="search-cell col-span-2 lg:col-span-1">
        <label for="search-from" class="search-label">Откуда</label>
        <PlaceSearch v-model="homePlace" input-id="search-from" kind="city" placeholder="Ваш город" bare />
      </div>
      <div class="search-cell col-span-2 lg:col-span-1">
        <label for="search-to" class="search-label">Куда</label>
        <PlaceSearch v-model="destination" input-id="search-to" placeholder="Город или аэропорт" bare />
      </div>
      <div class="search-cell">
        <label for="search-date-from" class="search-label">Когда</label>
        <DateRangeField v-model:from="dateFrom" v-model:to="dateTo" input-id="search-date-from" />
      </div>
      <!-- На телефоне дни — во всю ширину последней строкой, рядом с «Когда» — путешественники -->
      <div class="search-cell col-span-2 max-lg:order-last lg:col-span-1">
        <span class="search-label">Дней в поездке</span>
        <div class="flex items-center">
          <CompactStepper v-model="minDays" label="Дней, от" :min="1" :max="maxDays" />
          <span class="px-1 text-sm text-muted-foreground">до</span>
          <CompactStepper v-model="maxDays" label="Дней, до" :min="minDays" :max="60" />
        </div>
      </div>
      <div class="search-cell">
        <span class="search-label">{{ plural(travelers, 'Путешественник', 'Путешественника', 'Путешественников') }}</span>
        <CompactStepper v-model="travelers" label="Путешественников" :min="1" :max="9" />
      </div>
    </div>
    <button
      type="submit"
      class="h-14 shrink-0 rounded-full bg-cta px-9 text-lg font-semibold text-cta-foreground transition-colors hover:bg-cta-hover disabled:cursor-not-allowed disabled:opacity-60 lg:h-auto"
      :disabled="disabled || loading"
    >
      {{ loading ? 'Ищем…' : 'Найти' }}
    </button>
  </form>
</template>

<style scoped>
@reference '../style.css';

.search-cell {
  @apply flex min-h-15 min-w-0 flex-col justify-center gap-0.5 bg-card px-3.5 py-2 text-card-foreground;
}
.search-label {
  @apply text-xs text-muted-foreground;
}
</style>
