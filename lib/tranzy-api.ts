import {Agency, Route, Stop, StopTime, Trip} from '@/types/tranzy';

const TRANZY_BASE_URL = 'https://api.tranzy.ai/v1/opendata';
const API_KEY = process.env.TRANZY_API_KEY;

export async function getAgencies(): Promise<Agency[]> {
    const response = await fetch(`${TRANZY_BASE_URL}/agency`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
        },
        next: {
            revalidate: 60*60*24,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch agencies: ${response.status}`);
    }

    return response.json();
}

async function genericGetByAgencyId(agencyId: number, endpoint: string, revalidation?: number) {
    const response = await fetch(`${TRANZY_BASE_URL}/${endpoint}`, {
        headers: {
            'Accept': 'application/json',
            'X-API-KEY': API_KEY!,
            'X-Agency-Id': agencyId.toString(),
        },
        next: {
            revalidate: revalidation || 60*60*24,
        },
    });

    if (!response.ok) {
        throw new Error(`Failed to fetch ${endpoint}: ${response.status}`);
    }

    return response.json();
}

export async function getRoutesByAgencyId(agencyId: number): Promise<Route[]> {
    return genericGetByAgencyId(agencyId, "routes");
}

export async function getTripsByAgencyId(agencyId: number): Promise<Trip[]> {
    return genericGetByAgencyId(agencyId, "trips");
}

export async function getStopsByAgencyId(agencyId: number): Promise<Stop[]> {
    return genericGetByAgencyId(agencyId, "stops");
}

export async function getStopTimesByAgencyId(agencyId: number): Promise<StopTime[]> {
    return genericGetByAgencyId(agencyId, "stop_times");
}
