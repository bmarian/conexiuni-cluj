import {useEffect, useState} from "react";

export interface UserLocation {
  latitude: number;
  longitude: number;
}

/** Returns the user's geolocation (cached for 60s). Null while loading or if denied. */
export function useUserLocation() {
  const [location, setLocation] = useState<UserLocation | null>(null);

  useEffect(() => {
    if (!("geolocation" in navigator)) return;

    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLocation({latitude: pos.coords.latitude, longitude: pos.coords.longitude});
      },
      () => {
        // Permission denied or error
      },
      {enableHighAccuracy: false, timeout: 10_000, maximumAge: 60_000},
    );
  }, []);

  return location;
}
