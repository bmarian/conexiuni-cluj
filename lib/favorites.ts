import {RouteType} from "@/types/tranzy";

export interface FavoriteLine {
  route_short_name: string;
  route_long_name: string;
  route_color: string;
  route_type: RouteType;
}

export interface FavoriteStop {
  stop_name: string;
}

const FAV_LINES_KEY = "favorite_lines";
const FAV_STOPS_KEY = "favorite_stops";

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

export function getFavoriteLines(): FavoriteLine[] {
  return getFromStorage<FavoriteLine>(FAV_LINES_KEY);
}

export function getFavoriteStops(): FavoriteStop[] {
  return getFromStorage<FavoriteStop>(FAV_STOPS_KEY);
}

export function isLineFavorite(routeShortName: string): boolean {
  return getFavoriteLines().some((l) => l.route_short_name === routeShortName);
}

export function isStopFavorite(stopName: string): boolean {
  return getFavoriteStops().some((s) => s.stop_name === stopName);
}

export function toggleFavoriteLine(line: FavoriteLine): boolean {
  const items = getFavoriteLines();
  const exists = items.findIndex((l) => l.route_short_name === line.route_short_name);
  if (exists >= 0) {
    items.splice(exists, 1);
    saveToStorage(FAV_LINES_KEY, items);
    return false;
  }
  items.push(line);
  saveToStorage(FAV_LINES_KEY, items);
  return true;
}

export function toggleFavoriteStop(stopName: string): boolean {
  const items = getFavoriteStops();
  const exists = items.findIndex((s) => s.stop_name === stopName);
  if (exists >= 0) {
    items.splice(exists, 1);
    saveToStorage(FAV_STOPS_KEY, items);
    return false;
  }
  items.push({stop_name: stopName});
  saveToStorage(FAV_STOPS_KEY, items);
  return true;
}
