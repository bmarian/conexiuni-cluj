import {NextRequest, NextResponse} from "next/server";
import {getVehicles, getTrips} from "@/lib/cluj-api";
import type {VehicleWithDirection} from "@/types/vehicle-tracking";

export async function GET(request: NextRequest) {
  const routeId = request.nextUrl.searchParams.get("route_id");
  if (!routeId) {
    return NextResponse.json({error: "route_id is required"}, {status: 400});
  }

  const rid = Number(routeId);
  if (Number.isNaN(rid)) {
    return NextResponse.json({error: "route_id must be a number"}, {status: 400});
  }

  try {
    const [vehicles, trips] = await Promise.all([getVehicles(), getTrips()]);

    const tripMap = new Map(trips.map((t) => [t.trip_id, t]));

    const filtered: VehicleWithDirection[] = vehicles
      .filter((v) => v.route_id === rid && v.trip_id && tripMap.has(v.trip_id))
      .map((v) => {
        const trip = tripMap.get(v.trip_id!)!;
        return {...v, direction_id: trip.direction_id};
      });

    return NextResponse.json({vehicles: filtered});
  } catch {
    return NextResponse.json({error: "Failed to fetch vehicles"}, {status: 500});
  }
}
