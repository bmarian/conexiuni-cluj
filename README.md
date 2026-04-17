# Conexiuni Cluj

Real-time public transit tracker for Cluj-Napoca. Shows live bus/tram positions, next departures, full timetables, and stop information — all in a mobile-first web app.

## Features

- **Live vehicle tracking** — real-time bus and tram positions via Server-Sent Events
- **Next departures** — per-stop departure times with live ETA overrides when a vehicle is tracked
- **Full timetables** — weekday / Saturday / Sunday schedules; tap any departure to see the full trip with arrival times at every stop
- **Favorites** — save and drag-to-reorder favorite routes and stops, persisted locally
- **Offline-capable** — API responses cached in IndexedDB (shapes/routes cached for 7 days)
- **Dark mode** — follows system preference
- **Bilingual** — Romanian and English

## Stack

| Layer | Tech |
|-------|------|
| Frontend | Vue 3 + Vite, Pinia, Vue Router, vue-i18n, Tailwind CSS, Leaflet |
| Backend | Go + Fiber v3, SQLite |
| Transit data | [Tranzy API](https://tranzy.ai) (GTFS), CTP Cluj timetable CSVs |

## Getting started

### Prerequisites

- Node ≥ 20.19 or ≥ 22.12
- Go ≥ 1.25
- A Tranzy API key

### Environment

Copy `.env` and fill in your values:

```
ENV=development

TRANZY_BASE_URL=https://api.tranzy.ai/v1/opendata
CLUJ_AGENCY_ID=2
TRANZY_RATE_LIMIT=200ms
TRANZY_VEHICLES_DAILY_QUOTA=4500
TRANZY_DEFAULT_DAILY_QUOTA=500

CTP_CSV_BASE_URL=https://ctpcj.ro/orare/csv
CTP_CJ_RATE_LIMIT=1s

PORT=6698
DATABASE_PATH=../conexiuni-cluj.db
LOG_FILE_PATH=../conexiuni-cluj.log

VEHICLE_CACHE_SHELF_LIFE=20s
SHAPE_CACHE_SHELF_LIFE=168h
ROUTE_CACHE_SHELF_LIFE=168h
STOP_CACHE_SHELF_LIFE=168h
```

### Development

```bash
./dev.sh
```

Frontend → `http://localhost:5173`  
Backend API → `http://localhost:6698`

### Production build

```bash
./build.sh
cd backend && ./conexiuni-cluj
```

The server serves both the API and the built frontend at `http://localhost:6698`.

## Project structure

```
conexiuni-cluj/
├── frontend/
│   └── src/
│       ├── views/         # HomeView, StopView, RouteView, NotFoundView
│       ├── stores/        # Pinia stores (favorites, map, route, user)
│       ├── composables/   # API hooks, vehicle tracking, SSE stream
│       ├── components/    # MapComponent
│       ├── locales/       # ro.json, en.json
│       └── utils/         # Cache, time, geo helpers
└── backend/
    ├── handlers/          # HTTP route handlers
    ├── services/          # Tranzy + CTP Cluj API wrappers
    ├── models/            # Data models
    └── database/          # SQLite access
```

## Data sources

- **[Tranzy](https://tranzy.ai)** — GTFS static data (routes, stops, shapes, stop times) and live vehicle positions for Cluj agency ID `2`
- **[CTP Cluj](https://ctpcj.ro)** — official timetable CSVs scraped per route (`orar_{route}_{lv|s|d}.csv`)
