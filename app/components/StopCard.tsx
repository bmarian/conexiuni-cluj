"use client";

import {useState} from "react";
import type {Timetable, Schedule} from "@/types/ctpcj";
import {RouteWithTimetable} from "@/types/tranzy";

function isLightColor(hex: string): boolean {
  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);
  return (r * 299 + g * 587 + b * 114) / 1000 > 150;
}

function ScheduleSection({schedule}: {schedule: Schedule}) {
  if (schedule.times.length === 0) return null;

  return (
    <div>
      <h4 className="sticky top-0 z-10 -mx-3 bg-zinc-100 px-3 py-1.5 text-xs font-semibold text-zinc-600 dark:bg-zinc-900 dark:text-zinc-400">
        {schedule.service_name}
      </h4>
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-zinc-200 dark:border-zinc-700">
            <th className="py-1 pr-3 text-left font-medium text-zinc-500 dark:text-zinc-400">
              {schedule.in_stop_name}
            </th>
            <th className="py-1 text-left font-medium text-zinc-500 dark:text-zinc-400">
              {schedule.out_stop_name}
            </th>
          </tr>
        </thead>
        <tbody>
          {schedule.times.map((entry, i) => (
            <tr key={i} className="border-b border-zinc-100 dark:border-zinc-800">
              <td className="py-0.5 pr-3 text-zinc-700 dark:text-zinc-300">{entry.in_time}</td>
              <td className="py-0.5 text-zinc-700 dark:text-zinc-300">{entry.out_time}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TimetablePanel({timetable, routeName, onClose}: {timetable: Timetable; routeName: string; onClose: () => void}) {
  return (
    <div className="flex max-h-112 min-h-48 w-80 shrink-0 self-stretch flex-col rounded-lg border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800">
      <div className="flex items-center justify-between border-b border-zinc-200 px-3 py-2 dark:border-zinc-700">
        <span className="text-xs font-semibold text-zinc-700 dark:text-zinc-300">{routeName}</span>
        <button
          onClick={onClose}
          className="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
        >
          ✕
        </button>
      </div>
      <div className="flex-1 overflow-y-auto px-3 space-y-3 bg-zinc-50 dark:bg-zinc-800">
        <ScheduleSection schedule={timetable.weekday} />
        <ScheduleSection schedule={timetable.saturday} />
        <ScheduleSection schedule={timetable.sunday} />
      </div>
    </div>
  );
}

export default function StopCard({stopName, routes}: {stopName: string; routes: RouteWithTimetable[]}) {
  const [selectedRouteId, setSelectedRouteId] = useState<number | null>(null);

  const selectedRoute = routes.find(r => r.route.route_id === selectedRouteId);
  const activeTimetable = selectedRoute?.timetable ?? null;

  return (
    <li className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
        {stopName}
      </h2>

      {routes.length === 0 && (
        <p className="text-sm text-zinc-500">No routes at this stop.</p>
      )}

      <div className="flex gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          {routes.map(({route, timetable}) => {
            const canExpand = timetable !== null;
            const isSelected = selectedRouteId === route.route_id;

            return (
              <div
                key={route.route_id}
                className={`flex items-start gap-2 rounded px-1.5 py-1 transition-colors ${
                  canExpand ? "cursor-pointer hover:bg-zinc-100 dark:hover:bg-zinc-800" : ""
                } ${isSelected ? "bg-zinc-100 dark:bg-zinc-800" : ""}`}
                onClick={() => {
                  if (!canExpand) return;
                  setSelectedRouteId(isSelected ? null : route.route_id);
                }}
              >
                <span
                  className="mt-0.5 inline-block rounded px-2 py-0.5 text-xs font-bold leading-snug"
                  style={{
                    backgroundColor: route.route_color,
                    color: isLightColor(route.route_color) ? "#000" : "#fff",
                  }}
                >
                  {route.route_short_name}
                </span>

                <div className="min-w-0 text-sm">
                  <span className="font-medium text-zinc-800 dark:text-zinc-200">
                    {route.route_long_name}
                  </span>
                  {canExpand && (
                    <span className="ml-1 text-xs text-zinc-400">
                      {isSelected ? "◀" : "▶"}
                    </span>
                  )}

                  {route.headsigns.outbound.length > 0 && (
                    <p className="text-zinc-500 dark:text-zinc-400 text-xs">
                      → {route.headsigns.outbound.join(", ")}
                    </p>
                  )}
                  {route.headsigns.inbound.length > 0 && (
                    <p className="text-zinc-500 dark:text-zinc-400 text-xs">
                      ← {route.headsigns.inbound.join(", ")}
                    </p>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        {activeTimetable && selectedRoute && (
          <TimetablePanel
            timetable={activeTimetable}
            routeName={selectedRoute.route.route_short_name}
            onClose={() => setSelectedRouteId(null)}
          />
        )}
      </div>
    </li>
  );
}
