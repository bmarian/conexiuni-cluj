"use server";

import {Schedule, ScheduleEntry, Timetable} from '@/types/ctpcj';
import {fetchWithFallback} from '@/lib/cache';

const CTP_CJ_CSV_BASE = 'https://ctpcj.ro/orare/csv';

function cleanValue(s: string): string {
  return s.replace(/^["']+|["']+$/g, '').replace(/\.+$/, '').trim();
}

function parseCsv(csv: string): { route_long_name: string; schedule: Schedule } {
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

async function fetchSchedule(routeShortName: string, suffix: string): Promise<{ route_long_name: string; schedule: Schedule } | null> {
  try {
    const res = await fetch(`${CTP_CJ_CSV_BASE}/orar_${routeShortName}_${suffix}.csv`);
    if (!res.ok) return null;
    const csv = await res.text();
    return parseCsv(csv);
  } catch {
    return null;
  }
}

export async function getTimetable(routeShortName: string, revalidate: number = 60*60*24*365): Promise<Timetable | null> {
  return fetchWithFallback<Timetable | null>(routeShortName, async () => {
      const [weekdays, saturday, sunday] = await Promise.all([
        fetchSchedule(routeShortName, 'lv'),
        fetchSchedule(routeShortName, 's'),
        fetchSchedule(routeShortName, 'd'),
      ]);

      if (!weekdays && !saturday && !sunday) return null;

      const routeLongName = (weekdays ?? saturday ?? sunday)!.route_long_name;

      console.log(`Successfully fetched timetable for ${routeShortName} (lv:${!!weekdays} s:${!!saturday} d:${!!sunday})`);
      return {
        route_short_name: routeShortName,
        route_long_name: routeLongName,
        weekdays: weekdays?.schedule ?? null,
        saturday: saturday?.schedule ?? null,
        sunday: sunday?.schedule ?? null,
      };
    }, revalidate
  );
}
