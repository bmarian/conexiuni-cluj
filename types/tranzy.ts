import {Timetable} from "@/types/ctp";

export interface Agency {
    agency_id: number;
    agency_name: string;
    agency_timezone: string;
    agency_lang: string | null;
    agency_url: string | null;
    agency_urls: string[] | null;
}

export interface Vehicle {
    id: string;
    label: string;
    latitude?: number;
    longitude?: number;
    timestamp: string;
    vehicle_type: number;
    bike_accessible?: 'BIKE_INACCESSIBLE' | 'BIKE_ACCESSIBLE' | 'UNKNOWN';
    wheelchair_accessible?: 'NO_VALUE' | 'UNKNOWN' | 'WHEELCHAIR_ACCESSIBLE' | 'WHEELCHAIR_INACCESSIBLE';
    speed?: number;
    route_id?: number;
    trip_id?: string;
}

export enum RouteType {
    Tram = 0,
    Subway = 1,
    Rail = 2,
    Bus = 3,
    Ferry = 4,
    CableTram = 5,
    AerialLift = 6,
    Funicular = 7,
    Trolleybus = 11,
    Monorail = 12,
}

export interface Route {
    route_id: number;
    agency_id: number;
    route_short_name: string;
    route_long_name: string;
    route_color: string;
    route_type: RouteType;
    route_desc: string;
}

export interface Stop {
    stop_id: number;
    stop_name: string;
    stop_desc?: string;
    stop_lat: number;
    stop_lon: number;
    location_type?: number;
    stop_code?: string;
}

export enum DirectionType {
    Outbound = 0,
    Inbound = 1,
}

export interface Trip {
    direction_id: DirectionType
    route_id: number;
    trip_id: string;
    trip_headsign: string;
    block_id: number;
    shape_id: number;
    wheelchair_accessible?: number;
    bikes_allowed?: number;
}

export interface Shape {
    shape_id: string;
    shape_pt_lat: number;
    shape_pt_lon: number;
    shape_pt_sequence: number;
    shape_dist_traveled?: number;
}

export interface TripAtStop extends Trip {
    route: Route;
}

export interface StopWithTrips extends Stop {
    trips_at_stop: TripAtStop[];
}

export interface StopTime {
    trip_id: string;
    arrival_time?: string;
    departure_time?: string;
    stop_id: number;
    stop_sequence: number;
    stop_headsign?: string;
    pickup_type?: number;
    drop_off_type?: number;
    shape_dist_traveled?: number;
    timepoint?: number;
}

export const ROUTE_TYPE_LABELS: Record<RouteType, string> = {
    [RouteType.Tram]: 'Tram/Light Rail',
    [RouteType.Subway]: 'Subway/Metro',
    [RouteType.Rail]: 'Rail',
    [RouteType.Bus]: 'Bus',
    [RouteType.Ferry]: 'Ferry',
    [RouteType.CableTram]: 'Cable Tram',
    [RouteType.AerialLift]: 'Aerial Lift',
    [RouteType.Funicular]: 'Funicular',
    [RouteType.Trolleybus]: 'Trolleybus',
    [RouteType.Monorail]: 'Monorail',
};

export const DIRECTION_LABELS: Record<DirectionType, string> = {
    [DirectionType.Inbound]: 'Inbound',
    [DirectionType.Outbound]: 'Outbound',
}

export interface RouteGroup {
    route_id: number;
    route_short_name: string;
    route_long_name: string;
    route_color: string;
    headsigns: { outbound: string[]; inbound: string[] };
}

export interface RouteWithTimetable {
    route: RouteGroup;
    timetable: Timetable | null;
}
