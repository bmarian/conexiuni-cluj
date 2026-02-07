import {getRoutes} from "@/lib/cluj-api";
import TimetableViewer from "@/app/components/TimetableViewer";
import Image from "next/image";

export default async function Home() {
    const routes = await getRoutes();

    return (
        <div className="flex min-h-screen flex-col items-center bg-zinc-50 px-4 py-8 font-sans dark:bg-black overflow-x-clip">
            {/* Animated header */}
            <div className="relative mb-8 w-full max-w-2xl overflow-hidden">
                {/* Driving bus */}
                <div className="pointer-events-none h-10">
                    <div style={{
                        width: 'fit-content',
                        animation: 'drive 20s linear infinite',
                    }}>
                        <Image src="/bus.svg" alt="Bus" width={64} height={36}/>
                    </div>
                </div>

                {/* Road line */}
                <div className="mx-auto mb-6 h-0.5 w-full max-w-2xl bg-zinc-300 dark:bg-zinc-700"
                     style={{backgroundImage: "repeating-linear-gradient(90deg, transparent, transparent 12px, var(--background) 12px, var(--background) 20px)"}}/>

                <h1 className="animate-fade-slide-up text-center text-4xl font-bold text-zinc-900 dark:text-white">
                    Conexiuni Cluj
                </h1>
                <p className="animate-fade-slide-up mt-2 text-center text-zinc-500 dark:text-zinc-400"
                   style={{animationDelay: "0.15s"}}>
                    Public transport timetables
                </p>
            </div>

            <TimetableViewer routes={routes}/>
        </div>
    );
}
