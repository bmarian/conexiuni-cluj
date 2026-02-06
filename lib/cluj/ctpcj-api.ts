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

export async function getTimetable(routeShortName: string, revalidate: number = 60*60*24*365): Promise<Timetable | null> {
  return fetchWithFallback<Timetable | null>(routeShortName, async () => {
      try {
        const suffixes = ['lv', 's', 'd'] as const;
        const responses = await Promise.all(
          suffixes.map(suffix =>
            fetch(`${CTP_CJ_CSV_BASE}/orar_${routeShortName}_${suffix}.csv`)
          )
        );

        for (const res of responses) {
          if (!res.ok) return null;
        }

        const csvTexts = await Promise.all(responses.map(res => res.text()));
        const [weekdayParsed, saturdayParsed, sundayParsed] = csvTexts.map(parseCsv);

        console.log(`Successfully fetched timetable for ${routeShortName}`);
        return {
          route_short_name: routeShortName,
          route_long_name: weekdayParsed.route_long_name,
          weekday: weekdayParsed.schedule,
          saturday: saturdayParsed.schedule,
          sunday: sundayParsed.schedule,
        };
      } catch {
        return null;
      }
    }, revalidate
  );
}
