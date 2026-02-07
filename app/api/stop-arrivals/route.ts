import {NextRequest, NextResponse} from "next/server";
import {getRoutesForStop, getStopsForRoute, getVehicles, getTrips} from "@/lib/cluj-api";
import type {VehicleWithDirection} from "@/types/vehicle-tracking";

export async function GET(request: NextRequest) {
  const stopName = request.nextUrl.searchParams.get("stop_name");
  if (!stopName) {
    return NextResponse.json({error: "stop_name is required"}, {status: 400});
  }

  try {
    const routes = await getRoutesForStop(stopName);
    if (routes.length === 0) {
      return NextResponse.json({routes: []});
    }

    const [vehicles, trips] = await Promise.all([getVehicles(), getTrips()]);
    const tripMap = new Map(trips.map((t) => [t.trip_id, t]));

    // Build a set of route_ids we care about
    const routeIdSet = new Set(routes.map((r) => r.route_id));

    // Filter vehicles for these routes and attach direction_id
    const routeVehicleMap = new Map<number, VehicleWithDirection[]>();
    for (const v of vehicles) {
      if (v.route_id == null || !routeIdSet.has(v.route_id)) continue;
      if (!v.trip_id || !tripMap.has(v.trip_id)) continue;
      const trip = tripMap.get(v.trip_id)!;
      const enriched: VehicleWithDirection = {...v, direction_id: trip.direction_id};
      const list = routeVehicleMap.get(v.route_id) ?? [];
      list.push(enriched);
      routeVehicleMap.set(v.route_id, list);
    }

    // Fetch stops for each route in parallel
    const routeStopsEntries = await Promise.all(
      routes.map(async (r) => {
        const stops = await getStopsForRoute(r.route_short_name);
        return {route: r, stops};
      }),
    );

    const result = routeStopsEntries.map(({route, stops}) => ({
      route_id: route.route_id,
      route_short_name: route.route_short_name,
      route_color: route.route_color,
      route_type: route.route_type,
      stops: {outbound: stops.outbound, inbound: stops.inbound},
      vehicles: routeVehicleMap.get(route.route_id) ?? [],
    }));

    return NextResponse.json({routes: result});
  } catch {
    return NextResponse.json({error: "Failed to fetch stop arrivals"}, {status: 500});
  }
}
