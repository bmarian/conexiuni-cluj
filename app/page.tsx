import {getAgency, getFormattedStops} from "@/lib/tranzy-cluj-api";
import StopList from "@/app/components/StopList";

export default async function Home() {
  const {agency_name, agency_url} = await getAgency();
  const stops = await getFormattedStops();

  return (
    <div className="flex min-h-screen flex-col items-center bg-zinc-50 px-4 py-8 font-sans dark:bg-black">
      <h1 className="mb-8 text-3xl font-bold text-zinc-900 dark:text-white">
        <a href={agency_url || ""}>{agency_name}</a>
      </h1>
      <StopList stops={stops} />
    </div>
  );
}
