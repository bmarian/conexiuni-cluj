import {DirectionType, StopWithTrips, TripAtStop} from "@/types/tranzy";
import {Timetable} from "@/types/ctpcj";
import {getTimetable} from "@/lib/cluj/ctpcj-api";
import RouteItem from "@/app/components/RouteItem";

interface RouteGroup {
  route_id: number;
  route_short_name: string;
  route_long_name: string;
  route_color: string;
  headsigns: { outbound: string[]; inbound: string[] };
}

function groupRoutes(trips: TripAtStop[]): RouteGroup[] {
  const routeGroup = new Map<number, RouteGroup>();

  for (const trip of trips) {
    const {route_id, route_short_name, route_long_name, route_color} = trip.route;
    let group = routeGroup.get(route_id);
    if (!group) {
      group = {
        route_id,
        route_short_name,
        route_long_name,
        route_color,
        headsigns: {outbound: [], inbound: []}
      }
      routeGroup.set(route_id, group);
    }

    const dir = trip.direction_id === DirectionType.Outbound ? "outbound" : "inbound";
    if (!group.headsigns[dir].includes(trip.trip_headsign)) group.headsigns[dir].push(trip.trip_headsign);
  }

  return Array.from(routeGroup.values());
}

function normalize(s: string): string {
  return s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replace(/[^a-z0-9]/gi, "")
    .toLowerCase();
}

async function getRouteTimeTable(route: RouteGroup, stopName: string): Promise<Timetable | null> {
  const norm = normalize(stopName);
  const match = [...route.headsigns.outbound, ...route.headsigns.inbound].some(h => normalize(h) === norm);
  return match ? getTimetable(route.route_short_name) : null;
}

export default function StopList({ stops }: { stops: StopWithTrips[] }) {
  return (
    <ul className="flex w-full max-w-2xl flex-col gap-4">
      {stops.map((stop) => {
        const routes = groupRoutes(stop.trips_at_stop);

        return (
          <li
            key={stop.stop_id}
            className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
          >
            <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
              {stop.stop_name}
            </h2>

            {routes.length === 0 && (
              <p className="text-sm text-zinc-500">No routes at this stop.</p>
            )}

            <div className="flex flex-col gap-2">
              {routes.map(async (route) => {
                const timetable = await getRouteTimeTable(route, stop.stop_name);

                return (
                  <RouteItem
                    key={route.route_id}
                    route={route}
                    timetable={timetable}
                  />
                );
              })}
            </div>
          </li>
        );
      })}
    </ul>
  );
}
