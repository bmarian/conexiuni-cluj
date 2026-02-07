import Link from "next/link";
import type {Route} from "@/types/tranzy";
import {routeTypeLabel} from "@/app/utils/route-utils";

export default function RouteCard({route, index = 0}: { route: Route; index?: number }) {
  const color = route.route_color || "#7c3aed";
  const href = `/linii/${encodeURIComponent(route.route_short_name)}`;

  return (
    <Link
      href={href}
      className="animate-row flex items-start gap-3 rounded-lg border border-zinc-100 bg-white p-3 transition-colors hover:border-zinc-300 dark:border-zinc-800 dark:bg-zinc-900 dark:hover:border-zinc-600"
      style={{animationDelay: `${Math.min(index * 30, 300)}ms`}}
    >
      <div
        className="flex h-10 min-w-10 shrink-0 items-center justify-center rounded-md px-2 text-xs font-bold text-white"
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
    </Link>
  );
}
