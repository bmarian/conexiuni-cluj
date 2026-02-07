import {useEffect, useRef, useState} from "react";
import type {RouteStopInfo} from "@/lib/cluj-api";
import type {VehicleWithDirection, VehicleWorkerOutput} from "@/types/vehicle-tracking";
import {clientCache} from "@/app/utils/client-cache";

const POLL_INTERVAL = 30_000; // 1 minute

export function useRouteVehicles(
  routeId: number | undefined,
  outboundStops: RouteStopInfo[],
  inboundStops: RouteStopInfo[],
) {
  const [data, setData] = useState<VehicleWorkerOutput | null>(null);
  const workerRef = useRef<Worker | null>(null);

  useEffect(() => {
    if (!routeId) return;

    const worker = new Worker(
      new URL("../workers/vehicle-processor.worker.ts", import.meta.url),
    );
    workerRef.current = worker;

    worker.onmessage = (e: MessageEvent<VehicleWorkerOutput>) => {
      setData(e.data);
    };

    async function fetchAndProcess() {
      try {
        const cacheKey = `vehicles:${routeId}`;
        let json = clientCache.get<{vehicles: VehicleWithDirection[]}>(cacheKey);

        if (!json) {
          const res = await fetch(`/api/vehicles?route_id=${routeId}`);
          if (!res.ok) return;
          json = (await res.json()) as {vehicles: VehicleWithDirection[]};
          clientCache.set(cacheKey, json);
        }

        workerRef.current?.postMessage({
          vehicles: json.vehicles,
          outboundStops,
          inboundStops,
        });
      } catch {
        // Silent fail — maps render without vehicles
      }
    }

    fetchAndProcess();
    const interval = setInterval(fetchAndProcess, POLL_INTERVAL);

    return () => {
      clearInterval(interval);
      worker.terminate();
      workerRef.current = null;
    };
  }, [routeId, outboundStops, inboundStops]);

  return data;
}
