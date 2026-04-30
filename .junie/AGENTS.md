# Project Information for Agents

This document contains essential details for developers and agents working on the Conexiuni Cluj project.

## 1. Project Overview & Stack
Live bus tracker for Cluj-Napoca.
- **Backend**: Go 1.25, Fiber v3, SQLite (caching), Server-Sent Events (SSE) for live updates.
- **Frontend**: Vue 3 (Vapor mode), TypeScript, Vite 7, Tailwind CSS 4, Leaflet 1.9.

## 2. Repo Layout
- `backend/`: Go API + SQLite cache. Serves frontend `dist/`.
  - `handlers/`: API endpoint logic (one file per resource). `register.go` is the entry point.
  - `services/`: External integrations (Tranzy API, CTP scraper).
  - `models/`: Go structs (DTOs) mirrored in frontend `types/tranzy.ts`.
- `frontend/`: Vue 3 SPA.
  - `src/components/MapComponent.vue`: ALL Leaflet logic (markers, vehicles, shapes).
  - `src/stores/`: Pinia state (settings, favorites, map, etc.).
  - `src/utils/mapIcons.ts`: Marker SVG factories (per-theme branching lives here).
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

### Critical Gotchas
- **Vue Scoped CSS**: Never put `@keyframes` in `<style scoped>` if the animation name is set via `:style` binding. Use unscoped styles for keyframes.
- **Leaflet CSS**: `src/styles/leaflet.css` is deliberately **unlayered** (no `@layer`) to ensure it overrides library defaults.
- **Map Icons**: Side-view vehicle SVGs must NOT be rotated with `transform: rotate(heading)` as they appear upside-down at certain angles. Use a rotation-tolerant icon or a heading indicator.
- **Theme Consistency**: Adding/modifying themes requires updates to Pinia stores, global CSS, marker factories (`mapIcons.ts`), and `MapComponent.vue` watch arrays.
- **SQLite Cache**: Every Tranzy/CTP call is cached in SQLite with a TTL. `backend/database/cache.go` handles the generic Get/Set logic.
- **SSE Hub**: Vehicle positions are fanned out via an SSE hub in `handlers/vehicle_stream.go`.

### Frontend Patterns
- **MapComponent**: Only handles rendering. All marker HTML/SVG generation logic MUST live in `utils/mapIcons.ts`.
- **Persistence**: Stores persist state in `localStorage` prefixed with the store name.
