import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import svgLoader from 'vite-svg-loader'
import { VitePWA } from 'vite-plugin-pwa'

// Read CARTO_KEY from keys.env so it doesn't need to be duplicated into a separate frontend env file.
function readRootEnvValue(key: string): string {
  for (const file of ['../keys.env', '../.env']) {
    try {
      const content = readFileSync(fileURLToPath(new URL(file, import.meta.url)), 'utf-8')
      for (const line of content.split('\n')) {
        const trimmed = line.trim()
        if (!trimmed || trimmed.startsWith('#')) continue
        const eq = trimmed.indexOf('=')
        if (eq === -1) continue
        if (trimmed.slice(0, eq).trim() !== key) continue
        let value = trimmed.slice(eq + 1).trim()
        if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
          value = value.slice(1, -1)
        }
        return value
      }
    } catch {
      // file not found, keep looking
    }
  }
  return ''
}

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(Date.now().toString(36)),
    __CARTO_KEY__: JSON.stringify(readRootEnvValue('CARTO_KEY')),
  },
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
    chunkSizeWarningLimit: 650,
    rollupOptions: {
      onwarn(warning, warn) {
        if (warning.code === 'INVALID_ANNOTATION' && warning.id?.includes('/node_modules/@vueuse/core/dist/index.js')) {
          return
        }
        warn(warning)
      },
      output: {
        manualChunks(id: string) {
          if (id.includes('/node_modules/vue/') || id.includes('/node_modules/pinia/') || id.includes('/node_modules/vue-router/') || id.includes('/node_modules/vue-i18n/') || id.includes('/node_modules/@unhead/')) {
            return 'vue-vendor'
          }
          if (id.includes('/node_modules/@vuepic/vue-datepicker/') || id.includes('/node_modules/@vueuse/') || id.includes('/node_modules/date-fns/') || id.includes('/node_modules/vuedraggable/')) {
            return 'ui-vendor'
          }
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
        target: 'http://127.0.0.1:6698',
        changeOrigin: true,
      },
    },
  },
})
