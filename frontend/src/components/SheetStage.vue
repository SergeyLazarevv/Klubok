<script setup lang="ts">
import { computed, inject, nextTick, ref, watch } from 'vue'
import { CheckIcon, ExternalLinkIcon, ImageIcon, LoaderCircleIcon, MapPinIcon, PlusIcon, XIcon } from '@lucide/vue'
import { api, type LinkPreview, type TripItem, type TripItemStage } from '@/api/client'
import { plural, rub } from '@/lib/format'
import { parseEntry, parseRub, siteOf } from '@/lib/sheet'
import { tripSheetKey } from '@/lib/tripSheet'

/**
 * Пункты путевого листа на одном этапе поездки и строка добавления. Одно поле на всё: ссылка станет
 * карточкой с картинкой (превью со страницы объявления), текст — заметкой-делом. prompt — пользователь
 * вернулся с сайта жилья: сразу открываем поле и подсказываем вставить ссылку.
 * Ссылки — это варианты: в итог идёт цена только тех, что отмечены «Выбрано».
 */
const props = defineProps<{
  stage: TripItemStage
  addLabel: string
  placeholder: string
  prompt?: boolean
  hint?: string // подсказка, пока список пуст
  nights?: number // для жилья: цену за сутки из объявления умножаем на ночи
}>()
const emit = defineEmits<{ prompted: [] }>()

const sheet = inject(tripSheetKey)!
const items = computed(() => sheet.byStage(props.stage))
const links = computed(() => items.value.filter((i) => i.kind === 'link'))
const others = computed(() => items.value.filter((i) => i.kind !== 'link'))
// Несколько вариантов и ни один не выбран — подсказываем, что в итог они не входят
const needsChoice = computed(() => links.value.length > 1 && !links.value.some((i) => i.done))
// Дела и места после отметки зачёркиваем — они уже позади
const site = (i: TripItem) => (i.url ? siteOf(i.url).name : '')

const open = ref(false)
const text = ref('')
const price = ref('')
const priceTouched = ref(false)
const busy = ref(false)
const input = ref<HTMLInputElement>()

async function start() {
  open.value = true
  await nextTick()
  input.value?.focus()
}
watch(
  () => props.prompt,
  (p) => p && void start(),
)

function cancel() {
  open.value = false
  text.value = ''
  price.value = ''
  priceTouched.value = false
  preview.value = undefined
  emit('prompted')
}

// Превью ссылки прямо в форме: картинка, название и цена за сутки — пока пользователь смотрит, что добавляет
const preview = ref<LinkPreview>()
const previewLoading = ref(false)
const entry = computed(() => parseEntry(text.value, props.stage))
let previewTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => entry.value?.url,
  (url) => {
    clearTimeout(previewTimer)
    preview.value = undefined
    previewLoading.value = false
    if (!url) return
    previewLoading.value = true
    previewTimer = setTimeout(async () => {
      const { data } = await api.GET('/link-preview', { params: { query: { url } } })
      if (entry.value?.url !== url) return // уже вставили другую ссылку
      previewLoading.value = false
      preview.value = data
      // Цена из объявления — за сутки; за всю поездку умножаем на ночи. Своё значение не трогаем
      if (data?.per_night && !priceTouched.value) {
        price.value = String(Math.round((data.per_night.amount * (props.nights ?? 1)) / 100))
      }
    }, 350)
  },
)
const nightsLabel = computed(() => (props.nights ? `${props.nights} ${plural(props.nights, 'ночь', 'ночи', 'ночей')}` : ''))

async function submit() {
  const e = entry.value
  if (!e || busy.value) return
  busy.value = true
  try {
    const amount = parseRub(price.value)
    // Своего текста рядом со ссылкой нет — берём название со страницы
    const ownText = e.url ? text.value.replace(e.url, '').trim() : ''
    const p = preview.value
    const saved = await sheet.add({
      stage: props.stage,
      ...e,
      title: !ownText && p?.title ? p.title.slice(0, 300) : e.title,
      image_url: p?.image_url,
      description: p?.description?.slice(0, 500),
      price: amount === undefined ? undefined : { amount, currency: 'RUB' },
    })
    if (saved) cancel()
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div v-if="sheet.enabled.value" class="flex flex-col gap-2">
    <!-- Варианты по ссылкам: карточки с картинкой; выбранный подсвечен, его цена идёт в итог -->
    <ul v-if="links.length" class="flex flex-col gap-2">
      <li
        v-for="i in links"
        :key="i.id"
        class="group flex gap-3 rounded-2xl bg-card p-2.5 ring-1 transition-shadow sm:p-3"
        :class="i.done ? 'ring-2 ring-primary' : 'ring-border'"
      >
        <a :href="i.url" target="_blank" rel="noopener noreferrer" class="shrink-0" tabindex="-1" aria-hidden="true">
          <img v-if="i.image_url" :src="i.image_url" alt="" loading="lazy" referrerpolicy="no-referrer" class="h-20 w-20 rounded-xl object-cover sm:h-24 sm:w-32" />
          <span v-else class="flex h-20 w-20 items-center justify-center rounded-xl bg-muted text-muted-foreground sm:h-24 sm:w-32">
            <ImageIcon class="size-6" />
          </span>
        </a>
        <div class="flex min-w-0 flex-1 flex-col gap-1">
          <div class="flex items-start gap-1">
            <a
              :href="i.url"
              target="_blank"
              rel="noopener noreferrer"
              class="line-clamp-2 min-w-0 flex-1 text-sm leading-snug font-semibold break-words text-primary hover:underline"
            >
              {{ i.title }}
            </a>
            <button
              type="button"
              class="hit -mt-1 -mr-1 flex size-7 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-muted sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
              aria-label="Удалить из путевого листа"
              title="Удалить"
              @click="sheet.remove(i)"
            >
              <XIcon class="size-4" />
            </button>
          </div>
          <span v-if="i.description" class="line-clamp-1 text-xs text-muted-foreground max-sm:hidden">{{ i.description }}</span>
          <div class="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5">
            <span class="flex min-w-0 flex-wrap items-baseline gap-x-2 text-xs text-muted-foreground">
              <span>{{ site(i) }}</span>
              <span v-if="i.price" class="tabular-nums" :class="i.done ? 'text-sm font-semibold text-foreground' : ''">
                {{ rub(i.price) }}<template v-if="!i.done"> · вариант</template>
              </span>
            </span>
            <button
              type="button"
              class="hit flex h-8 items-center gap-1 rounded-full px-3 text-xs font-medium whitespace-nowrap transition-colors"
              :class="i.done ? 'bg-primary text-primary-foreground' : 'text-primary ring-1 ring-input hover:bg-accent'"
              :aria-pressed="i.done"
              :title="i.done ? 'Выбрано — цена входит в итог поездки' : 'Выбрать этот вариант — его цена войдёт в итог'"
              @click="sheet.toggle(i)"
            >
              <template v-if="i.done"><CheckIcon class="size-3.5" /> Выбрано</template>
              <template v-else>Выбрать</template>
            </button>
          </div>
        </div>
      </li>
    </ul>
    <p v-if="needsChoice" class="px-1 text-xs text-muted-foreground">Отметьте вариант, который выбрали, — в итог пойдёт только его цена.</p>

    <!-- Дела и места: чеклист -->
    <ul v-if="others.length" class="flex flex-col divide-y divide-border rounded-2xl bg-card ring-1 ring-primary/30">
      <li v-for="i in others" :key="i.id" class="group flex items-center gap-3 px-4 py-3">
        <button
          type="button"
          class="hit flex size-5 shrink-0 items-center justify-center rounded-md ring-1 ring-input transition-colors"
          :class="i.done ? 'bg-primary text-primary-foreground ring-primary' : 'hover:bg-muted'"
          :aria-pressed="i.done"
          :aria-label="i.done ? 'Отметить как не сделанное' : 'Отметить как сделанное'"
          title="Сделано"
          @click="sheet.toggle(i)"
        >
          <CheckIcon v-if="i.done" class="size-3.5" />
        </button>
        <div class="flex min-w-0 flex-1 flex-col" :class="i.done && 'opacity-60'">
          <a
            v-if="i.url"
            :href="i.url"
            target="_blank"
            rel="noopener noreferrer"
            class="flex min-w-0 items-start gap-1.5 font-medium text-primary hover:underline"
            :class="i.done && 'line-through'"
          >
            <span class="line-clamp-2 break-words">{{ i.title }}</span> <ExternalLinkIcon class="mt-1 size-3.5 shrink-0" />
          </a>
          <span v-else class="flex min-w-0 items-center gap-1.5 font-medium" :class="i.done && 'line-through'">
            <MapPinIcon v-if="i.kind === 'place'" class="size-3.5 shrink-0 text-muted-foreground" />
            <span class="break-words">{{ i.title }}</span>
          </span>
          <span v-if="i.price" class="text-xs text-muted-foreground sm:hidden">{{ rub(i.price) }}</span>
        </div>
        <span v-if="i.price" class="shrink-0 text-sm font-semibold tabular-nums max-sm:hidden">{{ rub(i.price) }}</span>
        <button
          type="button"
          class="hit flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-muted sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
          aria-label="Удалить из путевого листа"
          title="Удалить"
          @click="sheet.remove(i)"
        >
          <XIcon class="size-4" />
        </button>
      </li>
    </ul>

    <p v-if="!items.length && hint" class="rounded-2xl border border-dashed border-input px-4 py-3 text-sm text-muted-foreground">{{ hint }}</p>

    <form
      v-if="open"
      class="flex flex-col gap-2 rounded-2xl bg-card p-3 ring-1 ring-primary/40"
      @submit.prevent="submit"
      @keydown.esc="cancel"
    >
      <p v-if="prompt" class="px-1 text-sm">Нашли подходящий вариант? Вставьте ссылку на объявление — он останется в путевом листе.</p>
      <div class="flex flex-col gap-2 sm:flex-row">
        <input
          ref="input"
          v-model="text"
          type="text"
          :placeholder="placeholder"
          maxlength="2300"
          class="h-10 min-w-0 flex-1 rounded-xl bg-background px-3 text-sm ring-1 ring-input outline-none focus:ring-2 focus:ring-primary"
        />
        <div class="flex gap-2">
          <input
            v-model="price"
            type="text"
            inputmode="decimal"
            placeholder="Цена, ₽"
            aria-label="Цена за всю поездку, если знаете"
            class="h-10 w-28 min-w-0 rounded-xl bg-background px-3 text-sm tabular-nums ring-1 ring-input outline-none focus:ring-2 focus:ring-primary max-sm:flex-1"
            @input="priceTouched = true"
          />
          <button
            type="submit"
            :disabled="!text.trim() || busy"
            class="h-10 shrink-0 rounded-full bg-primary px-4 text-sm font-medium text-primary-foreground disabled:opacity-50"
          >
            Добавить
          </button>
          <button type="button" class="flex size-10 shrink-0 items-center justify-center rounded-full hover:bg-muted" aria-label="Отмена" @click="cancel">
            <XIcon class="size-4" />
          </button>
        </div>
      </div>
      <!-- Превью ссылки -->
      <div v-if="previewLoading" class="flex items-center gap-2 px-1 text-xs text-muted-foreground">
        <LoaderCircleIcon class="size-3.5 animate-spin" /> Смотрим объявление…
      </div>
      <div v-else-if="preview" class="flex gap-3 rounded-xl bg-muted/60 p-2">
        <img v-if="preview.image_url" :src="preview.image_url" alt="" referrerpolicy="no-referrer" class="h-16 w-20 shrink-0 rounded-lg object-cover" />
        <div class="flex min-w-0 flex-col gap-0.5 text-xs">
          <span class="line-clamp-2 text-sm font-medium">{{ preview.title || preview.site }}</span>
          <span class="text-muted-foreground">
            {{ preview.site }}
            <template v-if="preview.per_night">
              · {{ rub(preview.per_night) }} за сутки{{ nightsLabel ? ` × ${nightsLabel} — подставили в цену` : '' }}
            </template>
          </span>
        </div>
      </div>
      <p v-else-if="entry?.url" class="px-1 text-xs text-muted-foreground">
        Превью с этого сайта не получить — подпишите вариант и укажите цену сами.
      </p>
    </form>
    <button
      v-else
      type="button"
      class="flex w-fit items-center gap-1.5 rounded-full px-1 text-sm font-medium text-primary hover:underline"
      @click="start"
    >
      <PlusIcon class="size-4" /> {{ items.length ? 'Добавить ещё' : addLabel }}
    </button>
  </div>
</template>
