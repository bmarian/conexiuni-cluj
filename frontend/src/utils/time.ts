export const timeStringToMinutes = (timeString: string): number | null => {
  if (!timeString || !timeString.includes(':')) {
    return null;
  }

  const parts = timeString.trim().split(':');

  const hours = parseInt(parts[0]!, 10);
  const minutes = parseInt(parts[1]!, 10);

  if (isNaN(hours) || isNaN(minutes)) {
    return null;
  }

  return (hours * 60) + minutes;
}

export const getMinutesFromDate = (dateObject: Date): number => {
  const hours = dateObject.getHours();
  const minutes = dateObject.getMinutes();

  return (hours * 60) + minutes;
}

export const formatMinutesFromNow = (minutes: number, referenceDate: Date, nowLabel: string): string => {
  if (minutes === 0) return nowLabel;
  if (minutes < 60) return `${minutes}m`;
  const future = new Date(referenceDate.getTime() + minutes * 60_000);
  return `${future.getHours().toString().padStart(2, '0')}:${future.getMinutes().toString().padStart(2, '0')}`;
}
