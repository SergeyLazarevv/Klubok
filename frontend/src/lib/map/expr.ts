import type { ExpressionSpecification } from 'maplibre-gl'

/**
 * Общее для стилей карты: источник тайлов и выражения поверх схемы OpenMapTiles.
 *
 * Тайлы пока берём у OpenFreeMap (без ключа); свой PMTiles-файл подключается заменой VITE_MAP_TILES.
 */
export const TILES = import.meta.env.VITE_MAP_TILES ?? 'https://tiles.openfreemap.org/planet'

export const CYRILLIC = ['U+0301', 'U+0400-045F', 'U+0490-0491', 'U+04B0-04B1', 'U+2116']
export const LATIN = ['U+0000-00FF', 'U+0131', 'U+0152-0153', 'U+2000-206F', 'U+20AC', 'U+2212']

export const OMT_ATTRIBUTION =
  '<a href="https://openfreemap.org">OpenFreeMap</a> © <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'

export const GLYPHS = 'https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf'

export const name: ExpressionSpecification = ['coalesce', ['get', 'name:ru'], ['get', 'name']]

/** Ширина по зуму: [зум, ширина] — растёт экспоненциально, как и сами улицы на экране. */
export const zoomed = (...stops: [number, number | ExpressionSpecification][]): ExpressionSpecification =>
  ['interpolate', ['exponential', 1.6], ['zoom'], ...stops.flat()] as ExpressionSpecification

/** Во сколько раз дорога толще второстепенной улицы. */
export const roadK: ExpressionSpecification = ['match', ['get', 'class'],
  'motorway', 1.6, 'trunk', 1.5, 'primary', 1.3, 'secondary', 1.1, 'tertiary', 1, 0.7]

/** Ширина дороги по классу; casing — сколько добавить на обводку. На крупных зумах дороги «пухлые». */
export const roadWidth = (casing = 0): ExpressionSpecification =>
  zoomed(
    // Издалека — только трассы, тонкой линией: видно, как связаны города
    [5, ['match', ['get', 'class'], ['motorway', 'trunk'], 0.7 + casing * 0.8, 0]],
    [8, ['match', ['get', 'class'], ['motorway', 'trunk'], 1.4 + casing, 'primary', 0.8 + casing * 0.6, 0]],
    [10, ['+', ['*', roadK, 0.8], casing * 0.6]],
    [14, ['+', ['*', roadK, 3], casing * 2]],
    [18, ['+', ['*', roadK, 14], casing * 5]],
  )

/** Размер подписи по типу места: город, посёлок, район, микрорайон. */
export const placeSize = (city: number, town: number, suburb: number, other: number): ExpressionSpecification =>
  ['match', ['get', 'class'], 'city', city, 'town', town, ['suburb', 'quarter'], suburb, other]

/** Что показываем кружками уже с 14-го зума; кафе и магазины — только вблизи. */
export const sights: ExpressionSpecification = ['match', ['get', 'class'],
  ['museum', 'attraction', 'monument', 'castle', 'art_gallery', 'theatre', 'zoo', 'place_of_worship'], true, false]

export const roads = ['motorway', 'trunk', 'primary', 'secondary', 'tertiary', 'minor', 'service']
