<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { CalendarDaysIcon, CircleDotIcon, LoaderCircleIcon, MapPinIcon, MoonStarIcon, SearchIcon, UsersRoundIcon } from '@lucide/vue'
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
  <form class="flex flex-col gap-2.5 lg:flex-row" @submit.prevent="emit('search')">
    <div
      class="grid flex-1 grid-cols-[minmax(0,1fr)_auto] gap-px overflow-hidden rounded-[20px] bg-border ring-1 ring-border lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(11.5rem,auto)_auto_auto]"
    >
      <div class="search-cell col-span-2 lg:col-span-1">
        <CircleDotIcon class="search-icon" aria-hidden="true" />
        <div class="search-body">
          <label for="search-from" class="search-label">Откуда</label>
          <PlaceSearch v-model="homePlace" input-id="search-from" kind="city" placeholder="Ваш город" bare />
        </div>
      </div>
      <div class="search-cell col-span-2 lg:col-span-1">
        <MapPinIcon class="search-icon text-cta!" aria-hidden="true" />
        <div class="search-body">
          <label for="search-to" class="search-label">Куда</label>
          <PlaceSearch v-model="destination" input-id="search-to" placeholder="Город" bare />
        </div>
      </div>
      <div class="search-cell">
        <CalendarDaysIcon class="search-icon max-lg:hidden" aria-hidden="true" />
        <div class="search-body">
          <label for="search-date-from" class="search-label">Когда</label>
          <DateRangeField v-model:from="dateFrom" v-model:to="dateTo" input-id="search-date-from" />
        </div>
      </div>
      <!-- На телефоне дни — во всю ширину последней строкой, рядом с «Когда» — путешественники -->
      <div class="search-cell col-span-2 max-lg:order-last lg:col-span-1">
        <MoonStarIcon class="search-icon" aria-hidden="true" />
        <div class="search-body">
          <span class="search-label">Дней в поездке</span>
          <div class="flex items-center">
            <CompactStepper v-model="minDays" label="Дней, от" :min="1" :max="maxDays" />
            <span class="px-1 text-sm text-muted-foreground">до</span>
            <CompactStepper v-model="maxDays" label="Дней, до" :min="minDays" :max="60" />
          </div>
        </div>
      </div>
      <div class="search-cell">
        <UsersRoundIcon class="search-icon max-lg:hidden" aria-hidden="true" />
        <div class="search-body">
          <span class="search-label">Человек</span>
          <CompactStepper v-model="travelers" label="Путешественников" :min="1" :max="9" />
        </div>
      </div>
    </div>
    <button
      type="submit"
      class="flex h-14 shrink-0 items-center justify-center gap-2 rounded-[20px] bg-cta px-8 text-lg font-extrabold text-cta-foreground shadow-[0_10px_24px_-10px_rgb(217_72_43/0.8)] transition-[background-color,transform,box-shadow] duration-200 hover:bg-cta-hover active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-55 disabled:shadow-none lg:h-auto"
      :disabled="disabled || loading"
    >
      <LoaderCircleIcon v-if="loading" class="size-5 animate-spin" aria-hidden="true" />
      <SearchIcon v-else class="size-5" :stroke-width="2.5" aria-hidden="true" />
      {{ loading ? 'Ищем…' : 'Найти' }}
    </button>
  </form>
</template>

<style scoped>
@reference '../style.css';

.search-cell {
  @apply flex min-h-16 min-w-0 items-center gap-3 bg-card px-4 py-2 text-card-foreground transition-colors focus-within:bg-[#fffaf0];
}
.search-icon {
  @apply size-5 shrink-0 text-ink/45;
}
.search-body {
  @apply flex min-w-0 flex-1 flex-col justify-center gap-0.5;
}
.search-label {
  @apply text-xs font-semibold text-muted-foreground;
}
</style>
