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
  const stopsData = await Promise.all(
    stops.map(async (stop) => {
      const routes = groupRoutes(stop.trips_at_stop);
      const routesWithTimetables: RouteWithTimetable[] = await Promise.all(
        routes.map(async (route) => ({
          route,
          timetable: await getRouteTimeTable(route, stop.stop_name),
        }))
      );
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
