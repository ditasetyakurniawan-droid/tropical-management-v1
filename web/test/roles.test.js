import { describe, expect, it } from "vitest";
import {
  ROLE_ADMIN,
  ROLE_AUDITOR,
  ROLE_STAFF,
  isManagerRole,
  navigationForRole,
  personaForRole,
} from "../lib/roles.js";

describe("role personas and navigation", () => {
  it("maps the legacy backend roles into business personas", () => {
    expect(personaForRole(ROLE_ADMIN)).toBe("owner");
    expect(personaForRole(ROLE_AUDITOR)).toBe("pic");
    expect(personaForRole(ROLE_STAFF)).toBe("employee");
    expect(personaForRole("unknown")).toBe("employee");
    expect(isManagerRole(ROLE_ADMIN)).toBe(true);
    expect(isManagerRole(ROLE_AUDITOR)).toBe(true);
    expect(isManagerRole(ROLE_STAFF)).toBe(false);
  });

  it("keeps employee navigation intentionally narrow", () => {
    const links = navigationForRole(ROLE_STAFF);
    expect(links.map((item) => item.href)).toEqual(["/", "/workforce", "/sales", "/chat"]);
    expect(links.some((item) => item.href === "/users")).toBe(false);
    expect(links.some((item) => item.href === "/inventory")).toBe(false);
  });

  it("gives PIC operational tools but reserves access management for owner", () => {
    const picLinks = navigationForRole(ROLE_AUDITOR);
    const ownerLinks = navigationForRole(ROLE_ADMIN);
    expect(picLinks.some((item) => item.href === "/audits")).toBe(true);
    expect(picLinks.some((item) => item.href === "/inventory")).toBe(true);
    expect(picLinks.some((item) => item.href === "/users")).toBe(false);
    expect(ownerLinks.some((item) => item.href === "/users")).toBe(true);
  });
});
