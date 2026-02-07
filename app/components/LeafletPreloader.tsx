"use client";

import {useEffect} from "react";
import {loadLeaflet, ensureLeafletCSS} from "@/lib/leaflet-loader";

export default function LeafletPreloader() {
  useEffect(() => {
    // Start loading Leaflet JS + CSS as soon as the app mounts
    ensureLeafletCSS();
    loadLeaflet();
  }, []);

  return null;
}
