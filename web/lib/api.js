export const API_URL = process.env.NEXT_PUBLIC_API_URL || "";

export function token() {
  if (typeof window === "undefined") return "";
  return localStorage.getItem("tropical_token") || "";
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

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
    cache: "no-store",
  });

  const payload = await response.json().catch(() => ({}));

  if (response.status === 401 && typeof window !== "undefined") {
    window.location.href = "/login";
  }

  if (!response.ok) {
    throw new Error(payload.error || `HTTP ${response.status}`);
  }

  return payload;
}