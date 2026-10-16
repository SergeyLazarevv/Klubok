import type { Map as MapLibreMap } from 'maplibre-gl'
import { addCityImage } from './cities'

/**
 * Узоры и значки для мультяшного стиля рисуются на canvas прямо в браузере:
 * без спрайт-файла их проще крутить, пока подбираем дизайн.
 */
const RATIO = 2
const INK = '#5b3a29'

type Draw = (ctx: CanvasRenderingContext2D) => void

const images: Record<string, { w: number; h: number; draw: Draw }> = {
  // Белые барашки на воде
  waves: {
    w: 120,
    h: 72,
    draw(ctx) {
      ctx.strokeStyle = 'rgba(255,255,255,0.75)'
      ctx.lineWidth = 2.2
      ctx.lineCap = 'round'
      const wave = (x: number, y: number) => {
        ctx.beginPath()
        ctx.moveTo(x, y)
        ctx.quadraticCurveTo(x + 5, y - 6, x + 10, y)
        ctx.quadraticCurveTo(x + 15, y - 6, x + 20, y)
        ctx.stroke()
      }
      wave(10, 16)
      wave(76, 54)
    },
  },
  // Лес вперемешку: ёлочки и лиственные деревца со стволами, чтобы читались как деревья, а не кружки
  trees: {
    w: 48,
    h: 48,
    draw(ctx) {
      ctx.lineWidth = 1.2
      ctx.lineJoin = 'round'
      ctx.strokeStyle = '#24501f'
      const trunk = (x: number, y: number) => {
        ctx.fillStyle = '#8a5a3c'
        ctx.fillRect(x - 1, y, 2, 3.5)
      }
      // Ёлка: два яруса-треугольника
      const fir = (x: number, y: number) => {
        trunk(x, y)
        for (const [top, half, base] of [[y - 13, 4, y - 5], [y - 9, 5.5, y]]) {
          ctx.beginPath()
          ctx.moveTo(x, top!)
          ctx.lineTo(x + half!, base!)
          ctx.lineTo(x - half!, base!)
          ctx.closePath()
          ctx.fillStyle = '#2f8a3a'
          ctx.fill()
          ctx.stroke()
        }
      }
      // Лиственное: пышная крона из трёх кружков и блик
      const leafy = (x: number, y: number) => {
        trunk(x, y - 1)
        // Сначала крона чуть крупнее тёмным цветом — получается обводка без швов между кружками
        for (const [grow, color] of [[1.2, '#24501f'], [0, '#4caa45']] as const) {
          ctx.beginPath()
          ctx.arc(x - 2.5, y - 5, 3.5 + grow, 0, Math.PI * 2)
          ctx.arc(x + 2.5, y - 5, 3.5 + grow, 0, Math.PI * 2)
          ctx.arc(x, y - 8.5, 4 + grow, 0, Math.PI * 2)
          ctx.fillStyle = color
          ctx.fill()
        }
        ctx.beginPath()
        ctx.arc(x - 1.5, y - 9.5, 1.4, 0, Math.PI * 2)
        ctx.fillStyle = '#a6e07f'
        ctx.fill()
      }
      fir(10, 20)
      leafy(32, 16)
      leafy(14, 44)
      fir(38, 42)
    },
  },
  // Травяные пучки для парков и лугов
  grass: {
    w: 40,
    h: 40,
    draw(ctx) {
      ctx.strokeStyle = '#4f9d3a'
      ctx.lineWidth = 1.6
      ctx.lineCap = 'round'
      const tuft = (x: number, y: number) => {
        ctx.beginPath()
        ctx.moveTo(x - 4, y - 5)
        ctx.lineTo(x - 1, y)
        ctx.moveTo(x, y - 7)
        ctx.lineTo(x, y)
        ctx.moveTo(x + 4, y - 5)
        ctx.lineTo(x + 1, y)
        ctx.stroke()
      }
      tuft(10, 14)
      tuft(30, 34)
    },
  },
  // Песчаные крапинки
  sand: {
    w: 32,
    h: 32,
    draw(ctx) {
      ctx.fillStyle = '#d9a94f'
      for (const [x, y] of [[5, 6], [20, 4], [12, 18], [27, 22], [6, 27]]) {
        ctx.beginPath()
        ctx.arc(x!, y!, 1.3, 0, Math.PI * 2)
        ctx.fill()
      }
    },
  },
}

function render(name: string) {
  const img = images[name]
  if (!img) return undefined
  const canvas = document.createElement('canvas')
  canvas.width = img.w * RATIO
  canvas.height = img.h * RATIO
  const ctx = canvas.getContext('2d')!
  ctx.scale(RATIO, RATIO)
  img.draw(ctx)
  return ctx.getImageData(0, 0, canvas.width, canvas.height)
}

/** Подкладывает узоры и гербы по запросу стиля: картинка рисуется, когда слой её впервые попросит. */
export function provideImages(map: MapLibreMap) {
  // Картинку просит каждый тайл, где она есть, — пока первая ещё рисуется, отдаём тот же запрос, а не рисуем заново
  const pending = new Map<string, Promise<void>>()
  map.setMissingStyleImageResolver((id) => {
    let job = pending.get(id)
    if (!job) {
      job = (async () => {
        if (await addCityImage(map, id)) return
        const data = render(id)
        if (data && !map.hasImage(id)) map.addImage(id, data, { pixelRatio: RATIO })
      })()
      pending.set(id, job)
    }
    return job
  })
}

export { INK }
