"use client";

import {useState} from "react";
import type {Schedule, Timetable} from "@/types/ctp";

type TabKey = "weekdays" | "saturday" | "sunday";

const TAB_LABELS: Record<TabKey, string> = {
  weekdays: "Luni-Vineri",
  saturday: "Sâmbătă",
  sunday: "Duminică",
};

function ScheduleTable({schedule}: { schedule: Schedule }) {
  return (
    <div className="animate-fade-slide-up mt-4">
      <p className="mb-2 text-sm text-zinc-500 dark:text-zinc-400">
        {schedule.service_name} &mdash; din {schedule.service_start}
      </p>
      <div className="overflow-x-auto rounded-lg border border-zinc-200 dark:border-zinc-700">
        <table className="w-full text-sm">
          <thead>
          <tr className="bg-zinc-100 dark:bg-zinc-800">
            <th className="px-4 py-2 text-left font-medium text-zinc-700 dark:text-zinc-300">
              {schedule.in_stop_name}
            </th>
            <th className="px-4 py-2 text-left font-medium text-zinc-700 dark:text-zinc-300">
              {schedule.out_stop_name}
            </th>
          </tr>
          </thead>
          <tbody>
          {schedule.times.map((entry, i) => (
            <tr
              key={i}
              className="animate-row border-t border-zinc-100 dark:border-zinc-700 even:bg-zinc-50 dark:even:bg-zinc-900"
              style={{animationDelay: `${Math.min(i * 20, 400)}ms`}}
            >
              <td className="px-4 py-1.5 text-zinc-700 dark:text-zinc-300">{entry.in_time}</td>
              <td className="px-4 py-1.5 text-zinc-700 dark:text-zinc-300">{entry.out_time}</td>
            </tr>
          ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function TimetableDisplay({timetable}: { timetable: Timetable }) {
  const availableTabs = (Object.entries(TAB_LABELS) as [TabKey, string][])
    .filter(([key]) => timetable[key] !== null);

  const [activeTab, setActiveTab] = useState<TabKey>(
    availableTabs.length > 0 ? availableTabs[0][0] : "weekdays"
  );

  const activeSchedule = timetable[activeTab] as Schedule | null;

  if (availableTabs.length === 0) {
    return (
      <div className="mt-6 text-center text-zinc-400 dark:text-zinc-500">
        <p className="text-lg">Nu sunt date disponibile pentru această linie</p>
      </div>
    );
  }

  return (
    <div className="mt-6">
      {/* Tab switcher */}
      <div className="flex gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800">
        {availableTabs.map(([key, label]) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`relative flex-1 rounded-md px-4 py-2 text-sm font-medium transition-all duration-200 ${
              activeTab === key
                ? "bg-white text-zinc-900 shadow-sm dark:bg-zinc-700 dark:text-white"
                : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Active schedule */}
      {activeSchedule && <ScheduleTable key={activeTab} schedule={activeSchedule} />}
    </div>
  );
}
