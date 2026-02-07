import {getStops} from "@/lib/cluj-api";
import HomePage from "@/app/components/HomePage";
import Image from "next/image";

export default async function Home() {
  const stops = await getStops();

  return (
    <div className="mx-auto min-h-screen max-w-3xl px-4 py-8">
      <HomePage stops={stops} />
    </div>
  );
}
