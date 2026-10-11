<script setup lang="ts">
import { ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { MapPinIcon, PlaneIcon } from '@lucide/vue'
import { api, type Place } from '@/api/client'
import {
  Combobox,
  ComboboxAnchor,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox'

const props = withDefaults(defineProps<{ placeholder?: string; kind?: 'city' | 'airport' }>(), {
  placeholder: 'Город или код аэропорта',
})
const model = defineModel<Place | null>({ default: null })

const term = ref('')
const items = ref<Place[]>([])
const loading = ref(false)

let seq = 0

const load = useDebounceFn(async (q: string) => {
  const current = ++seq
  if (q.trim().length < 2) {
    items.value = []
    return
  }
  loading.value = true
  const { data } = await api.GET('/places', { params: { query: { q: q.trim(), kind: props.kind, limit: 8 } } })
  if (current !== seq) return // пользователь уже ввёл другой текст
  items.value = data?.items ?? []
  loading.value = false
}, 200)

const label = (p: Place | null) => (p ? `${p.name}, ${p.iata}` : '')

// После выбора в поле подставляется подпись точки — искать по ней не нужно
watch(term, (q) => q !== label(model.value) && load(q))
</script>

<template>
  <Combobox v-model="model" by="id" ignore-filter :reset-search-term-on-blur="false">
    <ComboboxAnchor class="w-full">
      <ComboboxInput v-model="term" :display-value="label" :placeholder="placeholder" class="h-10 text-base" />
    </ComboboxAnchor>
    <ComboboxList>
      <ComboboxEmpty class="p-3 text-sm text-muted-foreground">
        {{ term.trim().length < 2 ? 'Введите хотя бы две буквы' : loading ? 'Ищем…' : 'Ничего не нашли' }}
      </ComboboxEmpty>
      <ComboboxItem v-for="p in items" :key="p.id" :value="p" class="py-2">
        <PlaneIcon v-if="p.kind === 'airport'" class="text-muted-foreground" />
        <MapPinIcon v-else class="text-muted-foreground" />
        <span class="truncate">{{ p.name }}</span>
        <span class="ml-auto font-mono text-xs text-muted-foreground">{{ p.iata }} · {{ p.country_code }}</span>
      </ComboboxItem>
    </ComboboxList>
  </Combobox>
</template>
