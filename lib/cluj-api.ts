"use server";

import {Route, Trip} from "@/types/tranzy";
import {getDb} from "@/lib/db";

const API_KEY = process.env.TRANZY_API_KEY;
const TRANZY_BASE_URL = process.env.TRANZY_BASE_URL;
const CLUJ_AGENCY_ID = process.env.CLUJ_AGENCY_ID;

const TRANZY_CACHING_IDS = {
    AGENCIES: 'AGENCIES',
    VEHICLES: 'VEHICLES',
    ROUTES: 'ROUTES',
    TRIPS: 'TRIPS',
    SHAPES: 'SHAPES',
    STOPS: 'STOPS',
    STOP_TIMES: 'STOP_TIMES',
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
    VEHICLES: 8.64e+7, // 24H
    ROUTES: 8.64e+7, // 24H
    TRIPS: 8.64e+7, // 24H
    SHAPES: 8.64e+7, // 24H
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
        const args = [trip.trip_id, trip.route_id, trip.direction_id, trip.trip_headsign, trip.block_id, trip.shape_id];
        if (typeof trip.wheelchair_accessible !== 'undefined') {
            args.push(trip.wheelchair_accessible);
        }

        if (typeof trip.bikes_allowed !== 'undefined') {
            args.push(trip.bikes_allowed);
        }
        await db.execute({
            sql: `INSERT OR
                  REPLACE
                  INTO trips (trip_id, route_id, direction_id, trip_headsign, block_id, shape_id,
                              wheelchair_accessible, bikes_allowed)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
            args
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