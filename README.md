# Conexiuni Cluj

A fast, modern bus tracker for Cluj-Napoca — live vehicle positions, real-time departures, and full timetables, all in one place.

**→ [bus.bmarian.online](https://bus.bmarian.online/)**

---

## What it does

Conexiuni Cluj pulls together everything you need to get around Cluj-Napoca by public transit:

- **Live map** — see every CTP bus and tram moving in real time, updated every few seconds
- **Stop departures** — tap any stop to see the next buses, with live countdowns where available and scheduled fallbacks otherwise
- **Route timetables** — full schedules for every route, sourced directly from CTP Cluj-Napoca
- **Favorites** — pin the routes and stops you use most
- **Dark mode**
- **PWA** — installable on any device, works offline for cached data

There are also a few hidden surprises scattered around the app for the curious. 🐣

---

## Motivation

The official CTP Cluj-Napoca apps and the existing third-party options are functional but feel dated. Cluj is a growing city with a lot of daily commuters, students, and visitors who deserve a transit experience that feels as polished as the apps they use for everything else.

Conexiuni Cluj started as a personal tool and grew into something worth sharing. The goal was simple: build the bus app I actually wanted to use — live, fast, clean, and pleasant to open every morning.

---

## Tech stack

### Frontend
- **Vue 3** (Vapor) with TypeScript
- **Vite 7** for bundling
- **Vue Router 5** + **Pinia 3** for routing and state
- **vue-i18n 11** for localisation
- **Leaflet 1.9** with leaflet.markercluster and leaflet-geosearch for the interactive map
- **Tailwind CSS 4** for utility styling
- **vite-plugin-pwa** for service worker and offline support

### Backend
- **Go** (1.25) with **Fiber v3** as the HTTP framework
- **SQLite** (`go-sqlite3`) for persistent caching and quota tracking
- Server-sent events stream for pushing live vehicle positions to the client

---

## Data & credits

| Source | What it provides |
|---|---|
| [Tranzy.ai](https://tranzy.ai/) | Live GPS positions and GTFS data for all CTP Cluj-Napoca vehicles |
| [CTP Cluj-Napoca](https://www.ctpcj.ro/) | Official timetable CSVs for all routes |
| [OpenStreetMap](https://www.openstreetmap.org/) contributors | Map data, © OpenStreetMap contributors, [ODbL](https://opendatacommons.org/licenses/odbl/) |
| [CARTO](https://carto.com/) | Map tile rendering |
| [Nominatim](https://nominatim.org/) via [leaflet-geosearch](https://github.com/smeijer/leaflet-geosearch) | Address and location search |

---

## Running locally

### Prerequisites
- Node.js ≥ 20.19 or ≥ 22.12
- Go ≥ 1.25
- A Tranzy API key

### Backend

```bash
cd backend
cp .env.example .env   # fill in TRANZY_API_KEY and TRANZY_AGENCY_ID
go run .
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend dev server proxies API calls to the backend at `localhost:3000` by default.

### Production build

```bash
cd frontend
npm run build
```

Output goes to `frontend/dist/` — serve it from any static host alongside the backend.

---

## License

MIT
