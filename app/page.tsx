import {getAgency, getFormattedStops} from "@/lib/cluj/tranzy-cluj-api";
import StopList from "@/app/components/StopList";
import {getRoutes, getShapes, getStops, getStopTimes, getTrips, getVehicles, getTimetable} from "@/lib/cluj-api";

export default async function Home() {
  // const {agency_name, agency_url} = await getAgency();
  // const stops = await getFormattedStops();

  const routes = await getRoutes();
  const trips = await getTrips();
  const stops = await getStops();
  const stop_times = await getStopTimes();
  const vehicles = await getVehicles();
  const shpaes = await getShapes("1_0");
  const timetable = await getTimetable("1");

  return (
    <div className="flex min-h-screen flex-col items-center bg-zinc-50 px-4 py-8 font-sans dark:bg-black">
      {/*<h1 className="mb-8 text-3xl font-bold text-zinc-900 dark:text-white">*/}
      {/*  <a href={agency_url || ""}>{agency_name}</a>*/}
      {/*</h1>*/}
      {/*<StopList stops={stops} />*/}
    </div>
  );
}
