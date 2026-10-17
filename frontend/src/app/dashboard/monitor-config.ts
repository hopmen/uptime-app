export const intervalUnits = { seconds: 1, minutes: 60, hours: 3600 } as const;
export type IntervalUnit = keyof typeof intervalUnits;
export const maxIntervalSeconds = 2147483647;

export function intervalInSeconds(value: string, unit: IntervalUnit): number {
  const amount = Number(value);
  const seconds = amount * intervalUnits[unit];
  if (!Number.isSafeInteger(amount) || amount < 1 || seconds > maxIntervalSeconds) {
    throw new Error("Укажите положительное целое число. Интервал не должен превышать 2147483647 секунд.");
  }
  return seconds;
}

export function formatInterval(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600} ч`;
  if (seconds % 60 === 0) return `${seconds / 60} мин`;
  return `${seconds} сек`;
}
