# Conexiuni Cluj

Live bus tracker for Cluj-Napoca.

---

## What it does

- **Live map** - every CTP bus and tram moving in real time, updated every few seconds
- **Stop departures** - next buses with live countdowns, falling back to the schedule when there's no GPS signal
- **Route timetables** - full schedules sourced directly from CTP Cluj-Napoca
- **Route planner** - trip planning from A to B with transit, with "leave now", "leave at", and "arrive by" modes
- **Weather** - current temperature and conditions for Cluj
- **Favorites** - pin the routes and stops you actually use
- **Dark mode**
- **PWA** - installable, works offline for cached data

There are a few hidden things scattered around for the curious. 🐣

---

## Why it exists

The official CTP site works, but it doesn't have all the features that I want. I commute in Cluj every day and got tired of it, so I built something I'd actually want to open.

---

## Tech stack

### Frontend
- **Vue 3** (Vapor) with TypeScript
- **Vite 7** for bundling
- **Vue Router 5** + **Pinia 3** for routing and state
- **vue-i18n 11** for localisation
- **Leaflet 1.9** for the interactive map
- **vuedraggable** for reorderable favorites
- **@meteocons/svg** for weather icons
- **Tailwind CSS 4** for utility styling
- **vite-plugin-pwa** for service worker and offline support

### Backend
- **Go** (1.25) with **Fiber v3** as the HTTP framework
- **SQLite** (`go-sqlite3`) for persistent caching and quota tracking
- **OpenTripPlanner** (spawned subprocess) for transit route planning
- Server-sent events stream for pushing live vehicle positions to the client

---

## Data & credits

| Source | What it provides |
|---|---|
| [Tranzy.ai](https://tranzy.ai/) | Live GPS positions and GTFS data for all CTP Cluj-Napoca vehicles |
| [CTP Cluj-Napoca](https://www.ctpcj.ro/) | Official timetable CSVs for all routes |
| [Open-Meteo](https://open-meteo.com/) | Weather data |
| [OpenStreetMap](https://www.openstreetmap.org/) contributors | Map data, © OpenStreetMap contributors, [ODbL](https://opendatacommons.org/licenses/odbl/) |
| [CARTO](https://carto.com/) | Map tile rendering |
| [Nominatim](https://nominatim.org/) | Address and location search |

---

## Running locally

### Prerequisites
- Node.js ≥ 20.19 or ≥ 22.12
- Go ≥ 1.25
- A Tranzy API key
- Java 21+ (only needed for the route planner feature, which uses OpenTripPlanner)

### Backend

Create a `.env` file in the project root (or in `backend/`) with at minimum:

```env
TRANZY_API_KEY=your_key_here
```

Then run:

```bash
cd backend
go run .
```

The server listens on port `6698` by default.

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend dev server proxies API calls to the backend at `localhost:6698`.

### Production build

```bash
cd frontend
npm run build
```

Output goes to `backend/dist/` — the Go server picks it up and serves it automatically.
