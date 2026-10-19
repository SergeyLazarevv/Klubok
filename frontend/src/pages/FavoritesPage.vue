<script setup lang="ts">
import { computed, onUnmounted, reactive, watch } from 'vue'
import {
  ArrowRightIcon, BedDoubleIcon, CalendarIcon, CarIcon, ReceiptIcon, EqualIcon, PlaneIcon, RotateCcwIcon, SearchIcon, StarIcon, TrashIcon, TrendingDownIcon, TrendingUpIcon, UsersIcon,
} from '@lucide/vue'
import { api, type Favorite, type FavoriteCurrent } from '@/api/client'
import { hours, localDay, plural, rub } from '@/lib/format'
import { useFavoritesStore } from '@/stores/favorites'
import { useSheetStore } from '@/stores/sheet'
import { countedInTotal, sheetExtras } from '@/lib/sheet'
import DriveOptionCard from '@/components/DriveOptionCard.vue'
import TripOptionCard from '@/components/TripOptionCard.vue'

const favorites = useFavoritesStore()

// Актуальные цены подгружаются фоном по каждому маршруту; список сразу показывает цены на момент сохранения
const current = reactive(new Map<string, FavoriteCurrent | 'loading'>())

async function refresh(fav: Favorite) {
  current.set(fav.id, 'loading')
  const { data } = await api.GET('/favorites/{id}/current', { params: { path: { id: fav.id } } })
  current.set(fav.id, data ?? { status: 'unavailable' })
}

// Пересчёт по два маршрута одновременно — не нагружаем источник цен
async function refreshAll(list: Favorite[]) {
  const queue = list.filter((f) => !current.has(f.id))
  const worker = async () => {
    for (let f = queue.shift(); f; f = queue.shift()) await refresh(f)
  }
  await Promise.all([worker(), worker()])
}
watch(() => favorites.items, refreshAll, { immediate: true })

// Путевые листы всех поездок — для итога в превью. Тот же стор, что у плана поездки: отметил «Выбрано»
// в раскрытой карточке — итог в превью пересчитался сразу
const sheets = useSheetStore()
watch(() => favorites.items, (list) => list.forEach((f) => void sheets.load(f.id)), { immediate: true })
const extras = (f: Favorite) => sheetExtras(sheets.byFavorite[f.id] ?? [])
const unchosen = (f: Favorite) => (sheets.byFavorite[f.id] ?? []).filter((i) => i.price && !countedInTotal(i)).length
/** Цена самой поездки: актуальная, а пока не обновилась или источник не ответил — на момент сохранения. */
const tripTotal = (f: Favorite) => (cur(f)?.total ?? f.snapshot.total).amount

async function undo() {
  const fav = await favorites.undoRemove()
  if (fav) await refresh(fav)
}
onUnmounted(() => (favorites.lastRemoved = null)) // «Отменить» — только пока страница открыта

const cur = (f: Favorite) => {
  const c = current.get(f.id)
  return c === 'loading' ? undefined : c
}
// Цена ещё обновляется: запрос идёт или маршрут ждёт своей очереди
const pending = (f: Favorite) => !cur(f)
const searchLink = (f: Favorite) => ({
  path: '/',
  query: {
    from: f.params.from,
    to: f.params.to,
    date_from: f.params.date_from,
    date_to: f.params.date_to,
    min: f.params.min_days,
    max: f.params.max_days,
    weekends: f.params.weekends ? '1' : undefined,
  },
})
const statusText: Record<string, string> = {
  replaced: 'Сохранённого варианта больше нет — вот лучший с теми же параметрами',
  past: 'Даты поездки уже прошли',
  empty: 'Сейчас вариантов нет — источник не нашёл цен на эти даты',
  unavailable: 'Не удалось обновить цену — источник недоступен, попробуйте позже',
}

/** «Самара → Алматы, 15 нояб. – 22 нояб.» → маршрут и даты отдельно (заголовок собирает favoriteTitle). */
function titleParts(f: Favorite) {
  const i = f.title.indexOf(', ')
  const route = i > 0 ? f.title.slice(0, i) : f.title
  const [from, to] = route.split(' → ')
  return { from, to, dates: i > 0 ? f.title.slice(i + 2) : '' }
}

const count = computed(() => favorites.items.length)
</script>

<template>
  <section class="mx-auto flex max-w-6xl flex-col gap-8 px-4 py-8 sm:py-12">
    <header class="flex flex-col gap-2">
      <span class="eyebrow">Сохранённые поездки</span>
      <h1 class="display flex items-center gap-3">
        Избранное
        <span v-if="favorites.loaded && count" class="count">{{ count }}</span>
      </h1>
      <p class="max-w-xl text-muted-foreground">
        Маршруты с актуальной ценой: пересчитываем при каждом открытии и показываем, подешевело или подорожало.
      </p>
    </header>

    <Transition name="toast">
      <!-- Внизу экрана, над вкладками: видно, где бы в списке ни удалили -->
      <div
        v-if="favorites.lastRemoved"
        role="status"
        class="fixed inset-x-4 bottom-[calc(var(--tabbar-h)+1rem)] z-40 flex items-center justify-between gap-3 rounded-2xl bg-foreground py-1.5 pr-1.5 pl-4 text-sm text-background shadow-lg sm:right-6 sm:left-auto sm:w-[26rem]"
      >
        <span class="min-w-0">«{{ favorites.lastRemoved.title }}» убран из избранного</span>
        <button type="button" class="flex h-11 shrink-0 items-center gap-1.5 rounded-xl px-3 font-medium text-[#a8c7fa] hover:text-white active:bg-white/10" @click="undo">
          <RotateCcwIcon class="size-4" /> Отменить
        </button>
      </div>
    </Transition>

    <!-- Первая загрузка списка: скелетоны строк -->
    <div v-if="!favorites.loaded" class="flex flex-col gap-10" aria-busy="true" aria-label="Загружаем избранное">
      <div v-for="i in 3" :key="i" class="flex flex-col gap-4" :style="{ opacity: 1 - (i - 1) * 0.25 }">
        <div class="flex flex-wrap items-end justify-between gap-4 px-1">
          <div class="flex flex-col gap-2.5">
            <div class="skeleton h-3 w-24" />
            <div class="skeleton h-7 w-72 max-w-[70vw]" />
            <div class="flex gap-2"><div class="skeleton h-6 w-28 rounded-full" /><div class="skeleton h-6 w-20 rounded-full" /></div>
          </div>
          <div class="flex flex-col items-end gap-2"><div class="skeleton h-8 w-32" /><div class="skeleton h-3 w-24" /></div>
        </div>
        <div class="skeleton h-40 rounded-3xl" />
      </div>
    </div>

    <div v-else-if="!count" class="flex flex-col items-center gap-3 rounded-3xl bg-muted px-6 py-12 text-center">
      <span class="grid size-14 place-items-center rounded-full bg-background shadow-sm">
        <StarIcon class="size-6 text-amber-500" />
      </span>
      <p class="text-lg font-semibold tracking-tight">Пока пусто</p>
      <p class="max-w-sm text-sm text-muted-foreground">Найдите поездку и нажмите ☆ на понравившемся варианте — он появится здесь с актуальной ценой.</p>
      <RouterLink to="/" class="mt-2 flex h-10 items-center gap-1.5 rounded-full bg-cta px-5 text-sm font-medium text-cta-foreground hover:bg-cta-hover">
        <SearchIcon class="size-4" /> К поиску
      </RouterLink>
    </div>

    <TransitionGroup v-else tag="div" name="fav" appear class="relative flex flex-col gap-10">
      <article v-for="(f, i) in favorites.items" :key="f.id" class="flex flex-col gap-4" :style="{ '--i': Math.min(i, 8) }">
        <header class="flex flex-wrap items-end justify-between gap-x-6 gap-y-3 px-1">
          <div class="flex min-w-0 flex-col gap-2">
            <span v-if="titleParts(f).dates" class="eyebrow flex items-center gap-1.5">
              <CalendarIcon class="size-3.5" /> {{ titleParts(f).dates }}
            </span>
            <h2 class="route">
              <template v-if="titleParts(f).to">
                {{ titleParts(f).from }}
                <ArrowRightIcon class="route__arrow" />
                {{ titleParts(f).to }}
              </template>
              <template v-else>{{ f.title }}</template>
            </h2>
            <div class="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
              <span class="chip">
                <template v-if="f.pick.kind === 'drive'">
                  <CarIcon class="size-3.5 text-road" /> на машине<template v-if="f.snapshot.drive_hours">, {{ hours(f.snapshot.drive_hours) }}</template>
                </template>
                <template v-else>
                  <PlaneIcon class="size-3.5 text-flight" /> из г. {{ f.snapshot.hub_name ?? f.pick.hub_iata }}
                  <template v-if="f.snapshot.depart_at && f.snapshot.return_at">· {{ localDay(f.snapshot.depart_at) }} – {{ localDay(f.snapshot.return_at) }}</template>
                </template>
              </span>
              <span class="chip">
                <UsersIcon class="size-3.5" /> {{ f.params.travelers ?? 1 }} {{ plural(f.params.travelers ?? 1, 'человек', 'человека', 'человек') }}
              </span>
            </div>
          </div>

          <div class="flex items-end gap-3">
            <!-- Что с ценой с момента сохранения (сама цена — в карточке ниже); пока обновляется — скелетон -->
            <div class="flex min-w-32 flex-col items-end gap-1">
              <div v-if="pending(f)" class="skeleton h-8 w-36 rounded-full" />
              <template v-else-if="cur(f)?.status === 'found' && cur(f)?.diff">
                <span class="change" :class="cur(f)!.diff!.amount < 0 ? 'change--down' : cur(f)!.diff!.amount > 0 ? 'change--up' : ''">
                  <component :is="cur(f)!.diff!.amount < 0 ? TrendingDownIcon : cur(f)!.diff!.amount > 0 ? TrendingUpIcon : EqualIcon" class="size-4" />
                  <template v-if="cur(f)!.diff!.amount < 0">−{{ rub(-cur(f)!.diff!.amount) }}</template>
                  <template v-else-if="cur(f)!.diff!.amount > 0">+{{ rub(cur(f)!.diff!.amount) }}</template>
                  <template v-else>цена та же</template>
                </span>
                <span class="text-xs text-muted-foreground tabular-nums">
                  <s v-if="cur(f)!.diff!.amount !== 0">{{ rub(f.snapshot.total) }}</s><template v-else>{{ rub(f.snapshot.total) }}</template>
                  на {{ localDay(f.created_at) }}
                </span>
              </template>
              <span v-else class="text-xs text-muted-foreground tabular-nums">сохраняли за {{ rub(f.snapshot.total) }}</span>
            </div>
            <div class="flex items-center gap-2">
              <RouterLink
                :to="searchLink(f)"
                class="flex h-10 items-center gap-1.5 rounded-full px-3 text-sm font-medium whitespace-nowrap ring-1 ring-input transition-colors hover:bg-muted sm:px-4"
                title="Открыть поиск с этими параметрами"
                aria-label="Искать заново"
              >
                <SearchIcon class="size-4" /> <span class="hidden sm:inline">Искать заново</span>
              </RouterLink>
              <button
                type="button"
                class="flex size-10 items-center justify-center rounded-full ring-1 ring-input transition-colors hover:bg-[#fce8e6] hover:ring-[#f4c7c3]"
                aria-label="Удалить из избранного"
                title="Удалить из избранного"
                @click="favorites.remove(f)"
              >
                <TrashIcon class="size-4 text-muted-foreground" />
              </button>
            </div>
          </div>
        </header>

        <!-- Итог с путевым листом: билеты/дорога + выбранное жильё + траты -->
        <div v-if="extras(f).total" class="budget" :aria-busy="pending(f)">
          <span class="budget__part">
            <span class="budget__label">{{ f.pick.kind === 'drive' ? 'Дорога' : 'Билеты и дорога' }}</span>
            <div v-if="pending(f)" class="skeleton h-5 w-20" />
            <span v-else class="budget__value">{{ rub(tripTotal(f)) }}</span>
          </span>
          <template v-if="extras(f).stay">
            <span class="budget__op">+</span>
            <span class="budget__part">
              <span class="budget__label"><BedDoubleIcon class="size-3.5" /> Жильё</span>
              <span class="budget__value">{{ rub(extras(f).stay) }}</span>
            </span>
          </template>
          <template v-if="extras(f).other">
            <span class="budget__op">+</span>
            <span class="budget__part">
              <span class="budget__label"><ReceiptIcon class="size-3.5" /> Траты</span>
              <span class="budget__value">{{ rub(extras(f).other) }}</span>
            </span>
          </template>
          <span class="budget__op budget__op--eq">=</span>
          <span class="budget__part budget__part--total">
            <span class="budget__label">Итого на всех</span>
            <div v-if="pending(f)" class="skeleton h-6 w-28" />
            <span v-else class="budget__value">{{ rub(tripTotal(f) + extras(f).total) }}</span>
          </span>
          <span v-if="unchosen(f)" class="budget__note">
            {{ unchosen(f) }} {{ plural(unchosen(f), 'вариант', 'варианта', 'вариантов') }} без отметки «Выбрано» — не в итоге
          </span>
        </div>

        <p v-if="cur(f) && cur(f)!.status !== 'found'" class="rounded-2xl bg-muted px-4 py-2.5 text-sm text-muted-foreground">
          {{ statusText[cur(f)!.status] }}
        </p>

        <!-- Карточка варианта: пока цена обновляется — скелетон той же формы -->
        <div v-if="pending(f)" class="flex flex-col gap-4 rounded-3xl p-4 ring-1 ring-border sm:p-5" aria-busy="true">
          <div class="flex justify-between gap-4">
            <div class="flex flex-col gap-2"><div class="skeleton h-5 w-24 rounded-full" /><div class="skeleton h-6 w-56" /><div class="skeleton h-4 w-40" /></div>
            <div class="flex flex-col items-end gap-2"><div class="skeleton h-8 w-28" /><div class="skeleton h-3 w-32" /></div>
          </div>
          <div class="flex gap-3"><div class="skeleton h-4 w-36" /><div class="skeleton h-4 w-44" /><div class="skeleton h-5 w-28 rounded-full" /></div>
          <div class="flex gap-2"><div class="skeleton h-10 w-64 rounded-full" /><div class="skeleton h-10 w-36 rounded-full" /><div class="skeleton size-10 rounded-full" /></div>
        </div>
        <template v-else-if="cur(f)?.result">
          <TripOptionCard
            v-if="cur(f)!.option"
            :option="cur(f)!.option!"
            :home="cur(f)!.result!.home"
            :destination="cur(f)!.result!.destination"
            :travelers="f.params.travelers ?? 1"
            :date-from="f.params.date_from"
            :search-params="f.params"
            :favorite="f"
            :open-key="f.id"
          />
          <DriveOptionCard
            v-else-if="cur(f)!.drive"
            :drive="cur(f)!.drive!"
            :home="cur(f)!.result!.home"
            :destination="cur(f)!.result!.destination"
            :travelers="f.params.travelers ?? 1"
            :date-from="f.params.date_from"
            :search-params="f.params"
            :favorite="f"
            :open-key="f.id"
          />
        </template>
      </article>
    </TransitionGroup>
  </section>
</template>

<style scoped>
/*
 * Типографика — Inter с альтернативными начертаниями: cv11 — однослойная «a», ss01 — открытые цифры.
 * Крупный плотный заголовок, маршрут — вторым по силе, цены — моноширинными цифрами.
 */
.display {
  font-size: clamp(40px, 6vw, 56px);
  font-weight: 650;
  line-height: 1;
  letter-spacing: -0.04em;
  font-feature-settings: 'cv11', 'ss01';
}
.count {
  display: inline-grid;
  place-items: center;
  min-width: 1.5em;
  height: 1.5em;
  padding: 0 0.4em;
  border-radius: 999px;
  background: var(--accent);
  color: var(--accent-foreground);
  font-size: 0.36em;
  font-weight: 600;
  letter-spacing: 0;
  font-variant-numeric: tabular-nums;
  transform: translateY(-0.6em);
}
.eyebrow {
  font-size: 11.5px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted-foreground);
}
.route {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.15em 0.4em;
  font-size: clamp(22px, 3vw, 28px);
  font-weight: 600;
  line-height: 1.15;
  letter-spacing: -0.025em;
  font-feature-settings: 'cv11';
}
.route__arrow {
  width: 0.8em;
  height: 0.8em;
  color: var(--primary);
  stroke-width: 2.5;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--muted);
}
/* Изменение цены: крупные открытые цифры в пилюле, зелёная — подешевело, янтарная — подорожало */
.change {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  background: var(--muted);
  color: var(--muted-foreground);
  font-size: 15px;
  font-weight: 650;
  letter-spacing: -0.01em;
  white-space: nowrap;
  font-feature-settings: 'tnum', 'ss01';
}
.change--down {
  background: #c4eed0;
  color: #072711;
}
.change--up {
  background: #feefc3;
  color: #6b4a00;
}

/* Итог поездки «слагаемыми»: билеты + жильё + траты = итого */
.budget {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 8px 12px;
  padding: 12px 16px;
  border-radius: 20px;
  background: linear-gradient(90deg, #eef3fb, #f6f8fc);
}
.budget__part {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.budget__label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  color: var(--muted-foreground);
}
.budget__value {
  font-size: 17px;
  font-weight: 600;
  line-height: 1.2;
  letter-spacing: -0.01em;
  font-feature-settings: 'tnum', 'ss01';
}
.budget__op {
  padding-bottom: 2px;
  font-size: 17px;
  color: var(--muted-foreground);
}
.budget__op--eq {
  margin-left: auto;
}
.budget__part--total {
  align-items: flex-end;
}
.budget__note {
  flex-basis: 100%;
  font-size: 11.5px;
  color: var(--muted-foreground);
}
.budget__part--total .budget__label {
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  font-size: 10.5px;
}
.budget__part--total .budget__value {
  font-size: 24px;
  font-weight: 650;
  letter-spacing: -0.03em;
  color: var(--primary);
}

/* Строки появляются каскадом, удалённая — уезжает и схлопывается */
.fav-enter-active {
  transition: opacity 0.45s ease, transform 0.45s cubic-bezier(0.2, 0.8, 0.2, 1);
  transition-delay: calc(var(--i) * 60ms);
}
.fav-enter-from {
  opacity: 0;
  transform: translateY(12px);
}
.fav-leave-active {
  position: absolute;
  inset-inline: 0;
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.fav-leave-to {
  opacity: 0;
  transform: translateX(-24px);
}
.fav-move {
  transition: transform 0.4s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.toast-enter-active,
.toast-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(12px);
}
@media (prefers-reduced-motion: reduce) {
  .fav-enter-active,
  .fav-leave-active,
  .fav-move,
  .toast-enter-active,
  .toast-leave-active {
    transition: none;
  }
}
</style>
