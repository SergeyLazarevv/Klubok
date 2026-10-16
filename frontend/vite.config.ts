import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      manifest: {
        name: 'Klubok — дешёвые путешествия',
        short_name: 'Klubok',
        description: 'Самые дешёвые варианты поездки с учётом машины, поездов и самолётов',
        lang: 'ru',
        theme_color: '#ffffff',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: 'icons/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icons/icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: 'icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // API не кэшируем service worker'ом: цены должны быть свежими
        navigateFallbackDenylist: [/^\/api\//],
        // Карта: чем больше в кэше, тем быстрее она открывается и двигается. Гербы и тайлы не меняются под тем же
        // адресом (тайлы — в папке с версией сборки планеты), поэтому берём из кэша без сети.
        runtimeCaching: [
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/data/stickers/') || url.pathname.startsWith('/data/arms/'),
            handler: 'CacheFirst',
            options: {
              cacheName: 'map-arms',
              expiration: { maxEntries: 4000, maxAgeSeconds: 90 * 24 * 3600 },
            },
          },
          {
            // Справочник городов меняется при пересборке — показываем из кэша и тихо обновляем
            urlPattern: ({ url }) => url.pathname === '/data/cities.geojson',
            handler: 'StaleWhileRevalidate',
            options: { cacheName: 'map-data' },
          },
          {
            urlPattern: ({ url }) => url.host === 'tiles.openfreemap.org' && /^\/planet\/[^/]+\/\d+\/\d+\/\d+\.pbf$/.test(url.pathname),
            handler: 'CacheFirst',
            options: {
              cacheName: 'map-tiles',
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 20000, maxAgeSeconds: 60 * 24 * 3600, purgeOnQuotaError: true },
            },
          },
          {
            // Описание тайлов (TileJSON) указывает на текущую сборку планеты — его обновляем
            urlPattern: ({ url }) => url.host === 'tiles.openfreemap.org' && url.pathname === '/planet',
            handler: 'StaleWhileRevalidate',
            options: { cacheName: 'map-tilejson', cacheableResponse: { statuses: [200] } },
          },
          {
            urlPattern: ({ url }) => url.host === 'tiles.openfreemap.org' && url.pathname.startsWith('/fonts/'),
            handler: 'CacheFirst',
            options: {
              cacheName: 'map-glyphs',
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 500, maxAgeSeconds: 365 * 24 * 3600 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
})
