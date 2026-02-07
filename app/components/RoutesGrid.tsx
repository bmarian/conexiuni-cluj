"use client";

import {useState} from "react";
import type {Route} from "@/types/tranzy";
import RouteCard from "@/app/components/RouteCard";

function normalize(str: string) {
  return str.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
}

export default function RoutesGrid({routes}: { routes: Route[] }) {
  const [query, setQuery] = useState("");

  const filtered = query.trim()
    ? routes.filter((r) => {
        const q = normalize(query);
        return (
          normalize(r.route_short_name).includes(q) ||
          normalize(r.route_long_name).includes(q)
        );
      })
    : routes;

  return (
    <div className="mt-6">
      {/* Search */}
      <div className="relative">
        <svg
          className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          viewBox="0 0 24 24"
        >
          <circle cx={11} cy={11} r={8} />
          <path d="m21 21-4.35-4.35" strokeLinecap="round" />
        </svg>
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Caută o linie..."
          className="w-full rounded-lg border border-zinc-200 bg-white py-2.5 pl-10 pr-4 text-sm text-zinc-900 placeholder-zinc-400 outline-none transition-colors focus:border-purple-400 focus:ring-2 focus:ring-purple-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-white dark:placeholder-zinc-500 dark:focus:border-purple-500 dark:focus:ring-purple-900/30"
        />
      </div>

      {query.trim() && (
        <p className="mt-3 text-sm text-zinc-500 dark:text-zinc-400">
          {filtered.length} {filtered.length === 1 ? "linie găsită" : "linii găsite"}
        </p>
      )}

      {filtered.length > 0 ? (
        <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {filtered.map((route, i) => (
            <RouteCard key={route.route_id} route={route} index={i} />
          ))}
        </div>
      ) : (
        <div className="mt-12 text-center text-zinc-400 dark:text-zinc-500">
          <p className="text-lg">Nicio linie găsită</p>
          <p className="mt-1 text-sm">Încearcă un alt termen de căutare</p>
        </div>
      )}
    </div>
  );
}
