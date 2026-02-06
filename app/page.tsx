import {getRoutes} from "@/lib/cluj-api";
import TimetableViewer from "@/app/components/TimetableViewer";

export default async function Home() {
    const routes = await getRoutes();

    return (
        <div className="flex min-h-screen flex-col items-center bg-zinc-50 px-4 py-8 font-sans dark:bg-black">
            <TimetableViewer routes={routes}/>
        </div>
    );
}
