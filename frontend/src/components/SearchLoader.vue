<script setup lang="ts">
defineProps<{ text?: string }>()
</script>

<template>
  <Teleport to="body">
    <div
      class="fixed inset-0 z-[100] flex flex-col items-center justify-center gap-6 bg-background/60 backdrop-blur-md"
      role="status"
      aria-live="polite"
    >
      <div class="relative h-28 w-56">
        <div class="yarn-shadow absolute bottom-1 left-1/2 h-3 w-20 rounded-full bg-[#163056]/25 blur-sm" />
        <div class="yarn-roll absolute bottom-3 left-1/2 -ml-10 h-20 w-20">
          <svg viewBox="0 0 80 80" class="h-full w-full">
            <defs>
              <radialGradient id="yarn-g" cx=".35" cy=".3" r=".8">
                <stop offset="0" stop-color="#4f7cac" />
                <stop offset="1" stop-color="#163056" />
              </radialGradient>
              <clipPath id="yarn-c"><circle cx="40" cy="40" r="37" /></clipPath>
            </defs>
            <circle cx="40" cy="40" r="37" fill="url(#yarn-g)" />
            <g clip-path="url(#yarn-c)" fill="none" stroke-width="3" stroke-linecap="round">
              <path d="M0 24Q40 44 80 18" stroke="#dfe9f4" />
              <path d="M0 34Q40 56 80 28" stroke="#e2b45a" />
              <path d="M0 46Q40 68 80 40" stroke="#8fb0d2" />
              <path d="M0 58Q40 80 80 52" stroke="#f3d891" />
              <path d="M22 0Q44 40 18 80" stroke="#5d86b3" />
              <path d="M40 0Q64 40 38 80" stroke="#dfe9f4" />
              <path d="M58 0Q78 40 58 80" stroke="#e2b45a" />
            </g>
            <circle cx="40" cy="40" r="37" fill="none" stroke="#1f3a5f" stroke-width="2.5" />
          </svg>
        </div>
      </div>
      <p class="text-sm font-medium text-foreground/80">{{ text ?? 'Ищем лучшие маршруты…' }}</p>
    </div>
  </Teleport>
</template>

<style scoped>
/* Диаметр 80px, длина окружности ≈ 251px: пробег 100px ≈ 143° — клубок катится без проскальзывания */
.yarn-roll {
  animation: yarn-roll 1.6s ease-in-out infinite alternate;
}
.yarn-shadow {
  animation: yarn-shadow 1.6s ease-in-out infinite alternate;
  margin-left: -2.5rem;
}
@keyframes yarn-roll {
  from { transform: translateX(-50px) rotate(-143deg); }
  to { transform: translateX(50px) rotate(143deg); }
}
@keyframes yarn-shadow {
  from { transform: translateX(-50px); }
  to { transform: translateX(50px); }
}
@media (prefers-reduced-motion: reduce) {
  .yarn-roll { animation-duration: 4s; }
  .yarn-shadow { animation-duration: 4s; }
}
</style>
