<script setup lang="ts">
import { BedDoubleIcon, FuelIcon, PlaneTakeoffIcon } from '@lucide/vue'

/** Чем Клубок отличается от поиска билетов — три шага, связанные нитью. */
const steps = [
  {
    icon: PlaneTakeoffIcon,
    title: 'Билеты не только из вашего города',
    text: 'Покажем вылеты и поезда из соседних городов — иногда доехать до другого аэропорта выходит дешевле.',
    tone: 'bg-sky text-primary',
  },
  {
    icon: FuelIcon,
    title: 'Дорога до вокзала тоже стоит денег',
    text: 'Рядом с билетами прикинем бензин по расходу вашей машины и парковку у аэропорта — на всю компанию.',
    tone: 'bg-accent text-accent-foreground',
  },
  {
    icon: BedDoubleIcon,
    title: 'Ночёвки и остановки по пути',
    text: 'Едете на машине — покажем города, где переночевать, и что посмотреть по дороге.',
    tone: 'bg-[#fde3da] text-cta',
  },
]
</script>

<template>
  <section class="flex flex-col gap-8" aria-labelledby="how-title">
    <header class="flex max-w-2xl flex-col gap-1.5">
      <h2 id="how-title" class="font-display text-2xl font-semibold tracking-tight text-ink sm:text-3xl">
        Вся дорога, а не только билет
      </h2>
      <p class="text-muted-foreground sm:text-lg">
        Самый дешёвый билет — ещё не самая дешёвая поездка. Клубок разматывает весь путь от двери до двери — и показывает, из чего складывается цена.
      </p>
    </header>

    <ol class="relative grid gap-6 md:grid-cols-3 md:gap-8">
      <!-- Нить между шагами: на телефоне вертикальная, шире — горизонтальная -->
      <svg
        class="absolute top-7 left-7 h-[calc(100%-7rem)] w-1 md:top-7 md:left-7 md:h-1 md:w-[calc(66.6%-1rem)]"
        preserveAspectRatio="none"
        aria-hidden="true"
      >
        <line x1="0" y1="2" x2="100%" y2="2" class="max-md:hidden" stroke="var(--yarn)" stroke-width="3" stroke-dasharray="2 8" stroke-linecap="round" />
        <line x1="2" y1="0" x2="2" y2="100%" class="md:hidden" stroke="var(--yarn)" stroke-width="3" stroke-dasharray="2 8" stroke-linecap="round" />
      </svg>
      <li v-for="(s, i) in steps" :key="s.title" class="relative flex gap-4 md:flex-col">
        <span
          class="relative flex size-14 shrink-0 items-center justify-center rounded-full ring-[6px] ring-background"
          :class="s.tone"
        >
          <component :is="s.icon" class="size-6" :stroke-width="2.25" aria-hidden="true" />
          <span
            class="absolute -top-1 -right-1 flex size-6 items-center justify-center rounded-full bg-ink font-display text-xs font-semibold text-white"
          >
            {{ i + 1 }}
          </span>
        </span>
        <span class="flex flex-col gap-1.5 pt-1 md:pt-0">
          <span class="text-lg leading-snug font-extrabold text-ink">{{ s.title }}</span>
          <span class="text-muted-foreground">{{ s.text }}</span>
        </span>
      </li>
    </ol>
  </section>
</template>
