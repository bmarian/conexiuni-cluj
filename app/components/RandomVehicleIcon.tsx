"use client";

import {useState} from "react";
import Image from "next/image";

const VEHICLE_ICONS = ["/bus-icon.svg", "/tram-icon.svg", "/trolleybus-icon.svg"];

export default function RandomVehicleIcon({size = 28}: { size?: number }) {
  const [icon] = useState(() => VEHICLE_ICONS[Math.floor(Math.random() * VEHICLE_ICONS.length)]);

  return <Image src={icon} alt="" width={size} height={size} className="shrink-0" />;
}
