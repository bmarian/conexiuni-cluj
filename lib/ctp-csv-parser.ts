"use server";

import {ScheduleEntry} from "@/types/ctp";

function cleanValue(s: string): string {
    return s.replace(/^["']+|["']+$/g, '').replace(/\.+$/, '').trim();
}

export async function parseCsv(csv: string) {
    const lines = csv.split('\n').map(line => line.trim()).filter(Boolean);

    const meta: Record<string, string> = {};
    for (let i = 0; i < 5; i++) {
        const idx = lines[i].indexOf(',');
        const key = lines[i].substring(0, idx);
        meta[key] = cleanValue(lines[i].substring(idx + 1));
    }

    const times: ScheduleEntry[] = [];
    for (let i = 5; i < lines.length; i++) {
        const idx = lines[i].indexOf(',');
        const in_time = lines[i].substring(0, idx);
        const out_time = lines[i].substring(idx + 1);
        times.push({ in_time, out_time });
    }

    return {
        route_long_name: meta['route_long_name'],
        schedule: {
            service_name: meta['service_name'],
            service_start: meta['service_start'],
            in_stop_name: meta['in_stop_name'],
            out_stop_name: meta['out_stop_name'],
            times,
        },
    };
}