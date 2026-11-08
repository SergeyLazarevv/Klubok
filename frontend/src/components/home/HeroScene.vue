<script setup lang="ts">
import logoUrl from '@/assets/logo.svg?inline'
import { stickerUrl } from '@/lib/home'

/**
 * Украшение героя на десктопе: из клубка разматывается нить-маршрут и проходит через города-наклейки,
 * как поездка с остановками. Нить прорисовывается при загрузке, наклейки появляются по очереди.
 * Чисто декоративная — скрыта от скринридеров.
 */
const stops = [
  { id: 'Q1829', name: 'Калининград', x: 13, y: 47, r: -8 },
  { id: 'Q900', name: 'Казань', x: 42, y: 21, r: 6 },
  { id: 'Q39420', name: 'Сочи', x: 64, y: 53, r: -5 },
]
// Нить: от клубка (справа внизу) через Сочи, петлю, Казань и Калининград — и уходит влево за край
const THREAD =
  'M392 262 C350 272 316 236 307 172 C298 108 360 86 352 52 C344 18 280 26 202 68 C140 102 120 160 62 150 C24 143 8 178 -20 200'
</script>

<template>
  <div class="hero-scene pointer-events-none relative aspect-[3/2] w-full select-none" aria-hidden="true">
    <svg viewBox="0 0 480 320" class="absolute inset-0 h-full w-full overflow-visible" fill="none">
      <defs>
        <mask id="hero-thread-draw" maskUnits="userSpaceOnUse" x="-40" y="-20" width="560" height="360">
          <path class="thread-draw" :d="THREAD" pathLength="1" stroke="#fff" stroke-width="12" stroke-linecap="round" />
        </mask>
      </defs>
      <!-- Пунктир «пути» под нитью, как маршрут на карте -->
      <path :d="THREAD" stroke="var(--ink)" stroke-opacity=".12" stroke-width="10" stroke-linecap="round" transform="translate(3 6)" />
      <g mask="url(#hero-thread-draw)" stroke-linecap="round">
        <path :d="THREAD" stroke="var(--yarn)" stroke-width="4.5" />
        <!-- Скрутка нити: тёмные короткие штрихи поверх -->
        <path :d="THREAD" stroke="var(--yarn-deep)" stroke-width="4.5" stroke-dasharray="2 7" />
      </g>
    </svg>

    <img :src="logoUrl" alt="" class="yarn absolute right-[2%] bottom-[2%] w-[27%]" />

    <div
      v-for="(s, i) in stops"
      :key="s.id"
      class="stop absolute flex w-[19%] -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-1"
      :style="{ left: `${s.x}%`, top: `${s.y}%`, '--r': `${s.r}deg`, '--d': `${0.7 + i * 0.35}s` }"
    >
      <img :src="stickerUrl(s.id)" alt="" class="w-full drop-shadow-[0_6px_10px_rgb(31_58_95/0.18)]" />
      <span class="rounded-full bg-card px-2.5 py-0.5 text-xs font-extrabold whitespace-nowrap text-ink shadow-sm ring-1 ring-border">
        {{ s.name }}
      </span>
    </div>
  </div>
</template>

<style scoped>
.thread-draw {
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: thread-draw 2.2s cubic-bezier(0.65, 0, 0.35, 1) 0.15s forwards;
}
@keyframes thread-draw {
  to {
    stroke-dashoffset: 0;
  }
}

.stop {
  rotate: var(--r);
  opacity: 0;
  animation: stop-pop 0.5s cubic-bezier(0.34, 1.56, 0.64, 1) var(--d) forwards;
}
@keyframes stop-pop {
  from {
    opacity: 0;
    scale: 0.4;
  }
  to {
    opacity: 1;
    scale: 1;
  }
}

.yarn {
  animation: yarn-in 0.9s cubic-bezier(0.34, 1.3, 0.64, 1) both;
}
@keyframes yarn-in {
  from {
    opacity: 0;
    transform: translateX(-40px) rotate(-120deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .thread-draw,
  .stop,
  .yarn {
    animation: none;
    opacity: 1;
    stroke-dashoffset: 0;
  }
}
</style>
