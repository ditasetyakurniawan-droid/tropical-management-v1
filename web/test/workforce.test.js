import { describe, expect, it } from "vitest";
import { activeAttendance, addDaysISO, formatDate, formatTime, isoDate, stations } from "../lib/workforce.js";

describe("workforce UI helpers", () => {
  it("formats local dates and relative days", () => {
    const input = new Date(2026, 7, 30, 10, 15, 0);
    expect(isoDate(input)).toBe("2026-08-30");
    expect(addDaysISO(7, input)).toBe("2026-09-06");
    expect(formatDate("2026-08-30")).not.toBe("-");
    expect(formatDate("bad-date")).toBe("bad-date");
    expect(formatDate("")).toBe("-");
  });

  it("formats time safely", () => {
    expect(formatTime("11:45:00")).toBe("11:45");
    expect(formatTime("2026-08-30T11:45:00Z")).not.toBe("-");
    expect(formatTime("not-a-time")).toBe("-");
    expect(formatTime(null)).toBe("-");
  });

  it("finds the active attendance and exposes restaurant stations", () => {
    const active = { id: 2, clock_out: null };
    expect(activeAttendance([{ id: 1, clock_out: "2026-08-30T18:00:00Z" }, active])).toEqual(active);
    expect(activeAttendance([])).toBeNull();
    expect(stations.map(([value]) => value)).toContain("kitchen");
    expect(stations.map(([value]) => value)).toContain("service");
  });
});
