<script setup lang="ts">
import { ref } from 'vue'
import { ArrowRightIcon, LoaderCircleIcon } from '@lucide/vue'
import { commonsPhoto, POSTCARDS, stickerUrl, type Postcard } from '@/lib/home'

/** Подборка направлений открытками: нажали — город встаёт в «Куда», дальше решает страница. */
defineProps<{ picking?: string }>()
const emit = defineEmits<{ pick: [card: Postcard] }>()

// Фото с Wikimedia может не загрузиться — тогда открытка остаётся с гербом на тёплом фоне
const broken = ref(new Set<string>())
</script>

<template>
  <section class="flex flex-col gap-5" aria-labelledby="postcards-title">
    <header class="flex flex-col gap-1.5 px-4 sm:px-0">
      <h2 id="postcards-title" class="font-display text-2xl font-semibold tracking-tight text-ink sm:text-3xl">Куда съездить</h2>
      <p class="text-muted-foreground sm:text-lg">Выберите открытку — покажем, как добраться из вашего города и почём.</p>
    </header>

    <ul class="postcards -my-4 flex snap-x snap-mandatory gap-4 overflow-x-auto px-4 py-4 sm:-mx-4 sm:gap-5 sm:mask-r-from-90%">
      <li v-for="(c, i) in POSTCARDS" :key="c.id" class="shrink-0 snap-start scroll-ml-4">
        <button
          type="button"
          class="postcard group flex w-56 flex-col rounded-[22px] bg-card p-2 pb-3.5 text-left shadow-[0_1px_2px_rgb(31_58_95/0.06),0_8px_24px_-10px_rgb(31_58_95/0.25)] ring-1 ring-border/80 sm:w-60"
          :style="{ '--tilt': `${i % 2 ? 1.2 : -1.2}deg` }"
          :disabled="!!picking"
          :aria-label="`Варианты поездки: ${c.name}`"
          @click="emit('pick', c)"
        >
          <span class="relative block">
            <span class="block aspect-[4/3] overflow-hidden rounded-2xl bg-accent">
              <img
                v-if="!broken.has(c.id)"
                :src="commonsPhoto(c.photo)"
                alt=""
                loading="lazy"
                decoding="async"
                class="size-full object-cover transition-transform duration-500 ease-out group-hover:scale-105"
                @error="broken = new Set(broken).add(c.id)"
              />
            </span>
            <img
              :src="stickerUrl(c.id)"
              alt=""
              loading="lazy"
              class="absolute -right-1 -bottom-5 size-16 rotate-6 transition-transform duration-300 group-hover:rotate-0 group-hover:scale-110"
            />
            <span
              v-if="c.abroad"
              class="absolute top-2 left-2 rounded-full bg-card/90 px-2 py-0.5 text-xs font-bold text-ink backdrop-blur"
            >
              {{ c.abroad }}
            </span>
          </span>
          <span class="mt-3 flex flex-col gap-0.5 px-1.5 pr-16">
            <span class="font-display text-[17px] leading-tight font-semibold text-ink">{{ c.name }}</span>
            <span class="truncate text-sm text-muted-foreground">{{ c.landmark }}</span>
          </span>
          <span class="mt-2.5 flex items-center gap-1 px-1.5 text-sm font-bold text-cta">
            <template v-if="picking === c.id">
              <LoaderCircleIcon class="size-4 animate-spin" aria-hidden="true" />
              Ищем…
            </template>
            <template v-else>
              Смотреть варианты
              <ArrowRightIcon class="size-4 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
            </template>
          </span>
        </button>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.postcards {
  scrollbar-width: none;
}
.postcards::-webkit-scrollbar {
  display: none;
}

/* Открытки лежат чуть вразнобой, как на столе; под курсором выпрямляются и приподнимаются */
.postcard {
  transform: rotate(var(--tilt));
  transition:
    transform 0.3s cubic-bezier(0.34, 1.4, 0.64, 1),
    box-shadow 0.3s;
}
.postcard:hover,
.postcard:focus-visible {
  transform: rotate(0deg) translateY(-6px);
  box-shadow:
    0 1px 2px rgb(31 58 95 / 0.06),
    0 18px 36px -12px rgb(31 58 95 / 0.35);
}
.postcard:active {
  transform: rotate(0deg) translateY(-2px) scale(0.98);
}
.postcard:focus-visible {
  outline: 3px solid var(--ring);
  outline-offset: 3px;
}
.postcard:disabled {
  cursor: progress;
}
@media (prefers-reduced-motion: reduce) {
  .postcard,
  .postcard:hover {
    transform: none;
    transition: none;
  }
}
</style>
