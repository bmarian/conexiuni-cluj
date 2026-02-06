"use client";

import {useState} from "react";
import {Route} from "@/types/tranzy";
import {Schedule, Timetable} from "@/types/ctpcj";

function ScheduleTable({schedule, label}: { schedule: Schedule; label: string }) {
    return (
        <div className="mt-4">
            <h3 className="mb-1 text-lg font-semibold text-zinc-800 dark:text-zinc-200">{label}</h3>
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
                            className="border-t border-zinc-100 dark:border-zinc-700 even:bg-zinc-50 dark:even:bg-zinc-900">
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

export default function TimetableViewer({routes}: { routes: Route[] }) {
    const [selectedRoute, setSelectedRoute] = useState<string>("");
    const [timetable, setTimetable] = useState<Timetable | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

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

        if (!routeShortName) return;

        setLoading(true);
        try {
            const res = await fetch(`/api/timetable?route=${encodeURIComponent(routeShortName)}`);
            if (!res.ok) throw new Error("Failed to fetch timetable");
            const data = await res.json() as Timetable;
            setTimetable(data);
        } catch {
            setError("Failed to load timetable. Please try again.");
        } finally {
            setLoading(false);
        }
    }

    return (
        <div className="w-full max-w-2xl">
            <h2 className="mb-4 text-2xl font-bold text-zinc-900 dark:text-white">Timetable</h2>

            <select
                value={selectedRoute}
                onChange={(e) => handleRouteChange(e.target.value)}
                className="w-full rounded-lg border border-zinc-300 bg-white px-4 py-2 text-zinc-900 shadow-sm
                           focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500
                           dark:border-zinc-600 dark:bg-zinc-800 dark:text-white"
            >
                <option value="">Select a route...</option>
                {sortedRoutes.map((route) => (
                    <option key={route.route_id} value={route.route_short_name}>
                        {route.route_short_name} &mdash; {route.route_long_name}
                    </option>
                ))}
            </select>

            {loading && (
                <p className="mt-4 text-zinc-500 dark:text-zinc-400">Loading timetable...</p>
            )}

            {error && (
                <p className="mt-4 text-red-500">{error}</p>
            )}

            {timetable && (
                <div className="mt-6">
                    <h3 className="text-xl font-semibold text-zinc-900 dark:text-white">
                        {timetable.route_short_name} &mdash; {timetable.route_long_name}
                    </h3>

                    {timetable.weekdays && <ScheduleTable schedule={timetable.weekdays} label="Weekdays"/>}
                    {timetable.saturday && <ScheduleTable schedule={timetable.saturday} label="Saturday"/>}
                    {timetable.sunday && <ScheduleTable schedule={timetable.sunday} label="Sunday"/>}

                    {!timetable.weekdays && !timetable.saturday && !timetable.sunday && (
                        <p className="mt-4 text-zinc-500 dark:text-zinc-400">No schedule data available for this route.</p>
                    )}
                </div>
            )}
        </div>
    );
}
