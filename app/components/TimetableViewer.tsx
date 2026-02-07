"use client";

import {useRef, useState} from "react";
import {Route} from "@/types/tranzy";
import {Schedule, Timetable} from "@/types/ctp";

type TabKey = "weekdays" | "saturday" | "sunday";

const TAB_LABELS: Record<TabKey, string> = {
    weekdays: "Weekdays",
    saturday: "Saturday",
    sunday: "Sunday",
};

function ScheduleTable({schedule}: { schedule: Schedule }) {
    return (
        <div className="animate-fade-slide-up mt-4">
            <p className="mb-2 text-sm text-zinc-500 dark:text-zinc-400">
                {schedule.service_name} &mdash; from {schedule.service_start}
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
                        <tr key={i}
                            className="animate-row border-t border-zinc-100 dark:border-zinc-700 even:bg-zinc-50 dark:even:bg-zinc-900"
                            style={{animationDelay: `${i * 30}ms`}}>
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

function LoadingBus() {
    return (
        <div className="mt-6 flex items-center gap-3">
            <svg className="animate-pulse-bus text-blue-500 dark:text-blue-400" width="40" height="24"
                 viewBox="0 0 64 36" fill="none">
                <rect x="2" y="4" width="56" height="24" rx="4" fill="currentColor" opacity="0.9"/>
                <rect x="46" y="8" width="10" height="12" rx="2" fill="white" opacity="0.8"/>
                <rect x="6" y="8" width="8" height="8" rx="1.5" fill="white" opacity="0.7"/>
                <rect x="17" y="8" width="8" height="8" rx="1.5" fill="white" opacity="0.7"/>
                <rect x="28" y="8" width="8" height="8" rx="1.5" fill="white" opacity="0.7"/>
                <circle cx="14" cy="30" r="5" fill="#333"/>
                <circle cx="46" cy="30" r="5" fill="#333"/>
                <rect x="56" y="14" width="4" height="4" rx="1" fill="#fbbf24"/>
            </svg>
            <span className="text-zinc-500 dark:text-zinc-400">Loading timetable...</span>
        </div>
    );
}

export default function TimetableViewer({routes}: { routes: Route[] }) {
    const [selectedRoute, setSelectedRoute] = useState<string>("");
    const [timetable, setTimetable] = useState<Timetable | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [activeTab, setActiveTab] = useState<TabKey>("weekdays");
    const tabsRef = useRef<HTMLDivElement>(null);

    const sortedRoutes = [...routes].sort((a, b) => {
        const aNum = parseInt(a.route_short_name);
        const bNum = parseInt(b.route_short_name);
        if (!isNaN(aNum) && !isNaN(bNum)) return aNum - bNum;
        if (!isNaN(aNum)) return -1;
        if (!isNaN(bNum)) return 1;
        return a.route_short_name.localeCompare(b.route_short_name);
    });

    async function handleRouteChange(routeShortName: string) {
        setSelectedRoute(routeShortName);
        setTimetable(null);
        setError(null);
        setActiveTab("weekdays");

        if (!routeShortName) return;

        setLoading(true);
        try {
            const res = await fetch(`/api/timetable?route=${encodeURIComponent(routeShortName)}`);
            if (!res.ok) throw new Error("Failed to fetch timetable");
            const data = await res.json() as Timetable;
            setTimetable(data);

            // Auto-select first available tab
            if (data.weekdays) setActiveTab("weekdays");
            else if (data.saturday) setActiveTab("saturday");
            else if (data.sunday) setActiveTab("sunday");
        } catch {
            setError("Failed to load timetable. Please try again.");
        } finally {
            setLoading(false);
        }
    }

    const availableTabs = timetable
        ? (Object.entries(TAB_LABELS) as [TabKey, string][]).filter(([key]) => timetable[key] !== null)
        : [];

    const activeSchedule = timetable ? timetable[activeTab] as Schedule | null : null;

    return (
        <div className="w-full max-w-2xl">
            <h2 className="mb-4 text-2xl font-bold text-zinc-900 dark:text-white">Timetable</h2>

            <select
                value={selectedRoute}
                onChange={(e) => handleRouteChange(e.target.value)}
                className="w-full cursor-pointer rounded-lg border border-zinc-300 bg-white px-4 py-2.5 text-zinc-900
                           shadow-sm transition-all duration-200 hover:border-blue-400 hover:shadow-md
                           focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/20
                           dark:border-zinc-600 dark:bg-zinc-800 dark:text-white dark:hover:border-blue-500"
            >
                <option value="">Select a route...</option>
                {sortedRoutes.map((route) => (
                    <option key={route.route_id} value={route.route_short_name}>
                        {route.route_short_name} &mdash; {route.route_long_name}
                    </option>
                ))}
            </select>

            {loading && <LoadingBus/>}

            {error && (
                <p className="animate-fade-slide-up mt-4 text-red-500">{error}</p>
            )}

            {timetable && !loading && (
                <div className="animate-fade-slide-up mt-6">
                    <h3 className="text-xl font-semibold text-zinc-900 dark:text-white">
                        {timetable.route_short_name} &mdash; {timetable.route_long_name}
                    </h3>

                    {/* Tab switcher */}
                    {availableTabs.length > 0 && (
                        <div className="relative mt-4" ref={tabsRef}>
                            <div className="flex gap-1 rounded-lg bg-zinc-100 p-1 dark:bg-zinc-800">
                                {availableTabs.map(([key, label]) => (
                                    <button
                                        key={key}
                                        onClick={() => setActiveTab(key)}
                                        className={`relative flex-1 rounded-md px-4 py-2 text-sm font-medium transition-all duration-200
                                            ${activeTab === key
                                            ? "bg-white text-zinc-900 shadow-sm dark:bg-zinc-700 dark:text-white"
                                            : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                                        }`}
                                    >
                                        {label}
                                    </button>
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Active schedule */}
                    {activeSchedule && (
                        <ScheduleTable key={activeTab} schedule={activeSchedule}/>
                    )}

                    {availableTabs.length === 0 && (
                        <p className="mt-4 text-zinc-500 dark:text-zinc-400">No schedule data available for this
                            route.</p>
                    )}
                </div>
            )}
        </div>
    );
}
