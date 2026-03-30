export type TimetableEntry = {
  departure_in: string;
  departure_out: string;
};

export type DaySchedule = {
  service_name: string;
  service_start: string;
  entries: TimetableEntry[];
};

export type Timetable = {
  route_short_name: string;
  route_long_name: string;
  in_stop_name: string;
  out_stop_name: string;
  weekdays: DaySchedule;
  saturday: DaySchedule;
  sunday: DaySchedule;
};
