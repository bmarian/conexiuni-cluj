# Project Information for Agents

This document contains essential details for developers and agents working on the Conexiuni Cluj project.

## 1. Project Overview & Stack
Live bus tracker for Cluj-Napoca.
- **Backend**: Go 1.25, Fiber v3, SQLite (caching), Server-Sent Events (SSE) for live updates.
- **Frontend**: Vue 3 (Vapor mode), TypeScript, Vite 7, Tailwind CSS 4, Leaflet 1.9.

## 2. Repo Layout
- `backend/`: Go API + SQLite cache. Serves frontend `dist/`.
  - `main.go`: App boot, CORS, static-mount, warmup kickoff.
  - `handlers/`: API endpoint logic. `register.go` is the entry point.
  - `services/`: External integrations (Tranzy API, CTP scraper).
  - `models/`: Go structs (DTOs) mirrored in frontend `types/tranzy.ts`.
  - `database/`: SQLite init, schema, generic cache Get/Set with TTL.
- `frontend/`: Vue 3 SPA.
  - `src/components/MapComponent.vue`: ALL Leaflet logic (markers, vehicles, shapes).
  - `src/stores/`: Pinia state (settings, favorites, map, etc.).
  - `src/utils/mapIcons.ts`: Marker SVG factories (per-theme branching lives here).
  - `src/styles/`: Global CSS (leaflet.css, hungry.css, traditional.css).
- `api-tests/`: Bruno collection for manual API testing.
- `build.sh`: Builds frontend and moves output to `backend/dist`.
- `dev.sh`: Runs backend and frontend concurrently.

## 3. Build and Configuration

### Prerequisites
- **Node.js**: ≥ 20.19 or ≥ 22.12
- **Go**: 1.25
- **Tranzy API Key**: Required for live bus data.

### Environment Setup
1. **Backend**: Navigate to `backend/`, copy `.env.example` to `.env`, set `TRANZY_API_KEY` and `TRANZY_AGENCY_ID` (`cluj`).
2. **Frontend**: Navigate to `frontend/`, run `npm install`.

### Running Locally
- **Full Stack**: `./dev.sh` (requires `air` for Go hot reload) or run separately.
- **Backend**: `go run .` (from `backend/`).
- **Frontend**: `npm run dev` (from `frontend/`).
- Frontend proxies API calls to `localhost:3000`.

## 4. Testing & Verification (CRITICAL)
**THERE ARE NO AUTOMATED TESTS IN THIS PROJECT.**
- **DO NOT run `go test` or any other test commands after completing a task.**
- **DO NOT run `go build .` as it creates `.exe` files.** Use `go check` or `lint` tools instead.
- **DO NOT attempt to create new tests unless explicitly requested.**
- Verify changes via **Normal Checks** only:
  - **Backend**: Ensure it compiles and API returns expected JSON (use Bruno in `api-tests/`).
  - **Frontend**: Run `npm run type-check` and `npm run lint`.
  - **Manual Verification**: Run the app and verify the behavior in the browser.

## 5. Development Guidelines

### Go 1.25 Idioms
- Use `slices` and `maps` packages (e.g., `slices.Sorted`, `maps.Keys`).
- Use `strings.SplitSeq` for iterations in for-range loops.
- Use `omitzero` instead of `omitempty` for modern Go 1.25 JSON compatibility.
- Use `errors.Is` and `errors.As` for error handling.

### Backend Architecture
- **Caching**: Every Tranzy/CTP call is cached in SQLite with a TTL. `backend/database/cache.go` handles the generic Get/Set logic.
- **SSE Hub**: Vehicle positions are fanned out via an SSE hub in `handlers/vehicle_stream.go`.
- **Warmup**: `warmup.go` re-primes hot endpoints just before TTLs expire.
- **Quota**: `quota_persister.go` persists Tranzy daily quota across restarts.

### Frontend Data Flow
- `MapComponent.vue` only handles rendering. All marker HTML/SVG generation logic MUST live in `utils/mapIcons.ts`.
- `useStopInfoApi` fetches aggregate data; `useVehicleStream` handles SSE updates.
- `useVehicleTracking` computes ETA from projected position on polyline.
- **Persistence**: Stores persist state in `localStorage` prefixed with the store name.

### Critical Gotchas
- **Vue Scoped CSS**: Never put `@keyframes` in `<style scoped>` if the animation name is set via `:style` binding. Use unscoped styles for keyframes.
- **Leaflet CSS**: `src/styles/leaflet.css` is deliberately **unlayered** (no `@layer`) to ensure it overrides library defaults.
- **Map Vehicle Icons**: Never rotate side-view vehicle SVGs with `transform: rotate(heading)`. Use a rotation-tolerant icon or a heading indicator.
- **Vehicle Popups**: Non-stopView ALWAYS shows a popup (route name + speed). StopView shows route badge always, popup only on click.
- **Leaflet Bar**: `.leaflet-bar` is reused (zoom, search). Target specific classes or exclude zoom/search when styling custom bars.
- **Geosearch Icon**: To replace the magnifying glass emoji, unset both `::before` and `::after` with `all: unset !important`.
- **CSS Triangles**: Don't use `linear-gradient` for 1px-tall triangles; use inline-SVG background images.
- **Traditional Theme Radius**: Global `border-radius: 0 !important` affects everything including dots/markers; that is intentional.

### Themes Checklist
When adding or modifying a theme, ensure you update:
- **`src/stores/settings.ts`**: Add theme identifier, `watch` to toggle `html[data-*]`, and activate/deactivate logic.
- **Global CSS**: Add theme-specific variables and overrides in `src/styles/`. Import in `main.ts`.
- **`src/utils/mapIcons.ts`**: Update `makeStopIcon`, `makeSelectedStopIcon`, `makeHighlightIcon`, and `getVehicleMarkerHtml` with theme branches.
- **`src/components/MapComponent.vue`**:
  - Add theme to `themeOpts()`.
  - `mapInit()`: Toggle theme class on `mapContainer`.
  - Add `watch` for theme to re-render markers.
  - Add theme ref to ALL THREE watch arrays (shapes, highlighted stops, vehicles).
- **`src/components/SettingsButton.vue`**: Add to `activeSpecialTheme`, `onSpecialThemeChange`, and the unified `<select>`.
- **Localization**: Add theme name and related strings to `en.json` and `ro.json`.
- **Low Performance Mode**: If it's a "heavy" theme (like Traditional), ensure it supports a solid-color mode for both light and dark variants.

## 6. Theme Reference: Traditional (Windows XP Luna)
- **Identifiers**: `traditionalActive`, `data-traditional`, `.traditional-theme`.
- **Colors**: Luna blue `#245EDC`, Bliss gradient, XP tan `#ECE9D8`. Dark variant uses deep navy/dark grass.
- **UI**: 28px title bar "🚌 Conexiuni Cluj" on drawer. No close/min/max buttons. Tahoma font.
- **Markers**: Clippy for stops (nested paperclip wires + googly eyes). XP mouse cursor for vehicles (rotated `heading + 45`).
- **404**: BSOD (`NotFoundView.vue`) with Lucida Console and `0x00000404`. Unlocks theme on any interaction.
- **RouteView**: Emojis (📍, ❤️, 🙎‍♂️) instead of SVG indicators.
