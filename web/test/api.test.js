import assert from "node:assert/strict";
import test from "node:test";

import {
  api,
  clearSession,
  sessionUser,
  setSession,
  token,
} from "../lib/api.js";

class MemoryStorage {
  constructor() {
    this.values = new Map();
  }
  getItem(key) {
    return this.values.has(key) ? this.values.get(key) : null;
  }
  setItem(key, value) {
    this.values.set(key, String(value));
  }
  removeItem(key) {
    this.values.delete(key);
  }
}

function installBrowser() {
  const redirects = [];
  globalThis.localStorage = new MemoryStorage();
  globalThis.sessionStorage = new MemoryStorage();
  globalThis.window = {
    location: {
      replace(path) {
        redirects.push(path);
      },
    },
  };
  return redirects;
}

function uninstallBrowser() {
  delete globalThis.window;
  delete globalThis.localStorage;
  delete globalThis.sessionStorage;
  delete globalThis.fetch;
}

test.afterEach(() => {
  uninstallBrowser();
});

test("session helpers are safe during server rendering", () => {
  assert.equal(token(), "");
  assert.equal(sessionUser(), null);
  assert.doesNotThrow(() => setSession("jwt", { id: 1 }));
  assert.doesNotThrow(() => clearSession());
});

test("session helpers use sessionStorage and remove legacy localStorage", () => {
  installBrowser();
  localStorage.setItem("tropical_token", "legacy");
  localStorage.setItem("tropical_user", "legacy");

  setSession("jwt-123", { id: 7, role: "admin" });
  assert.equal(token(), "jwt-123");
  assert.deepEqual(sessionUser(), { id: 7, role: "admin" });
  assert.equal(localStorage.getItem("tropical_token"), null);
  assert.equal(localStorage.getItem("tropical_user"), null);

  clearSession();
  assert.equal(token(), "");
  assert.equal(sessionUser(), null);
});

test("sessionUser tolerates malformed browser storage", () => {
  installBrowser();
  sessionStorage.setItem("tropical_user", "not-json");
  assert.equal(sessionUser(), null);
});

test("api attaches JWT and merges request headers", async () => {
  installBrowser();
  setSession("jwt-abc", { id: 1 });

  let request;
  globalThis.fetch = async (url, options) => {
    request = { url, options };
    return {
      ok: true,
      status: 200,
      async json() {
        return { status: "ok" };
      },
    };
  };

  const result = await api("/api/dashboard", {
    method: "GET",
    headers: { "X-Test": "1" },
  });

  assert.deepEqual(result, { status: "ok" });
  assert.equal(request.url, "/api/dashboard");
  assert.equal(request.options.headers.Authorization, "Bearer jwt-abc");
  assert.equal(request.options.headers["X-Test"], "1");
  assert.equal(request.options.cache, "no-store");
});

test("api clears the session and redirects on 401", async () => {
  const redirects = installBrowser();
  setSession("jwt-expired", { id: 1 });
  globalThis.fetch = async () => ({
    ok: false,
    status: 401,
    async json() {
      return { error: "unauthorized" };
    },
  });

  await assert.rejects(() => api("/api/dashboard"), /unauthorized/);
  assert.equal(token(), "");
  assert.deepEqual(redirects, ["/login"]);
});

test("api falls back to HTTP status when the response body is not JSON", async () => {
  installBrowser();
  globalThis.fetch = async () => ({
    ok: false,
    status: 503,
    async json() {
      throw new Error("invalid JSON");
    },
  });

  await assert.rejects(() => api("/api/dashboard"), /HTTP 503/);
});

test("api converts AbortError into a stable timeout message", async () => {
  installBrowser();
  globalThis.fetch = async () => {
    const error = new Error("aborted");
    error.name = "AbortError";
    throw error;
  };

  await assert.rejects(
    () => api("/api/dashboard"),
    /Permintaan ke server melewati batas waktu/,
  );
});

test("api preserves non-timeout network errors and respects external signals", async () => {
  installBrowser();
  const controller = new AbortController();
  const expected = new Error("network down");
  let seenSignal;
  globalThis.fetch = async (_url, options) => {
    seenSignal = options.signal;
    throw expected;
  };

  await assert.rejects(
    () => api("/api/dashboard", { signal: controller.signal }),
    (error) => error === expected,
  );
  assert.equal(seenSignal, controller.signal);
});
