import {useEffect, useRef, useState} from "react";
import {clientCache} from "@/app/utils/client-cache";
import type {ETAWorkerOutput} from "@/app/workers/eta-processor.worker";

const POLL_INTERVAL = 60_000;

export function useStopArrivals(stopName: string) {
  const [data, setData] = useState<ETAWorkerOutput | null>(null);
  const [loading, setLoading] = useState(true);
  const workerRef = useRef<Worker | null>(null);

  useEffect(() => {
    if (!stopName) return;

    const worker = new Worker(
      new URL("../workers/eta-processor.worker.ts", import.meta.url),
    );
    workerRef.current = worker;

    worker.onmessage = (e: MessageEvent<ETAWorkerOutput>) => {
      setData(e.data);
      setLoading(false);
    };

    async function fetchAndProcess() {
      try {
        const cacheKey = `stop-arrivals:${stopName}`;
        let payload = clientCache.get<unknown>(cacheKey);

        if (!payload) {
          const res = await fetch(`/api/stop-arrivals?stop_name=${encodeURIComponent(stopName)}`);
          if (!res.ok) return;
          payload = await res.json();
          clientCache.set(cacheKey, payload);
        }

        const json = payload as {routes: unknown[]};
        workerRef.current?.postMessage({
          stopName,
          routes: json.routes,
        });
      } catch {
        // Silent fail — component renders without ETAs
      }
    }

    fetchAndProcess();
    const interval = setInterval(fetchAndProcess, POLL_INTERVAL);

    return () => {
      clearInterval(interval);
      worker.terminate();
      workerRef.current = null;
    };
  }, [stopName]);

  return {data, loading};
}
