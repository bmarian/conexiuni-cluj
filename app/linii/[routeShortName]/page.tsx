import {getRoutes, getTimetable} from "@/lib/cluj-api";
import TimetableDisplay from "@/app/components/TimetableDisplay";
import {RecentLineTracker} from "@/app/components/RecentTracker";
import Link from "next/link";

export default async function RouteTimetablePage({params, searchParams}: {
  params: Promise<{ routeShortName: string }>;
  searchParams: Promise<{ from?: string }>;
}) {
  const {routeShortName} = await params;
  const {from} = await searchParams;
  const decoded = decodeURIComponent(routeShortName);

  const [timetable, routes] = await Promise.all([
    getTimetable(decoded),
    getRoutes(),
  ]);

  const route = routes.find((r) => r.route_short_name === decoded);
  const color = route?.route_color || "#7c3aed";

  const backHref = from === "home"
    ? "/"
    : from
      ? `/statii/${encodeURIComponent(from)}`
      : "/linii";
  const backLabel = from === "home"
    ? "← Acasă"
    : from
      ? `← Stația ${from}`
      : "← Toate liniile";

  return (
    <div className="mx-auto min-h-screen max-w-3xl px-4 py-8">
      <Link
        href={backHref}
        className="inline-flex items-center gap-1 text-sm text-zinc-500 transition-colors hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-white"
      >
        {backLabel}
      </Link>

      <div className="mt-4 flex items-center gap-3">
        <div
          className="flex h-12 min-w-12 shrink-0 items-center justify-center rounded-lg px-2 text-base font-bold text-white"
          style={{backgroundColor: color}}
        >
          {decoded}
        </div>
        <div className="min-w-0">
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
    </div>
  );
}
