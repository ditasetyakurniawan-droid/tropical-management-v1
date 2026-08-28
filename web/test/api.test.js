import { afterEach, describe, expect, it, vi } from "vitest";

const navigation = vi.hoisted(() => ({
  redirectTo: vi.fn(),
}));

vi.mock("../lib/navigation", () => navigation);

import {
  api,
  clearSession,
  sessionUser,
  setSession,
  token,
} from "../lib/api.js";

afterEach(() => {
  vi.restoreAllMocks();
  navigation.redirectTo.mockReset();
  sessionStorage.clear();
  localStorage.clear();
  delete globalThis.fetch;
});

describe("session helpers", () => {
  it("are safe during server rendering", () => {
    const browserWindow = globalThis.window;
    vi.stubGlobal("window", undefined);
    expect(token()).toBe("");
    expect(sessionUser()).toBeNull();
    expect(() => setSession("jwt", { id: 1 })).not.toThrow();
    expect(() => clearSession()).not.toThrow();
    vi.stubGlobal("window", browserWindow);
    vi.unstubAllGlobals();
  });

  it("use sessionStorage and remove legacy localStorage", () => {
    localStorage.setItem("tropical_token", "legacy");
    localStorage.setItem("tropical_user", "legacy");

    setSession("jwt-123", { id: 7, role: "admin" });
    expect(token()).toBe("jwt-123");
    expect(sessionUser()).toEqual({ id: 7, role: "admin" });
    expect(localStorage.getItem("tropical_token")).toBeNull();
    expect(localStorage.getItem("tropical_user")).toBeNull();

    clearSession();
    expect(token()).toBe("");
    expect(sessionUser()).toBeNull();
  });

  it("tolerates malformed browser storage", () => {
    sessionStorage.setItem("tropical_user", "not-json");
    expect(sessionUser()).toBeNull();
  });
});

describe("api", () => {
  it("attaches JWT and merges request headers", async () => {
    setSession("jwt-abc", { id: 1 });
    let request;
    globalThis.fetch = vi.fn(async (url, options) => {
      request = { url, options };
      return {
        ok: true,
        status: 200,
        json: async () => ({ status: "ok" }),
      };
    });

    await expect(api("/api/dashboard", {
      method: "GET",
      headers: { "X-Test": "1" },
    })).resolves.toEqual({ status: "ok" });

    expect(request.url).toBe("/api/dashboard");
    expect(request.options.headers.Authorization).toBe("Bearer jwt-abc");
    expect(request.options.headers["X-Test"]).toBe("1");
    expect(request.options.cache).toBe("no-store");
  });

  it("clears the session and redirects on 401", async () => {
    setSession("jwt-expired", { id: 1 });
    globalThis.fetch = vi.fn(async () => ({
      ok: false,
      status: 401,
      json: async () => ({ error: "unauthorized" }),
    }));

    await expect(api("/api/dashboard")).rejects.toThrow("unauthorized");
    expect(token()).toBe("");
    expect(navigation.redirectTo).toHaveBeenCalledWith("/login");
  });

  it("falls back to HTTP status when the response body is not JSON", async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: false,
      status: 503,
      json: async () => { throw new Error("invalid JSON"); },
    }));

    await expect(api("/api/dashboard")).rejects.toThrow("HTTP 503");
  });

  it("converts AbortError into a stable timeout message", async () => {
    globalThis.fetch = vi.fn(async () => {
      const error = new Error("aborted");
      error.name = "AbortError";
      throw error;
    });

    await expect(api("/api/dashboard")).rejects.toThrow(
      "Permintaan ke server melewati batas waktu",
    );
  });

  it("preserves network errors and respects external signals", async () => {
    const controller = new AbortController();
    const expected = new Error("network down");
    let seenSignal;
    globalThis.fetch = vi.fn(async (_url, options) => {
      seenSignal = options.signal;
      throw expected;
    });

    await expect(api("/api/dashboard", { signal: controller.signal })).rejects.toBe(expected);
    expect(seenSignal).toBe(controller.signal);
  });
});
