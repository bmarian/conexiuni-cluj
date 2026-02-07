import {RouteType} from "@/types/tranzy";

export interface RecentLine {
  route_short_name: string;
  route_long_name: string;
  route_color: string;
  route_type: RouteType;
  timestamp: number;
}

export interface RecentStop {
  stop_name: string;
  timestamp: number;
}

const RECENT_LINES_KEY = "recent_lines";
const RECENT_STOPS_KEY = "recent_stops";
const MAX_ITEMS = 10;

function getFromStorage<T>(key: string): T[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveToStorage<T>(key: string, items: T[]): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(key, JSON.stringify(items));
  } catch {
    // localStorage full or unavailable
  }
}

export function addRecentLine(line: Omit<RecentLine, "timestamp">): void {
  const items = getFromStorage<RecentLine>(RECENT_LINES_KEY);
  const filtered = items.filter(
    (l) => l.route_short_name !== line.route_short_name,
  );
  filtered.unshift({...line, timestamp: Date.now()});
  saveToStorage(RECENT_LINES_KEY, filtered.slice(0, MAX_ITEMS));
}

export function addRecentStop(stopName: string): void {
  const items = getFromStorage<RecentStop>(RECENT_STOPS_KEY);
  const filtered = items.filter((s) => s.stop_name !== stopName);
  filtered.unshift({stop_name: stopName, timestamp: Date.now()});
  saveToStorage(RECENT_STOPS_KEY, filtered.slice(0, MAX_ITEMS));
}

export function getRecentLines(): RecentLine[] {
  return getFromStorage<RecentLine>(RECENT_LINES_KEY);
}

export function getRecentStops(): RecentStop[] {
  return getFromStorage<RecentStop>(RECENT_STOPS_KEY);
}
