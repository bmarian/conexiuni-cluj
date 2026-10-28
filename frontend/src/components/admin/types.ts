export type Named = {id: number; name: string; color?: string; count: number}

export type Accuracy = {
  label: string
  n: number
  within_1: number
  median_abs_sec: number
  p90_abs_sec: number
  bias_sec: number
}

export type ErrorDetail = {endpoint: string; status: number; text: string; count: number; last_at: string; path: string}

export type CitySummary = {
  slug: string
  name: string
  realtime: boolean
  watching: number
  people_today: number
  problems: string[]
  errors: ErrorDetail[]
}

export type LoggedProblem = {first: string; last: string; count: number; text: string}

export type Site = {
  generated_at: string
  problems: string[]
  cities: CitySummary[]
  server: {
    started_at: string
    revision: string
    modified: boolean
    heap_bytes: number
    db_bytes: number
    disk_free: number
    disk_total: number
    requests: number
    errors: number
    problems: LoggedProblem[]
    sync?: {chains: number; devices: Record<string, number>; db_bytes: number}
  }
}

export type Overview = {
  generated_at: string
  problems: string[]
  now: {
    streams: number
    people: number
    buses_tracked: number
    buses_in_service: number
    lines_live: number
    feed_at: string | null
    fix_age_sec: number
    poll_sec: number
    polled_by: '' | 'viewers' | 'learner'
    feed_error?: string
    stops: Named[]
    lines: Named[]
  }
  accuracy: {
    today: Accuracy[]
    week: Accuracy[]
    daily: (Accuracy & {date: string})[]
    timetable: Accuracy[]
  }
  people: {
    today: number
    new: number
    last_week: number
    platforms: {platform: string; today: number; last_week: number}[]
    daily: {date: string; total: number; web: number; android: number; wear: number}[]
    versions: {platform: string; version: string; today: number; last_seen: string}[]
    stops: Named[]
    lines: Named[]
    plans: number
    places: number
    installs: number
  }
  quota: {
    quotas: {name: string; used: number; limit: number}[]
    learning: {enabled: boolean; dedicated: boolean; used: number; budget: number}
  }
  data: {
    warmup_at: string | null
    warmup_next: string
    warmup_running: boolean
    route_changes: {line: string; color: string; detected_at: number; kinds: string[]}[]
    samples_today: number
    samples_hour: number
    delays_at: string | null
    archive: {enabled: boolean; ran_at: string | null; error?: string; bytes: number; oldest_date?: string}
  }
  traffic: {
    db_bytes: number
    requests: number
    errors: number
    endpoints: {key: string; calls: number; avg_ms: number; errors: number}[]
    error_details: ErrorDetail[]
  }
}

export type LearningWindow = {
  day_type: string
  days: number
  from?: string
  last_year_days: number
}

export type LineHour = {hour: number; trip_sec?: number; delay_sec?: number}

export type LineDirection = {
  trip_sec?: number
  delay_sec?: number
  hours: LineHour[]
  slowest?: {from: string; to: string; hour: number; extra_sec: number}
}

export type LineStats = {day_type: string; directions: LineDirection[]}
