// Гербы-наклейки для карты: белая кайма по контуру герба и мягкая тень снизу.
//   public/data/arms/<QID>.webp     — исходные гербы 96 px (их собирает build-cities.mjs)
//   public/data/stickers/<QID>.webp — готовые наклейки, карта кладёт их на карту без обработки
//   public/data/stickers/city-dot.webp — кружок для города без герба
// Раньше наклейка рисовалась в браузере (16 копий силуэта с тенью на каждый герб) и съедала секунды главного
// потока при загрузке карты. Теперь тот же код canvas выполняется один раз при сборке — в Chromium, чтобы
// тень и кайма вышли в точности как в браузере (у Skia в Node тень заметно слабее).
// Перерисовываются только новые и изменившиеся гербы. Запуск отдельно: node scripts/build-stickers.mjs
// Chromium — системный; другой путь можно задать в CHROMIUM_PATH.
import { existsSync } from 'node:fs'
import { mkdir, readFile, readdir, rm, stat } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import sharp from 'sharp'

const DATA = new URL('../public/data/', import.meta.url)
const ARMS = new URL('arms/', DATA)
const STICKERS = new URL('stickers/', DATA)
const BATCH = 50

const CHROMIUM = [
  process.env.CHROMIUM_PATH,
  '/snap/bin/chromium',
  '/usr/bin/chromium',
  '/usr/bin/chromium-browser',
  '/usr/bin/google-chrome',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
].find((p) => p && existsSync(p))

/** Выполняется в браузере: герб (data URL) → наклейка (PNG data URL, без потерь). */
async function renderStickers(sources) {
  const STROKE = 3 // белая кайма, в пикселях исходника
  const out = []
  for (const src of sources) {
    const img = new Image()
    img.src = src
    await img.decode()
    const pad = STROKE + 3
    const canvas = document.createElement('canvas')
    canvas.width = img.width + pad * 2
    canvas.height = img.height + pad * 2
    const ctx = canvas.getContext('2d')

    // Силуэт герба белым — из него, сдвигая по кругу, собираем кайму
    const silhouette = document.createElement('canvas')
    silhouette.width = img.width
    silhouette.height = img.height
    const sctx = silhouette.getContext('2d')
    sctx.drawImage(img, 0, 0)
    sctx.globalCompositeOperation = 'source-in'
    sctx.fillStyle = '#fff'
    sctx.fillRect(0, 0, img.width, img.height)

    ctx.shadowColor = 'rgba(90, 70, 60, 0.35)'
    ctx.shadowBlur = 4
    ctx.shadowOffsetY = 1.5
    for (let a = 0; a < 16; a++) {
      const t = (a / 16) * Math.PI * 2
      ctx.drawImage(silhouette, pad + Math.cos(t) * STROKE, pad + Math.sin(t) * STROKE)
    }
    ctx.shadowColor = 'transparent'
    ctx.drawImage(img, pad, pad)
    out.push(canvas.toDataURL('image/png'))
  }
  return out
}

const DOT = 'city-dot.webp'

/** Выполняется в браузере: город без герба — пастельный кружок с белой каймой. */
function renderDot() {
  const size = 28
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = size
  const ctx = canvas.getContext('2d')
  ctx.beginPath()
  ctx.arc(size / 2, size / 2, 9, 0, Math.PI * 2)
  ctx.fillStyle = '#c9b3e0'
  ctx.shadowColor = 'rgba(90, 70, 60, 0.35)'
  ctx.shadowBlur = 3
  ctx.fill()
  ctx.shadowColor = 'transparent'
  ctx.lineWidth = 4
  ctx.strokeStyle = '#fff'
  ctx.stroke()
  return canvas.toDataURL('image/png')
}

const toWebp = (png, to) =>
  sharp(Buffer.from(png.slice(png.indexOf(',') + 1), 'base64'))
    .webp({ quality: 85, alphaQuality: 90 })
    .toFile(fileURLToPath(to))

const mtime = (url) => stat(url).then((s) => s.mtimeMs, () => 0)

export async function buildStickers() {
  await mkdir(STICKERS, { recursive: true })
  const arms = (await readdir(ARMS)).filter((f) => f.endsWith('.webp'))
  const todo = []
  const needDot = !(await mtime(new URL(DOT, STICKERS)))
  for (const f of arms) if ((await mtime(new URL(f, STICKERS))) < (await mtime(new URL(f, ARMS)))) todo.push(f)

  if (todo.length || needDot) {
    if (!CHROMIUM) throw new Error('не найден Chromium для наклеек гербов — укажите путь в CHROMIUM_PATH')
    const browser = await chromium.launch({ executablePath: CHROMIUM })
    try {
      const page = await browser.newPage()
      if (needDot) await toWebp(await page.evaluate(renderDot), new URL(DOT, STICKERS))
      for (let i = 0; i < todo.length; i += BATCH) {
        const batch = todo.slice(i, i + BATCH)
        const sources = await Promise.all(
          batch.map(async (f) => `data:image/webp;base64,${(await readFile(new URL(f, ARMS))).toString('base64')}`),
        )
        const pngs = await page.evaluate(renderStickers, sources)
        await Promise.all(pngs.map((png, j) => toWebp(png, new URL(batch[j], STICKERS))))
      }
    } finally {
      await browser.close()
    }
  }
  // Наклейки гербов, которых больше нет
  const wanted = new Set([...arms, DOT])
  for (const f of await readdir(STICKERS)) if (!wanted.has(f)) await rm(new URL(f, STICKERS))

  let bytes = 0
  for (const f of arms) bytes += (await stat(new URL(f, STICKERS))).size
  console.log(`наклеек ${arms.length} (новых ${todo.length}) — ${(bytes / 1024 / 1024).toFixed(1)} МБ`)
}

if (import.meta.url === `file://${process.argv[1]}`) await buildStickers()
