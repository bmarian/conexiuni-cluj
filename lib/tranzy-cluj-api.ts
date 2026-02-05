import {Agency, Route, StopWithTrips, Trip} from "@/types/tranzy";
import {
    getAgencies,
    getRoutesByAgencyId,
    getStopsByAgencyId,
    getStopTimesByAgencyId,
    getTripsByAgencyId
} from "@/lib/tranzy-api";

const CLUJ_AGENCY_ID = 2;

export async function getAgency(): Promise<Agency> {
    const agencies = await getAgencies();
    const clujAgency = agencies.find((agency) => agency.agency_id === CLUJ_AGENCY_ID);

    if (!clujAgency) {
        throw new Error("Cluj agency not found.");
    }

    return clujAgency;
}

export async function getFormattedStops(): Promise<StopWithTrips[]> {
    const routes = await getRoutesByAgencyId(CLUJ_AGENCY_ID);
    const trips = await getTripsByAgencyId(CLUJ_AGENCY_ID);
    const stops = await getStopsByAgencyId(CLUJ_AGENCY_ID);
    const stopTimes = await getStopTimesByAgencyId(CLUJ_AGENCY_ID);

    if (!routes.length || ! trips.length || !stops.length || !stopTimes.length) {
        throw new Error("There was an error while getting the stops.");
    }

    return stops.map((stop) => {
        const tripsAtStop = stopTimes
            .filter((stopTime) => stopTime.stop_id === stop.stop_id)
            .map((stopTime) => {
                const trip = trips.find((trip) => trip.trip_id === stopTime.trip_id) as Trip;
                const route = routes.find((route) => route.route_id === trip.route_id) as Route;
                return {...trip, route};
            });

        return {...stop, trips_at_stop: tripsAtStop};
    });
}