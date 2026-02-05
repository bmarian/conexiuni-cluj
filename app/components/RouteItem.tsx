"use client";

import {useState} from "react";
import type {Timetable, Schedule} from "@/types/ctpcj";

interface RouteItemProps {
  route: {
    route_id: number;
    route_short_name: string;
    route_long_name: string;
    route_color: string;
    headsigns: { outbound: string[]; inbound: string[] };
  };
  timetable: Timetable | null;
}

function isLightColor(hex: string): boolean {
  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);
  return (r * 299 + g * 587 + b * 114) / 1000 > 150;
}

function ScheduleSection({schedule}: {schedule: Schedule}) {
  if (schedule.times.length === 0) return null;

  return (
    <div className="mt-2">
      <h4 className="text-xs font-semibold text-zinc-600 dark:text-zinc-400">
        {schedule.service_name}
      </h4>
      <div className="mt-1 overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-zinc-200 dark:border-zinc-700">
              <th className="py-1 pr-4 text-left font-medium text-zinc-500 dark:text-zinc-400">
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
                <td className="py-0.5 pr-4 text-zinc-700 dark:text-zinc-300">{entry.in_time}</td>
                <td className="py-0.5 text-zinc-700 dark:text-zinc-300">{entry.out_time}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function RouteItem({route, timetable}: RouteItemProps) {
  const [expanded, setExpanded] = useState(false);
  const canExpand = timetable !== null;

  return (
    <div>
      <div
        className={`flex items-start gap-2 ${canExpand ? "cursor-pointer" : ""}`}
        onClick={() => canExpand && setExpanded(prev => !prev)}
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
              {expanded ? "▲" : "▼"}
            </span>
          )}

          {route.headsigns.outbound.length > 0 && (
            <p className="text-zinc-500 dark:text-zinc-400">
              → {route.headsigns.outbound.join(", ")}
            </p>
          )}
          {route.headsigns.inbound.length > 0 && (
            <p className="text-zinc-500 dark:text-zinc-400">
              ← {route.headsigns.inbound.join(", ")}
            </p>
          )}
        </div>
      </div>

      {expanded && timetable && (
        <div className="ml-8 mt-2 space-y-3 rounded border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-700 dark:bg-zinc-800">
          <ScheduleSection schedule={timetable.weekday} />
          <ScheduleSection schedule={timetable.saturday} />
          <ScheduleSection schedule={timetable.sunday} />
        </div>
      )}
    </div>
  );
}
