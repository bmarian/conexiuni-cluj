"use client";

import {useEffect, useRef, useState} from "react";
import Image from "next/image";
import type {Schedule, Timetable} from "@/types/ctp";
import {RouteType} from "@/types/tranzy";

type TabKey = "weekdays" | "saturday" | "sunday";

const TAB_LABELS: Record<TabKey, string> = {
  weekdays: "Luni-Vineri",
  saturday: "Sâmbătă",
  sunday: "Duminică",
};

function getVehicleIcon(routeType?: RouteType): string {
  if (routeType === RouteType.Tram) return "/tram-icon.svg";
  if (routeType === RouteType.Trolleybus) return "/trolleybus-icon.svg";
  return "/bus-icon.svg";
}

function getTodayTab(): TabKey {
  const day = new Date().getDay();
  if (day === 0) return "sunday";
  if (day === 6) return "saturday";
  return "weekdays";
}

/** Returns true if the tab's day(s) have already passed relative to today this week */
function isPastDay(tab: TabKey, todayTab: TabKey): boolean {
  if (tab === todayTab) return false;
  // Order: weekdays (Mon-Fri) < saturday < sunday
  const order: Record<TabKey, number> = {weekdays: 0, saturday: 1, sunday: 2};
  return order[tab] < order[todayTab];
}

function nowMinutes(): number {
  const now = new Date();
  return now.getHours() * 60 + now.getMinutes();
}

function timeToMinutes(time: string): number {
  const [h, m] = time.split(":").map(Number);
  return h * 60 + m;
}

function formatWait(minutes: number): string {
  if (minutes < 1) return "acum";
  if (minutes < 60) return `în ${Math.round(minutes)} min`;
  const h = Math.floor(minutes / 60);
  const m = Math.round(minutes % 60);
  return m > 0 ? `în ${h}h ${m}min` : `în ${h}h`;
}

function ScheduleTable({schedule, isToday, isPast, routeType, now}: {
  schedule: Schedule;
  isToday: boolean;
  isPast: boolean;
  routeType?: RouteType;
  now: number;
}) {
  const nextRowRef = useRef<HTMLTableRowElement>(null);
  const hasScrolled = useRef(false);
  const icon = getVehicleIcon(routeType);

  const nextInIndex = isToday
    ? schedule.times.findIndex((e) => timeToMinutes(e.in_time) >= now)
    : -1;
  const nextOutIndex = isToday
    ? schedule.times.findIndex((e) => timeToMinutes(e.out_time) >= now)
    : -1;

  // Row to scroll to: earliest of the two next indices
  const scrollTargetIndex = isToday
    ? nextInIndex >= 0 && nextOutIndex >= 0
      ? Math.min(nextInIndex, nextOutIndex)
      : nextInIndex >= 0 ? nextInIndex : nextOutIndex
    : -1;

  useEffect(() => {
    if (nextRowRef.current && !hasScrolled.current) {
      hasScrolled.current = true;
      const container = nextRowRef.current.closest(".overflow-auto");
      if (container) {
        const rowTop = nextRowRef.current.offsetTop;
        container.scrollTo({top: rowTop - container.clientHeight / 2, behavior: "smooth"});
      }
    }
  }, [scrollTargetIndex]);

  return (
    <div className="animate-fade-slide-up mt-4">
      <p className="mb-2 text-sm text-zinc-500 dark:text-zinc-400">
        {schedule.service_name} &mdash; din {schedule.service_start}
      </p>

      {/* Past day banner */}
      {isPast && (
        <div className="mb-3 flex items-center gap-2 rounded-lg bg-zinc-100 px-3 py-2 text-sm text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
          <span className="text-base">📅</span>
          Orarul pentru o zi trecută
        </div>
      )}

      {/* Next bus card for today */}
      {isToday && (nextInIndex >= 0 || nextOutIndex >= 0) && (
        <div className="mb-3 flex items-center gap-3 rounded-xl border-2 border-purple-200 bg-purple-50 px-4 py-3 dark:border-purple-800 dark:bg-purple-950/40">
          <Image src={icon} alt="" width={28} height={28} className="shrink-0" />
          <div className="flex flex-col gap-1">
            <p className="text-xs font-medium text-purple-600 dark:text-purple-400">Următoarea plecare</p>
            <div className="flex flex-wrap gap-x-4 gap-y-0.5">
              {nextInIndex >= 0 && (
                <p className="text-base font-bold text-purple-700 dark:text-purple-300">
                  <span className="text-xs font-medium text-purple-500 dark:text-purple-400">{schedule.in_stop_name}: </span>
                  {schedule.times[nextInIndex].in_time}
                  <span className="ml-1.5 text-sm font-normal text-purple-500 dark:text-purple-400">
                    {formatWait(timeToMinutes(schedule.times[nextInIndex].in_time) - now)}
                  </span>
                </p>
              )}
              {nextOutIndex >= 0 && (
                <p className="text-base font-bold text-purple-700 dark:text-purple-300">
                  <span className="text-xs font-medium text-purple-500 dark:text-purple-400">{schedule.out_stop_name}: </span>
                  {schedule.times[nextOutIndex].out_time}
                  <span className="ml-1.5 text-sm font-normal text-purple-500 dark:text-purple-400">
                    {formatWait(timeToMinutes(schedule.times[nextOutIndex].out_time) - now)}
                  </span>
                </p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* No more buses today */}
      {isToday && nextInIndex === -1 && nextOutIndex === -1 && (
        <div className="mb-3 flex items-center gap-2 rounded-lg bg-zinc-100 px-3 py-2 text-sm text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
          <span className="text-base">🏁</span>
          Nu mai sunt curse astăzi
        </div>
      )}

      <div className="overflow-auto rounded-lg border border-zinc-200 max-h-[60vh] md:max-h-125 dark:border-zinc-700">
        <table className="w-full text-sm">
          <thead className="sticky top-0 z-10">
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
          {schedule.times.map((entry, i) => {
            const inPast = isToday && timeToMinutes(entry.in_time) < now;
            const outPast = isToday && timeToMinutes(entry.out_time) < now;
            const isNextIn = isToday && i === nextInIndex;
            const isNextOut = isToday && i === nextOutIndex;
            const isScrollTarget = i === scrollTargetIndex;

            return (
              <tr
                key={i}
                ref={isScrollTarget ? nextRowRef : undefined}
                className={`animate-row ${
                  (isPast || (inPast && outPast))
                    ? ""
                    : "even:bg-zinc-50 dark:even:bg-zinc-900"
                }`}
                style={{animationDelay: `${Math.min(i * 20, 400)}ms`}}
              >
                <td className={`px-4 py-1.5 w-1/2 ${
                  isNextIn
                    ? "font-semibold text-purple-700 bg-purple-50 dark:text-purple-300 dark:bg-purple-950/30"
                    : (isPast || inPast)
                      ? "text-zinc-400 line-through decoration-zinc-300 dark:text-zinc-600 dark:decoration-zinc-700"
                      : "text-zinc-700 dark:text-zinc-300"
                }`}>
                  {entry.in_time}
                </td>
                <td className={`px-4 py-1.5 w-1/2 ${
                  isNextOut
                    ? "font-semibold text-purple-700 bg-purple-50 dark:text-purple-300 dark:bg-purple-950/30"
                    : (isPast || outPast)
                      ? "text-zinc-400 line-through decoration-zinc-300 dark:text-zinc-600 dark:decoration-zinc-700"
                      : "text-zinc-700 dark:text-zinc-300"
                }`}>
                  {entry.out_time}
                </td>
              </tr>
            );
          })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default function TimetableDisplay({timetable, routeType}: { timetable: Timetable; routeType?: RouteType }) {
  const availableTabs = (Object.entries(TAB_LABELS) as [TabKey, string][])
    .filter(([key]) => timetable[key] !== null);

  const todayTab = getTodayTab();
  const defaultTab = availableTabs.find(([key]) => key === todayTab)?.[0]
    ?? (availableTabs.length > 0 ? availableTabs[0][0] : "weekdays");

  const [activeTab, setActiveTab] = useState<TabKey>(defaultTab);
  const [now, setNow] = useState(nowMinutes);

  // Update current time every 30 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      setNow(nowMinutes());
    }, 30_000);
    return () => clearInterval(interval);
  }, []);

  const activeSchedule = timetable[activeTab] as Schedule | null;
  const isToday = activeTab === todayTab;
  const pastDay = isPastDay(activeTab, todayTab);

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
            {key === todayTab && (
              <span className="ml-1.5 inline-block h-1.5 w-1.5 rounded-full bg-purple-500" />
            )}
          </button>
        ))}
      </div>

      {/* Active schedule */}
      {activeSchedule && (
        <ScheduleTable
          key={activeTab}
          schedule={activeSchedule}
          isToday={isToday}
          isPast={pastDay}
          routeType={routeType}
          now={now}
        />
      )}
    </div>
  );
}
