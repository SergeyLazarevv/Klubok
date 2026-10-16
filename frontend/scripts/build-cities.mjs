// Собирает города для «Карты 2»: города России, крупные города соседних стран, куда ездят на машине,
// и столицы мира — с гербом и главной достопримечательностью.
//   public/data/cities.geojson — точки городов (≈250 КБ, грузится один раз)
//   public/data/arms/<QID>.webp — гербы 96 px (по 2–6 КБ) — для списков и карточек
//   public/data/stickers/<QID>.webp — они же наклейками для карты (scripts/build-stickers.mjs)
// Источник — Wikidata и Wikimedia Commons; ручные правки — scripts/cities-curated.mjs.
// Скачанные гербы кэшируются в .cache/arms, повторный запуск качает только новые.
// Запуск: node scripts/build-cities.mjs
import { mkdir, readdir, rm, stat, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import sharp from 'sharp'
import { buildStickers } from './build-stickers.mjs'
import { capitals, extraRussia, preferredCapital, russia } from './cities-curated.mjs'

const UA = 'klubok-map/0.1 (https://github.com/SergeyLazarevv)'
const OUT = new URL('../public/data/', import.meta.url)
const ARMS = new URL('arms/', OUT)
const CACHE = new URL('../.cache/arms/', import.meta.url)
const ARMS_PX = 96

async function sparql(query) {
  const url = `https://query.wikidata.org/sparql?query=${encodeURIComponent(query)}`
  const res = await fetch(url, { headers: { Accept: 'application/sparql-results+json', 'User-Agent': UA } })
  if (!res.ok) throw new Error(`Wikidata ${res.status}: ${await res.text()}`)
  return (await res.json()).results.bindings.map((b) =>
    Object.fromEntries(Object.entries(b).map(([k, v]) => [k, v.value])),
  )
}

const qid = (uri) => uri.slice(uri.lastIndexOf('/') + 1)
const point = (wkt) => wkt.match(/Point\(([-\d.]+) ([-\d.]+)\)/).slice(1, 3).map((n) => +(+n).toFixed(4))
/** Имя файла Commons из ссылки Special:FilePath. */
const commonsFile = (url) => (url ? decodeURIComponent(url.slice(url.lastIndexOf('/') + 1)) : undefined)

// Города России: в Wikidata они размечены по-разному — «город России» (два разных элемента),
// «большой город» или просто «город»
const RU_CITY = `
  VALUES ?type { wd:Q7930989 wd:Q106389302 wd:Q1549591 wd:Q515 }
  ?c wdt:P31 ?type ; wdt:P17 wd:Q159 .
  FILTER NOT EXISTS { ?c wdt:P576 [] }`

const cityRows = await sparql(`
SELECT ?c ?name ?pop ?coord ?arms ?photo WHERE {
  ${RU_CITY}
  ?c wdt:P625 ?coord .
  OPTIONAL { ?c wdt:P1082 ?pop }
  OPTIONAL { ?c wdt:P94 ?arms }
  OPTIONAL { ?c wdt:P18 ?photo }
  ?c rdfs:label ?name FILTER(lang(?name) = "ru")
}`)

// Самые известные (по числу статей в Википедиях) храмы, кремли, монументы и мосты в каждом городе
const landmarkRows = await sparql(`
SELECT ?c ?lname (MAX(?links) AS ?n) WHERE {
  VALUES ?cls { wd:Q16970 wd:Q2977 wd:Q44613 wd:Q57821 wd:Q57831 wd:Q4989906 wd:Q179700
                wd:Q860861 wd:Q12280 wd:Q32815 wd:Q39715 wd:Q23413 }
  ${RU_CITY}
  ?l wdt:P131 ?c ; wikibase:sitelinks ?links ; wdt:P31/wdt:P279? ?cls .
  FILTER(?links >= 3)
  ?l rdfs:label ?lname FILTER(lang(?lname) = "ru")
} GROUP BY ?c ?lname`)

// Соседние страны, куда из России ездят на своей машине: города от 100 тыс. жителей (Уральск, Гродно, Батуми).
// Тип в Wikidata у них разный, поэтому берём любые поселения и отсекаем районы и агломерации по названию
const NEIGHBOURS = 'wd:Q232 wd:Q184 wd:Q230 wd:Q399 wd:Q227 wd:Q265 wd:Q813 wd:Q711' // KZ BY GE AM AZ UZ KG MN
const NOT_CITY = /(район|агломерац|муниципалитет|микрорайон|посёлок|поселок|область)/i
const NOT_CITY_IDS = new Set(['Малиновка', 'Кярки']) // части Минска и Баку
const neighbourRows = await sparql(`
SELECT ?c ?name ?pop ?coord ?arms ?photo ?cname ?cc WHERE {
  VALUES ?country { ${NEIGHBOURS} }
  ?c wdt:P17 ?country ; wdt:P1082 ?pop ; wdt:P625 ?coord .
  FILTER(?pop >= 100000)
  ?c wdt:P31/wdt:P279* wd:Q486972 .
  FILTER NOT EXISTS { ?c wdt:P576 [] }
  OPTIONAL { ?c wdt:P94 ?arms }
  OPTIONAL { ?c wdt:P18 ?photo }
  ?country wdt:P297 ?cc .
  ?c rdfs:label ?name FILTER(lang(?name) = "ru")
  ?country rdfs:label ?cname FILTER(lang(?cname) = "ru")
}`)

// Столицы: герб самой столицы, а если его нет — герб страны
const capitalRows = await sparql(`
SELECT ?cap ?cname ?cc ?name ?coord ?pop ?arms ?countryArms ?photo WHERE {
  ?country wdt:P31 wd:Q3624078 ; wdt:P36 ?cap .
  FILTER NOT EXISTS { ?country wdt:P576 [] }
  ?cap wdt:P625 ?coord .
  OPTIONAL { ?cap wdt:P1082 ?pop }
  OPTIONAL { ?cap wdt:P94 ?arms }
  OPTIONAL { ?country wdt:P94 ?countryArms }
  OPTIONAL { ?cap wdt:P18 ?photo }
  OPTIONAL { ?country wdt:P297 ?cc }
  ?cap rdfs:label ?name FILTER(lang(?name) = "ru")
  ?country rdfs:label ?cname FILTER(lang(?cname) = "ru")
}`)

// У города в Wikidata бывает несколько значений населения, координат и гербов — берём наибольшее население
// и первые попавшиеся герб и фото
const cities = new Map()
for (const r of cityRows) {
  // У части городов в Wikidata подпись муниципалитета: «городской округ Моршанск», «городское поселение Белебей»
  const name = r.name.replace(/^(городской округ|городское поселение) /, '')
  const c = cities.get(r.c) ?? { id: qid(r.c), name, pop: 0, coord: point(r.coord) }
  c.pop = Math.max(c.pop, Math.round(+(r.pop ?? 0)))
  c.arms ??= commonsFile(r.arms)
  c.photo ??= commonsFile(r.photo)
  cities.set(r.c, c)
}

const best = new Map()
for (const r of landmarkRows) {
  if ((best.get(r.c)?.n ?? -1) < +r.n) best.set(r.c, { name: r.lname, n: +r.n })
}

// Ручные правки по имени достаются самому крупному городу с этим именем
const largestByName = new Map()
for (const [uri, c] of cities) {
  const prev = largestByName.get(c.name)
  if (!prev || cities.get(prev).pop < c.pop) largestByName.set(c.name, uri)
}

const places = []
for (const [uri, c] of cities) {
  const curated = largestByName.get(c.name) === uri ? russia[c.name] : undefined
  places.push({ ...c, landmark: curated ?? best.get(uri)?.name, capital: c.name === 'Москва' })
}
for (const [name, [lon, lat, pop]] of Object.entries(extraRussia)) {
  if (!largestByName.has(name)) places.push({ id: `ru-${name}`, name, pop, coord: [lon, lat], landmark: russia[name] })
}

// Города соседних стран; столицы среди них добавятся ниже как столицы
const neighbours = new Map()
for (const r of neighbourRows) {
  if (NOT_CITY.test(r.name) || NOT_CITY_IDS.has(r.name)) continue
  const c = neighbours.get(r.c) ?? { id: qid(r.c), name: r.name, country: r.cname, cc: r.cc, pop: 0, coord: point(r.coord) }
  c.pop = Math.max(c.pop, Math.round(+r.pop))
  c.arms ??= commonsFile(r.arms)
  c.photo ??= commonsFile(r.photo)
  neighbours.set(r.c, c)
}

const seenCountries = new Set()
for (const r of capitalRows) {
  if (r.cname === 'Россия' || seenCountries.has(r.cname)) continue // Москва уже есть среди городов России
  const preferred = preferredCapital[r.cname]
  if (preferred && r.name !== preferred) continue
  seenCountries.add(r.cname)
  places.push({
    id: qid(r.cap),
    name: r.name,
    country: r.cname,
    cc: r.cc,
    pop: Math.round(+(r.pop ?? 0)),
    coord: point(r.coord),
    arms: commonsFile(r.arms ?? r.countryArms),
    photo: commonsFile(r.photo),
    landmark: capitals[r.cname],
    capital: true,
  })
}

const capitalIds = new Set(places.filter((p) => p.capital).map((p) => p.id))
for (const c of neighbours.values()) if (!capitalIds.has(c.id)) places.push(c)

// --- Гербы: скачиваем превью с Commons (120 px — стандартный размер миниатюр), обрезаем поля, сжимаем в WebP ---
await mkdir(CACHE, { recursive: true })
// Папку не пересоздаём: запущенный Vite теряет её из виду и начинает отдавать вместо гербов index.html
await mkdir(ARMS, { recursive: true })

const exists = (url) => stat(url).then(() => true, () => false)

async function download(file, to) {
  const url = `https://commons.wikimedia.org/wiki/Special:FilePath/${encodeURIComponent(file)}?width=120`
  for (let attempt = 0; attempt < 5; attempt++) {
    const res = await fetch(url, { headers: { 'User-Agent': UA } })
    if (res.ok) return writeFile(to, Buffer.from(await res.arrayBuffer()))
    if (res.status !== 429 && res.status < 500) throw new Error(`${res.status} ${file}`)
    await new Promise((r) => setTimeout(r, 2000 * 2 ** attempt)) // Commons просит не спешить
  }
  throw new Error(`не скачался ${file}`)
}

async function processArms(p) {
  const cached = new URL(`${p.id}.img`, CACHE)
  if (!(await exists(cached))) await download(p.arms, cached)
  await sharp(fileURLToPath(cached))
    .trim()
    .resize(ARMS_PX, ARMS_PX, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .webp({ quality: 82, alphaQuality: 90 })
    .toFile(fileURLToPath(new URL(`${p.id}.webp`, ARMS)))
}

const queue = places.filter((p) => p.arms)
let failed = 0
async function worker() {
  for (let p = queue.shift(); p; p = queue.shift()) {
    try {
      await processArms(p)
    } catch (e) {
      failed++
      p.arms = undefined
      console.warn(`герб ${p.name}: ${e.message}`)
    }
  }
}
await Promise.all(Array.from({ length: 4 }, worker))

const features = places.map((p) => ({
  type: 'Feature',
  geometry: { type: 'Point', coordinates: p.coord },
  properties: {
    id: p.id,
    name: p.name,
    pop: p.pop,
    capital: !!p.capital,
    arms: !!p.arms,
    ...(p.country && { country: p.country }),
    ...(p.cc && { cc: p.cc }),
    ...(p.landmark && { landmark: p.landmark }),
    ...(p.photo && { photo: p.photo }),
  },
}))
await writeFile(new URL('cities.geojson', OUT), JSON.stringify({ type: 'FeatureCollection', features }))

// Гербы городов, которых больше нет в выборке, удаляем по одному
const wanted = new Set(places.filter((p) => p.arms).map((p) => `${p.id}.webp`))
for (const f of await readdir(ARMS)) if (!wanted.has(f)) await rm(new URL(f, ARMS))

const files = await readdir(ARMS)
let bytes = 0
for (const f of files) bytes += (await stat(new URL(f, ARMS))).size
console.log(
  `${features.length} мест (${seenCountries.size} столиц), гербов ${files.length} — ${(bytes / 1024 / 1024).toFixed(1)} МБ` +
    (failed ? `, не скачалось ${failed}` : ''),
)

await buildStickers()
