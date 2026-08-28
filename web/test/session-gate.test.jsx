import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const navState = vi.hoisted(() => ({ pathname: "/" }));
const auth = vi.hoisted(() => ({
  api: vi.fn(),
  clearSession: vi.fn(),
  setSession: vi.fn(),
  token: vi.fn(),
}));
const navigation = vi.hoisted(() => ({ redirectTo: vi.fn() }));

vi.mock("next/navigation", () => ({
  usePathname: () => navState.pathname,
}));
vi.mock("../lib/api", () => auth);
vi.mock("../lib/navigation", () => navigation);

import SessionGate from "../components/SessionGate";

beforeEach(() => {
  navState.pathname = "/";
  auth.api.mockReset();
  auth.clearSession.mockReset();
  auth.setSession.mockReset();
  auth.token.mockReset();
  navigation.redirectTo.mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("SessionGate", () => {
  it("allows the login page without validating a session", () => {
    navState.pathname = "/login";
    render(<SessionGate><div>login-content</div></SessionGate>);
    expect(screen.getByText("login-content")).not.toBeNull();
    expect(auth.api).not.toHaveBeenCalled();
  });

  it("ends a protected session when no token exists", async () => {
    auth.token.mockReturnValue("");
    render(<SessionGate><div>protected</div></SessionGate>);

    await waitFor(() => {
      expect(auth.clearSession).toHaveBeenCalledTimes(1);
      expect(navigation.redirectTo).toHaveBeenCalledWith("/login?reason=signed-out");
    });
  });

  it("validates a live token, stores claims, and renders protected content", async () => {
    auth.token.mockReturnValue("jwt-live");
    auth.api.mockResolvedValue({
      exp: Math.floor(Date.now() / 1000) + 1800,
      sub: 1,
      role: "admin",
      name: "Tropical Admin",
    });

    render(<SessionGate><div>protected-content</div></SessionGate>);

    expect(screen.getByText("Menyiapkan ruang kerja operasional")).not.toBeNull();
    await waitFor(() => expect(screen.getByText("protected-content")).not.toBeNull());
    expect(auth.api).toHaveBeenCalledWith("/api/auth/me");
    expect(auth.setSession).toHaveBeenCalledWith(
      "jwt-live",
      expect.objectContaining({ role: "admin", sub: 1 }),
    );
  });

  it("rejects an expired token", async () => {
    auth.token.mockReturnValue("jwt-expired");
    auth.api.mockResolvedValue({ exp: Math.floor(Date.now() / 1000) - 10 });

    render(<SessionGate><div>protected</div></SessionGate>);

    await waitFor(() => {
      expect(navigation.redirectTo).toHaveBeenCalledWith("/login?reason=expired");
    });
  });

  it("signs out when server-side validation fails", async () => {
    auth.token.mockReturnValue("jwt-invalid");
    auth.api.mockRejectedValue(new Error("unauthorized"));

    render(<SessionGate><div>protected</div></SessionGate>);

    await waitFor(() => {
      expect(navigation.redirectTo).toHaveBeenCalledWith("/login?reason=signed-out");
    });
  });
});
