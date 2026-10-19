import { computed, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'

/** Телефон: подробности варианта — отдельным экраном поверх списка, а не раскрытием внутри него. */
export const useIsPhone = () => useMediaQuery('(max-width: 639.98px)')

/**
 * Раскрыт ли вариант. На телефоне состояние живёт в адресе (?trip=<key>): системное «назад» закрывает экран
 * и возвращает к списку на то же место, а ссылкой можно открыть сразу нужный вариант. На широком экране
 * вариант раскрывается внутри карточки, адрес не меняется.
 */
export function useTripOpen(key: () => string | undefined) {
  const route = useRoute()
  const router = useRouter()
  const phone = useIsPhone()
  const local = ref(false)
  const routed = computed(() => phone.value && !!key())

  const open = computed({
    get: () => (routed.value ? route.query.trip === key() : local.value),
    set: (v: boolean) => {
      if (!routed.value) {
        local.value = v
        return
      }
      if (v) {
        router.push({ query: { ...route.query, trip: key() } })
        return
      }
      // Открыли из списка — возвращаемся по истории, чтобы «вперёд» не открывало экран снова; пришли по ссылке — просто убираем
      const back = window.history.state?.back
      if (typeof back === 'string' && !back.includes('trip=')) router.back()
      else router.replace({ query: { ...route.query, trip: undefined } })
    },
  })

  return { open, screen: computed(() => routed.value && open.value) }
}
