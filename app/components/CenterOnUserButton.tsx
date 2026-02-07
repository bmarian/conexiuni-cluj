"use client";

import type {LeafletLib} from "@/lib/leaflet-loader";

/**
 * Adds a "center on user" Leaflet control button that matches the native zoom controls.
 * Call this after the map is created. Returns a cleanup function to remove the control.
 */
export function addCenterOnUserControl(
  L: LeafletLib,
  map: L.Map,
  lat: number,
  lon: number,
): L.Control {
  const CenterControl = L.Control.extend({
    options: {position: "topleft" as const},
    onAdd() {
      const container = L.DomUtil.create("div", "leaflet-bar leaflet-control");
      const link = L.DomUtil.create("a", "", container);
      link.href = "#";
      link.title = "Centrează pe locația ta";
      link.role = "button";
      link.setAttribute("aria-label", "Centrează pe locația ta");
      Object.assign(link.style, {
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        width: "30px",
        height: "30px",
      });
      link.innerHTML = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#333" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><line x1="12" y1="2" x2="12" y2="6"/><line x1="12" y1="18" x2="12" y2="22"/><line x1="2" y1="12" x2="6" y2="12"/><line x1="18" y1="12" x2="22" y2="12"/></svg>`;

      L.DomEvent.disableClickPropagation(container);
      L.DomEvent.on(link, "click", (e) => {
        L.DomEvent.preventDefault(e);
        map.setView([lat, lon], 16, {animate: true});
      });

      return container;
    },
  });

  const control = new CenterControl();
  control.addTo(map);
  return control;
}
