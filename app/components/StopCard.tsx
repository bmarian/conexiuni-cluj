"use client";

import {useState, useRef, useEffect, useCallback} from "react";
import type {Timetable, Schedule} from "@/types/ctpcj";
import {RouteWithTimetable} from "@/types/tranzy";

function isLightColor(hex: string): boolean {
  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);
  return (r * 299 + g * 587 + b * 114) / 1000 > 150;
}

type DayTab = "weekday" | "saturday" | "sunday";

const DAY_LABELS: Record<DayTab, string> = {
  weekday: "L-V",
  saturday: "S",
  sunday: "D",
};

function getDefaultDay(): DayTab {
  const day = new Date().getDay();
  if (day === 0) return "sunday";
  if (day === 6) return "saturday";
  return "weekday";
}

function timeToMinutes(time: string): number {
  const [h, m] = time.split(":").map(Number);
  return h * 60 + m;
}

function getCurrentDayTab(): DayTab {
  const day = new Date().getDay();
  if (day === 0) return "sunday";
  if (day === 6) return "saturday";
  return "weekday";
}

function ScheduleTable({schedule, fallbackInName, fallbackOutName, isToday, scrollContainerRef}: {schedule: Schedule; fallbackInName: string; fallbackOutName: string; isToday: boolean; scrollContainerRef: React.RefObject<HTMLDivElement | null>}) {
  const nextRowRef = useRef<HTMLTableRowElement>(null);

  useEffect(() => {
    if (nextRowRef.current && scrollContainerRef.current) {
      const container = scrollContainerRef.current;
      const row = nextRowRef.current;
      // Scroll so the highlighted row is near the top of the visible area
      container.scrollTop = row.offsetTop - container.offsetTop - 30;
    }
  }, [schedule, scrollContainerRef]);

  if (schedule.times.length === 0) {
    return <p className="px-3 py-4 text-center text-xs text-zinc-500">No schedule available.</p>;
  }

  const clean = (s: string) => s.replace(/^["']+|["']+$/g, '').replace(/\.+$/, '').trim();
  const inName = clean(schedule.in_stop_name || fallbackInName);
  const outName = clean(schedule.out_stop_name || fallbackOutName);

  const now = new Date();
  const nowMinutes = now.getHours() * 60 + now.getMinutes();

  // Find the index of the next departure (first row where either time is in the future)
  let nextDepartureIdx = -1;
  if (isToday) {
    for (let i = 0; i < schedule.times.length; i++) {
      const entry = schedule.times[i];
      const inMin = entry.in_time ? timeToMinutes(entry.in_time) : -1;
      const outMin = entry.out_time ? timeToMinutes(entry.out_time) : -1;
      const latestTime = Math.max(inMin, outMin);
      if (latestTime >= nowMinutes) {
        nextDepartureIdx = i;
        break;
      }
    }
  }

  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="sticky top-0 z-10 border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800">
          <th className="py-1.5 pr-3 text-left font-medium text-zinc-500 dark:text-zinc-400">
            {inName}
          </th>
          <th className="py-1.5 text-left font-medium text-zinc-500 dark:text-zinc-400">
            {outName}
          </th>
        </tr>
      </thead>
      <tbody>
        {schedule.times.map((entry, i) => {
          const isPast = isToday && (nextDepartureIdx === -1 || i < nextDepartureIdx);
          const isNext = isToday && i === nextDepartureIdx;

          return (
            <tr
              key={i}
              ref={isNext ? nextRowRef : undefined}
              className={`border-b border-zinc-100 dark:border-zinc-800 ${
                isNext
                  ? "bg-emerald-50 dark:bg-emerald-950"
                  : ""
              }`}
            >
              <td className={`py-0.5 pr-3 ${
                isPast
                  ? "text-zinc-400 dark:text-zinc-600"
                  : isNext
                    ? "font-semibold text-emerald-700 dark:text-emerald-400"
                    : "text-zinc-700 dark:text-zinc-300"
              }`}>{entry.in_time}</td>
              <td className={`py-0.5 ${
                isPast
                  ? "text-zinc-400 dark:text-zinc-600"
                  : isNext
                    ? "font-semibold text-emerald-700 dark:text-emerald-400"
                    : "text-zinc-700 dark:text-zinc-300"
              }`}>{entry.out_time}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function TimetablePanel({timetable, routeName, routeColor, onClose}: {timetable: Timetable; routeName: string; routeColor: string; onClose: () => void}) {
  const [activeDay, setActiveDay] = useState<DayTab>(getDefaultDay);
  const scrollRef = useRef<HTMLDivElement>(null);

  const scheduleMap: Record<DayTab, Schedule> = {
    weekday: timetable.weekday,
    saturday: timetable.saturday,
    sunday: timetable.sunday,
  };

  return (
    <div className="flex max-h-112 min-h-48 w-80 shrink-0 self-stretch flex-col rounded-lg border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-800">
      <div className="flex items-center gap-2 border-b border-zinc-200 px-3 py-2 dark:border-zinc-700">
        <span
          className="inline-block rounded px-2 py-0.5 text-xs font-bold"
          style={{
            backgroundColor: routeColor,
            color: isLightColor(routeColor) ? "#000" : "#fff",
          }}
        >
          {routeName}
        </span>
        <div className="flex flex-1 gap-0.5 rounded-md bg-zinc-200 p-0.5 dark:bg-zinc-700">
          {(Object.keys(DAY_LABELS) as DayTab[]).map((day) => (
            <button
              key={day}
              onClick={() => setActiveDay(day)}
              className={`flex-1 rounded px-2 py-1 text-[11px] font-semibold transition-colors ${
                activeDay === day
                  ? "bg-white text-zinc-900 shadow-sm dark:bg-zinc-600 dark:text-zinc-100"
                  : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
              }`}
            >
              {DAY_LABELS[day]}
            </button>
          ))}
        </div>
        <button
          onClick={onClose}
          className="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
        >
          ✕
        </button>
      </div>
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-3 bg-zinc-50 dark:bg-zinc-800">
        <ScheduleTable
          schedule={scheduleMap[activeDay]}
          fallbackInName={timetable.weekday.in_stop_name}
          fallbackOutName={timetable.weekday.out_stop_name}
          isToday={activeDay === getCurrentDayTab()}
          scrollContainerRef={scrollRef}
        />
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
            routeColor={selectedRoute.route.route_color}
            onClose={() => setSelectedRouteId(null)}
          />
        )}
      </div>
    </li>
  );
}
