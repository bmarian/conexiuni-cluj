import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import svgLoader from 'vite-svg-loader'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    tailwindcss(),
    vue(),
    svgLoader(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['bus.svg', 'favicon.ico', '404.png', 'apple-touch-icon-180x180.png'],
      manifest: {
        id: '/',
        name: 'Conexiuni Cluj',
        short_name: 'Conexiuni',
        description: 'Transport public Cluj-Napoca în timp real',
        theme_color: '#0f172a',
        background_color: '#0f172a',
        display_override: ['standalone', 'minimal-ui', 'browser'],
        display: 'standalone',
        start_url: '/',
        scope: '/',
        handle_links: 'preferred',
        launch_handler: {
          client_mode: ['navigate-existing', 'auto'],
        },
        icons: [
          { src: 'pwa-64x64.png', sizes: '64x64', type: 'image/png' },
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: 'maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,ico,woff2}'],
        navigateFallbackDenylist: [/^\/api/],
        runtimeCaching: [
          {
            urlPattern: /\/api\/(routes|stops|stop_info|stop_times|shapes|timetable)(\?|$)/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'api-static',
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 500, maxAgeSeconds: 60 * 60 },
            },
          },
          {
            urlPattern: /^https:\/\/[a-z0-9]+\.basemaps\.cartocdn\.com\/.*/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'map-tiles',
              cacheableResponse: { statuses: [0, 200] },
              expiration: { maxEntries: 600, maxAgeSeconds: 60 * 60 * 24 * 30 },
            },
          },
          {
            urlPattern: /^https:\/\/unpkg\.com\/leaflet.*/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'leaflet-assets',
              cacheableResponse: { statuses: [0, 200] },
              expiration: { maxEntries: 30, maxAgeSeconds: 60 * 60 * 24 * 90 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  build: {
    outDir: '../backend/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (id.includes('/node_modules/leaflet') || id.includes('/node_modules/leaflet-geosearch')) {
            return 'leaflet-vendor'
          }
          if (id.includes('/utils/mapIcons')) {
            return 'map-icons'
          }
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:6698',
        changeOrigin: true,
      },
    },
  },
})
