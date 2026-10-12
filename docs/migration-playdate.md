# Playdate companion: moving off `/api/playdate/export`

`/api/playdate/export` is gone. Everything it carried is in the network bundle, which is the same file the website, the phone app and the watch sync. This note maps the old export onto it.

## What to fetch

| Before | Now |
| --- | --- |
| `GET /api/playdate/export` once per sync | `GET /api/network` once per sync (send `Accept-Encoding: gzip`; the body is a few MB uncompressed) |
| re-sync on a timer | `GET /api/network/version` → `{version, static_version, generated_at}`; re-sync when `version` changes |
| `GET /api/shapes?shape_ids=…` | unchanged, `format=polyline` is the compact form |

## Shape of the bundle

```json
{
  "version": "…", "static_version": "…", "generated_at": "2026-10-12T05:00:00Z",
  "routes": [{
    "route_id": 5, "route_short_name": "5", "route_long_name": "P-ța Gării - Zorilor", "route_color": "#…",
    "timetable": {"route_long_name": "…", "in_stop_name": "…", "out_stop_name": "…",
                  "weekdays": {"entries": [{"departure_in": "05:30", "departure_out": "05:45"}, …]},
                  "saturday": {…}, "sunday": {…}},
    "trips": [{"trip_id": "5_0", "stop_ids": [391, 12, …], "stop_sequences": [1, 2, …]},
              {"trip_id": "5_1", "stop_ids": […], "stop_sequences": […]}],
    "offsets": {"weekday": [[…24 arrays…]], "saturday": [[…]], "sunday": [[…]]}
  }],
  "stops": [{"stop_id": 391, "stop_name": "…", "stop_lat": 46.77, "stop_lon": 23.59, …}]
}
```

## Field by field

**`stops`** is the old `stops` list with the same four fields (plus `stop_code`, `stop_desc`, `location_type`, which you can ignore).

**`routes[].directions["0"]`** is now `trips[]` where `trip_id` ends in `_0`; `directions["1"]` is the trip ending in `_1`. A route may have only one of them.

- `stops[]` of a direction → `trip.stop_ids`, in order. Names come from the `stops` list; the old export duplicated them.
- `headsign` → the name of the trip's last stop.
- `hourly_offset_seconds[hour][i]` (cumulative, one array per hour) → `route.offsets[dayType][hour]`, which is **one flat array for the whole route**, trip after trip in `trips` order, and holds the seconds **from the previous stop**, not cumulative. To rebuild the old array for a trip: skip the entries of the trips before it, then take a running sum over its `stop_ids`. The first stop's value is the learned departure delay at the terminus (usually 0 to 120 s), which the old export folded in the same way.
- The old export had one set of hourly offsets. The bundle has one per kind of day (`weekday`, `saturday`, `sunday`); pick the one matching the device clock.

**`timetable`** is unchanged. As before, trips ending in `_0` leave at `departure_in` and trips ending in `_1` leave at `departure_out`.

## Arrivals at a stop

Offline, the math is what it was: `departure + Σ offsets up to the stop`, in the current hour's array, in seconds until it is drawn. Keep a run listed until `rules.late_grace_min` minutes after its predicted arrival: that is how late runs usually are, learned on the server.

If the Playdate is online when a stop is open, `GET /api/live?stop_ids=391&once=1` answers with the server's own prediction, live buses included:

```json
{"as_of": 1760246400,
 "rows": [{"trip_id": "5_0", "stop_id": 391, "entries": [{"at": 1760246520, "live": true, "vehicle_id": 1234}, {"at": 1760247240, "live": false}]}],
 "vehicles": [{"id": 1234, "trip_id": "5_0", "route_id": 5, "latitude": 46.7, "longitude": 23.5, "heading": 182, "speed": 23, "timestamp": "…"}]}
```

`at` is the predicted arrival as unix seconds; show `round((at - now) / 60)`. Every row holds at most 3 entries; a trip with no row has nothing due in the next 8 hours.

## Learned data

The server's learned travel times carry over; offsets differ from an older export by seconds, not minutes. Nothing to do on the Playdate side beyond re-syncing.
