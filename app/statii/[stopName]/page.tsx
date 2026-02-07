import {getRoutesForStop} from "@/lib/cluj-api";
import RouteCard from "@/app/components/RouteCard";
import {RecentStopTracker} from "@/app/components/RecentTracker";
import {FavoriteStopButton} from "@/app/components/FavoriteButton";
import BackButton from "@/app/components/BackButton";

function sortRoutes(a: string, b: string) {
  const numA = parseInt(a);
  const numB = parseInt(b);
  if (!isNaN(numA) && !isNaN(numB)) return numA - numB;
  if (!isNaN(numA)) return -1;
  if (!isNaN(numB)) return 1;
  return a.localeCompare(b, "ro");
}

export default async function StopDetailPage({params}: {
  params: Promise<{ stopName: string }>;
}) {
  const {stopName} = await params;
  const decodedName = decodeURIComponent(stopName);
  const routes = await getRoutesForStop(decodedName);

  const sorted = routes
    .sort((a, b) => sortRoutes(a.route_short_name, b.route_short_name));

  return (
    <div className="mx-auto min-h-screen max-w-5xl px-4 py-8">
      <RecentStopTracker stopName={decodedName} />
      <BackButton fallbackHref="/statii" fallbackLabel="← Înapoi" />

      <div className="mt-4 flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <h1 className="animate-fade-slide-up text-3xl font-bold text-zinc-900 dark:text-white">
            {decodedName}
          </h1>
          <p
            className="animate-fade-slide-up mt-1 text-zinc-500 dark:text-zinc-400"
            style={{animationDelay: "0.1s"}}
          >
            {sorted.length} {sorted.length === 1 ? "linie" : "linii"} de transport
          </p>
        </div>
        <FavoriteStopButton stopName={decodedName} />
      </div>

      {sorted.length > 0 ? (
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {sorted.map((route, i) => (
            <RouteCard key={route.route_id} route={route} index={i} />
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
