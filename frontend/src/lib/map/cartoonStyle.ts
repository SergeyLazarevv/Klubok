import type { SkySpecification, StyleSpecification } from 'maplibre-gl'
import { FONT, FONT_BOLD, FONT_FACES, FONT_WATER } from './fonts'
import { INK } from './patterns'
import { GLYPHS, OMT_ATTRIBUTION, TILES, name, placeSize, roadWidth, roads, sights, zoomed } from './expr'

/**
 * «Мультяшный» стиль в духе диснеевских карт: толстая обводка, жёлтые дороги, волны на воде,
 * деревца в лесах и пастельные 3D-домики. Цвета задаёт тема: сочная (страница «Карта»)
 * или мягкая пастельная («Карта 2»).
 */
export interface CartoonTheme {
  name: string
  ink: string // обводка дорог и рельсы
  paper: string
  residential: string
  industrial: string
  water: string
  waterDeep: string
  foam: string
  grass: string
  park: string
  parkEdge: string
  wood: string
  farmland: string
  sand: string
  motorway: string
  major: string
  tertiary: string
  minor: string
  title: string // подписи городов
  subtitle: string // подписи районов
  boundary: string
  buildings: [string, string, string, string, string] // по высоте: до 6 м, 12, 20, 40 и выше
  poi: [string, string, string, string, string, string, string] // музеи, театры, еда, парки, храмы, транспорт, прочее
  sky: SkySpecification
}

export const CARTOON: CartoonTheme = {
  name: 'Клубок — мультик',
  ink: INK,
  paper: '#fcefd2',
  residential: '#fbe1c2',
  industrial: '#ead9f5',
  water: '#45c0ec',
  waterDeep: '#1d6fa3',
  foam: '#c9f3ff',
  grass: '#b5e28a',
  park: '#98d86b',
  parkEdge: '#3f8a33',
  wood: '#6fbf5a',
  farmland: '#f6e7a1',
  sand: '#ffe3a0',
  motorway: '#ff9e3d',
  major: '#ffd34d',
  tertiary: '#ffe999',
  minor: '#fff8e6',
  title: '#6b2d8f',
  subtitle: '#a0466b',
  boundary: '#b26bd6',
  buildings: ['#ffc2d1', '#ffe08a', '#b9e4c9', '#c9b8f0', '#9fd0ff'],
  poi: ['#ff5d8f', '#b26bd6', '#ff9e3d', '#4fb34a', '#ffd34d', '#3a8fd9', '#e8a27c'],
  sky: {
    'sky-color': '#6fc7ff',
    'horizon-color': '#ffd6ec',
    'fog-color': '#fde7c8',
    'sky-horizon-blend': 0.6,
    'horizon-fog-blend': 0.5,
    'fog-ground-blend': 0.8,
  },
}

/** Та же мультяшность, но приглушённая: светлая вода, молочная земля, мягкая коричневая обводка. */
export const PASTEL: CartoonTheme = {
  name: 'Клубок — пастель',
  ink: '#b0907a',
  paper: '#fdf6e9',
  residential: '#fbeadb',
  industrial: '#f1e8f7',
  water: '#8ad6f2',
  waterDeep: '#5aaad3',
  foam: '#e6f8ff',
  grass: '#d6efbd',
  park: '#c4e6a6',
  parkEdge: '#8cc276',
  wood: '#b2dd9a',
  farmland: '#f9f0cc',
  sand: '#fdf0cc',
  motorway: '#ffcf9f',
  major: '#ffe7a8',
  tertiary: '#fff2cc',
  minor: '#ffffff',
  title: '#6f5a8e',
  subtitle: '#a0809a',
  boundary: '#cfb2dd',
  buildings: ['#ffd9e2', '#fff0bf', '#d4f0dc', '#e1d6f7', '#cfe6ff'],
  poi: ['#f7a8c0', '#cdb0e6', '#ffc79a', '#9fd48f', '#ffe08f', '#9cc8ee', '#ecc4ae'],
  sky: {
    'sky-color': '#bfe6ff',
    'horizon-color': '#fff0f5',
    'fog-color': '#fdf6e9',
    'sky-horizon-blend': 0.7,
    'horizon-fog-blend': 0.6,
    'fog-ground-blend': 0.8,
    'atmosphere-blend': ['interpolate', ['linear'], ['zoom'], 0, 0.8, 5, 0.8, 7, 0],
  },
}

export function cartoonStyle(C: CartoonTheme = CARTOON): StyleSpecification {
  return {
    version: 8,
    name: C.name,
    glyphs: GLYPHS,
    'font-faces': FONT_FACES,
    sources: {
      omt: {
        type: 'vector',
        url: TILES,
        attribution: OMT_ATTRIBUTION,
      },
    },
    projection: { type: 'globe' },
    sky: C.sky,
    light: { anchor: 'viewport', color: '#fff1d0', intensity: 0.35, position: [1.2, 210, 35] },
    layers: [
      { id: 'paper', type: 'background', paint: { 'background-color': C.paper } },

      // --- Земля ---
      {
        id: 'landcover',
        type: 'fill',
        source: 'omt',
        'source-layer': 'landcover',
        paint: {
          'fill-color': ['match', ['get', 'class'],
            'wood', C.wood, 'grass', C.grass, 'farmland', C.farmland, 'sand', C.sand,
            'wetland', '#9fd8c0', 'ice', '#ffffff', 'rock', '#e3d3c0', C.grass],
          'fill-opacity': 0.9,
        },
      },
      {
        id: 'landcover-wood-trees',
        type: 'fill',
        source: 'omt',
        'source-layer': 'landcover',
        minzoom: 12,
        filter: ['==', ['get', 'class'], 'wood'],
        paint: { 'fill-pattern': 'trees' },
      },
      {
        id: 'landcover-sand-dots',
        type: 'fill',
        source: 'omt',
        'source-layer': 'landcover',
        minzoom: 11,
        filter: ['==', ['get', 'class'], 'sand'],
        paint: { 'fill-pattern': 'sand' },
      },
      {
        id: 'landuse',
        type: 'fill',
        source: 'omt',
        'source-layer': 'landuse',
        paint: {
          'fill-color': ['match', ['get', 'class'],
            ['residential', 'suburb', 'neighbourhood'], C.residential,
            ['industrial', 'commercial', 'retail', 'railway'], C.industrial,
            'cemetery', '#c9e6b0',
            'hospital', '#ffd6de',
            ['school', 'university', 'college', 'kindergarten'], '#fff0b3',
            ['stadium', 'pitch', 'playground'], '#9bdc6e',
            C.residential],
          'fill-opacity': ['interpolate', ['linear'], ['zoom'], 9, 0.5, 13, 0.9],
        },
      },
      {
        id: 'park',
        type: 'fill',
        source: 'omt',
        'source-layer': 'park',
        paint: { 'fill-color': C.park },
      },
      {
        id: 'park-grass',
        type: 'fill',
        source: 'omt',
        'source-layer': 'park',
        minzoom: 12,
        paint: { 'fill-pattern': 'grass' },
      },
      {
        id: 'park-edge',
        type: 'line',
        source: 'omt',
        'source-layer': 'park',
        minzoom: 11,
        paint: { 'line-color': C.parkEdge, 'line-width': zoomed([11, 0.8], [16, 2.5]), 'line-dasharray': [3, 2] },
      },

      // --- Вода: тёмная «чернильная» кромка, светлая пена у берега и барашки ---
      {
        id: 'water',
        type: 'fill',
        source: 'omt',
        'source-layer': 'water',
        filter: ['!=', ['get', 'brunnel'], 'tunnel'],
        paint: { 'fill-color': C.water },
      },
      {
        id: 'water-foam',
        type: 'line',
        source: 'omt',
        'source-layer': 'water',
        minzoom: 10,
        paint: { 'line-color': C.foam, 'line-width': zoomed([10, 2], [16, 10]), 'line-blur': 2, 'line-opacity': 0.9 },
      },
      {
        id: 'water-waves',
        type: 'fill',
        source: 'omt',
        'source-layer': 'water',
        minzoom: 10,
        paint: { 'fill-pattern': 'waves' },
      },
      {
        id: 'water-edge',
        type: 'line',
        source: 'omt',
        'source-layer': 'water',
        paint: { 'line-color': C.waterDeep, 'line-width': zoomed([8, 0.8], [16, 3]) },
      },
      {
        id: 'waterway',
        type: 'line',
        source: 'omt',
        'source-layer': 'waterway',
        filter: ['!=', ['get', 'brunnel'], 'tunnel'],
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: {
          'line-color': C.water,
          'line-width': zoomed(
            [9, ['match', ['get', 'class'], 'river', 1.5, 0]],
            [12, ['match', ['get', 'class'], 'river', 3, 0.8]],
            [16, ['match', ['get', 'class'], 'river', 10, 4]],
          ),
        },
      },

      // --- Аэропорты ---
      {
        id: 'aeroway',
        type: 'line',
        source: 'omt',
        'source-layer': 'aeroway',
        minzoom: 11,
        filter: ['match', ['get', 'class'], ['runway', 'taxiway'], true, false],
        paint: { 'line-color': '#c7b7e8', 'line-width': zoomed([11, 2], [16, 30]) },
      },

      // --- Дороги: коричневая обводка снизу, цветная заливка сверху ---
      {
        id: 'road-casing',
        type: 'line',
        source: 'omt',
        'source-layer': 'transportation',
        filter: ['match', ['get', 'class'], roads, true, false],
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: {
          'line-color': C.ink,
          'line-width': roadWidth(1),
          'line-opacity': ['match', ['get', 'brunnel'], 'tunnel', 0.3, 1],
        },
      },
      {
        id: 'road',
        type: 'line',
        source: 'omt',
        'source-layer': 'transportation',
        filter: ['match', ['get', 'class'], roads, true, false],
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: {
          'line-color': ['match', ['get', 'class'],
            ['motorway', 'trunk'], C.motorway,
            ['primary', 'secondary'], C.major,
            'tertiary', C.tertiary,
            C.minor],
          'line-width': roadWidth(),
        },
      },
      {
        id: 'path',
        type: 'line',
        source: 'omt',
        'source-layer': 'transportation',
        minzoom: 14,
        filter: ['match', ['get', 'class'], ['path', 'track'], true, false],
        layout: { 'line-cap': 'round' },
        paint: { 'line-color': '#c0784a', 'line-width': zoomed([14, 1.2], [18, 4]), 'line-dasharray': [0.1, 2] },
      },
      {
        id: 'rail',
        type: 'line',
        source: 'omt',
        'source-layer': 'transportation',
        minzoom: 10,
        filter: ['==', ['get', 'class'], 'rail'],
        paint: { 'line-color': C.ink, 'line-width': zoomed([10, 1.5], [16, 5]) },
      },
      {
        id: 'rail-ties',
        type: 'line',
        source: 'omt',
        'source-layer': 'transportation',
        minzoom: 10,
        filter: ['==', ['get', 'class'], 'rail'],
        paint: { 'line-color': C.paper, 'line-width': zoomed([10, 0.8], [16, 3]), 'line-dasharray': [2, 2] },
      },

      {
        id: 'boundary-region',
        type: 'line',
        source: 'omt',
        'source-layer': 'boundary',
        minzoom: 4,
        filter: ['all', ['==', ['get', 'admin_level'], 4], ['!=', ['get', 'maritime'], 1]],
        paint: { 'line-color': C.boundary, 'line-width': zoomed([4, 0.6], [10, 2]), 'line-dasharray': [2, 2], 'line-opacity': 0.7 },
      },
      {
        id: 'boundary-country',
        type: 'line',
        source: 'omt',
        'source-layer': 'boundary',
        filter: ['all', ['==', ['get', 'admin_level'], 2], ['!=', ['get', 'maritime'], 1]],
        layout: { 'line-join': 'round' },
        paint: { 'line-color': C.boundary, 'line-width': zoomed([1, 1], [6, 2.5], [10, 4]) },
      },

      // --- Пастельные домики: цвет зависит от высоты, вырастают из земли при приближении ---
      {
        id: 'building',
        type: 'fill-extrusion',
        source: 'omt',
        'source-layer': 'building',
        minzoom: 14,
        filter: ['!=', ['get', 'hide_3d'], true],
        paint: {
          'fill-extrusion-color': ['step', ['coalesce', ['get', 'render_height'], 5],
            C.buildings[0], 6, C.buildings[1], 12, C.buildings[2], 20, C.buildings[3], 40, C.buildings[4]],
          'fill-extrusion-height': ['interpolate', ['linear'], ['zoom'],
            14, 0, 15, ['coalesce', ['get', 'render_height'], 5]],
          'fill-extrusion-base': ['coalesce', ['get', 'render_min_height'], 0],
          'fill-extrusion-opacity': 0.95,
          'fill-extrusion-vertical-gradient': true,
        },
      },

      // --- Подписи ---
      {
        id: 'waterway-name',
        type: 'symbol',
        source: 'omt',
        'source-layer': 'waterway',
        minzoom: 9,
        layout: {
          'symbol-placement': 'line',
          'text-field': name,
          'text-font': [FONT_WATER],
          'text-size': zoomed([10, 12], [16, 18]),
          'text-letter-spacing': 0.1,
          'symbol-spacing': 400,
          'text-pitch-alignment': 'viewport',
        },
        paint: { 'text-color': '#ffffff', 'text-halo-color': C.waterDeep, 'text-halo-width': 1.5 },
      },
      {
        id: 'water-name',
        type: 'symbol',
        source: 'omt',
        'source-layer': 'water_name',
        layout: {
          'text-field': name,
          'text-font': [FONT_WATER],
          'text-size': zoomed([8, 12], [14, 18]),
          'text-letter-spacing': 0.1,
        },
        paint: { 'text-color': '#ffffff', 'text-halo-color': C.waterDeep, 'text-halo-width': 1.5 },
      },
      {
        id: 'road-name',
        type: 'symbol',
        source: 'omt',
        'source-layer': 'transportation_name',
        minzoom: 13,
        filter: ['match', ['get', 'class'], roads, true, false],
        layout: {
          'symbol-placement': 'line',
          'text-field': name,
          'text-font': [FONT],
          'text-size': zoomed([13, 10], [17, 14]),
          'text-max-angle': 30,
        },
        paint: { 'text-color': C.ink, 'text-halo-color': '#fffaf0', 'text-halo-width': 1.5 },
      },
      {
        id: 'poi',
        type: 'circle',
        source: 'omt',
        'source-layer': 'poi',
        minzoom: 14,
        filter: ['any', sights, ['>=', ['zoom'], 17]],
        paint: {
          'circle-radius': zoomed([14, 4], [18, 8]),
          'circle-color': ['match', ['get', 'class'],
            ['museum', 'attraction', 'monument', 'castle', 'art_gallery'], C.poi[0],
            ['theatre', 'cinema', 'music', 'entertainment'], C.poi[1],
            ['cafe', 'restaurant', 'fast_food', 'bar', 'pub', 'ice_cream', 'bakery'], C.poi[2],
            ['park', 'garden', 'playground', 'zoo'], C.poi[3],
            ['place_of_worship'], C.poi[4],
            ['railway', 'bus', 'harbor', 'ferry_terminal'], C.poi[5],
            C.poi[6]],
          'circle-stroke-color': C.ink,
          'circle-stroke-width': 2,
          'circle-pitch-alignment': 'viewport',
        },
      },
      {
        id: 'poi-name',
        type: 'symbol',
        source: 'omt',
        'source-layer': 'poi',
        minzoom: 15,
        filter: ['any', sights, ['>=', ['zoom'], 17.5]],
        layout: {
          'text-field': name,
          'text-font': [FONT],
          'text-size': 12,
          'text-anchor': 'top',
          'text-offset': [0, 0.9],
          'text-max-width': 8,
          'text-optional': true,
        },
        paint: { 'text-color': '#7a3b1d', 'text-halo-color': '#ffffff', 'text-halo-width': 1.5 },
      },
      {
        id: 'country',
        type: 'symbol',
        source: 'omt',
        'source-layer': 'place',
        maxzoom: 7,
        filter: ['==', ['get', 'class'], 'country'],
        layout: {
          'text-field': name,
          'text-font': [FONT_BOLD],
          'text-size': zoomed([1, 10], [6, 15]),
          'text-transform': 'uppercase',
          'text-letter-spacing': 0.15,
          'text-max-width': 7,
        },
        paint: { 'text-color': C.title, 'text-opacity': 0.55, 'text-halo-color': C.paper, 'text-halo-width': 1.5 },
      },
      {
        id: 'place',
        type: 'symbol',
        source: 'omt',
        'source-layer': 'place',
        filter: ['match', ['get', 'class'], ['city', 'town', 'village', 'suburb', 'quarter', 'neighbourhood'], true, false],
        layout: {
          'text-field': name,
          'text-font': [FONT_BOLD],
          'text-size': zoomed(
            [5, placeSize(13, 10, 0, 0)],
            [12, placeSize(22, 16, 12, 11)],
            [16, placeSize(26, 20, 16, 14)],
          ),
          'text-max-width': 8,
          'text-padding': 6,
        },
        paint: {
          'text-color': ['match', ['get', 'class'], ['city', 'town'], C.title, C.subtitle],
          'text-halo-color': '#ffffff',
          'text-halo-width': 2,
        },
      },
    ],
  }
}
