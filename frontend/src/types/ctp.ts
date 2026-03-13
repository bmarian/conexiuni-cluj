export type TimetableEntry = {
  DepartureIn: string,
  DepartureOut: string,
}

export type DaySchedule = {
  ServiceName: string,
  ServiceStart: string,
  Entries: TimetableEntry[],
};

export type Timetable = {
  RouteShortName: string,
  RouteLongName: string,
  InStopName: string,
  OutStopName: string,
  Weekdays: DaySchedule,
  Saturday: DaySchedule,
  Sunday: DaySchedule,
};
