<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { type DateValue, isSameDay, parseDate } from '@internationalized/date'
import {
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
  RangeCalendarCell,
  RangeCalendarCellTrigger,
  RangeCalendarGrid,
  RangeCalendarGridBody,
  RangeCalendarGridHead,
  RangeCalendarGridRow,
  RangeCalendarHeadCell,
  RangeCalendarHeader,
  RangeCalendarHeading,
  RangeCalendarNext,
  RangeCalendarPrev,
  RangeCalendarRoot,
  type DateRange,
} from 'reka-ui'
import { ChevronLeftIcon, ChevronRightIcon, XIcon } from '@lucide/vue'
import { useMediaQuery } from '@vueuse/core'
import { dateRangeLabel } from '@/lib/format'
import { maxRangeDays, maxSearchDate, todayISO } from '@/stores/search'

/** Период поиска одним полем: календарь выезжает снизу (на телефоне) или открывается окном, период — двумя касаниями. */
const props = defineProps<{ inputId?: string }>()
const from = defineModel<string>('from', { required: true })
const to = defineModel<string>('to', { required: true })

const open = ref(false)
// Даты — классы @internationalized/date: в обычном ref Vue развернул бы их в простые объекты
const draft = shallowRef<DateRange>({ start: undefined, end: undefined })
const picked = shallowRef<DateValue>() // начало, которое календарь уже отметил; конец ещё может быть не выбран
const wide = useMediaQuery('(min-width: 40rem)')

watch(open, (o) => {
  if (!o) return
  draft.value = { start: parseDate(from.value), end: parseDate(to.value) }
  picked.value = draft.value.start
})

const minValue = parseDate(todayISO())
const maxStart = parseDate(maxSearchDate())
const maxValue = maxStart.add({ days: maxRangeDays })

// Начало — не дальше 90 дней (лимит backend); конец, пока его выбирают, — до 60 дней после начала
const choosingEnd = computed(
  () => !!picked.value && !(draft.value.start && draft.value.end && isSameDay(draft.value.start, picked.value)),
)
const isDateDisabled = computed(() => (d: DateValue) => !choosingEnd.value && d.compare(maxStart) > 0)
const complete = computed(() => !!draft.value.start && !!draft.value.end)

function apply() {
  if (!draft.value.start || !draft.value.end) return
  from.value = draft.value.start.toString()
  to.value = draft.value.end.toString()
  open.value = false
}
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogTrigger
      :id="props.inputId"
      class="h-7 w-full min-w-0 truncate text-left text-base font-semibold outline-none"
    >
      {{ dateRangeLabel(from, to) }}
    </DialogTrigger>
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-foreground/30 data-[state=open]:animate-in data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed inset-x-0 bottom-0 z-50 flex max-h-[92dvh] flex-col gap-4 overflow-y-auto rounded-t-3xl bg-card px-4 pt-3 pb-[calc(1rem+env(safe-area-inset-bottom))] shadow-xl outline-none
               data-[state=open]:animate-in data-[state=open]:slide-in-from-bottom
               sm:inset-x-auto sm:top-1/2 sm:bottom-auto sm:left-1/2 sm:w-[44rem] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-3xl sm:p-6
               sm:data-[state=open]:slide-in-from-bottom-0 sm:data-[state=open]:zoom-in-95"
      >
        <div class="mx-auto h-1 w-10 shrink-0 rounded-full bg-border sm:hidden" />
        <header class="flex items-start gap-3">
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <DialogTitle class="text-lg font-semibold tracking-tight">Когда ищем поездку</DialogTitle>
            <DialogDescription class="text-sm text-muted-foreground">
              Выберите первый и последний день — вылет и возвращение будут внутри этого периода
            </DialogDescription>
          </div>
          <DialogClose class="-mt-1 -mr-2 flex size-11 shrink-0 items-center justify-center rounded-full hover:bg-muted" aria-label="Закрыть">
            <XIcon class="size-5" />
          </DialogClose>
        </header>

        <RangeCalendarRoot
          v-slot="{ weekDays, grid }"
          v-model="draft"
          v-model:start-value="picked"
          locale="ru"
          :week-starts-on="1"
          :number-of-months="wide ? 2 : 1"
          :min-value="minValue"
          :max-value="maxValue"
          :maximum-days="maxRangeDays"
          :is-date-disabled="isDateDisabled"
          fixed-weeks
          prevent-deselect
          class="flex flex-col gap-2"
        >
          <RangeCalendarHeader class="flex items-center justify-between">
            <RangeCalendarPrev class="flex size-11 items-center justify-center rounded-full hover:bg-muted disabled:opacity-30" aria-label="Предыдущий месяц">
              <ChevronLeftIcon class="size-5" />
            </RangeCalendarPrev>
            <RangeCalendarHeading class="text-sm font-semibold first-letter:uppercase" />
            <RangeCalendarNext class="flex size-11 items-center justify-center rounded-full hover:bg-muted disabled:opacity-30" aria-label="Следующий месяц">
              <ChevronRightIcon class="size-5" />
            </RangeCalendarNext>
          </RangeCalendarHeader>
          <div class="flex flex-col gap-6 sm:flex-row">
            <RangeCalendarGrid v-for="month in grid" :key="month.value.toString()" class="w-full border-collapse">
              <RangeCalendarGridHead>
                <RangeCalendarGridRow>
                  <RangeCalendarHeadCell v-for="day in weekDays" :key="day" class="pb-1 text-xs font-normal text-muted-foreground">
                    {{ day }}
                  </RangeCalendarHeadCell>
                </RangeCalendarGridRow>
              </RangeCalendarGridHead>
              <RangeCalendarGridBody>
                <RangeCalendarGridRow v-for="(week, i) in month.rows" :key="i">
                  <RangeCalendarCell v-for="d in week" :key="d.toString()" :date="d" class="p-0 py-0.5">
                    <RangeCalendarCellTrigger :day="d" :month="month.value" class="day" />
                  </RangeCalendarCell>
                </RangeCalendarGridRow>
              </RangeCalendarGridBody>
            </RangeCalendarGrid>
          </div>
        </RangeCalendarRoot>

        <footer class="flex items-center gap-3 border-t pt-3">
          <span class="min-w-0 flex-1 text-sm">
            <template v-if="complete">{{ dateRangeLabel(draft.start!.toString(), draft.end!.toString()) }}</template>
            <span v-else class="text-muted-foreground">Теперь выберите последний день</span>
          </span>
          <button
            type="button"
            class="h-11 shrink-0 rounded-full bg-cta px-6 font-semibold text-cta-foreground transition-colors hover:bg-cta-hover disabled:opacity-50"
            :disabled="!complete"
            @click="apply"
          >
            Готово
          </button>
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
@reference '../style.css';

/* Период — сплошной полосой: края скруглены, середина подсвечена */
.day {
  @apply flex h-11 w-full items-center justify-center text-sm tabular-nums outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring;
  border-radius: 9999px;
  -webkit-tap-highlight-color: transparent;
}
.day[data-today] {
  @apply font-semibold text-primary;
}
.day[data-outside-view] {
  @apply invisible;
}
.day[data-disabled] {
  @apply pointer-events-none text-muted-foreground/35;
}
.day[data-highlighted],
.day[data-selected] {
  @apply bg-accent text-accent-foreground;
  border-radius: 0;
}
.day[data-selection-start],
.day[data-highlighted-start] {
  border-top-left-radius: 9999px;
  border-bottom-left-radius: 9999px;
}
.day[data-selection-end],
.day[data-highlighted-end] {
  border-top-right-radius: 9999px;
  border-bottom-right-radius: 9999px;
}
.day[data-selection-start],
.day[data-selection-end] {
  @apply bg-primary font-semibold text-primary-foreground;
}
</style>
