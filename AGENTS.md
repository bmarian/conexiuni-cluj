# conexiuni-cluj Agent Guide

Public transit web app for Cluj-Napoca, Romania. It shows live vehicles, route shapes, stop departures, timetable fallbacks, route planning, weather, CTP news, favorites, sharing, installable PWA support, Romanian and English UI, and three visual themes: Default, Arcade, and Legacy Blue.

Stack: Go backend with Fiber v3 and SQLite, plus a Vue 3 TypeScript frontend built by Vite, Tailwind CSS v4, Pinia, vue-i18n, Leaflet, and vite-plugin-pwa.

## Comments Policy

Do not add comments unless the code is genuinely non-obvious. Most architecture, theme, and API notes belong in this guide, not inline.

When a comment is warranted, make it short and human. No decorative dividers, no generated-looking summaries, and no em dash prose in comments.

Good examples:

```go
// Buffered so slow SSE writers don't block the hub.
```

```css
/* contain: layout prevents Leaflet resize reflows from propagating to drawer/shell */
```

## Repo Map

```text
conexiuni-cluj/
├── AGENTS.md                 Agent guide. Keep this current when architecture shifts.
├── README.md                 User-facing project overview and run instructions.
├── update.sh                 Deployment-server-only script. Do not run during local agent work.
├── .env / keys.env           Runtime config and secrets. Never commit real secrets.
├── backend/
│   ├── main.go               Boot, logging, DB, clients, Fiber middleware, routes, static frontend serving.
│   ├── config.go             Env loading and defaults.
│   ├── logging.go            Daily app/access log rotation.
│   ├── quota_persister.go    Tranzy daily quota persistence adapter.
│   ├── database/             SQLite schema, generic TTL cache, Tranzy quota table.
│   ├── handlers/             API handlers and route registration.
│   ├── models/               JSON DTOs mirrored by frontend types.
│   ├── services/
│   │   ├── tranzy/           Tranzy API client with rate limiting, quota, retries.
│   │   ├── ctp-cj/           CTP CSV timetable scraper/parser.
│   │   └── otp/              OpenTripPlanner jar, Cluj PBF, generated GTFS zip, router config.
│   └── dist/                 Built frontend committed/deployed with backend.
├── frontend/
│   ├── src/
│   │   ├── main.ts           App bootstrap. All global CSS imports live here.
│   │   ├── App.vue           Persistent map plus drawer shell.
│   │   ├── main.css          Tailwind and global app styles.
│   │   ├── router/           Home, stop, route, plan, and catch-all routes.
│   │   ├── views/            HomeView, StopView, RouteView, RoutePlanningView, NotFoundView.
│   │   ├── components/       Map, settings, search, weather/news, sharing, theme secrets.
│   │   ├── stores/           Pinia stores: settings, favorites, map, route, user, planner.
│   │   ├── composables/      API and browser side-effect hooks.
│   │   ├── styles/           dark.css, leaflet.css, arcade.css, legacy-blue.css.
│   │   ├── utils/            map icons, API fetch helper, time/trip/geo helpers.
│   │   ├── types/            Tranzy, CTP, and map DTO types.
│   │   └── locales/          ro.json and en.json.
│   ├── package.json          Vite 7, Vue beta, Pinia 3, Router 5, Tailwind 4.
│   └── vite.config.ts        PWA config, proxy, chunking, build output to ../backend/dist.
└── api-tests/                Bruno collection for manual API checks.
```

There is no `build.sh` or `dev.sh` now. Use the local commands in README. Do not use `update.sh` except on the deployment server.

## Backend

Boot order in `backend/main.go`:

1. Force Europe/Bucharest timezone, then load `.env` and `keys.env` from the current or parent directory.
2. Set up rotated app/access logs. Access logs use an HMAC-pseudonymized client id from `LOG_IP_HASH_SALT`.
3. Connect SQLite, initialize schemas, start vacuum scheduling, DST cache invalidation, and buffered stats flushing.
4. Create Tranzy and CTP clients. Tranzy vehicle quota is persisted in SQLite because upstream daily quota does not reset when this process restarts.
5. Parse adaptive vehicle polling schedules and initialize the SSE vehicle hub.
6. Configure stats middleware, Fiber logger, CORS, and all `/api` routes.
7. If `backend/dist` exists, load `index.html`, install OG metadata handlers, serve PWA files with safe cache headers, serve static assets, and use SPA fallback.
8. Start cache warmup, start the background vehicle learning sampler, and defer OTP cleanup.

Primary env vars:

```text
TRANZY_API_KEY                 required
ENV                            development by default; development makes static caches effectively never expire
PORT                           default 6698
DATABASE_PATH                  default ../conexiuni-cluj.db
LOG_DIR                        default ../logs
LOG_IP_HASH_SALT               required in production; used for pseudonymized visitor/log ids
ADMIN_TOKEN                    required in production; exchanged for an HttpOnly admin session cookie
TRANZY_BASE_URL                default https://api.tranzy.ai/v1/opendata
CLUJ_AGENCY_ID                 default 2
CTP_CSV_BASE_URL               default https://ctpcj.ro/orare/csv
TRANZY_DEFAULT_DAILY_QUOTA     default 144; Tranzy shelf life = 24h * 6 / quota
TRANZY_VEHICLES_DAILY_QUOTA    default 4500; separate quota for `/vehicles`
TIMETABLE_CACHE_SHELF_LIFE     default 24h
NEWS_CACHE_SHELF_LIFE          default 4h
OTP_URL                        optional external OTP server. Defaults to local http://localhost:18080
OTP_MX                         Java max heap for local OTP. Default 2G
VEHICLE_SCHEDULE               weekday adaptive polling slots
VEHICLE_SCHEDULE_WEEKEND       weekend adaptive polling slots
VEHICLE_LEARNING_ENABLED       default true; background sampling for learned travel-time segments
VEHICLE_LEARNING_DAILY_QUOTA_MAX default 3000; cap for background learning `/vehicles` calls
VEHICLE_LEARNING_MIN_QUOTA_REMAINING default 10% of vehicle quota, minimum 50
```

Vehicle learning uses live `/vehicles` snapshots to populate stop-to-stop segment travel profiles. Foreground SSE users always drive the freshest sampling; the background sampler only fills quiet periods and preserves the configured quota reserve. Its daily budget is the lower of `VEHICLE_LEARNING_DAILY_QUOTA_MAX` and half of the average unused vehicle quota from recent active days, then it spreads that budget across the remaining day.

### API Surface

All routes are registered in `backend/handlers/register.go`.

| Method and path | Handler file | Notes |
|---|---|---|
| `GET /api/routes` | `route.go` | Optional `route_id`, `route_short_name`; list is filtered by availability when ready. |
| `GET /api/stops` | `stop.go` | Optional `stop_id`; list is filtered to served stops when availability is ready. |
| `GET /api/shapes?shape_id=...&shape_ids=a,b` | `shape.go` | Route polylines. |
| `GET /api/trips?route_id=...&trip_id=...` | `trip.go` | Trip metadata. |
| `GET /api/stop_times?route_short_name=...` | `stop_time.go` | `route_short_name` is required. |
| `GET /api/timetable?route_short_name=...` | `timetable.go` | Scraped from CTP CSV exports. |
| `GET /api/stop_info?stop_id=...` | `stop_info.go` | Aggregate used by StopView. |
| `GET /api/vehicles?route_id=...&trip_id=...&trip_ids=a,b` | `vehicle.go` | Single-shot live vehicles, `Cache-Control: no-store`. |
| `GET /api/vehicles/stream?trip_ids=a,b` | `vehicle_stream.go` | SSE stream, adaptive polling from `vehicle_interval.go`. |
| `GET /api/plan_routes?...` | `plan_routes.go` | Route planner backed by OpenTripPlanner. |
| `GET /api/news` | `news.go` | Scraped CTP news; database-backed cache with TTL expiration, serves stale on fetch failure. |
| `POST /api/stats/event` | `register.go` | Same-origin first-party PWA install events. |
| `POST /api/admin/login` / `logout` | `admin.go` | Admin token login/logout. Login sets an HttpOnly cookie. |
| `GET /api/admin/stats` / `logs` | `admin.go` | Token-protected admin stats and access-log tail. |

Cached API endpoints send `Cache-Control: max-age=...` based on configured shelf lives (`TRANZY_DEFAULT_DAILY_QUOTA`, `TIMETABLE_CACHE_SHELF_LIFE`, `NEWS_CACHE_SHELF_LIFE`). `plan_routes` uses `max-age=300`; live vehicles use `no-store`; SSE uses `no-cache`.

### OTP And Planning

Route planning is in `backend/handlers/plan_routes.go` and `frontend/src/views/RoutePlanningView.vue`.

The backend uses OpenTripPlanner through GraphQL. If `OTP_URL` is unset, it starts or talks to a local OTP server at `localhost:18080`. OTP data lives in `backend/services/otp/cluj` and includes `cluj.pbf`, `gtfs.zip`, and `router-config.json`. `update.sh --update-pbf` refreshes the Romania PBF, crops it to Cluj bounds with Osmosis, and leaves OTP assets in place.

The frontend planner supports current location or searched origins, destination search, favorites and recents, leave-now, leave-at, and arrive-by modes. The planner store keeps selected itinerary and time filter state for the session.

## Frontend

`App.vue` renders a persistent `MapComponent` plus a responsive drawer that contains the router view. The drawer is bottom-sheet style on portrait mobile, slide-in on mobile landscape, and fixed right-column on desktop.

Routes:

```text
/                         HomeView
/stop/:stopId             StopView
/route/:routeId/:direction RouteView
/plan                     RoutePlanningView
/admin                    AdminView
/*                        NotFoundView
```

Main data flow:

- `HomeView` loads routes and stops, shows favorites, universal search, weather/news buttons, and route planning entry points.
- `StopView` uses `/api/stop_info`, pushes shapes and highlighted stops into the map store, and opens the vehicle SSE stream for relevant trips.
- `RouteView` uses selected route context from the route store, renders stops/timetable, updates map shapes, and can launch `RoutePong`.
- `RoutePlanningView` calls `/api/plan_routes`, renders itineraries, walking polylines, transfer details, and favorite destinations.
- `MapComponent` owns all Leaflet objects. It renders map controls, base tiles, user location, pins, stops, route shapes, walking polylines, highlighted stops, and vehicles.
- `main.ts` sends a same-origin aggregate stat when the browser fires `appinstalled`.

API helpers:

- `utils/api.ts` wraps `/api` fetch calls without frontend-side response caching.
- `composables/useRoutesApi.ts`, `useStopsApi.ts`, `useStopInfoApi.ts`, and `useRouteShapeInfoApi.ts` are the main data hooks.
- `composables/useVehicleStream.ts` wraps `EventSource`.
- `composables/useVehicleTracking.ts` projects vehicles onto route shapes for ETA math.
- `composables/useOnline.ts` mirrors `navigator.onLine`.

Persistent state:

- `settings.*`: light/dark/system mode, locale, Arcade/Legacy Blue unlock and active flags, weather/news visibility, auto map behavior.
- `favorites:*`: routes, stops, favorite destinations, recent destinations. Favorites are localStorage only; the old IndexedDB migration has been removed.
- Route planning selection is session-only in `planner.ts`.

## Themes

Theme names are user-facing and code-facing now:

| Theme | Store refs | Root attribute | Map class | CSS file |
|---|---|---|---|---|
| Default | none | none | none | base styles plus `dark.css` |
| Arcade | `arcadeUnlocked`, `arcadeActive` | `html[data-arcade]` | `.arcade-theme` | `styles/arcade.css` |
| Legacy Blue | `legacyBlueUnlocked`, `legacyBlueActive` | `html[data-legacy-blue]` | `.legacy-blue-theme` | `styles/legacy-blue.css` |

Special themes are mutually exclusive. `activateArcade()` calls `deactivateLegacyBlue()`, and `activateLegacyBlue()` calls `deactivateArcade()`.

Unlocks:

- Arcade: click the settings button 10 times. The midway toast says `insert coin?`; the unlock activates Arcade.
- Legacy Blue: visit any unmatched route and interact with the BSOD, or search `metrou` / `M1` and click the fake metro result from `MetroLegacyBlue.vue`.

Marker rendering lives in `frontend/src/utils/mapIcons.ts`. `IconThemeOptions` must include every active theme ref. `MapComponent.vue` must pass those refs through `themeOpts()`.

Important theme invariants:

- Non-stop vehicle markers always show route/title and rounded speed next to the marker.
- Stop-view vehicle markers always show the route badge, and only show the popup when clicked.
- `makeHighlightIcon` needs a branch for every special theme. Current order is Arcade, Legacy Blue, Default.
- `MapComponent.mapInit()` must toggle every special map class on first paint.
- Every map-render watcher that depends on theme visuals must include `arcadeActive` and `legacyBlueActive`.

### Legacy Blue Notes

Legacy Blue is a Windows XP Luna homage. Keep identifiers as `legacyBlue*`, `data-legacy-blue`, and `.legacy-blue-theme`.

- It uses square UI, Luna blue chrome, Bliss-like shell backgrounds, Tahoma, XP-style task panes, and emoji replacements for many inline icons.
- Do not implement square corners with broad universal selectors like `html[data-legacy-blue] *`. Enumerate the real rounded utility/classes in `legacy-blue.css`; broad universal matching caused severe drawer/map resize lag.
- Avoid CSS `filter` on many Legacy Blue elements. The flat XP look is intentional and filters add many GPU layers on older devices.
- Leaflet-specific square corners live in `styles/leaflet.css` under `.legacy-blue-theme.*`.
- The Legacy Blue vehicle icon is an XP cursor rotated by `heading + 45`; do not replace it with a rotated side-view bus.

### Arcade Notes

Arcade uses chomping route lines, ghost stop icons, the drawer chase transition, and warmer yellow/orange UI styling. Its key animation class is `.arcade-chomp`.

`ArcadeTransition.vue` teleports into `.app-drawer`. If inline styles set animation names in future transitions, keyframes must be unscoped so Vue does not hash them away.

## Leaflet And Map Gotchas

- `frontend/src/styles/leaflet.css` is intentionally unlayered so it can beat Leaflet and leaflet-geosearch library CSS. Do not wrap it in `@layer`.
- Leaflet reuses `.leaflet-bar` for zoom, geosearch, and custom controls. Target `.leaflet-control-zoom-*`, `.leaflet-control-geosearch`, or `.leaflet-bar:not(...)` explicitly.
- Leaflet geosearch draws the magnifier with both `::before` and `::after`. To replace it, reset both pseudo-elements and add the replacement only once.
- Do not rotate asymmetric side-view vehicle SVGs. Use a circular container plus heading indicator, or a rotation-tolerant icon like the Legacy Blue cursor.
- `RoutePong.vue` keeps game state in plain `let` values so the animation loop does not trigger Vue reactivity every frame.

## Styling Rules

- Global CSS must be imported from `frontend/src/main.ts`; Vite will not discover new files automatically.
- Prefer existing utility/component classes over one-off new abstractions.
- Keep text fitting in compact drawer and popover controls. Avoid viewport-scaled font sizes.
- The settings special-theme dropdown is one `<select>` inside `.select-wrap`; do not add a second selector.
- `dark.css` contains global dark-mode overrides that are separate from special-theme CSS.

## DTO Sync

Backend model structs in `backend/models` and frontend DTOs in `frontend/src/types` must stay aligned. When adding fields to backend JSON responses, update the TypeScript type and all consuming UI code in the same change.

## Build And Run

Development:

```bash
cd backend
go run .
```

```bash
cd frontend
npm install
npm run dev
```

The frontend dev server proxies `/api` to `http://localhost:6698`.

Production frontend build:

```bash
cd frontend
npm run build
```

This writes to `backend/dist`, which the Go server serves automatically.

Deployment server only:

```bash
./update.sh
./update.sh --update-pbf
./update.sh --delete-db
./update.sh --delete-logs
```

`update.sh` pulls, installs frontend deps, builds frontend, builds the Go binary, copies env files into `backend/`, prepares OTP/Osmosis assets, optionally refreshes/crops PBF data, and restarts the `conexiuni-cluj` systemd service. `--delete-db` removes `conexiuni-cluj.db` before restart; `--delete-logs` removes the `logs/` directory. It is for the deployment host only. Agents should not run it in local development.

## Testing And Verification

There is no dedicated automated test suite in the repo right now. Use these checks after changes:

- `cd frontend && npm run build` for TypeScript and production bundling.
- `cd backend && go test ./...` for Go compile/test coverage.
- Bruno requests in `api-tests/` for manual API checks.
- For frontend behavior, run backend and frontend dev servers, then check home, stop, route, plan, settings/theme switching, and map vehicle rendering.

## Moving This File

Keep a root `AGENTS.md` if you want Codex and similar agents to receive repo-wide instructions automatically. You can also add nested `AGENTS.md` files inside folders for scoped instructions, for example `frontend/AGENTS.md` or `backend/AGENTS.md`.

You can move the guide into a folder only if your tools are configured to read it there. A root file is the safest default. If another assistant expects `.junie/AGENTS.md`, use a copy or symlink rather than removing the root guide.
