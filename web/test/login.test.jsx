import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({
  clearSession: vi.fn(),
  setSession: vi.fn(),
}));
const navigation = vi.hoisted(() => ({ redirectTo: vi.fn() }));

vi.mock("../lib/api", () => ({
  API_URL: "http://api.local",
  clearSession: auth.clearSession,
  setSession: auth.setSession,
}));
vi.mock("../lib/navigation", () => navigation);

import Login from "../app/login/page";

beforeEach(() => {
  auth.clearSession.mockReset();
  auth.setSession.mockReset();
  navigation.redirectTo.mockReset();
  globalThis.fetch = vi.fn();
  window.history.replaceState({}, "", "/login");
});

describe("Login", () => {
  it("clears stale session state and explains an idle timeout", async () => {
    window.history.replaceState({}, "", "/login?reason=idle");
    render(<Login />);

    await waitFor(() => expect(auth.clearSession).toHaveBeenCalledTimes(1));
    expect(screen.getByText(/Sesi berakhir setelah 15 menit/)).not.toBeNull();
  });

  it("submits credentials, stores the session, and redirects", async () => {
    globalThis.fetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        token: "jwt-123",
        user: { id: 1, role: "admin", name: "Tropical Admin" },
      }),
    });

    render(<Login />);
    fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: "admin@tropical.local" } });
    fireEvent.change(screen.getByLabelText(/Kata Sandi/i), { target: { value: "ChangeThis123!" } });
    fireEvent.submit(screen.getByRole("button", { name: /Masuk ke Pusat Kontrol/i }).closest("form"));

    await waitFor(() => expect(auth.setSession).toHaveBeenCalledWith(
      "jwt-123",
      expect.objectContaining({ role: "admin" }),
    ));
    expect(navigation.redirectTo).toHaveBeenCalledWith("/");
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "http://api.local/api/auth/login",
      expect.objectContaining({
        method: "POST",
        cache: "no-store",
        body: JSON.stringify({
          email: "admin@tropical.local",
          password: "ChangeThis123!",
        }),
      }),
    );
  });

  it("renders the API error returned by a failed login", async () => {
    globalThis.fetch.mockResolvedValue({
      ok: false,
      status: 401,
      json: async () => ({ error: "email atau password salah" }),
    });

    render(<Login />);
    fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: "bad@example.com" } });
    fireEvent.change(screen.getByLabelText(/Kata Sandi/i), { target: { value: "wrong-password" } });
    fireEvent.submit(screen.getByRole("button", { name: /Masuk ke Pusat Kontrol/i }).closest("form"));

    await waitFor(() => expect(screen.getByText("email atau password salah")).not.toBeNull());
    expect(auth.setSession).not.toHaveBeenCalled();
  });

  it("renders a stable message for network failures", async () => {
    globalThis.fetch.mockRejectedValue(new Error("network down"));

    render(<Login />);
    fireEvent.change(screen.getByLabelText(/Email/i), { target: { value: "admin@tropical.local" } });
    fireEvent.change(screen.getByLabelText(/Kata Sandi/i), { target: { value: "password" } });
    fireEvent.submit(screen.getByRole("button", { name: /Masuk ke Pusat Kontrol/i }).closest("form"));

    await waitFor(() => expect(screen.getByText(/API tidak dapat dihubungi/)).not.toBeNull());
  });
});
