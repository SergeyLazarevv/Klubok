<script setup lang="ts">
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { XIcon } from '@lucide/vue'
import { arrivalOf, label, type Trip } from '@/lib/tripMap'
import TripMapFull from '@/components/TripMapFull.vue'

const props = defineProps<Trip>()
const open = defineModel<boolean>('open', { required: true })
</script>

<template>
  <DialogRoot v-model:open="open">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-foreground/30 data-[state=open]:animate-in data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed inset-0 z-50 flex flex-col bg-card outline-none sm:inset-6 sm:overflow-hidden sm:rounded-xl sm:border sm:shadow-xl"
      >
        <header class="flex items-start gap-3 border-b px-4 pt-[max(0.75rem,env(safe-area-inset-top))] pb-3">
          <div class="flex min-w-0 flex-1 flex-col">
            <DialogTitle class="truncate font-semibold">
              {{ label(props.home) }} → {{ label(arrivalOf(props)) }}
            </DialogTitle>
            <DialogDescription class="text-xs text-muted-foreground">
              Заправки и кафе вдоль дороги, достопримечательности по пути и на месте
            </DialogDescription>
          </div>
          <DialogClose
            class="-my-1 -mr-2 flex size-11 shrink-0 items-center justify-center rounded-full hover:bg-accent"
            aria-label="Закрыть карту"
          >
            <XIcon class="size-5" />
          </DialogClose>
        </header>
        <TripMapFull :home="home" :option="option" :drive="drive" :destination="destination" :stops="stops" class="min-h-0 flex-1" />
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
