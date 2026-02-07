"use client";

import {useState} from "react";
import type {Stop} from "@/types/tranzy";

function removeDiacritics(str: string) {
  return str.normalize("NFD").replace(/[\u0300-\u036f]/g, "");
}

export default function StopsGrid({stops}: { stops: Stop[] }) {
  const [query, setQuery] = useState("");

  const filtered = query.trim()
    ? stops.filter((s) =>
        removeDiacritics(s.stop_name.toLowerCase()).includes(
          removeDiacritics(query.toLowerCase())
        )
      )
    : stops;

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
          placeholder="Cauta o statie..."
          className="w-full rounded-lg border border-zinc-200 bg-white py-2.5 pl-10 pr-4 text-sm text-zinc-900 placeholder-zinc-400 outline-none transition-colors focus:border-purple-400 focus:ring-2 focus:ring-purple-100 dark:border-zinc-700 dark:bg-zinc-900 dark:text-white dark:placeholder-zinc-500 dark:focus:border-purple-500 dark:focus:ring-purple-900/30"
        />
      </div>

      {/* Results count when searching */}
      {query.trim() && (
        <p className="mt-3 text-sm text-zinc-500 dark:text-zinc-400">
          {filtered.length} {filtered.length === 1 ? "statie gasita" : "statii gasite"}
        </p>
      )}

      {/* Grid */}
      {filtered.length > 0 ? (
        <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
          {filtered.map((stop, i) => (
            <div
              key={stop.stop_id}
              className="animate-row rounded-lg border border-zinc-100 bg-white px-3 py-2.5 text-sm text-zinc-800 transition-colors hover:border-purple-200 hover:bg-purple-50 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-200 dark:hover:border-purple-800 dark:hover:bg-purple-950/20"
              style={{animationDelay: `${Math.min(i * 15, 300)}ms`}}
            >
              {stop.stop_name}
            </div>
          ))}
        </div>
      ) : (
        <div className="mt-12 text-center text-zinc-400 dark:text-zinc-500">
          <p className="text-lg">Nicio statie gasita</p>
          <p className="mt-1 text-sm">Incearca un alt termen de cautare</p>
        </div>
      )}
    </div>
  );
}
