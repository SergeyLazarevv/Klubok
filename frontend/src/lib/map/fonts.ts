import interCyr from '@fontsource-variable/inter/files/inter-cyrillic-wght-normal.woff2?url'
import interLat from '@fontsource-variable/inter/files/inter-latin-wght-normal.woff2?url'
import interLatExt from '@fontsource-variable/inter/files/inter-latin-ext-wght-normal.woff2?url'
import interCyrItalic from '@fontsource-variable/inter/files/inter-cyrillic-wght-italic.woff2?url'
import interLatItalic from '@fontsource-variable/inter/files/inter-latin-wght-italic.woff2?url'
import type { StyleSpecification } from 'maplibre-gl'
import { CYRILLIC, LATIN } from './expr'

/**
 * Шрифты карты — вариативный Inter, как в приложении: спокойный и хорошо читается мелко.
 * Вес MapLibre берёт из названия; ₽ и ≈ — в наборе latin-ext. Вода подписана курсивом, как на картах.
 */
export const FONT = 'Inter Medium'
export const FONT_BOLD = 'Inter SemiBold'
export const FONT_WATER = 'Inter Medium Italic'

const upright = [
  { url: interCyr, 'unicode-range': CYRILLIC },
  { url: interLat, 'unicode-range': LATIN },
  { url: interLatExt, 'unicode-range': ['U+20BD', 'U+2248'] },
]
const italic = [
  { url: interCyrItalic, 'unicode-range': CYRILLIC },
  { url: interLatItalic, 'unicode-range': LATIN },
]

export const FONT_FACES: StyleSpecification['font-faces'] = {
  [FONT]: upright,
  [FONT_BOLD]: upright,
  [FONT_WATER]: italic,
}
