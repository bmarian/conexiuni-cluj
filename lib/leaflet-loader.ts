// Singleton Leaflet loader — imports leaflet once and caches the result
let leafletPromise: Promise<typeof import("leaflet")> | null = null;
let leafletInstance: typeof import("leaflet") | null = null;

export function getLeaflet(): typeof import("leaflet") | null {
  return leafletInstance;
}

export function loadLeaflet(): Promise<typeof import("leaflet")> {
  if (leafletInstance) return Promise.resolve(leafletInstance);

  if (!leafletPromise) {
    leafletPromise = import("leaflet").then((mod) => {
      leafletInstance = mod.default ? mod.default : (mod as typeof import("leaflet"));
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
