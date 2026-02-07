"use client";

import {useEffect} from "react";
import {addRecentLine, addRecentStop} from "@/lib/recent-history";
import {RouteType} from "@/types/tranzy";

export function RecentLineTracker({routeShortName, routeLongName, routeColor, routeType}: {
  routeShortName: string;
  routeLongName: string;
  routeColor: string;
  routeType: RouteType;
}) {
  useEffect(() => {
    addRecentLine({
      route_short_name: routeShortName,
      route_long_name: routeLongName,
      route_color: routeColor,
      route_type: routeType,
    });
  }, [routeShortName, routeLongName, routeColor, routeType]);

  return null;
}

export function RecentStopTracker({stopName}: { stopName: string }) {
  useEffect(() => {
    addRecentStop(stopName);
  }, [stopName]);

  return null;
}
