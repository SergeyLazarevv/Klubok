import * as maplibregl from 'maplibre-gl'
import type { CameraOptions, StyleSpecification } from 'maplibre-gl'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import 'maplibre-gl/dist/maplibre-gl.css'
import { provideImages } from './patterns'

// MapLibre ищет воркер рядом со своим файлом, а Vite кладёт бандл в другое место — указываем путь явно
maplibregl.setWorkerUrl(workerUrl)

export { maplibregl }
export type MapLibreMap = maplibregl.Map

/** Полноэкранная карта страницы «Карта»: стиль, начальный вид, узоры и стандартные кнопки. */
export function createStyledMap(container: HTMLElement, style: StyleSpecification, camera: CameraOptions) {
  const map = new maplibregl.Map({
    container,
    style,
    ...camera,
    center: camera.center ?? [0, 0],
    maxPitch: 75,
    hash: true, // позиция карты в адресе: удобно делиться видом и не терять его при перезагрузке
  })
  provideImages(map)
  map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'top-right')
  map.addControl(new maplibregl.ScaleControl({ unit: 'metric' }), 'bottom-left')
  return map
}
