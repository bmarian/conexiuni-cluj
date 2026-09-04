# Conexiuni Cluj — HTTP API

Public transit data for Cluj-Napoca: routes, stops, trips, shapes, live vehicle
positions, CTP timetables, journey planning, and news.

The API is a cached, normalized layer in front of [Tranzy](https://tranzy.ai)
(GTFS + realtime), [CTP Cluj-Napoca](https://ctpcj.ro) (CSV timetables and
news), and a local [OpenTripPlanner](https://www.opentripplanner.org) instance.
It exists so clients get stable JSON, consistent nulls, deterministic route
colors, and heavy caching instead of hitting upstream quotas directly.

---

## Table of contents

- [Quick start](#quick-start)
- [Conventions](#conventions)
  - [Base URL](#base-url)
  - [Scale](#scale)
  - [Content type](#content-type)
  - [Errors](#errors)
  - [Caching and ETags](#caching-and-etags)
  - [Time and timezone](#time-and-timezone)
  - [Rate limits](#rate-limits)
  - [Sentinel values](#sentinel-values)
  - [Known quirks](#known-quirks)
- [Endpoints](#endpoints)
  - [GET /api/routes](#get-apiroutes)
  - [GET /api/stops](#get-apistops)
  - [GET /api/trips](#get-apitrips)
  - [GET /api/shapes](#get-apishapes)
  - [GET /api/stop_times](#get-apistop_times)
  - [GET /api/stop_info](#get-apistop_info)
  - [GET /api/timetable](#get-apitimetable)
  - [GET /api/vehicles](#get-apivehicles)
  - [GET /api/vehicles/stream](#get-apivehiclesstream)
  - [GET /api/plan_routes](#get-apiplan_routes)
  - [GET /api/news](#get-apinews)
  - [GET /api/resolve-location](#get-apiresolve-location)
  - [POST /api/stats/event](#post-apistatsevent)
- [Enumerations](#enumerations)
- [Recipes](#recipes)

---

## Quick start

Base URL: `https://bus.bmarian.online/api`

```bash
# Every route that currently has a published timetable (105 of them)
curl -s https://bus.bmarian.online/api/routes | jq '.[0:3]'

# Everything a stop screen needs, in one call
curl -s 'https://bus.bmarian.online/api/stop_info?stop_id=155' | jq '.stop_name'
# "Unirii"

# Live vehicles on route 25 (route_id 14)
curl -s 'https://bus.bmarian.online/api/vehicles?route_id=14' | jq 'length'

# Plan a trip
curl -s 'https://bus.bmarian.online/api/plan_routes?from_lat=46.7693&from_lng=23.5899&to_lat=46.7772&to_lng=23.6236' | jq '.plans | length'
```

---

## Conventions

### Base URL

`https://bus.bmarian.online/api`

Every example in this document runs against it as written.

### Scale

Approximate payload sizes:

| Endpoint | Items | Body |
|---|---|---|
| `/routes` | 105 | ~19 KB |
| `/stops` | 787 | ~120 KB |
| `/shapes?shape_ids=14_0` | 487 points | ~40 KB |
| `/vehicles` | ~390 | ~90 KB |
| `/plan_routes` (downtown) | 8 itineraries | ~210 KB |

`/plan_routes` is the heaviest response because it inlines the shapes and stops
for every route it used.

### Content type

All responses are JSON, except `/api/vehicles/stream` which is
`text/event-stream`.

Every endpoint is `GET` with query string parameters, except
`POST /api/stats/event`, which takes a JSON body.

### Errors

Errors are a JSON object with a single `error` string, occasionally accompanied
by `retry_after_seconds`.

```json
{ "error": "invalid route_id" }
```

| Status | Meaning |
|---|---|
| `400 Bad Request` | Missing required parameter, or a parameter failed to parse. |
| `422 Unprocessable Entity` | Well-formed but unusable input (e.g. a Maps link with no coordinates). |
| `500 Internal Server Error` | Upstream fetch, database, or enrichment failure. |
| `502 Bad Gateway` | A third-party link could not be resolved. |
| `503 Service Unavailable` | Planner still starting, or news unavailable with no cache. |

`503` from `/api/plan_routes` also sends `Retry-After: 10`. OTP boots lazily and
needs a few seconds after a cold start.

### Caching and ETags

Every `/api` response passes through ETag middleware, except the SSE stream.
Cached endpoints send `Cache-Control: no-cache`, which means "revalidate every
time", not "don't cache". Send the `ETag` you received back as `If-None-Match`
and unchanged bodies short-circuit to `304 Not Modified` with an empty body.

```bash
etag=$(curl -s -D - -o /dev/null https://bus.bmarian.online/api/routes | tr -d '\r' | awk '/^[Ee]tag:/{print $2}')
curl -s -o /dev/null -w '%{http_code} %{size_download}\n' -H "If-None-Match: $etag" https://bus.bmarian.online/api/routes
# 304 0
```

| Endpoint | `Cache-Control` | Server-side TTL |
|---|---|---|
| `/routes`, `/stops`, `/trips`, `/shapes`, `/stop_times`, `/stop_info` | `no-cache` | `24h * 6 / TRANZY_DEFAULT_DAILY_QUOTA` (default 144 → 1h) |
| `/timetable` | `no-cache` | `TIMETABLE_CACHE_SHELF_LIFE` (default `24h`) |
| `/news` | `no-cache` | `NEWS_CACHE_SHELF_LIFE` (default `4h`) |
| `/plan_routes` | `public, max-age=300` | none |
| `/vehicles` | `no-store` | current adaptive vehicle interval |
| `/vehicles/stream` | `no-cache` | n/a |
| `/resolve-location`, `/stats/event` | `no-store` | none |

Server-side data is cached in SQLite. A cache miss triggers an upstream fetch; a
hit is served straight from the database, so most requests never reach Tranzy.

### Time and timezone

The server runs on `Europe/Bucharest`. Unless stated otherwise:

- Timetable clock times are local `HH:MM` strings.
- Vehicle `timestamp` is the upstream Tranzy string (ISO 8601).
- Planner `start_time_ms` / `end_time_ms` are Unix epoch **milliseconds**.

### Rate limits

The API itself is not rate limited, but its upstream is. Tranzy enforces a daily
quota tracked in two independent buckets, persisted in SQLite so a process
restart does not reset it:

| Bucket | Env var | Default | Used by |
|---|---|---|---|
| Static GTFS | `TRANZY_DEFAULT_DAILY_QUOTA` | 144 | routes, stops, trips, shapes, stop_times |
| Realtime | `TRANZY_VEHICLES_DAILY_QUOTA` | 4500 | vehicles, vehicle stream, learning sampler |

Practical consequences:

- Prefer `/api/vehicles/stream` over polling `/api/vehicles` in a loop. The
  stream shares a single upstream poll across every connected subscriber.
- Prefer `/api/stop_info` over assembling the same data from four calls.
- Honor `ETag`. A `304` costs the server almost nothing.

### Sentinel values

Upstream data is patchy. Rather than emitting `null`, missing fields are
normalized to sentinels so clients can use non-nullable types:

| Field | Missing value |
|---|---|
| `latitude`, `longitude`, `speed`, `shape_dist_traveled` | `-1.0` |
| `route_id` (vehicles), `wheelchair_accessible` / `bikes_allowed` (trips) | `-1` |
| `trip_id` (vehicles) | `"-1"` |
| `shape_id` | `"-1.0"` |
| `bike_accessible`, `wheelchair_accessible` (vehicles) | `"UNKNOWN"` |
| `stop_desc`, `stop_code` | `""` |
| `location_type` | `0` |

`route_color` is never upstream-provided. It is derived from the route short name
(FNV-1a hash → HSL → hex), so the same route is always the same color across
clients and sessions.

### Known quirks

| Behaviour | Detail |
|---|---|
| `stop_info` unknown id → `200 null` | Not `404`, not an error object. The body is literal `null`. |
| `stop_times` unknown route → `200 null` | Also `null` rather than `[]`. |
| `timetable` unknown route → `200` stub | Your `route_short_name` echoed back, every other field empty. Treat an empty `route_long_name` as "no such route". |
| `stop_info` trip id arrays can be `null` | `outgoing_trip_ids` / `incoming_trip_ids` serialize as `null`, not `[]`, when a direction has no trips. Stop `155` returns `null` for `incoming_trip_ids`. |
| `plan_routes` bad coords → `400` with a success-shaped body | The body is `{"plans":[],"stops":{},"shapes":{}}`. Check the status code, not the body. |
| `shape_dist_traveled` is always `-1` | Upstream never populates it. Compute distance from the coordinates. |
| `location_type` is always `0` | No station or entrance records are currently served. |

Every list endpoint other than `stop_times` returns `[]` when nothing matches:

```bash
curl -s 'https://bus.bmarian.online/api/routes?route_id=999999'   # []
curl -s 'https://bus.bmarian.online/api/stop_times?route_short_name=NOPE'  # null
```

---

## Endpoints

### `GET /api/routes`

Lists routes. With no parameters, the list is filtered to routes that actually
have a published CTP timetable, so retired routes do not appear. Passing a
filter bypasses that filter and returns the raw match.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `route_id` | integer | no | Return only this route. |
| `route_short_name` | string | no | Return only routes with this short name, e.g. `25`, `M42`. |

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/routes?route_short_name=25'
```

**Response `200`**

```json
[
  {
    "route_id": 14,
    "agency_id": 2,
    "route_short_name": "25",
    "route_long_name": "Str. Bucium - Str. Unirii",
    "route_type": 11,
    "route_desc": "Str. Bucium - Str. Unirii",
    "route_color": "#462EE0"
  }
]
```

`route_short_name` is not numeric. Real values include `24B`, `48L`, `52L`, and
`M26`. Treat it as a string.

**Errors** — `400` if `route_id` is not an integer.

---

### `GET /api/stops`

Lists stops. With no parameters, the list is filtered to stops actually served by
at least one vehicle. Passing `stop_id` bypasses the filter.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `stop_id` | integer | no | Return only this stop. |

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/stops?stop_id=155'
```

**Response `200`**

```json
[
  {
    "stop_id": 155,
    "stop_name": "Unirii",
    "stop_desc": "",
    "stop_lat": 46.76896,
    "stop_lon": 23.62968,
    "location_type": 0,
    "stop_code": ""
  }
]
```

`stop_desc` and `stop_code` are `""` and `location_type` is `0` on every stop
currently served.

**Errors** — `400` if `stop_id` is not an integer.

---

### `GET /api/trips`

Trip metadata: headsign, direction, and the shape that draws the trip on a map.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `route_id` | integer | no | Only trips on this route. |
| `trip_id` | string | no | Only this trip. |

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/trips?route_id=14'
```

**Response `200`**

```json
[
  {
    "trip_id": "14_0",
    "route_id": 14,
    "direction_id": 0,
    "trip_headsign": "Snagov Nord",
    "block_id": 14,
    "shape_id": "14_0",
    "wheelchair_accessible": -1,
    "bikes_allowed": -1
  },
  {
    "trip_id": "14_1",
    "route_id": 14,
    "direction_id": 1,
    "trip_headsign": "Disp. Clabucet",
    "block_id": 14,
    "shape_id": "14_1",
    "wheelchair_accessible": -1,
    "bikes_allowed": -1
  }
]
```

`direction_id` is `0` for outbound (tur) and `1` for inbound (retur).

Trip IDs are normalized to `{route_id}_{direction_id}`, so there are exactly two
trips per route and `shape_id` always equals `trip_id`. Shape and trip IDs can be
constructed without a lookup: route 14 inbound is `14_1`.

`wheelchair_accessible` and `bikes_allowed` are `-1` (unknown) on every trip
currently served. Vehicle-level accessibility from `/vehicles` is populated.

**Errors** — `400` if `route_id` is not an integer.

---

### `GET /api/shapes`

Polyline points for drawing routes. Points are returned as individual rows; sort
by `shape_pt_sequence` before drawing.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `shape_id` | string | no | A single shape. |
| `shape_ids` | string | no | Comma-separated list of shape IDs. Blank entries are ignored. |

Omitting both returns every shape, which is large. Prefer `shape_ids`.

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/shapes?shape_ids=14_0,14_1'
```

**Response `200`**

```json
[
  {
    "shape_id": "14_0",
    "shape_pt_lat": 46.75123,
    "shape_pt_lon": 23.54317,
    "shape_pt_sequence": 0,
    "shape_dist_traveled": -1
  },
  {
    "shape_id": "14_0",
    "shape_pt_lat": 46.75125,
    "shape_pt_lon": 23.5432,
    "shape_pt_sequence": 1,
    "shape_dist_traveled": -1
  }
]
```

One direction of route 25 is 487 points. Fetching both directions of one route is
about 80 KB, so request only the shapes you will draw.

`shape_dist_traveled` is `-1` on every point currently served. Compute cumulative
distance from the coordinates if you need it.

---

### `GET /api/stop_times`

Per-trip stop sequences for one route, enriched with travel-time estimates.

`offset_arrival_time` is the estimated travel time in seconds **from the previous
stop in the same trip** — not cumulative, not absolute. The first stop of every
trip is `0`. For an ETA further along, sum the offsets forward from the vehicle's
current position.

Offsets come from one of two sources:

1. **Geometric estimate** — distance along the shape, rounded up to the whole
   second, plus a flat 30s penalty on the segment after the first stop.
   `offset_confidence` is `0`.
2. **Learned profile** — the backend samples live vehicle positions to build
   per-segment, time-of-day travel profiles. When a profile exists it replaces
   the geometric estimate and `offset_confidence` is greater than `0`.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `route_short_name` | string | **yes** | e.g. `25`. |
| `ref_hour` | integer `0`–`23` | no | Evaluate learned profiles as if it were this hour today. Useful for previewing rush-hour timings. Out-of-range or non-numeric values are ignored. |

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/stop_times?route_short_name=25&ref_hour=8'
```

**Response `200`**

```json
[
  {
    "trip_id": "14_0",
    "stop_id": 1,
    "offset_arrival_time": 0,
    "offset_confidence": 0,
    "stop_sequence": 0,
    "stop_headsign": "Disp. Clăbucet",
    "route_short_name": "25",
    "stop_lat": 46.75144,
    "stop_lon": 23.54292
  },
  {
    "trip_id": "14_0",
    "stop_id": 2,
    "offset_arrival_time": 100,
    "offset_confidence": 0.9212598425196851,
    "stop_sequence": 1,
    "stop_headsign": "Primăverii",
    "route_short_name": "25",
    "stop_lat": 46.75181,
    "stop_lon": 23.54553
  }
]
```

The response covers both directions — 33 rows for route 25, spanning trips `14_0`
and `14_1`. Group by `trip_id` and sort by `stop_sequence`.

`stop_headsign` is the name of that stop, not the trip's destination, despite the
GTFS field name.

`ref_hour` changes the learned estimates. For stop 2 of trip `14_0`,
`offset_confidence` is `0.9310` with no `ref_hour` and `0.9213` at `ref_hour=8`.

**Errors** — `400` if `route_short_name` is missing. An unknown
`route_short_name` returns `200` with a body of literal `null`, not `[]`.

---

### `GET /api/stop_info`

The aggregate endpoint that powers the stop screen. One call returns the stop,
the trips serving it split by direction, and for every route at that stop the
full stop sequence plus the CTP timetable. Use this instead of fanning out to
`/stops`, `/trips`, `/stop_times`, and `/timetable`.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `stop_id` | integer | **yes** | The stop to describe. |

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/stop_info?stop_id=155'
```

**Response `200`** (abridged — the real body is ~40 KB)

```json
{
  "stop_id": 155,
  "stop_name": "Unirii",
  "stop_desc": "",
  "stop_lat": 46.76896,
  "stop_lon": 23.62968,
  "location_type": 0,
  "stop_code": "",
  "outgoing_trip_ids": ["12_0", "13_0"],
  "incoming_trip_ids": null,
  "shapes_info": [
    {
      "route_short_name": "24",
      "route_long_name": "Str. Unirii - Str. Bucium",
      "route_id": 12,
      "route_type": 3,
      "route_color": "#C5DB1F",
      "stop_time": [
        {
          "trip_id": "12_0",
          "stop_id": 154,
          "offset_arrival_time": 0,
          "offset_confidence": 0,
          "stop_sequence": 0,
          "stop_headsign": "Disp. Unirii",
          "route_short_name": "24",
          "stop_lat": 46.76814,
          "stop_lon": 23.6302
        },
        {
          "trip_id": "12_0",
          "stop_id": 155,
          "offset_arrival_time": 62,
          "offset_confidence": 0.36078431372549025,
          "stop_sequence": 1,
          "stop_headsign": "Unirii",
          "route_short_name": "24",
          "stop_lat": 46.76896,
          "stop_lon": 23.62968
        }
      ],
      "timetable": { "…": "a full Timetable object, see /api/timetable" }
    },
    {
      "route_short_name": "24B",
      "route_long_name": "Str. Unirii - Vivo Center",
      "route_id": 13,
      "route_type": 3,
      "route_color": "#A119D7",
      "stop_time": [],
      "timetable": {}
    }
  ]
}
```

The JSON key is `stop_time` (singular) even though it holds an array. Each entry
holds every stop of that route in both directions — 36 rows per route at this
stop — so filter to the trip you need.

`outgoing_trip_ids` and `incoming_trip_ids` are what you pass to
`/api/vehicles/stream?trip_ids=...`. Both serialize as `null`, not `[]`, when the
stop has no trips in that direction; stop 155 returns `"incoming_trip_ids":
null`. Normalize before use.

**Errors** — `400` if `stop_id` is missing or not an integer. An unknown
`stop_id` returns `200` with a body of literal `null`, not `404`.

---

### `GET /api/timetable`

The scheduled timetable for one route, scraped and parsed from the CTP CSV
exports.

#### Reading the two directions

Each day (`weekdays`, `saturday`, `sunday`) describes both directions at once,
and each direction is independently either explicit or frequency-based:

- `entries[].departure_out` holds explicit departure times from `out_stop_name`,
  and `entries[].departure_in` from `in_stop_name`.
- When a direction has no published times, its column is an empty string on
  every entry, and `out_frequency` / `in_frequency` describes it as a service
  window with an "every N–M minutes" headway instead. Those keys are omitted
  when the direction has explicit times.

A non-empty `entries` therefore does not mean both directions have explicit
times. Resolve each direction separately: if `out_frequency` is present, the
outbound direction is headway-based; otherwise read the non-empty
`departure_out` values. Same for `in_frequency` / `departure_in`.

`service_start` is the date the schedule took effect, formatted `DD.MM.YYYY`. It
is not a clock time.

`service_name` is free text from CTP and is inconsistently accented and spaced —
`Luni-Vineri`, `Luni - Vineri`, `Sambata`, `Duminica` have all been observed.
Match on the field name, not the value.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `route_short_name` | string | no (effectively yes) | e.g. `25`. Omitting it queries CTP for an empty route name and yields an empty timetable. |

**Request** — both directions explicit

```bash
curl -s 'https://bus.bmarian.online/api/timetable?route_short_name=25'
```

```json
{
  "route_short_name": "25",
  "route_long_name": "Str. Bucium - Str. Unirii",
  "in_stop_name": "Disp. Clabucet",
  "out_stop_name": "Snagov Nord",
  "weekdays": {
    "service_name": "Luni-Vineri",
    "service_start": "11.08.2026",
    "entries": [
      { "departure_in": "05:03", "departure_out": "05:00" },
      { "departure_in": "05:17", "departure_out": "05:15" },
      { "departure_in": "05:29", "departure_out": "05:30" }
    ]
  },
  "saturday": {
    "service_name": "Sambata",
    "service_start": "15.08.2026",
    "entries": [
      { "departure_in": "05:22", "departure_out": "05:30" },
      { "departure_in": "05:37", "departure_out": "05:40" }
    ]
  },
  "sunday": {
    "service_name": "Duminica",
    "service_start": "16.08.2026",
    "entries": [{ "departure_in": "05:58", "departure_out": "06:00" }]
  }
}
```

**Request** — one direction explicit, the other frequency-based

```bash
curl -s 'https://bus.bmarian.online/api/timetable?route_short_name=M26'
```

```json
{
  "route_short_name": "M26",
  "route_long_name": "Cluj-Napoca - Floresti / Cetate",
  "in_stop_name": "M-Floresti Cetate Pl.",
  "out_stop_name": "Cluj-Napoca",
  "weekdays": {
    "service_name": "Luni - Vineri",
    "service_start": "17.08.2026",
    "entries": [
      { "departure_in": "04:45", "departure_out": "" },
      { "departure_in": "05:00", "departure_out": "" },
      { "departure_in": "05:10", "departure_out": "" }
    ],
    "out_frequency": {
      "start": "05:00",
      "end": "22:40",
      "min_minutes": 10,
      "max_minutes": 20
    }
  },
  "saturday": {
    "service_name": "Sambata",
    "service_start": "22.08.2026",
    "entries": [
      { "departure_in": "05:10", "departure_out": "" },
      { "departure_in": "06:10", "departure_out": "" }
    ],
    "out_frequency": { "start": "05:45", "end": "22:40", "min_minutes": 10, "max_minutes": 20 }
  },
  "sunday": {
    "service_name": "Duminica",
    "service_start": "23.08.2026",
    "entries": [{ "departure_in": "06:10", "departure_out": "" }],
    "out_frequency": { "start": "05:45", "end": "22:40", "min_minutes": 10, "max_minutes": 20 }
  }
}
```

Every `departure_out` is `""` here; the outbound direction lives entirely in
`out_frequency`. Frequency-based directions are rare — 3 of 105 routes, all
`M26`.

**Errors** — an unknown `route_short_name` returns `200` with a stub object:
your value echoed back in `route_short_name` and every other field empty. Treat
an empty `route_long_name` as "no such route".

---

### `GET /api/vehicles`

A single snapshot of live vehicle positions. Always `Cache-Control: no-store`.

For anything that updates continuously, use the SSE stream instead. Polling this
endpoint burns the realtime quota linearly with the number of clients, while the
stream shares one upstream poll across all of them.

Filters combine: `route_id` narrows to a route, `trip_id` / `trip_ids` narrow to
specific trips.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `route_id` | integer | no | Only vehicles on this route. |
| `trip_id` | string | no | Only vehicles on this trip. |
| `trip_ids` | string | no | Comma-separated trip IDs. |

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/vehicles?route_id=14'
```

**Response `200`**

```json
[
  {
    "id": 348,
    "label": "113",
    "latitude": 46.75165,
    "longitude": 23.5438316,
    "timestamp": "2026-09-04T11:23:43.000Z",
    "vehicle_type": 11,
    "bike_accessible": "BIKE_INACCESSIBLE",
    "wheelchair_accessible": "WHEELCHAIR_ACCESSIBLE",
    "speed": 12.849898699122617,
    "route_id": 14,
    "trip_id": "14_0"
  },
  {
    "id": 283,
    "label": "101",
    "latitude": 46.7685916,
    "longitude": 23.58458,
    "timestamp": "2026-09-04T12:45:15.000Z",
    "vehicle_type": 11,
    "bike_accessible": "BIKE_INACCESSIBLE",
    "wheelchair_accessible": "WHEELCHAIR_ACCESSIBLE",
    "speed": 15.211615739815663,
    "route_id": 14,
    "trip_id": "14_1"
  }
]
```

- `label` is the fleet number painted on the vehicle, as a string. It is not
  unique across vehicle types; use `id`.
- `speed` is km/h, unrounded.
- `timestamp` is ISO 8601 UTC with milliseconds: `2026-09-04T11:23:43.000Z`. It
  is the upstream GPS fix time and can lag the request by minutes — the two
  vehicles above differ by over an hour.
- A vehicle with `latitude` / `longitude` of `-1` reported no position.
- `trip_id` of `"-1"` and `route_id` of `-1` mean the vehicle is not on an
  assigned trip.

An unfiltered `/api/vehicles` returns around 390 vehicles: roughly 282 buses
(`vehicle_type` 3), 83 trolleybuses (11), and 26 trams (0).

**Errors** — `400` if `route_id` is not an integer. Unmatched filters return
`[]`.

---

### `GET /api/vehicles/stream`

Server-Sent Events stream of live vehicle positions for a fixed set of trips.

The server maintains one shared upstream poller. Its interval adapts to the time
of day using the `VEHICLE_SCHEDULE` / `VEHICLE_SCHEDULE_WEEKEND` slot config, so
updates are frequent during service hours and sparse overnight.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `trip_ids` | string | **yes** | Comma-separated trip IDs. At least one non-empty ID required. |

**Response headers**

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

**Message format** — each `data:` line is a JSON array of `Vehicle` objects: the
full current set for the subscribed trips, not a delta. Replace local state with
each batch rather than merging.

A full snapshot arrives immediately on connect, so no separate `/api/vehicles`
call is needed to prime the UI.

```
data: [{"id":348,"label":"113","latitude":46.75165,"longitude":23.5438316,"timestamp":"2026-09-04T11:23:43.000Z","vehicle_type":11,"bike_accessible":"BIKE_INACCESSIBLE","wheelchair_accessible":"WHEELCHAIR_ACCESSIBLE","speed":12.849898699122617,"route_id":14,"trip_id":"14_0"}]

: ping

data: [{"id":348,"label":"113","latitude":46.75201,"longitude":23.5442,"timestamp":"2026-09-04T12:47:11.000Z","vehicle_type":11,"bike_accessible":"BIKE_INACCESSIBLE","wheelchair_accessible":"WHEELCHAIR_ACCESSIBLE","speed":14.2,"route_id":14,"trip_id":"14_0"}]
```

A `: ping` comment is sent every 25 seconds so proxies do not close an idle
connection. `EventSource` ignores comment lines automatically.

There are no named event types; everything arrives as the default `message`
event. To change which trips you follow, close the connection and open a new one.

```bash
curl -N 'https://bus.bmarian.online/api/vehicles/stream?trip_ids=14_0,14_1'
```

**Errors** — `400` if `trip_ids` is missing or empty.

---

### `GET /api/plan_routes`

Journey planning between two coordinates, backed by OpenTripPlanner and enriched
with this API's own stop, route, and timetable data.

The response is normalized so a client can render everything without extra calls:
`plans` reference stops and routes by ID, and the `stops` and `shapes` maps
contain every referenced entity.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `from_lat` | float | **yes** | Origin latitude, `-90`–`90`. |
| `from_lng` | float | **yes** | Origin longitude, `-180`–`180`. |
| `to_lat` | float | **yes** | Destination latitude. |
| `to_lng` | float | **yes** | Destination longitude. |
| `time` | string | no | Departure or arrival time. Accepts RFC3339 (`2026-09-04T15:04:05+03:00`) or local `2026-09-04T15:04` / `2026-09-04T15:04:05`. Defaults to now. |
| `arrive_by` | `true` / `1` | no | Treat `time` as the desired arrival time instead of departure. Anything else means depart-at. |

NaN and infinity are rejected, as are coordinates outside valid geographic
ranges.

A walk-only itinerary is requested separately and appended when available, so
short trips always offer a "just walk" option alongside transit.

**Request**

```bash
curl -s 'https://bus.bmarian.online/api/plan_routes?from_lat=46.7693&from_lng=23.5899&to_lat=46.7772&to_lng=23.6236&time=2026-09-04T18:00&arrive_by=true'
```

**Response `200`**

```json
{
  "plans": [
    {
      "legs": [
        {
          "route_id": 13,
          "trip_id": "13_1",
          "start_stop_id": 8,
          "dest_stop_id": 171,
          "ride_seconds": 409,
          "intermediate_stop_ids": [9, 10, 168, 169, 170]
        }
      ],
      "is_direct": true,
      "walk_start_meters": 267.24,
      "walk_end_meters": 367.4,
      "walk_transfer_meters": 0,
      "transit_duration_sec": 409,
      "total_distance": 4029.34,
      "generalized_cost": 4125,
      "number_of_transfers": 0,
      "start_time_ms": 1788526033000,
      "end_time_ms": 1788526962000,
      "walk_segments": [
        { "geometry": "_um|Gqj~nCLj@Rx@FRuA…", "distance_m": 267.24, "duration_sec": 213 },
        { "geometry": "who|GugdoCD?CQGBq@RE…", "distance_m": 367.4, "duration_sec": 307 }
      ]
    }
  ],
  "stops": {
    "8": {
      "stop_id": 8,
      "stop_name": "Piata Mihai Viteazu",
      "stop_desc": "",
      "stop_lat": 46.77116,
      "stop_lon": 23.59176,
      "location_type": 0,
      "stop_code": ""
    }
  },
  "shapes": {
    "1": {
      "route_short_name": "1",
      "route_long_name": "Str. Bucium - P-ta 1 Mai",
      "route_id": 1,
      "route_type": 11,
      "route_color": "#C62F24",
      "stop_time": [{ "…": "26 StopTime objects" }],
      "timetable": { "…": "a full Timetable object" }
    }
  }
}
```

The query above returns 8 itineraries, 7 stops, and 15 shapes — about 210 KB.
This is the heaviest endpoint; cache it client-side for the 300s the
`Cache-Control` header allows.

**Field notes**

| Field | Meaning |
|---|---|
| `legs` | Transit legs only. A walk-only itinerary has an empty `legs` array. |
| `is_direct` | No transfers. |
| `walk_start_meters` / `walk_end_meters` | Access and egress walking distance. |
| `walk_transfer_meters` | Walking between transit legs. |
| `transit_duration_sec` | Time spent riding, excluding walking and waiting. |
| `generalized_cost` | OTP's internal comparison score. Lower is better. Use it to rank, not to display. |
| `start_time_ms` / `end_time_ms` | Unix epoch milliseconds for the whole itinerary. |
| `walk_segments[].geometry` | Google encoded polyline, precision 5. One segment per walking portion, in order. |
| `stops` | Keyed by stop ID as a string. Contains every stop referenced by any leg, including `intermediate_stop_ids`. |
| `shapes` | Keyed by **route ID** as a string, not trip ID. Look up `shapes[String(leg.route_id)]`. |

Each `shapes` entry carries a populated `stop_time` array and `timetable` object,
so a route detail view can be rendered from the plan response alone.

An empty result is a `200` with `"plans": []`, not an error.

**Errors**

- `400` — missing or unparseable coordinates, out-of-range coordinates, or an
  unparseable `time`. Bad coordinates currently return a success-shaped body,
  `{"plans":[],"stops":{},"shapes":{}}`, instead of `{"error": "..."}`. Branch on
  the status code, not on the presence of an `error` key.
- `500` — `{"error": "routing failed"}` when OTP or enrichment fails.
- `503` — `{"error": "planner still starting", "retry_after_seconds": 10}` or
  `{"error": "planner unavailable", "retry_after_seconds": 10}`, with a
  `Retry-After: 10` header. Retry rather than surfacing an error.

---

### `GET /api/news`

Latest CTP Cluj-Napoca announcements, scraped from the CTP news page and cached
in SQLite for `NEWS_CACHE_SHELF_LIFE` (default 4h). If the scrape fails but a
cached copy exists, the stale copy is served with `200` rather than an error.

**Parameters** — none.

**Request**

```bash
curl -s https://bus.bmarian.online/api/news
```

**Response `200`**

```json
[
  {
    "url": "https://www.ctpcj.ro/index.php/ro/despre-noi/stiri/abonamente-transport-elevi-2026-2027/1745",
    "date": "1 Sep 2026",
    "title": "Transport public gratuit pentru elevi în anul școlar 2026-2027"
  },
  {
    "url": "https://www.ctpcj.ro/index.php/ro/despre-noi/stiri/relocare-statia-autotransilvania-floresti/1872",
    "date": "28 Aug 2026",
    "title": "Relocare stația AutoTransilvania - Florești"
  }
]
```

`date` is free text passed through from the source markup — `"1 Sep 2026"`, not
ISO 8601. Titles are Romanian. Items are ordered newest first; the endpoint
currently returns 6.

**Errors** — `503` `{"error": "news unavailable"}` when the scrape fails and
nothing is cached.

---

### `GET /api/resolve-location`

Resolves a shortened Google Maps link to coordinates, so a shared location link
can be used as a planner origin or destination.

This is not a general URL fetcher. Only `https://maps.app.goo.gl/<id>` links are
accepted; anything else is rejected without a network request. Redirects are not
followed automatically — a single `HEAD` is issued with a 5 second timeout and
the `Location` header is parsed.

Coordinates are extracted in priority order: the pinned location (`!3d..!4d..`),
then the camera center (`/@lat,lon`), then `?q=lat,lon`. The place name from
`/maps/place/<name>` becomes `label` when present, otherwise `label` is the
formatted coordinate pair.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `url` | string | **yes** | Must match `https?://maps.app.goo.gl/<alphanumeric>`. |

**Request**

```bash
curl -s --get https://bus.bmarian.online/api/resolve-location \
  --data-urlencode 'url=https://maps.app.goo.gl/AbC123xyz'
```

**Response `200`**

```json
{ "lat": 46.769379, "lon": 23.589957, "label": "Piața Unirii" }
```

**Errors**

| Status | Body |
|---|---|
| `400` | `{"error": "url required"}` |
| `400` | `{"error": "only maps.app.goo.gl links are supported"}` |
| `400` | `{"error": "invalid url"}` |
| `502` | `{"error": "failed to resolve link"}` |
| `422` | `{"error": "no coordinates found in link"}` |

---

### `POST /api/stats/event`

First-party analytics for PWA installs. Same-origin only, with an allowlist of
exactly one event.

The endpoint always returns `204 No Content`, including for rejected, malformed,
or cross-origin requests. It never confirms whether the event was recorded.

**Request body**

```json
{ "metric": "pwa_install", "key": "appinstalled" }
```

Any other `metric` / `key` pair is silently discarded.

**Request**

```bash
curl -s -o /dev/null -w '%{http_code}\n' \
  -X POST https://bus.bmarian.online/api/stats/event \
  -H 'Content-Type: application/json' \
  -d '{"metric":"pwa_install","key":"appinstalled"}'
# 204
```

Events are attributed to an HMAC-pseudonymized client ID derived from the request
and `LOG_IP_HASH_SALT`. Raw IP addresses are never stored.

---

## Enumerations

### `route_type` (GTFS)

| Value | Meaning |
|---|---|
| `0` | Tram |
| `1` | Subway |
| `2` | Rail |
| `3` | Bus |
| `4` | Ferry |
| `5` | Cable tram |
| `6` | Aerial lift |
| `7` | Funicular |
| `11` | Trolleybus |
| `12` | Monorail |

Only `0`, `3`, and `11` occur in Cluj.

### `location_type` (GTFS)

| Value | Meaning |
|---|---|
| `0` | Stop or platform |
| `1` | Station |
| `2` | Entrance / exit |
| `3` | Generic node |
| `4` | Boarding area |

Every stop currently served is `0`.

### `direction_id`

| Value | Meaning |
|---|---|
| `0` | Outbound (tur) |
| `1` | Inbound (retur) |

### Accessibility strings (vehicles)

| Field | Values |
|---|---|
| `bike_accessible` | `BIKE_ACCESSIBLE`, `BIKE_INACCESSIBLE` |
| `wheelchair_accessible` | `WHEELCHAIR_ACCESSIBLE`, `WHEELCHAIR_INACCESSIBLE` |

The suffix is `INACCESSIBLE`, not `NOT_ACCESSIBLE`. `"UNKNOWN"` is substituted
when upstream omits the field. Treat unrecognized values as unknown.

### Accessibility integers (trips)

`wheelchair_accessible` and `bikes_allowed` on trips follow GTFS: `0` unknown,
`1` accessible, `2` not accessible, `-1` when upstream omitted the field. In
practice **every trip currently returns `-1`** — use the vehicle-level strings
instead.

---

## Recipes

### Render a route on a map

```bash
# 1. Resolve the route
curl -s 'https://bus.bmarian.online/api/routes?route_short_name=25'
#    -> route_id 14

# 2. Get its trips to find shape IDs and directions
curl -s 'https://bus.bmarian.online/api/trips?route_id=14'
#    -> 14_0 (Snagov Nord), 14_1 (Disp. Clabucet)

# 3. Fetch both direction shapes in one call
curl -s 'https://bus.bmarian.online/api/shapes?shape_ids=14_0,14_1'

# 4. Follow the vehicles on those trips
curl -N 'https://bus.bmarian.online/api/vehicles/stream?trip_ids=14_0,14_1'
```

Because trip IDs are `{route_id}_{direction_id}`, step 2 is skippable if you only
need the geometry: `shape_ids=14_0,14_1` is derivable from the route ID alone.

### Build a departure board for a stop

```bash
curl -s 'https://bus.bmarian.online/api/stop_info?stop_id=155'
```

Then, per entry in `shapes_info`:

1. Pick today's `DaySchedule` from `timetable` (weekdays / Saturday / Sunday).
2. For each direction independently: if `out_frequency` is present, show
   "every N–M min" within its window; otherwise find the next non-empty
   `departure_out` after now. Repeat with `in_frequency` / `departure_in`.
3. Open the vehicle stream on `outgoing_trip_ids` (which may be `null`) and refine
   using `stop_time[].offset_arrival_time` summed from the vehicle's current stop.

### Estimate arrival at a downstream stop

`offset_arrival_time` is per-segment. Given a vehicle at `stop_sequence` `k` on
trip `T` and a target at sequence `n`:

```
eta_seconds = sum(offset_arrival_time for stops in T where k < stop_sequence <= n)
```

`offset_confidence` of `0` means a purely geometric guess; higher values mean the
segment has a learned profile behind it.

