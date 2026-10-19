import { shallowRef, watchEffect, type MaybeRefOrGetter, toValue } from 'vue'
import {
  CloudDrizzleIcon, CloudFogIcon, CloudIcon, CloudLightningIcon, CloudRainIcon, CloudSnowIcon, CloudSunIcon, SunIcon,
} from '@lucide/vue'
import { api, type WeatherDay, type WeatherResponse } from '@/api/client'
import { addDays, todayISO } from '@/stores/search'
import { plural } from '@/lib/format'

/** Погода на даты поездки (GET /weather): ближайшие 16 дней — прогноз, дальше — «обычно в эти дни». */

const MAX_DAYS = 62 // как на backend

export interface WeatherQuery {
  lat: number
  lon: number
  from: string // YYYY-MM-DD, местные даты точки
  to: string
}

// Один запрос на точку и даты за вкладку: карточки с одинаковыми датами не спрашивают погоду каждая сама
const requests = new Map<string, Promise<WeatherResponse | undefined>>()

function fetchWeather(q: WeatherQuery) {
  // Прошедшие дни не спрашиваем (погода нужна на поездку впереди), длинный диапазон обрезаем
  const today = todayISO()
  const from = q.from < today ? today : q.from
  const to = q.to > addDays(from, MAX_DAYS - 1) ? addDays(from, MAX_DAYS - 1) : q.to
  if (to < from) return Promise.resolve(undefined)
  const key = `${q.lat.toFixed(1)},${q.lon.toFixed(1)},${from},${to}`
  let req = requests.get(key)
  if (!req) {
    req = api
      .GET('/weather', { params: { query: { lat: q.lat, lon: q.lon, date_from: from, date_to: to } } })
      .then(({ data }) => data)
      .catch(() => undefined)
    requests.set(key, req)
    req.then((data) => !data && requests.delete(key)) // ошибку не запоминаем — в следующий раз спросим снова
  }
  return req
}

/**
 * Погода для компонента: пересчитывается при смене точки и дат. loading — запрос идёт; ответа нет (источник
 * недоступен, ошибка) — weather остаётся undefined, а loading снимается, чтобы блок не висел скелетоном.
 */
export function useWeather(query: MaybeRefOrGetter<WeatherQuery | undefined>) {
  const weather = shallowRef<WeatherResponse>()
  const loading = shallowRef(false)
  let seq = 0
  watchEffect(() => {
    const q = toValue(query)
    const current = ++seq
    weather.value = undefined
    loading.value = !!q
    if (!q) return
    fetchWeather(q).then((data) => {
      if (current !== seq) return
      weather.value = data
      loading.value = false
    })
  })
  return { weather, loading }
}

// --- Подписи и значки ---

interface Kind {
  label: string
  icon: typeof SunIcon
  tone: string // цвет контура и заливка: контурный светлый значок на сером фоне выглядел пустым
}

export function weatherKind(code: number): Kind {
  if (code <= 1) return { label: 'Ясно', icon: SunIcon, tone: 'text-amber-500 fill-amber-300' }
  if (code === 2) return { label: 'Переменная облачность', icon: CloudSunIcon, tone: 'text-slate-500 fill-amber-200' }
  if (code === 3) return { label: 'Облачно', icon: CloudIcon, tone: 'text-slate-500 fill-slate-300' }
  if (code === 45 || code === 48) return { label: 'Туман', icon: CloudFogIcon, tone: 'text-slate-500 fill-slate-200' }
  if (code >= 95) return { label: 'Гроза', icon: CloudLightningIcon, tone: 'text-violet-600 fill-violet-200' }
  if ((code >= 71 && code <= 77) || code === 85 || code === 86) return { label: 'Снег', icon: CloudSnowIcon, tone: 'text-sky-500 fill-sky-100' }
  if (code >= 51 && code <= 57) return { label: 'Морось', icon: CloudDrizzleIcon, tone: 'text-sky-600 fill-sky-200' }
  return { label: 'Дождь', icon: CloudRainIcon, tone: 'text-sky-600 fill-sky-200' }
}

/** «+12°», «−3°», «0°» — со знаком, как в прогнозах */
export function temp(c: number) {
  const v = Math.round(c)
  return v > 0 ? `+${v}°` : v < 0 ? `−${-v}°` : '0°'
}

const isSnow = (code: number) => (code >= 71 && code <= 77) || code === 85 || code === 86

/** Вероятность осадков в этот день, 0–1: у прогноза — из прогноза, у нормы — доля дождливых дней в эти даты. */
export function wetChance(d: WeatherDay): number {
  if (d.source === 'climate') return d.wet_share ?? 0
  if (d.precip_probability != null) return d.precip_probability / 100
  return d.precip_mm >= 1 ? 1 : 0
}

export interface WeatherSummary {
  kind: Kind
  range: string // «+4…+10°» — от ночного минимума до дневного максимума
  wet: string // «дожди ~45% дней», «дождь 2 дня из 4», «без осадков»
  source: string // «прогноз», «обычно в эти дни», «прогноз и норма»
  hint: string // для подсказки при наведении
}

export function summarize(days: WeatherDay[]): WeatherSummary | undefined {
  if (!days.length) return undefined
  const tMin = Math.min(...days.map((d) => d.t_min))
  const tMax = Math.max(...days.map((d) => d.t_max))
  // Значок — самая частая погода (осадки важнее, см. ниже)
  const counts = new Map<string, { n: number; code: number }>()
  for (const d of days) {
    const k = weatherKind(d.code).label
    counts.set(k, { n: (counts.get(k)?.n ?? 0) + 1, code: d.code })
  }
  const top = [...counts.values()].sort((a, b) => b.n - a.n)[0]
  const expectedWet = days.reduce((s, d) => s + wetChance(d), 0)
  const share = expectedWet / days.length
  const snow = days.filter((d) => isSnow(d.code)).length > days.length / 2
  const what = snow ? 'снег' : 'дождь'
  const whatMany = snow ? 'снегопады' : 'дожди'
  let wet: string
  if (share < 0.15) wet = 'без осадков'
  else if (days.length <= 7) {
    const n = Math.max(1, Math.round(expectedWet))
    wet = n >= days.length ? `${what} каждый день` : `${what} ~${n} ${plural(n, 'день', 'дня', 'дней')} из ${days.length}`
  } else wet = `${whatMany} ~${Math.round(share * 100)}% дней`

  const forecast = days.filter((d) => d.source === 'forecast').length
  const source = forecast === days.length ? 'прогноз' : forecast === 0 ? 'обычно в эти дни' : 'прогноз и норма'
  // Осадки в трети дней и чаще — значок дождя или снега, даже если по дням чаще «облачно»
  const code = share >= 0.35 ? (snow ? 71 : 61) : top.code
  return {
    kind: weatherKind(code),
    range: tMin === tMax ? temp(tMax) : `${temp(tMin)}…${temp(tMax)}`,
    wet,
    source,
    hint:
      forecast === days.length
        ? 'Прогноз Open-Meteo: ночью от, днём до'
        : forecast === 0
          ? 'Обычная погода в эти даты — средние за 5 прошлых лет (Open-Meteo): ночью от, днём до'
          : 'Ближайшие дни — прогноз, дальше — средние за 5 прошлых лет (Open-Meteo)',
  }
}
