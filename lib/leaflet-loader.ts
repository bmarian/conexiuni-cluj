// Singleton Leaflet loader — imports leaflet once and caches the result
import type L from "leaflet";

export type LeafletLib = typeof L;

let leafletPromise: Promise<LeafletLib> | null = null;
let leafletInstance: LeafletLib | null = null;

export function getLeaflet(): LeafletLib | null {
  return leafletInstance;
}

export function loadLeaflet(): Promise<LeafletLib> {
  if (leafletInstance) return Promise.resolve(leafletInstance);

  if (!leafletPromise) {
    leafletPromise = import("leaflet").then((mod) => {
      leafletInstance = mod.default ? mod.default : (mod as LeafletLib);
      return leafletInstance;
    });
  }

  return leafletPromise;
}

export function ensureLeafletCSS(): void {
  if (typeof document === "undefined") return;
  if (document.querySelector('link[href*="leaflet.css"]')) return;
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = "https://unpkg.com/leaflet@1.9.4/dist/leaflet.css";
  document.head.appendChild(link);
}
