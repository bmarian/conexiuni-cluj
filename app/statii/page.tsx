import {getStops} from "@/lib/cluj-api";
import StopsGrid from "@/app/components/StopsGrid";

export default async function StatiiPage() {
  const allStops = await getStops();

  // Deduplicate by stop_name and sort alphabetically
  const seen = new Set<string>();
  const uniqueStops = allStops.filter((stop) => {
    const name = stop.stop_name.trim();
    if (seen.has(name)) return false;
    seen.add(name);
    return true;
  }).sort((a, b) => a.stop_name.localeCompare(b.stop_name, "ro"));

  return (
    <div className="mx-auto min-h-screen max-w-5xl px-4 py-8">
      <h1 className="animate-fade-slide-up text-3xl font-bold text-zinc-900 dark:text-white">
        Statii
      </h1>
      <p className="animate-fade-slide-up mt-1 text-zinc-500 dark:text-zinc-400" style={{animationDelay: "0.1s"}}>
        {uniqueStops.length} statii de transport public
      </p>

      <StopsGrid stops={uniqueStops} />
    </div>
  );
}
