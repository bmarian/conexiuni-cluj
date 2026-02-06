"use server";

import {Route, Shape, Stop, StopTime, Trip, Vehicle} from "@/types/tranzy";
import {getDb} from "@/lib/db";

const API_KEY = process.env.TRANZY_API_KEY;
const TRANZY_BASE_URL = process.env.TRANZY_BASE_URL;
const CLUJ_AGENCY_ID = process.env.CLUJ_AGENCY_ID;

const TRANZY_CACHING_IDS = {
    AGENCIES: 'AGENCIES',
    VEHICLES: 'VEHICLES',
    ROUTES: 'ROUTES',
    TRIPS: 'TRIPS',
    SHAPES_PREFIX: 'SHAPES',
    STOPS: 'STOPS',
    STOP_TIMES: 'STOP_TIMES',

    TIMETABLE_PREFIX: 'TIMETABLE',
};

const TRANZY_ROUTES_IDS = {
    AGENCIES: 'agencies',
    VEHICLES: 'vehicles',
    ROUTES: 'routes',
    TRIPS: 'trips',
    SHAPES: 'shapes',
    STOPS: 'stops',
    STOP_TIMES: 'stop_times',
}

const CACHE_VALIDITY = {
    AGENCIES: 8.64e+7, // 24H
    VEHICLES: 10000, // 10S
    ROUTES: 8.64e+7, // 24H
    TRIPS: 8.64e+7, // 24H
    SHAPES: 10000, // 10S
    STOPS: 8.64e+7, // 24H
    STOP_TIMES: 8.64e+7, // 24H

    TIMETABLE: 6.048e+8, // 1 WEEK
};

const IS_INITIALIZED = {
    CACHE: false,
    AGENCIES: false,
    VEHICLES: false,
    ROUTES: false,
    TRIPS: false,
    SHAPES: false,
    STOPS: false,
    STOP_TIMES: false,

    TIMETABLE: false,
};

async function shouldInvalidateCache(cacheId: string) {
    const db = await getDb();
    if (!IS_INITIALIZED.CACHE) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS cache_times
            (
                id        TEXT PRIMARY KEY,
                timestamp INTEGER NOT NULL,
                lifespan  INTEGER NOT NULL
            )
        `);
        IS_INITIALIZED.CACHE = true;
    }

    const result = await db.execute({
        sql: `SELECT timestamp, lifespan
              FROM cache_times
              WHERE id = ?`,
        args: [cacheId],
    });

    if (result.rows.length === 0) {
        return true;
    }

    const {timestamp, lifespan} = result.rows[0];
    return Date.now() - (timestamp as number) > (lifespan as number);
}

export async function getVehicles() {
    const db = await getDb();
    if (!IS_INITIALIZED.VEHICLES) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS vehicles
            (
                id                    TEXT PRIMARY KEY,
                label                 TEXT    NOT NULL,
                latitude              REAL,
                longitude             REAL,
                timestamp             TEXT    NOT NULL,
                vehicle_type          INTEGER NOT NULL,
                bike_accessible       TEXT,
                wheelchair_accessible TEXT,
                speed                 REAL,
                route_id              INTEGER,
                trip_id               TEXT
            )
        `)
        IS_INITIALIZED.VEHICLES = true;
    }

    const isCacheInvalid = await shouldInvalidateCache(TRANZY_CACHING_IDS.VEHICLES);
    if (!isCacheInvalid) {
        const results = await db.execute(
            `SELECT *
             FROM vehicles`
        );
        return results.rows.map(row => ({
            id: row.id,
            label: row.label,
            latitude: row.latitude,
            longitude: row.longitude,
            timestamp: row.timestamp,
            vehicle_type: row.vehicle_type,
            bike_accessible: row.bike_accessible,
            wheelchair_accessible: row.wheelchair_accessible,
            speed: row.speed,
            route_id: row.route_id,
            trip_id: row.trip_id,
        })) as Vehicle[];
    }

    const response = await fetch(`${TRANZY_BASE_URL!}/${TRANZY_ROUTES_IDS.VEHICLES}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': CLUJ_AGENCY_ID!,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${TRANZY_ROUTES_IDS.VEHICLES}: ${response.status}`);
    }

    const vehicles = await response.json() as Vehicle[];
    for (const vehicle of vehicles) {
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO vehicles (id, label, latitude, longitude, timestamp, vehicle_type,
                                 bike_accessible, wheelchair_accessible, speed, route_id, trip_id)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            args: [vehicle.id, vehicle.label, vehicle.latitude ?? null, vehicle.longitude ?? null, vehicle.timestamp, vehicle.vehicle_type, vehicle.bike_accessible ?? null, vehicle.wheelchair_accessible ?? null, vehicle.speed ?? null, vehicle.route_id ?? null, vehicle.trip_id ?? null],
        });
    }

    await db.execute({
        sql: `INSERT OR
              REPLACE
              INTO cache_times (id, timestamp, lifespan)
              VALUES (?, ?, ?)`,
        args: [TRANZY_CACHING_IDS.VEHICLES, Date.now(), CACHE_VALIDITY[TRANZY_CACHING_IDS.VEHICLES]],
    })

    return vehicles;
}

export async function getRoutes() {
    const db = await getDb();
    if (!IS_INITIALIZED.ROUTES) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS routes
            (
                route_id         INTEGER PRIMARY KEY,
                agency_id        INTEGER NOT NULL,
                route_short_name TEXT    NOT NULL,
                route_long_name  TEXT    NOT NULL,
                route_type       INTEGER NOT NULL,
                route_desc       TEXT    NOT NULL
            )
        `)
        IS_INITIALIZED.ROUTES = true;
    }

    const isCacheInvalid = await shouldInvalidateCache(TRANZY_CACHING_IDS.ROUTES);
    if (!isCacheInvalid) {
        const results = await db.execute(
            `SELECT *
             FROM routes`
        );
        return results.rows.map(row => ({
            route_id: row.route_id,
            agency_id: row.agency_id,
            route_short_name: row.route_short_name,
            route_long_name: row.route_long_name,
            route_type: row.route_type,
            route_desc: row.route_desc,
        })) as Route[];
    }

    const response = await fetch(`${TRANZY_BASE_URL!}/${TRANZY_ROUTES_IDS.ROUTES}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': CLUJ_AGENCY_ID!,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${TRANZY_ROUTES_IDS.ROUTES}: ${response.status}`);
    }

    const routes = await response.json() as Route[];
    for (const route of routes) {
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO routes (route_id, agency_id, route_short_name, route_long_name, route_type, route_desc)
                  VALUES (?, ?, ?, ?, ?, ?)`,
            args: [route.route_id, route.agency_id, route.route_short_name, route.route_long_name, route.route_type, route.route_desc],
        });
    }

    await db.execute({
        sql: `INSERT OR
              REPLACE
              INTO cache_times (id, timestamp, lifespan)
              VALUES (?, ?, ?)`,
        args: [TRANZY_CACHING_IDS.ROUTES, Date.now(), CACHE_VALIDITY[TRANZY_CACHING_IDS.ROUTES]],
    })

    return routes;
}

export async function getTrips() {
    const db = await getDb();
    if (!IS_INITIALIZED.TRIPS) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS trips
            (
                trip_id               TEXT PRIMARY KEY,
                route_id              INTEGER NOT NULL,
                direction_id          INTEGER NOT NULL,
                trip_headsign         TEXT    NOT NULL,
                block_id              INTEGER NOT NULL,
                shape_id              INTEGER NOT NULL,
                wheelchair_accessible INTEGER,
                bikes_allowed         INTEGER
            )
        `)
        IS_INITIALIZED.TRIPS = true;
    }

    const isCacheInvalid = await shouldInvalidateCache(TRANZY_CACHING_IDS.TRIPS);
    if (!isCacheInvalid) {
        const results = await db.execute(
            `SELECT *
             FROM trips`
        );
        return results.rows.map(row => ({
            trip_id: row.trip_id,
            route_id: row.route_id,
            direction_id: row.direction_id,
            trip_headsign: row.trip_headsign,
            block_id: row.block_id,
            shape_id: row.shape_id,
            wheelchair_accessible: row.wheelchair_accessible,
            bikes_allowed: row.bikes_allowed,
        })) as Trip[];
    }

    const response = await fetch(`${TRANZY_BASE_URL!}/${TRANZY_ROUTES_IDS.TRIPS}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': CLUJ_AGENCY_ID!,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${TRANZY_ROUTES_IDS.TRIPS}: ${response.status}`);
    }

    const trips = await response.json() as Trip[];
    for (const trip of trips) {
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO trips (trip_id, route_id, direction_id, trip_headsign, block_id, shape_id,
                              wheelchair_accessible, bikes_allowed)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            args: [trip.trip_id, trip.route_id, trip.direction_id, trip.trip_headsign, trip.block_id, trip.shape_id, trip.wheelchair_accessible ?? null, trip.bikes_allowed ?? null]
        });
    }

    await db.execute({
        sql: `INSERT OR
              REPLACE
              INTO cache_times (id, timestamp, lifespan)
              VALUES (?, ?, ?)`,
        args: [TRANZY_CACHING_IDS.TRIPS, Date.now(), CACHE_VALIDITY[TRANZY_CACHING_IDS.TRIPS]],
    })

    return trips;
}

export async function getShapes(shapeId: string) {
    const db = await getDb();
    if (!IS_INITIALIZED.SHAPES) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS shapes
            (
                shape_id            TEXT    NOT NULL,
                shape_pt_lat        REAL    NOT NULL,
                shape_pt_lon        REAL    NOT NULL,
                shape_pt_sequence   INTEGER NOT NULL,
                shape_dist_traveled REAL,
                PRIMARY KEY (shape_id, shape_pt_sequence)
            )
        `)
        IS_INITIALIZED.SHAPES = true;
    }

    const cacheId = `${TRANZY_CACHING_IDS.SHAPES_PREFIX}_${shapeId}`;
    const isCacheInvalid = await shouldInvalidateCache(cacheId);
    if (!isCacheInvalid) {
        const results = await db.execute({
            sql: `SELECT *
                  FROM shapes
                  WHERE shape_id = ?`,
            args: [shapeId],
        });
        return results.rows.map(row => ({
            shape_id: row.shape_id,
            shape_pt_lat: row.shape_pt_lat,
            shape_pt_lon: row.shape_pt_lon,
            shape_pt_sequence: row.shape_pt_sequence,
            shape_dist_traveled: row.shape_dist_traveled,
        })) as Shape[];
    }

    const response = await fetch(`${TRANZY_BASE_URL!}/${TRANZY_ROUTES_IDS.SHAPES}?shape_id=${shapeId}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': CLUJ_AGENCY_ID!,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${TRANZY_ROUTES_IDS.SHAPES} for shape_id ${shapeId}: ${response.status}`);
    }

    const shapes = await response.json() as Shape[];
    for (const shape of shapes) {
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO shapes (shape_id, shape_pt_lat, shape_pt_lon, shape_pt_sequence, shape_dist_traveled)
                  VALUES (?, ?, ?, ?, ?)`,
            args: [shape.shape_id, shape.shape_pt_lat, shape.shape_pt_lon, shape.shape_pt_sequence, shape.shape_dist_traveled ?? null],
        });
    }

    await db.execute({
        sql: `INSERT OR
              REPLACE
              INTO cache_times (id, timestamp, lifespan)
              VALUES (?, ?, ?)`,
        args: [cacheId, Date.now(), CACHE_VALIDITY[TRANZY_CACHING_IDS.SHAPES_PREFIX]],
    })

    return shapes;
}

export async function getStops() {
    const db = await getDb();
    if (!IS_INITIALIZED.STOPS) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS stops
            (
                stop_id       INTEGER PRIMARY KEY,
                stop_name     TEXT NOT NULL,
                stop_desc     TEXT,
                stop_lat      REAL NOT NULL,
                stop_lon      REAL NOT NULL,
                location_type INTEGER,
                stop_code     TEXT
            )
        `)
        IS_INITIALIZED.STOPS = true;
    }

    const isCacheInvalid = await shouldInvalidateCache(TRANZY_CACHING_IDS.STOPS);
    if (!isCacheInvalid) {
        const results = await db.execute(
            `SELECT *
             FROM stops`
        );
        return results.rows.map(row => ({
            stop_id: row.stop_id,
            stop_name: row.stop_name,
            stop_desc: row.stop_desc,
            stop_lat: row.stop_lat,
            stop_lon: row.stop_lon,
            location_type: row.location_type,
            stop_code: row.stop_code,
        })) as Stop[];
    }

    const response = await fetch(`${TRANZY_BASE_URL!}/${TRANZY_ROUTES_IDS.STOPS}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': CLUJ_AGENCY_ID!,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${TRANZY_ROUTES_IDS.STOPS}: ${response.status}`);
    }

    const stops = await response.json() as Stop[];
    for (const stop of stops) {
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO stops (stop_id, stop_name, stop_desc, stop_lat, stop_lon, location_type, stop_code)
                  VALUES (?, ?, ?, ?, ?, ?, ?)`,
            args: [stop.stop_id, stop.stop_name, stop.stop_desc ?? null, stop.stop_lat, stop.stop_lon, stop.location_type ?? null, stop.stop_code ?? null],
        });
    }

    await db.execute({
        sql: `INSERT OR
              REPLACE
              INTO cache_times (id, timestamp, lifespan)
              VALUES (?, ?, ?)`,
        args: [TRANZY_CACHING_IDS.STOPS, Date.now(), CACHE_VALIDITY[TRANZY_CACHING_IDS.STOPS]],
    })

    return stops;
}

export async function getStopTimes() {
    const db = await getDb();
    if (!IS_INITIALIZED.STOP_TIMES) {
        await db.execute(`
            CREATE TABLE IF NOT EXISTS stop_times
            (
                trip_id             TEXT    NOT NULL,
                arrival_time        TEXT,
                departure_time      TEXT,
                stop_id             INTEGER NOT NULL,
                stop_sequence       INTEGER NOT NULL,
                stop_headsign       TEXT,
                pickup_type         INTEGER,
                drop_off_type       INTEGER,
                shape_dist_traveled REAL,
                timepoint           INTEGER,
                PRIMARY KEY (trip_id, stop_sequence)
            )
        `)
        IS_INITIALIZED.STOP_TIMES = true;
    }

    const isCacheInvalid = await shouldInvalidateCache(TRANZY_CACHING_IDS.STOP_TIMES);
    if (!isCacheInvalid) {
        const results = await db.execute(
            `SELECT *
             FROM stop_times`
        );
        return results.rows.map(row => ({
            trip_id: row.trip_id,
            arrival_time: row.arrival_time,
            departure_time: row.departure_time,
            stop_id: row.stop_id,
            stop_sequence: row.stop_sequence,
            stop_headsign: row.stop_headsign,
            pickup_type: row.pickup_type,
            drop_off_type: row.drop_off_type,
            shape_dist_traveled: row.shape_dist_traveled,
            timepoint: row.timepoint,
        })) as StopTime[];
    }

    const response = await fetch(`${TRANZY_BASE_URL!}/${TRANZY_ROUTES_IDS.STOP_TIMES}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': CLUJ_AGENCY_ID!,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${TRANZY_ROUTES_IDS.STOP_TIMES}: ${response.status}`);
    }

    const stopTimes = await response.json() as StopTime[];
    for (const st of stopTimes) {
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO stop_times (trip_id, arrival_time, departure_time, stop_id, stop_sequence,
                                   stop_headsign, pickup_type, drop_off_type, shape_dist_traveled, timepoint)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            args: [st.trip_id, st.arrival_time ?? null, st.departure_time ?? null, st.stop_id, st.stop_sequence, st.stop_headsign ?? null, st.pickup_type ?? null, st.drop_off_type ?? null, st.shape_dist_traveled ?? null, st.timepoint ?? null],
        });
    }

    await db.execute({
        sql: `INSERT OR
              REPLACE
              INTO cache_times (id, timestamp, lifespan)
              VALUES (?, ?, ?)`,
        args: [TRANZY_CACHING_IDS.STOP_TIMES, Date.now(), CACHE_VALIDITY[TRANZY_CACHING_IDS.STOP_TIMES]],
    })

    return stopTimes;
}