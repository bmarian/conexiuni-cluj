import { statSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";

import { defineConfig, type UserConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import svgLoader from "vite-svg-loader";
import { VitePWA } from "vite-plugin-pwa";
import { releaseVersion } from "./release.ts";

export default defineConfig(async ({ command }): Promise<UserConfig> => ({
  define: {
    __APP_VERSION__: JSON.stringify(Date.now().toString(36)),
    __APP_RELEASE__: JSON.stringify(command === "build" ? await releaseVersion() : ""),
  },
  plugins: [
    tailwindcss(),
    vue(),
    svgLoader({
      // Icons are scaled, which needs the viewBox.
      svgoConfig: {
        plugins: [{ name: "preset-default", params: { overrides: { removeViewBox: false } } }],
      },
    }),
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: ["bus.svg", "favicon.ico", "apple-touch-icon-180x180.png"],
      manifest: {
        id: "/",
        name: "Ce autobuz iau?",
        short_name: "Ce autobuz iau?",
        description: "Transport public Cluj-Napoca în timp real",
        theme_color: "#121318",
        background_color: "#121318",
        display_override: ["standalone", "minimal-ui", "browser"],
        display: "standalone",
        start_url: "/",
        scope: "/",
        handle_links: "preferred",
        launch_handler: {
          client_mode: ["navigate-existing", "auto"],
        },
        share_target: {
          action: "/plan",
          method: "GET",
          params: { title: "shared_title", text: "shared_text", url: "shared_url" },
        },
        icons: [
          { src: "pwa-64x64.png", sizes: "64x64", type: "image/png" },
          { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
          { src: "pwa-512x512.png", sizes: "512x512", type: "image/png" },
          {
            src: "maskable-icon-512x512.png",
            sizes: "512x512",
            type: "image/png",
            purpose: "maskable",
          },
        ],
      },
      workbox: {
        globPatterns: ["**/*.{js,mjs,css,html,svg,png,ico,woff2}"],
        globIgnores: ["**/node_modules/**/*", "sw.js", "workbox-*.js"],
        navigateFallbackDenylist: [/^\/api/],
        runtimeCaching: [
          {
            // Names the current tile set, so it has to be fresh.
            urlPattern: /^https:\/\/tiles\.openfreemap\.org\/planet$/,
            handler: "NetworkFirst",
            options: {
              cacheName: "map-tilejson",
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            urlPattern: /^https:\/\/tiles\.openfreemap\.org\/.+/,
            handler: "CacheFirst",
            options: {
              cacheName: "map-tiles-v3",
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 600, maxAgeSeconds: 60 * 60 * 24 * 30 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: "../backend/dist",
    // Remove this when chrome or vite fixes it https://github.com/vite-pwa/vite-plugin-pwa/issues/945
    emptyOutDir: true,
    modulePreload: false,
    // MapLibre ships as one minified file; no chunk of ours should outgrow it.
    chunkSizeWarningLimit: Math.ceil(
      statSync(
        fileURLToPath(new URL("./node_modules/maplibre-gl/dist/maplibre-gl.mjs", import.meta.url)),
      ).size / 1000,
    ),
    rollupOptions: {
      onwarn(warning, warn) {
        if (
          warning.code === "INVALID_ANNOTATION" &&
          warning.id?.includes("/node_modules/@vueuse/core/dist/index.js")
        ) {
          return;
        }
        warn(warning);
      },
      output: {
        manualChunks(id: string) {
          if (
            id.includes("/node_modules/vue/") ||
            id.includes("/node_modules/pinia/") ||
            id.includes("/node_modules/vue-router/") ||
            id.includes("/node_modules/vue-i18n/") ||
            id.includes("/node_modules/@unhead/")
          ) {
            return "vue-vendor";
          }
          if (
            id.includes("/node_modules/@vuepic/vue-datepicker/") ||
            id.includes("/node_modules/@vueuse/") ||
            id.includes("/node_modules/date-fns/") ||
            id.includes("/node_modules/vuedraggable/")
          ) {
            return "ui-vendor";
          }
          if (id.includes("/node_modules/maplibre-gl/")) {
            return "maplibre-vendor";
          }
          if (id.includes("/utils/mapIcons")) {
            return "map-icons";
          }
        },
      },
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:6698",
        changeOrigin: true,
      },
    },
  },
}));
