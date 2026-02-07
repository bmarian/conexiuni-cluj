import type {RouteStopInfo} from "@/lib/cluj-api";
import Link from "next/link";

function StopLine({stops, color, label}: { stops: RouteStopInfo[]; color: string; label: string }) {
  // Ensure dark colors get a readable accent — use the color for dots/line but always readable text
  return (
    <div>
      <p className="mb-2 px-1 text-xs font-medium text-zinc-500 dark:text-zinc-400">{label}</p>
      <div className="flex flex-col">
        {stops.map((stop, i) => {
          const isFirst = i === 0;
          const isLast = i === stops.length - 1;
          const isTerminal = isFirst || isLast;

          return (
            <div key={`${stop.stop_id}-${i}`} className="flex items-stretch">
              {/* Left column: line + dot */}
              <div className="relative flex w-8 shrink-0 flex-col items-center">
                {/* Top segment of line */}
                {!isFirst && (
                  <div className="w-0.5 grow" style={{backgroundColor: color}} />
                )}
                {isFirst && <div className="grow" />}

                {/* Stop dot */}
                <div
                  className={`relative z-10 shrink-0 rounded-full border-2 ${
                    isTerminal ? "h-3.5 w-3.5" : "h-2.5 w-2.5"
                  }`}
                  style={{
                    borderColor: color,
                    backgroundColor: isTerminal ? color : "var(--background)",
                  }}
                />

                {/* Bottom segment of line */}
                {!isLast && (
                  <div className="w-0.5 grow" style={{backgroundColor: color}} />
                )}
                {isLast && <div className="grow" />}
              </div>

              {/* Right column: stop name */}
              <Link
                href={`/statii/${encodeURIComponent(stop.stop_name)}`}
                className={`flex min-h-8 items-center py-1 pl-2 transition-colors hover:text-purple-600 dark:hover:text-purple-400 ${
                  isTerminal
                    ? "text-sm font-semibold text-zinc-900 dark:text-white"
                    : "text-sm text-zinc-600 dark:text-zinc-400"
                }`}
              >
                {stop.stop_name}
              </Link>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default function LinearRouteMap({outbound, inbound, color}: {
  outbound: RouteStopInfo[];
  inbound: RouteStopInfo[];
  color: string;
}) {
  if (outbound.length === 0 && inbound.length === 0) return null;

  const firstDir = outbound.length > 0 ? outbound : null;
  const secondDir = inbound.length > 0 ? inbound : null;

  // Build labels from terminal stops
  const outLabel = firstDir
    ? `${firstDir[0].stop_name} → ${firstDir[firstDir.length - 1].stop_name}`
    : "";
  const inLabel = secondDir
    ? `${secondDir[0].stop_name} → ${secondDir[secondDir.length - 1].stop_name}`
    : "";

  return (
    <div className="animate-fade-slide-up mt-6">
      <h2 className="mb-3 text-sm font-semibold text-zinc-900 dark:text-white">
        Hartă liniară
      </h2>

      <div className="grid gap-4 md:grid-cols-2">
        {firstDir && (
          <div className="rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-700 dark:bg-zinc-900">
            <StopLine stops={firstDir} color={color} label={outLabel} />
          </div>
        )}

        {secondDir && (
          <div className="rounded-lg border border-zinc-200 bg-white px-3 py-3 dark:border-zinc-700 dark:bg-zinc-900">
            <StopLine stops={secondDir} color={color} label={inLabel} />
          </div>
        )}
      </div>
    </div>
  );
}
