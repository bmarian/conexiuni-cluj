import {getRoutesForStop} from "@/lib/cluj-api";
import {RouteType} from "@/types/tranzy";
import Link from "next/link";

function routeTypeLabel(type: RouteType): string {
  switch (type) {
    case RouteType.Tram: return "Tramvai";
    case RouteType.Bus: return "Autobuz";
    case RouteType.Trolleybus: return "Troleibuz";
    default: return "Linie";
  }
}

function sortRoutes(a: string, b: string) {
  const numA = parseInt(a);
  const numB = parseInt(b);
  if (!isNaN(numA) && !isNaN(numB)) return numA - numB;
  if (!isNaN(numA)) return -1;
  if (!isNaN(numB)) return 1;
  return a.localeCompare(b, "ro");
}

export default async function StopDetailPage({params}: { params: Promise<{ stopName: string }> }) {
  const {stopName} = await params;
  const decodedName = decodeURIComponent(stopName);
  const routes = await getRoutesForStop(decodedName);

  const sorted = routes
    .filter((r) => r.route_color !== "#000" && r.route_color !== "#000000")
    .sort((a, b) => sortRoutes(a.route_short_name, b.route_short_name));

  return (
    <div className="mx-auto min-h-screen max-w-5xl px-4 py-8">
      <Link
        href="/statii"
        className="inline-flex items-center gap-1 text-sm text-zinc-500 transition-colors hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-white"
      >
        ← Toate stațiile
      </Link>

      <h1 className="animate-fade-slide-up mt-4 text-3xl font-bold text-zinc-900 dark:text-white">
        {decodedName}
      </h1>
      <p
        className="animate-fade-slide-up mt-1 text-zinc-500 dark:text-zinc-400"
        style={{animationDelay: "0.1s"}}
      >
        {sorted.length} {sorted.length === 1 ? "linie" : "linii"} de transport
      </p>

      {sorted.length > 0 ? (
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {sorted.map((route, i) => {
            const color = route.route_color ? `${route.route_color}` : "#7c3aed";
            return (
              <div
                key={route.route_id}
                className="animate-row flex items-start gap-3 rounded-lg border border-zinc-100 bg-white p-3 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-600"
                style={{animationDelay: `${Math.min(i * 30, 300)}ms`}}
              >
                <div
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md text-sm font-bold text-white"
                  style={{backgroundColor: color}}
                >
                  {route.route_short_name}
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-zinc-500 dark:text-zinc-400">
                    {routeTypeLabel(route.route_type)}
                  </div>
                  <div className="mt-0.5 line-clamp-2 text-xs text-zinc-700 dark:text-zinc-300">
                    {route.route_long_name}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="mt-12 text-center text-zinc-400 dark:text-zinc-500">
          <p className="text-lg">Nicio linie găsită pentru această stație</p>
        </div>
      )}
    </div>
  );
}
