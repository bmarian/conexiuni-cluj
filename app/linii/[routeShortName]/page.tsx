import {Suspense} from "react";
import {getRoutes, getShapesForRoute, getStopsForRoute, getTimetable} from "@/lib/cluj-api";
import TimetableDisplay from "@/app/components/TimetableDisplay";
import LinearRouteMap from "@/app/components/LinearRouteMap";
import RouteMapWrapper from "@/app/components/RouteMap";
import {RecentLineTracker} from "@/app/components/RecentTracker";
import {FavoriteLineButton} from "@/app/components/FavoriteButton";
import BackButton from "@/app/components/BackButton";

function MapSkeleton({height, label}: { height: string; label: string }) {
  return (
    <div className="animate-fade-slide-up mt-6">
      <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">{label}</h2>
      <div
        className="flex items-center justify-center rounded-lg border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900"
        style={{height}}
      >
        <div className="flex items-center gap-2 text-sm text-zinc-400">
          <svg className="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          Se încarcă...
        </div>
      </div>
    </div>
  );
}

async function RouteMaps({routeShortName, color, routeId, routeType}: {
  routeShortName: string;
  color: string;
  routeId?: number;
  routeType?: number;
}) {
  const [routeStops, routeShapes] = await Promise.all([
    getStopsForRoute(routeShortName),
    getShapesForRoute(routeShortName),
  ]);

  return (
    <>
      <LinearRouteMap outbound={routeStops.outbound} inbound={routeStops.inbound} color={color} routeShortName={routeShortName} routeId={routeId} routeType={routeType} />
      <RouteMapWrapper
        outboundShape={routeShapes.outbound}
        inboundShape={routeShapes.inbound}
        outboundStops={routeStops.outbound}
        inboundStops={routeStops.inbound}
        color={color}
        routeId={routeId}
        routeType={routeType}
      />
    </>
  );
}

export default async function RouteTimetablePage({params}: {
  params: Promise<{ routeShortName: string }>;
}) {
  const {routeShortName} = await params;
  const decoded = decodeURIComponent(routeShortName);

  const [timetable, routes] = await Promise.all([
    getTimetable(decoded),
    getRoutes(),
  ]);

  const route = routes.find((r) => r.route_short_name === decoded);
  const color = route?.route_color || "#7c3aed";

  return (
    <div className="mx-auto min-h-screen max-w-3xl px-4 py-8">
      <BackButton fallbackHref="/linii" fallbackLabel="← Înapoi" />

      <div className="mt-4 flex items-center gap-3">
        <div
          className="flex h-12 min-w-12 shrink-0 items-center justify-center rounded-lg px-2 text-base font-bold text-white"
          style={{backgroundColor: color}}
        >
          {decoded}
        </div>
        <div className="min-w-0 flex-1">
          <h1 className="animate-fade-slide-up text-2xl font-bold text-zinc-900 dark:text-white">
            Linia {decoded}
          </h1>
          {timetable.route_long_name && (
            <p
              className="animate-fade-slide-up mt-0.5 truncate text-sm text-zinc-500 dark:text-zinc-400"
              style={{animationDelay: "0.1s"}}
            >
              {timetable.route_long_name}
            </p>
          )}
        </div>
        {route && (
          <FavoriteLineButton
            routeShortName={route.route_short_name}
            routeLongName={route.route_long_name}
            routeColor={route.route_color}
            routeType={route.route_type}
          />
        )}
      </div>

      {route && (
        <RecentLineTracker
          routeShortName={route.route_short_name}
          routeLongName={route.route_long_name}
          routeColor={route.route_color}
          routeType={route.route_type}
        />
      )}
      <TimetableDisplay timetable={timetable} routeType={route?.route_type} />

      <Suspense fallback={
        <>
          <MapSkeleton height="300px" label="Hartă liniară" />
          <MapSkeleton height="400px" label="Hartă traseu" />
        </>
      }>
        <RouteMaps routeShortName={decoded} color={color} routeId={route?.route_id} routeType={route?.route_type} />
      </Suspense>
    </div>
  );
}
