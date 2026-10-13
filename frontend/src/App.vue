<template>
  <div class="min-h-dvh bg-background">
    <header class="border-b">
      <div class="mx-auto flex h-14 max-w-6xl items-center px-4">
        <RouterLink to="/" class="flex items-center gap-2 text-lg font-bold tracking-tight">
          <!-- Логотип-клубок -->
          <svg viewBox="0 0 64 64" class="size-8 text-primary" aria-hidden="true">
            <circle cx="32" cy="32" r="18" fill="none" stroke="currentColor" stroke-width="4" />
            <path
              d="M17 26c8 4 22 4 30 0M15 34c10 5 24 5 34 0M20 43c7 3 17 3 24 0M46 44c4 4 8 6 12 6"
              fill="none"
              stroke="currentColor"
              stroke-width="3"
              stroke-linecap="round"
            />
          </svg>
          Klubok
        </RouterLink>
      </div>
    </header>
    <main>
      <RouterView />
    </main>
  </div>
</template>
