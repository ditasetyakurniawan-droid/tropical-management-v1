import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ user: null, api: vi.fn() }));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }) => <a href={href} {...props}>{children}</a>,
}));
vi.mock("../lib/api", () => ({
  api: (...args) => auth.api(...args),
  sessionUser: () => auth.user,
}));

import Dashboard from "../app/page";
import { isoDate } from "../lib/workforce";

function apiFixture(path) {
  const today = isoDate();
  if (path === "/api/workforce/summary") return Promise.resolve({ shifts_today: 2, on_duty: 1, pending_time_off: 1, open_tasks: 2 });
  if (path.startsWith("/api/workforce/shifts?")) return Promise.resolve([{ id: 1, shift_date: today, start_time: "11:00", end_time: "19:00", station: "service" }]);
  if (path.startsWith("/api/workforce/tasks?")) return Promise.resolve([{ id: 1, title: "Opening service", station: "service", assigned_to_name: "Ayu", priority: "high", status: "open" }]);
  if (path === "/api/dashboard") return Promise.resolve({ sales_today: 1500000, orders_today: 52, audit_score: 96, open_issues: 2, inventory_alerts: 1, total_items: 20 });
  if (path === "/api/sales") return Promise.resolve([{ id: 1, business_date: today, revenue: 1500000 }]);
  if (path === "/api/audits") return Promise.resolve([{ id: 1, restaurant: "Tropical Steak House", score: 96 }]);
  if (path === "/api/issues") return Promise.resolve([{ id: 1, severity: "high", status: "open" }]);
  if (path === "/api/inventory") return Promise.resolve([{ id: 1, name: "Beef Striploin", stock: 4, reorder_level: 5, unit: "kg" }]);
  return Promise.resolve([]);
}

beforeEach(() => {
  auth.api.mockReset();
  auth.api.mockImplementation(apiFixture);
});

describe("role-aware dashboard", () => {
  it("keeps employee home focused on personal shift work", async () => {
    auth.user = { id: 3, name: "Ayu Service", role: "staff" };
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText(/Selamat bekerja, Ayu Service/)).not.toBeNull());
    expect(screen.getByText("Quick Access")).not.toBeNull();
    expect(auth.api).toHaveBeenCalledTimes(3);
    expect(auth.api).not.toHaveBeenCalledWith("/api/dashboard");
  });

  it("shows PIC operational command without owner wording", async () => {
    auth.user = { id: 2, name: "PIC Malam", role: "auditor" };
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText("Kontrol Operasional Shift")).not.toBeNull());
    expect(screen.getByText("Kesiapan tim hari ini")).not.toBeNull();
    expect(screen.queryByText("Pusat Kendali Bisnis")).toBeNull();
    expect(auth.api).toHaveBeenCalledWith("/api/dashboard");
  });

  it("labels the admin dashboard as owner intelligence", async () => {
    auth.user = { id: 1, name: "Owner Tropical", role: "admin" };
    render(<Dashboard />);
    await waitFor(() => expect(screen.getByText("Pusat Kendali Bisnis")).not.toBeNull());
    expect(screen.getByText(/Owner Intelligence/)).not.toBeNull();
  });
});
