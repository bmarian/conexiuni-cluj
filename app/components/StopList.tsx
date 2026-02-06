import {DirectionType, RouteGroup, RouteWithTimetable, StopWithTrips, TripAtStop} from "@/types/tranzy";
import {Timetable} from "@/types/ctpcj";
import {getTimetable} from "@/lib/cluj/ctpcj-api";
import StopCard from "@/app/components/StopCard";

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

export default async function StopList({ stops }: { stops: StopWithTrips[] }) {
  // Pre-fetch all timetables to know which routes exist in CTPCJ
  const allRouteNames = new Set<string>();
  stops.forEach(stop => {
    groupRoutes(stop.trips_at_stop).forEach(r => allRouteNames.add(r.route_short_name));
  });

  const timetableMap = new Map<string, Timetable | null>();
  await Promise.all(
    Array.from(allRouteNames).map(async (name) => {
      timetableMap.set(name, await getTimetable(name));
    })
  );

  const stopsData = await Promise.all(
    stops.map(async (stop) => {
      const routes = groupRoutes(stop.trips_at_stop);
      // Only include routes that have a timetable from CTPCJ
      const routesWithTimetables: RouteWithTimetable[] = routes
        .filter(route => timetableMap.get(route.route_short_name) !== null)
        .map(route => ({
          route,
          // Attach the timetable only if this stop matches a headsign
          timetable: (() => {
            const norm = normalize(stop.stop_name);
            const match = [...route.headsigns.outbound, ...route.headsigns.inbound].some(h => normalize(h) === norm);
            return match ? timetableMap.get(route.route_short_name) ?? null : null;
          })(),
        }));
      return { stop, routesWithTimetables };
    })
  );

  return (
    <ul className="flex w-full max-w-4xl flex-col gap-4">
      {stopsData.map(({ stop, routesWithTimetables }) => (
        <StopCard
          key={stop.stop_id}
          stopName={stop.stop_name}
          routes={routesWithTimetables}
        />
      ))}
    </ul>
  );
}
