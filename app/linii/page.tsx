import {getRoutes} from "@/lib/cluj-api";
import RoutesGrid from "@/app/components/RoutesGrid";

function sortRoutes(a: string, b: string) {
  const numA = parseInt(a);
  const numB = parseInt(b);
  if (!isNaN(numA) && !isNaN(numB)) return numA - numB;
  if (!isNaN(numA)) return -1;
  if (!isNaN(numB)) return 1;
  return a.localeCompare(b, "ro");
}

export default async function LiniiPage() {
  const allRoutes = await getRoutes();

  const routes = allRoutes
    .filter((r) => r.route_color !== "#000" && r.route_color !== "#000000")
    .sort((a, b) => sortRoutes(a.route_short_name, b.route_short_name));

  return (
    <div className="mx-auto min-h-screen max-w-5xl px-4 py-8">
      <h1 className="animate-fade-slide-up text-3xl font-bold text-zinc-900 dark:text-white">
        Linii
      </h1>
      <p className="animate-fade-slide-up mt-1 text-zinc-500 dark:text-zinc-400" style={{animationDelay: "0.1s"}}>
        {routes.length} linii de transport public
      </p>

      <RoutesGrid routes={routes} />
    </div>
  );
}
