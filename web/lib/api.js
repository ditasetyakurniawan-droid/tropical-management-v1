export const API_URL = process.env.NEXT_PUBLIC_API_URL || "";

const TOKEN_KEY = "tropical_token";
const USER_KEY = "tropical_user";
const REQUEST_TIMEOUT_MS = 15_000;

function clearLegacyLocalStorage() {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  } catch {
    // Storage can be unavailable in restricted browser contexts.
  }
}

export function token() {
  if (typeof window === "undefined") return "";
  clearLegacyLocalStorage();
  try {
    return sessionStorage.getItem(TOKEN_KEY) || "";
  } catch {
    return "";
  }
}

export function sessionUser() {
  if (typeof window === "undefined") return null;
  clearLegacyLocalStorage();
  try {
    const raw = sessionStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function setSession(jwt, user) {
  if (typeof window === "undefined") return;
  clearLegacyLocalStorage();
  try {
    sessionStorage.setItem(TOKEN_KEY, jwt);
    sessionStorage.setItem(USER_KEY, JSON.stringify(user));
  } catch {
    clearSession();
  }
}

export function clearSession() {
  if (typeof window === "undefined") return;
  try {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(USER_KEY);
  } catch {
    // Ignore storage cleanup failures and continue with navigation.
  }
  clearLegacyLocalStorage();
}

export async function api(path, options = {}) {
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };

  const jwt = token();

  if (jwt) {
    headers.Authorization = `Bearer ${jwt}`;
  }

  const controller = options.signal ? null : new AbortController();
  const timeout = controller
    ? globalThis.setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
    : null;

  let response;
  try {
    response = await fetch(`${API_URL}${path}`, {
      ...options,
      headers,
      signal: options.signal || controller?.signal,
      cache: "no-store",
    });
  } catch (error) {
    if (error?.name === "AbortError") {
      throw new Error("Permintaan ke server melewati batas waktu.");
    }
    throw error;
  } finally {
    if (timeout) globalThis.clearTimeout(timeout);
  }

  const payload = await response.json().catch(() => ({}));

  if (response.status === 401 && typeof window !== "undefined") {
    clearSession();
    window.location.replace("/login");
  }

  if (!response.ok) {
    throw new Error(payload.error || `HTTP ${response.status}`);
  }

  return payload;
}
