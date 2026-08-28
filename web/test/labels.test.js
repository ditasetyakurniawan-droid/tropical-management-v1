import { describe, expect, it } from "vitest";

import {
  channelLabel,
  roleLabel,
  severityLabel,
  statusLabel,
} from "../lib/labels.js";

describe("label helpers", () => {
  it("translate known values", () => {
    expect(roleLabel("admin")).toBe("Admin");
    expect(statusLabel("in_progress")).toBe("Diproses");
    expect(severityLabel("critical")).toBe("Kritis");
    expect(channelLabel("dine-in")).toBe("Makan di Tempat");
  });

  it("preserve unknown values and provide fallbacks", () => {
    expect(roleLabel("owner")).toBe("owner");
    expect(statusLabel("waiting_review")).toBe("waiting review");
    expect(severityLabel("urgent")).toBe("urgent");
    expect(channelLabel("kiosk")).toBe("kiosk");
    expect(roleLabel("")).toBe("-");
    expect(statusLabel(null)).toBe("-");
    expect(severityLabel(undefined)).toBe("-");
    expect(channelLabel(null)).toBe("-");
  });
});
