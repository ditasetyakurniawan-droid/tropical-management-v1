import { describe, expect, it } from "vitest";

import {
  channelLabel,
  priorityLabel,
  roleLabel,
  severityLabel,
  stationLabel,
  statusLabel,
  timeOffTypeLabel,
} from "../lib/labels.js";

describe("label helpers", () => {
  it("translate known restaurant and workforce values", () => {
    expect(roleLabel("admin")).toBe("Owner");
    expect(roleLabel("auditor")).toBe("PIC");
    expect(roleLabel("staff")).toBe("Karyawan");
    expect(statusLabel("in_progress")).toBe("Diproses");
    expect(statusLabel("approved")).toBe("Disetujui");
    expect(severityLabel("critical")).toBe("Kritis");
    expect(channelLabel("dine-in")).toBe("Makan di Tempat");
    expect(stationLabel("cashier")).toBe("Kasir");
    expect(timeOffTypeLabel("permission")).toBe("Izin");
    expect(priorityLabel("high")).toBe("Tinggi");
  });

  it("preserve unknown values and provide fallbacks", () => {
    expect(roleLabel("guest")).toBe("guest");
    expect(statusLabel("waiting_review")).toBe("waiting review");
    expect(severityLabel("urgent")).toBe("urgent");
    expect(channelLabel("kiosk")).toBe("kiosk");
    expect(stationLabel("patio")).toBe("patio");
    expect(timeOffTypeLabel("training")).toBe("training");
    expect(priorityLabel("critical")).toBe("critical");
    expect(roleLabel("")).toBe("-");
    expect(statusLabel(null)).toBe("-");
    expect(severityLabel(undefined)).toBe("-");
    expect(channelLabel(null)).toBe("-");
  });
});
