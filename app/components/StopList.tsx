import {DirectionType, StopWithTrips, TripAtStop} from "@/types/tranzy";
import {getTimetable} from "@/lib/cluj/ctpcj-api";

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

async function getRouteTimeTable(route: RouteGroup, stopName: string) {
  if (route.headsigns.outbound.includes(stopName) || route.headsigns.inbound.includes(stopName)) {
    try {
      const timeTable = await getTimetable(route.route_short_name);
      console.log(timeTable);
    } catch (e) {
      console.log(`Failed to get route timetable: ${route.route_short_name}`);
    }
  }
}

function isLightColor(hex: string): boolean {
  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);

  return (r * 299 + g * 587 + b * 114) / 1000 > 150;
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
                const routeTimeTable = await getRouteTimeTable(route, stop.stop_name);

                return (
                    <div key={route.route_id} className="flex items-start gap-2">
                  <span
                      className="mt-0.5 inline-block rounded px-2 py-0.5 text-xs font-bold leading-snug"
                      style={{
                        backgroundColor: `${route.route_color}`,
                        color: isLightColor(route.route_color) ? "#000" : "#fff",
                      }}
                  >
                    {route.route_short_name}
                  </span>

                      <div className="min-w-0 text-sm">
                    <span className="font-medium text-zinc-800 dark:text-zinc-200">
                      {route.route_long_name}
                    </span>

                        {route.headsigns.outbound.length > 0 && (
                            <p className="text-zinc-500 dark:text-zinc-400">
                              → {route.headsigns.outbound.join(", ")}
                            </p>
                        )}
                        {route.headsigns.inbound.length > 0 && (
                            <p className="text-zinc-500 dark:text-zinc-400">
                              ← {route.headsigns.inbound.join(", ")}
                            </p>
                        )}
                      </div>
                    </div>
                );
              })}
            </div>
          </li>
        );
      })}
    </ul>
  );
}
