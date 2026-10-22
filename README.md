# Ce autobuz iau?

A live bus and tram tracker for Cluj-Napoca, București, Iași, Oradea and Constanța. I ride CTP every day and got tired of the official site, so I built the one I actually wanted to use.

**[ceautobuziau.ro](https://ceautobuziau.ro)** (formerly Conexiuni Cluj)

![Universitate in București: metro and bus departures from every platform together, and the stations around it on the map](readme/desktop-stop.png)

## What it does

- Live buses, trams and trolleybuses on the map
- Departures for any stop. Tracked buses get a live countdown, the rest show approximate times
- Stations with every platform's departures in one list, and a metro, tram, trolleybus or bus icon
- The times get better on their own: the server watches the buses and learns how long each stretch really takes, hour by hour
- Full timetables for every line, from the operator
- A route planner: leave now, leave at, arrive by, or the last one today
- Step-by-step guidance for a planned trip
- Follow a bus and get told when it comes, and reminders for the buses you take often (experimental)
- Favorite stops, places and lines
- Operator news, and a notification when a line you follow changes
- Installable as an app. After the first visit every stop, line and timetable opens without the server: the whole network comes down once (about 180 KB for Cluj) and only the buses stay live
- Move your favorites and settings to another phone, the Android app or your watch by scanning a code
- Works from a hardware keyboard too, no arrow keys needed (hjkl and letter shortcuts)

<p align="center">
  <img src="readme/phone-home.png" width="200" alt="Favorite lines and stations in București, the metro stations marked with an M">
  <img src="readme/phone-route.png" width="200" alt="Line 25's stops with live arrival times, a favorite stop marked with a heart">
  <img src="readme/phone-planner.png" width="200" alt="A planned trip from the dorm to the university, ready to start">
  <img src="readme/phone-timetable.png" width="200" alt="A line's timetable, with the times already gone faded out">
</p>

### Light and dark

<p align="center">
  <img src="readme/theme-default.png" width="380" alt="The app in light mode on the left and dark mode on the right">
</p>

### Apps

There's an [Android app](https://github.com/bmarian/conexiuni-cluj-mobile) and a [Wear OS app](https://github.com/bmarian/conexiuni-cluj-wearos) too.

### On a Playdate

There's a [Playdate companion](https://github.com/bmarian/conexiuni-cluj-playdate) for Cluj too. It syncs once over Wi-Fi, then stops, lines and timetables work offline.

<p align="center">
  <img src="readme/playdate-station.png" width="260" alt="A stop on the Playdate with the next arrivals">
  <img src="readme/playdate-route.png" width="260" alt="Line 25 on the Playdate, with the bus moving between stops">
  <img src="readme/playdate-timetable.png" width="260" alt="Line 25's weekday timetable on the Playdate">
</p>

---

## Running it yourself

You'll need Node 22.12+, Go 1.26+, and a [Tranzy](https://tranzy.ai/) API key for each city except București.

Put your keys in a `keys.env` next to this README.

```bash
TRANZY_API_KEY_CLUJ=your_tranzy_key
TRANZY_API_KEY_IASI=your_tranzy_key
TRANZY_API_KEY_ORADEA=your_tranzy_key
TRANZY_API_KEY_CONSTANTA=your_tranzy_key
```

Then the backend and the frontend, each in its own terminal:

```bash
cd backend
go run .
```

```bash
cd frontend
npm install
npm run dev
```

The backend listens on `:6698` and the dev server proxies `/api` to it. Everything else has a default in [`.env`](.env). Cities listed in `db/disabled-cities.json` are switched off.

**The network bundle** (`/api/{city}/network`) is every line with its timetable, stop sequence and learned travel times for each hour of each kind of day, plus every stop and the station it belongs to. Every client builds stop pages, line pages and timetables from it; there are no per-stop or per-line endpoints. `/api/{city}/network/version` tells it when the lines changed. The learned travel times are read once per quarter hour, so those answers hold still between refreshes. **Arrivals** come from `/api/{city}/live`: the server snaps every bus once per poll, predicts its arrival at each downstream stop, follows it onto its next trip at a through-running terminus and merges the result with the timetable into final rows per trip and stop. Subscribe by `stop_ids` and `trip_ids` for an SSE stream (a snapshot, then only the rows and buses that changed, gzipped), or add `once=1` for a single JSON answer. Shapes come as encoded polylines with `/api/{city}/shapes?format=polyline`. Older devices: [docs/migration-embedded.md](docs/migration-embedded.md).

**Push notifications** need a VAPID key pair in `keys.env` (`VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, and `VAPID_SUBJECT` as a contact email or URL). Without it the server runs with push turned off.

**The route planner**, place search and station names read `backend/services/osm/romania-latest.osm.pbf`, which [`update.sh`](update.sh) downloads from Geofabrik (`--update-pbf` refreshes it).

**Production** is `npm run build` in `frontend/`, which writes to `backend/dist/`, and the Go server serves it directly. `update.sh` is what I deploy with.

## Credits

| Source                                                                             | Used for                                                                                     |
| ---------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| [Tranzy.ai](https://tranzy.ai/)                                                    | Live positions, lines and stops for Cluj-Napoca, Iași, Oradea and Constanța                 |
| [CTP Cluj-Napoca](https://ctpcj.ro/)                                               | Cluj-Napoca's timetables and news                                                            |
| [gtfs.ro](https://gtfs.ro/)                                                        | Timetables for Iași, Oradea and Constanța                                                    |
| [TPBI](https://tpbi.ro/)                                                           | Lines, timetables, live positions and news for București                                     |
| [CTP Iași](https://www.sctpiasi.ro/), [OTL](https://www.otl.ro/), [CT BUS](https://www.ctbus.ro/) | News for Iași, Oradea and Constanța                                           |
| [OpenStreetMap](https://www.openstreetmap.org/) contributors                       | Map data, walking routes, place search, © OpenStreetMap contributors, [ODbL](https://opendatacommons.org/licenses/odbl/) |
| [Geofabrik](https://www.geofabrik.de/)                                             | OpenStreetMap extract                                                                        |
| [OpenFreeMap](https://openfreemap.org/), [OpenMapTiles](https://openmaptiles.org/) | Map tiles                                                                                    |
| [MapLibre](https://maplibre.org/)                                                  | The map                                                                                      |
