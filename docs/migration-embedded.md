## 1. Ask the server for the arrival times

```
GET /api/live?stop_ids=391&once=1
```

```json
{
  "as_of": 1760246400,
  "rows": [
    {
      "trip_id": "5_0",
      "stop_id": 391,
      "entries": [
        { "at": 1760246520, "live": true, "vehicle_id": 1234 },
        { "at": 1760247240, "live": false },
        { "at": 1760248140, "live": false }
      ]
    },
    {
      "trip_id": "8_1",
      "stop_id": 391,
      "entries": [
        { "at": 1760246410, "live": true, "soon": true, "vehicle_id": 877 }
      ]
    }
  ],
  "vehicles": []
}
```

- One row per trip that serves the stop and has a timetable; at most 3 entries per row, soonest first, nothing beyond 8 hours.
- `at` is the predicted arrival as unix seconds. Show `round((at - now) / 60)` minutes, clamped at 0; nothing else needs computing between fetches.
- `live: true` means a tracked bus; the server has already snapped it to the route, learned the travel times, followed it onto its next trip at a through-running terminus and dropped timetable runs it covers. `live: false` is the timetable corrected by the learned delays. Nothing needs the 3-minute grace the old offline math needed.
- The route for a row is the number before `_` in `trip_id`. Names and colours come from the bundle below, cached once.
- `vehicles` is empty unless you also pass `trip_ids=`; a display does not need it.
- Fetch once a minute while the display is on. The answer is a few hundred bytes; `Accept-Encoding: gzip` is honoured.

## 2. Stay offline with the bundle

If the device must work from an hourly fetch, `GET /api/network` (a few MB, gzip-compressed) once a day replaces `/api/stop_info`; `GET /api/network/version` says when it changed.

For the stop:

1. Find every `routes[].trips[]` whose `stop_ids` contains the stop. Note the index `k` of the stop in the trip and the trip's position in `trips`.
2. The trip's offsets for the current hour are a slice of `route.offsets[dayType][hour]`: skip `len(stop_ids)` entries for each earlier trip, then take `len(stop_ids)` entries. They are seconds **from the previous stop**; the first entry is the learned departure delay at the terminus.
3. Minutes to the stop = `Σ offsets[0..k]` / 60.
4. Departures: `route.timetable[dayType].entries[].departure_in` for trips ending in `_0`, `departure_out` for `_1`. Arrival = departure + seconds to the stop; round to the minute only when drawing.
5. Keep runs until `rules.late_grace_min` minutes after the predicted arrival: that is how late runs usually are.

`dayType` is `weekday`, `saturday` or `sunday` by the device clock. `stops[]` carries names and coordinates.
