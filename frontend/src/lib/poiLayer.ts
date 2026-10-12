import { h, render, type Component } from 'vue'
import L from 'leaflet'
import 'leaflet.markercluster'
import 'leaflet.markercluster/dist/MarkerCluster.css'
import { FuelIcon, LandmarkIcon, UtensilsIcon } from '@lucide/vue'
import { api, type Poi } from '@/api/client'

export type PoiCategory = Poi['category']

export const categories: { id: PoiCategory; label: string; icon: Component }[] = [
  { id: 'fuel', label: 'Заправки', icon: FuelIcon },
  { id: 'food', label: 'Кафе', icon: UtensilsIcon },
  { id: 'attraction', label: 'Места', icon: LandmarkIcon },
]

const kindLabels: Record<string, string> = {
  fuel: 'АЗС',
  cafe: 'Кафе',
  restaurant: 'Ресторан',
  fast_food: 'Фастфуд',
  attraction: 'Достопримечательность',
  museum: 'Музей',
  viewpoint: 'Смотровая площадка',
  gallery: 'Галерея',
  zoo: 'Зоопарк',
  theme_park: 'Парк развлечений',
  castle: 'Замок',
  fort: 'Крепость',
  monument: 'Памятник',
  ruins: 'Руины',
  archaeological_site: 'Археологический памятник',
  manor: 'Усадьба',
  monastery: 'Монастырь',
}

const iconHtml = new Map<PoiCategory, string>()

/** SVG иконки Lucide строкой — для divIcon Leaflet. */
function svg(category: PoiCategory) {
  let html = iconHtml.get(category)
  if (!html) {
    const el = document.createElement('div')
    render(h(categories.find((c) => c.id === category)!.icon, { size: 14, strokeWidth: 2.25 }), el)
    html = el.innerHTML
    iconHtml.set(category, html)
  }
  return html
}

function poiIcon(category: PoiCategory) {
  return L.divIcon({
    html: svg(category),
    className: `poi-marker poi-${category}`,
    iconSize: [26, 26],
    iconAnchor: [13, 13],
    popupAnchor: [0, -12],
  })
}

/** Группа маркеров одной категории: близкие точки собираются в кружок с числом. */
export function poiCluster(category: PoiCategory) {
  return L.markerClusterGroup({
    maxClusterRadius: 45,
    showCoverageOnHover: false,
    spiderfyOnMaxZoom: true,
    iconCreateFunction: (cluster) =>
      L.divIcon({
        html: `${svg(category)}<span>${cluster.getChildCount()}</span>`,
        className: `poi-cluster poi-${category}`,
        iconSize: [44, 26],
        iconAnchor: [22, 13],
      }),
  })
}

export function poiMarker(p: Poi) {
  const m = L.marker([p.location.lat, p.location.lon], { icon: poiIcon(p.category), title: title(p) })
  m.bindPopup(() => popup(p, p.details_pending), { className: 'poi-popup', maxWidth: 280, minWidth: 220 })
  // Описание и фото подгружаются при первом открытии карточки и дальше лежат на сервере
  if (p.details_pending) {
    m.on('popupopen', async () => {
      const full = await fetchDetails(p.id)
      if (full) m.setPopupContent(popup(full, false))
    })
  }
  return m
}

const detailsCache = new Map<string, Promise<Poi | undefined>>()

function fetchDetails(id: string) {
  let details = detailsCache.get(id)
  if (!details) {
    const [type, osmId] = id.split('/') as ['node' | 'way' | 'relation', string]
    details = api
      .GET('/pois/{type}/{id}', { params: { path: { type, id: Number(osmId) } } })
      .then(({ data }) => data)
    detailsCache.set(id, details)
  }
  return details
}

const title = (p: Poi) => p.name ?? p.brand ?? kindLabels[p.kind] ?? 'Без названия'

const cuisineLabels: Record<string, string> = {
  regional: 'местная кухня',
  russian: 'русская кухня',
  kazakh: 'казахская кухня',
  uzbek: 'узбекская кухня',
  caucasian: 'кавказская кухня',
  georgian: 'грузинская кухня',
  asian: 'азиатская кухня',
  japanese: 'японская кухня',
  sushi: 'суши',
  chinese: 'китайская кухня',
  italian: 'итальянская кухня',
  european: 'европейская кухня',
  pizza: 'пицца',
  burger: 'бургеры',
  shawarma: 'шаурма',
  kebab: 'шашлык',
  grill: 'гриль',
  coffee_shop: 'кофейня',
  tea: 'чай',
  cake: 'торты',
  pastry: 'выпечка',
  pancake: 'блины',
  dumplings: 'пельмени',
  sandwich: 'сэндвичи',
  chicken: 'курица',
  ice_cream: 'мороженое',
  breakfast: 'завтраки',
}

function fee(v: string) {
  if (v === 'no') return 'Вход свободный'
  if (v === 'yes') return 'Вход платный'
  return `Вход: ${v}`
}

// Карточка места. Тексты из OSM и Википедии — только через textContent, без HTML-строк.
function popup(p: Poi, loading: boolean) {
  const root = document.createElement('div')
  const add = (tag: string, className: string, text: string, parent: HTMLElement = root) => {
    const el = document.createElement(tag)
    el.className = className
    el.textContent = text
    parent.append(el)
    return el
  }
  const link = (parent: HTMLElement, href: string, text: string, className = '') => {
    const a = document.createElement('a')
    a.href = href
    a.target = '_blank'
    a.rel = 'noopener'
    a.className = className
    a.textContent = text
    parent.append(a)
    return a
  }

  if (p.photo) {
    const img = document.createElement('img')
    img.src = p.photo.url
    img.alt = title(p)
    img.loading = 'lazy'
    img.className = 'poi-popup-photo'
    root.append(img)
    link(root, p.photo.page, `Фото: ${p.photo.credit ?? 'Wikimedia Commons'}`, 'poi-popup-credit')
  }

  add('strong', 'poi-popup-title', title(p))
  const cuisine = (p.cuisine ?? []).map((c) => cuisineLabels[c]).filter(Boolean)
  const sub = [kindLabels[p.kind], p.brand && p.brand !== title(p) ? p.brand : undefined, ...cuisine.slice(0, 2)]
  if (sub.some(Boolean)) add('div', 'poi-popup-sub', sub.filter(Boolean).join(' · '))

  if (p.description) add('p', 'poi-popup-desc', p.description)
  else if (loading) add('p', 'poi-popup-loading', 'Загружаем описание…')

  if (p.category === 'attraction' && p.fee) add('div', 'poi-popup-meta', fee(p.fee))
  if (p.opening_hours) add('div', 'poi-popup-meta', p.opening_hours === '24/7' ? 'Круглосуточно' : p.opening_hours)
  const phone = p.phone?.split(';')[0]?.trim()
  if (phone && /^[+\d\s()-]{5,}$/.test(phone)) {
    const meta = add('div', 'poi-popup-meta', '')
    const a = document.createElement('a')
    a.href = `tel:${phone.replace(/[^+\d]/g, '')}`
    a.textContent = phone
    meta.append(a)
  }

  const links = document.createElement('div')
  links.className = 'poi-popup-links'
  const wiki = wikipediaUrl(p.wikipedia)
  if (wiki) link(links, wiki, 'Википедия')
  if (p.website && /^https?:\/\//i.test(p.website)) link(links, p.website, 'Сайт')
  link(links, `https://www.openstreetmap.org/${p.id}`, 'OpenStreetMap')
  root.append(links)
  return root
}

/** «ru:Самарская площадь» → ссылка на статью. */
function wikipediaUrl(tag?: string) {
  const m = tag?.match(/^([a-z-]{2,12}):(.+)$/)
  if (!m) return undefined
  return `https://${m[1]}.wikipedia.org/wiki/${encodeURIComponent(m[2]!.replaceAll(' ', '_'))}`
}
