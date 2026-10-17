import { describe, expect, it } from "vitest";
import { formatInterval, intervalInSeconds, maxIntervalSeconds, type IntervalUnit } from "./monitor-config";

describe("monitor interval", () => {
  it.each<[string, IntervalUnit, number]>([
    ["15", "seconds", 15], ["5", "minutes", 300], ["2", "hours", 7200],
    ["1", "seconds", 1], [String(maxIntervalSeconds), "seconds", maxIntervalSeconds],
  ])("converts %s %s to seconds", (value, unit, expected) => {
    expect(intervalInSeconds(value, unit)).toBe(expected);
  });

  it.each(["", "0", "-1", "1.5", "NaN", "Infinity", "2147483648"])("rejects invalid interval %s", (value) => {
    expect(() => intervalInSeconds(value, "seconds")).toThrow();
  });

  it("rejects overflow after conversion", () => {
    expect(() => intervalInSeconds("596524", "hours")).toThrow();
  });

  it.each([[15, "15 сек"], [300, "5 мин"], [7200, "2 ч"]])("formats %s seconds", (seconds, expected) => {
    expect(formatInterval(seconds as number)).toBe(expected);
  });
});
