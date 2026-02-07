import {getRoutesForStop} from "@/lib/cluj-api";
import RouteCard from "@/app/components/RouteCard";
import {RecentStopTracker} from "@/app/components/RecentTracker";
import Link from "next/link";

function sortRoutes(a: string, b: string) {
  const numA = parseInt(a);
  const numB = parseInt(b);
  if (!isNaN(numA) && !isNaN(numB)) return numA - numB;
  if (!isNaN(numA)) return -1;
  if (!isNaN(numB)) return 1;
  return a.localeCompare(b, "ro");
}

export default async function StopDetailPage({params, searchParams}: {
  params: Promise<{ stopName: string }>;
  searchParams: Promise<{ from?: string }>;
}) {
  const {stopName} = await params;
  const {from} = await searchParams;
  const decodedName = decodeURIComponent(stopName);
  const routes = await getRoutesForStop(decodedName);

  const sorted = routes
    .sort((a, b) => sortRoutes(a.route_short_name, b.route_short_name));

  const backHref = from === "home" ? "/" : "/statii";
  const backLabel = from === "home" ? "← Acasă" : "← Toate stațiile";

  return (
    <div className="mx-auto min-h-screen max-w-5xl px-4 py-8">
      <RecentStopTracker stopName={decodedName} />
      <Link
        href={backHref}
        className="inline-flex items-center gap-1 text-sm text-zinc-500 transition-colors hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-white"
      >
        {backLabel}
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
          {sorted.map((route, i) => (
            <RouteCard key={route.route_id} route={route} index={i} fromStop={decodedName} />
          ))}
        </div>
      ) : (
        <div className="mt-12 text-center text-zinc-400 dark:text-zinc-500">
          <p className="text-lg">Nicio linie găsită pentru această stație</p>
        </div>
      )}
    </div>
  );
}
