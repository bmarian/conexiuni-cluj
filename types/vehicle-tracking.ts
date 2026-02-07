import type {Vehicle} from "./tranzy";
import type {RouteStopInfo} from "@/lib/cluj-api";

export interface VehicleWithDirection extends Vehicle {
  direction_id?: number; // 0=outbound, 1=inbound
}

export interface VehicleWorkerInput {
  vehicles: VehicleWithDirection[];
  outboundStops: RouteStopInfo[];
  inboundStops: RouteStopInfo[];
}

export interface VehicleWorkerOutput {
  /** stop_id → vehicles near that stop (for LinearRouteMap) */
  stopVehicleMap: {
    outbound: Record<number, VehicleWithDirection[]>;
    inbound: Record<number, VehicleWithDirection[]>;
  };
  /** vehicles split by direction (for RouteMap / Leaflet) */
  directionVehicles: {
    outbound: VehicleWithDirection[];
    inbound: VehicleWithDirection[];
  };
}
