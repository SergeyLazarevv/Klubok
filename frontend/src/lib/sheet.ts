import type { DriveStop, TripItem, TripItemCreate, TripItemStage } from '@/api/client'

/**
 * Путевой лист (ADR-0019): что пользователь добавил к поездке. Ссылки — на чужие сайты, поэтому по адресу
 * узнаём сайт и что это — жильё, аренда машины или экскурсия, — чтобы подписать пункт, даже если название не ввели.
 */
type Category = 'stay' | 'car' | 'tour'
const sites: [RegExp, string, Category?][] = [
  [/(^|\.)sutochno\.ru$/, 'Суточно.ру', 'stay'],
  [/(^|\.)avito\.ru$/, 'Авито'],
  [/(^|\.)ostrovok\.ru$/, 'Островок', 'stay'],
  [/(^|\.)101hotels\.com$/, '101 Отель', 'stay'],
  [/(^|\.)tvil\.ru$/, 'Твил', 'stay'],
  [/(^|\.)booking\.com$/, 'Booking', 'stay'],
  [/(^|\.)airbnb\.[a-z.]+$/, 'Airbnb', 'stay'],
  [/(^|\.)travel\.yandex\.ru$/, 'Яндекс Путешествия', 'stay'],
  [/(^|\.)localrent\.com$/, 'Localrent', 'car'],
  [/(^|\.)myrentacar\.com$/, 'MyRentacar', 'car'],
  [/(^|\.)getrentacar\.com$/, 'GetRentacar', 'car'],
  [/(^|\.)rentalcars\.com$/, 'Rentalcars', 'car'],
  [/(^|\.)tripster\.ru$/, 'Трипстер', 'tour'],
  [/(^|\.)sputnik8\.com$/, 'Спутник8', 'tour'],
]

const categoryTitle: Record<Category, string> = { stay: 'Жильё', car: 'Аренда машины', tour: 'Экскурсия' }

/** Название, которое мы придумали сами («Жильё — Суточно.ру»), — его можно заменить названием со страницы. */
export const isAutoTitle = (title: string) => /^(Жильё|Аренда машины|Экскурсия|Ссылка) — /.test(title)

/** Сайты, с которых сервер умеет брать превью (тот же список, что linkpreview.allowed). */
const previewHosts = /(^|\.)(sutochno\.ru|ostrovok\.ru|101hotels\.com|tvil\.ru|tripster\.ru|sputnik8\.com|localrent\.com)$/
export function hasPreview(url: string) {
  try {
    return previewHosts.test(new URL(url).hostname)
  } catch {
    return false
  }
}

export function siteOf(url: string): { name: string; category?: Category } {
  let host = ''
  try {
    host = new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return { name: url }
  }
  const hit = sites.find(([re]) => re.test(host))
  return hit ? { name: hit[1], category: hit[2] } : { name: host }
}

const urlRe = /https?:\/\/[^\s<>"]+/i

/**
 * Одна строка ввода — и заметка, и ссылка. Из приложений Авито, Суточно и других «Поделиться» отдаёт
 * «Название — город https://…»: ссылку забираем, остальное становится названием.
 */
export function parseEntry(text: string, stage: TripItemStage): Pick<TripItemCreate, 'kind' | 'title' | 'url'> | undefined {
  const raw = text.trim()
  if (!raw) return
  const url = raw.match(urlRe)?.[0].replace(/[.,;)]+$/, '')
  if (!url) return { kind: 'note', title: raw.slice(0, 300) }
  const rest = raw.replace(url, '').replace(/^[\s,:;—–-]+|[\s,:;—–-]+$/g, '')
  const site = siteOf(url)
  const what = site.category ? categoryTitle[site.category] : stage === 'stay' ? categoryTitle.stay : 'Ссылка'
  return { kind: 'link', url, title: (rest || `${what} — ${site.name}`).slice(0, 300) }
}

/** «12 000», «12000 ₽», «1 500,50» → копейки; пусто или мусор — undefined. */
export function parseRub(text: string): number | undefined {
  const n = Number(text.replace(/[\s ₽]/g, '').replace(',', '.'))
  return text.trim() && Number.isFinite(n) && n >= 0 ? Math.round(n * 100) : undefined
}

/** Идёт ли пункт в итог: ссылки — это варианты (жильё, аренда), в итог только выбранная; дела и места с ценой — всегда. */
export const countedInTotal = (i: TripItem) => i.kind !== 'link' || i.done

/** Добавления путевого листа к цене поездки, копейки: жильё (этап «где остановиться») и остальные траты. */
export function sheetExtras(items: TripItem[]) {
  let stay = 0
  let other = 0
  for (const i of items) {
    if (!countedInTotal(i) || !i.price) continue
    if (i.stage === 'stay') stay += i.price.amount
    else other += i.price.amount
  }
  return { stay, other, total: stay + other }
}

/**
 * Ночёвки в городах-остановках, для которых выбрано своё жильё с ценой, копейки: оценка Росстата уже в цене
 * поездки, а теперь вместо неё в итог идёт цена объявления — оценку из итога вычитаем, чтобы не считать дважды.
 */
export function replacedStopStays(items: TripItem[], stops: DriveStop[] = []) {
  let sum = 0
  for (const s of stops) {
    const own = items.some((i) => i.city === s.city.id && i.stage === 'stay' && countedInTotal(i) && i.price)
    if (own && s.stay) sum += s.stay.amount
  }
  return sum
}
