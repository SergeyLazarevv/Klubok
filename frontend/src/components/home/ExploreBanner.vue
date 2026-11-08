<script setup lang="ts">
import { ArrowRightIcon } from '@lucide/vue'
import { stickerUrl } from '@/lib/home'

/** Приглашение на «Карту» — для тех, кто ещё не выбрал направление. */
const scattered = [
  { id: 'Q656', x: 6, y: 18, r: -10, s: 1 },
  { id: 'Q2214', x: 34, y: 6, r: 8, s: 0.8 },
  { id: 'Q900', x: 58, y: 30, r: -4, s: 1.1 },
  { id: 'Q153676', x: 22, y: 56, r: 12, s: 0.9 },
  { id: 'Q959', x: 76, y: 8, r: 6, s: 0.85 },
  { id: 'Q1953', x: 70, y: 62, r: -12, s: 0.95 },
  { id: 'Q15757', x: 46, y: 70, r: 4, s: 0.75 },
]
</script>

<template>
  <section
    class="relative isolate grid overflow-hidden rounded-[32px] bg-ink text-white md:grid-cols-[1.1fr_1fr]"
    aria-labelledby="explore-title"
  >
    <!-- Параллели и меридианы — намёк на глобус -->
    <svg class="absolute -right-32 -bottom-40 -z-10 size-[36rem] text-white/[0.06]" viewBox="0 0 200 200" fill="none" aria-hidden="true">
      <circle cx="100" cy="100" r="98" stroke="currentColor" stroke-width="1.2" />
      <ellipse cx="100" cy="100" rx="60" ry="98" stroke="currentColor" stroke-width="1.2" />
      <ellipse cx="100" cy="100" rx="22" ry="98" stroke="currentColor" stroke-width="1.2" />
      <path d="M2 100h196M14 52h172M14 148h172" stroke="currentColor" stroke-width="1.2" />
    </svg>

    <div class="flex flex-col items-start gap-4 p-7 sm:p-10 lg:p-14">
      <span class="rounded-full bg-white/10 px-3 py-1 text-sm font-bold text-yarn">Карта</span>
      <h2 id="explore-title" class="font-display text-3xl leading-[1.1] font-semibold tracking-tight sm:text-4xl">
        Не знаете, куда&nbsp;поехать?
      </h2>
      <p class="max-w-md text-lg text-white/75">
        Откройте карту — Клубок покажет цены на поездки из вашего города во все стороны. Выбирайте, куда дешевле.
      </p>
      <RouterLink
        to="/explore"
        class="group mt-2 flex h-13 items-center gap-2 rounded-[18px] bg-yarn px-6 text-lg font-extrabold text-ink transition-[background-color,transform] hover:bg-[#ebc275] active:scale-[0.98]"
      >
        Открыть карту
        <ArrowRightIcon class="size-5 transition-transform group-hover:translate-x-1" :stroke-width="2.5" aria-hidden="true" />
      </RouterLink>
    </div>

    <div class="relative h-56 md:h-auto md:min-h-80" aria-hidden="true">
      <svg class="absolute inset-0 size-full" viewBox="0 0 400 300" preserveAspectRatio="none" fill="none">
        <path
          d="M-20 260 C60 200 40 120 120 110 S220 190 260 120 S330 30 420 60"
          stroke="var(--yarn)"
          stroke-width="3"
          stroke-linecap="round"
          stroke-dasharray="2 9"
          vector-effect="non-scaling-stroke"
        />
      </svg>
      <img
        v-for="c in scattered"
        :key="c.id"
        :src="stickerUrl(c.id)"
        alt=""
        loading="lazy"
        class="sticker absolute w-[clamp(3.5rem,17%,5.5rem)]"
        :style="{ left: `${c.x}%`, top: `${c.y}%`, '--r': `${c.r}deg`, scale: c.s }"
      />
    </div>
  </section>
</template>

<style scoped>
.sticker {
  rotate: var(--r);
  filter: drop-shadow(0 8px 14px rgb(0 0 0 / 0.3));
  transition: rotate 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
}
section:hover .sticker {
  rotate: calc(var(--r) * -1);
}
@media (prefers-reduced-motion: reduce) {
  .sticker {
    transition: none;
  }
}
</style>
