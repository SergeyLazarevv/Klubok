import type { InjectionKey, Ref } from 'vue'
import type { StopSuggestion } from '@/api/client'
import { hours } from '@/lib/format'

/** Выбор городов-остановок (ADR-0023): план поездки отдаёт его карте, чтобы заехать можно было и с карты. */
export interface StopPicker {
  suggestions: Ref<StopSuggestion[] | undefined>
  chosen: Ref<string[]>
  busy: Ref<string | undefined>
  error: Ref<string | undefined> // id города, у которого пересчёт не удался
  toggle: (s: StopSuggestion) => Promise<void>
}

export const stopPickerKey: InjectionKey<StopPicker> = Symbol('stopPicker')

export const detourLabel = (s: StopSuggestion) =>
  s.detour.duration_min < 10 ? 'почти без крюка' : `+${hours(s.detour.duration_min / 60)}`
