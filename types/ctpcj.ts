export interface ScheduleEntry {
  in_time: string;   // e.g. "05:00"
  out_time: string;  // e.g. "05:13"
}

export interface Schedule {
  service_name: string;     // e.g. "Luni-Vineri"
  service_start: string;    // e.g. "08.01.2026"
  in_stop_name: string;     // e.g. "Disp. Grigorescu"
  out_stop_name: string;    // e.g. "P-ta Garii Sud"
  times: ScheduleEntry[];
}

export interface Timetable {
  route_short_name: string;
  route_long_name: string;
  weekday: Schedule | null;
  saturday: Schedule | null;
  sunday: Schedule | null;
}
