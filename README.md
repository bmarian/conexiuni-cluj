# Conexiuni Cluj

A fast, mobile-friendly web app for navigating public transit in Cluj-Napoca, Romania.

Built with Next.js and powered by the [Tranzy API](https://tranzy.ai) and [CTP Cluj](https://ctpcj.ro) timetable data.

<p align="center">
  <img src="https://github.com/bmarian/conexiuni-cluj/blob/master/readme/HomePage.jpg?raw=true" width="250" />
  <img src="https://github.com/bmarian/conexiuni-cluj/blob/master/readme/BusPage.jpg?raw=true" width="250" />
  <img src="https://github.com/bmarian/conexiuni-cluj/blob/master/readme/StationPage.jpg?raw=true" width="250" />
</p>

## Features

### Nearby Stations
When you open the app, it detects your location and shows the 5 closest transit stops with walking distances. An interactive map displays your position alongside the nearby stations.

### Line Timetables
Browse all bus, tram, and trolleybus lines. Each line page shows the full timetable with:
- Automatic detection of the current day (weekday, Saturday, Sunday)
- Clear visual distinction between past and upcoming departures
- A highlighted "next departure" card showing the wait time
- Auto-scroll to the next bus so you don't have to search

### Route Maps
Every line includes two map views:
- A **linear map** showing the ordered sequence of stops for both directions
- An **interactive map** (OpenStreetMap) tracing the actual route on the city map, with stop markers and a direction arrow

### Stop Pages
View all the lines that serve a specific stop, with route numbers, names, and vehicle types (bus, tram, trolleybus).

### Recently Viewed
The home page remembers your recently viewed lines and stops so you can quickly get back to the routes you use most.

### Search
Find any line or station by name across the entire CTP network.

## Data Sources

- **[Tranzy API](https://tranzy.ai)** — routes, stops, trips, shapes, and real-time data
- **[CTP Cluj](https://ctpcj.ro)** — official timetable schedules (CSV)
- **[OpenStreetMap](https://www.openstreetmap.org)** — interactive map tiles via Leaflet
