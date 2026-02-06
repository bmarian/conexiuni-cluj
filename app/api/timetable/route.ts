import {NextRequest, NextResponse} from "next/server";
import {getTimetable} from "@/lib/cluj-api";

export async function GET(request: NextRequest) {
    const routeShortName = request.nextUrl.searchParams.get("route");
    if (!routeShortName) {
        return NextResponse.json({error: "Missing route parameter"}, {status: 400});
    }

    try {
        const timetable = await getTimetable(routeShortName);
        return NextResponse.json(timetable);
    } catch {
        return NextResponse.json({error: "Failed to fetch timetable"}, {status: 500});
    }
}
