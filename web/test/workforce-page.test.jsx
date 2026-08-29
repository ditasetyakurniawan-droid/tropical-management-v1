import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ user: null, api: vi.fn() }));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }) => <a href={href} {...props}>{children}</a>,
}));
vi.mock("../lib/api", () => ({
  api: (...args) => auth.api(...args),
  sessionUser: () => auth.user,
}));

import WorkforcePage from "../app/workforce/page";
import { isoDate } from "../lib/workforce";

function apiFixture(path, options = {}) {
  const today = isoDate();
  if (path === "/api/workforce/summary") return Promise.resolve({ shifts_today: 1, on_duty: 0, pending_time_off: 1, open_tasks: 1 });
  if (path.startsWith("/api/workforce/shifts?")) return Promise.resolve([{ id: 10, employee_id: 3, employee_name: "Ayu Service", shift_date: today, start_time: "11:00", end_time: "19:00", station: "service", status: "scheduled" }]);
  if (path.startsWith("/api/workforce/attendance?")) return Promise.resolve([]);
  if (path === "/api/workforce/time-off" && !options.method) return Promise.resolve([{ id: 5, employee_id: 3, employee_name: "Ayu Service", start_date: today, end_date: today, type: "permission", reason: "Keperluan keluarga", status: "pending" }]);
  if (path.startsWith("/api/workforce/tasks?")) return Promise.resolve([{ id: 7, shift_date: today, title: "Cek kebersihan area service", station: "service", assigned_to_id: 3, assigned_to_name: "Ayu Service", priority: "high", status: "open" }]);
  if (path === "/api/users") return Promise.resolve([{ id: 3, name: "Ayu Service", role: "staff", active: true }, { id: 4, name: "Bima Kitchen", role: "staff", active: true }, { id: 1, name: "Owner", role: "admin", active: true }]);
  if (options.method) return Promise.resolve({ id: 99, status: "ok" });
  return Promise.resolve([]);
}

beforeEach(() => {
  auth.user = { id: 3, name: "Ayu Service", role: "staff" };
  auth.api.mockReset();
  auth.api.mockImplementation(apiFixture);
});

describe("WorkforcePage role-specific operations", () => {
  it("shows employee self service without manager controls", async () => {
    render(<WorkforcePage />);
    await waitFor(() => expect(screen.getByText(/Hari kerja Ayu Service/)).not.toBeNull());
    expect(screen.getByText("Ajukan izin atau cuti")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Clock In" })).not.toBeNull();
    expect(screen.queryByText("Schedule Builder")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Clock In" }));
    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/workforce/attendance/clock-in",
      expect.objectContaining({ method: "POST" }),
    ));

    fireEvent.click(screen.getByRole("button", { name: /Ubah status Cek kebersihan/ }));
    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/workforce/tasks",
      expect.objectContaining({ method: "PATCH" }),
    ));

    fireEvent.change(screen.getByPlaceholderText(/Tuliskan alasan/), { target: { value: "Kontrol keluarga" } });
    fireEvent.submit(screen.getByRole("button", { name: "Kirim Pengajuan" }).closest("form"));
    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/workforce/time-off",
      expect.objectContaining({ method: "POST" }),
    ));
  });

  it("lets PIC schedule the team and review time off without owner access controls", async () => {
    auth.user = { id: 2, name: "PIC Malam", role: "auditor" };
    render(<WorkforcePage />);
    await waitFor(() => expect(screen.getByText("Schedule Builder")).not.toBeNull());
    expect(screen.getByText(/Perubahan akun dan role tetap khusus Owner/)).not.toBeNull();
    expect(screen.queryByText(/Kelola akses karyawan/)).toBeNull();

    fireEvent.change(screen.getByDisplayValue("Pilih anggota tim"), { target: { value: "3" } });
    fireEvent.submit(screen.getByRole("button", { name: "Terbitkan Shift" }).closest("form"));
    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/workforce/shifts",
      expect.objectContaining({ method: "POST" }),
    ));

    fireEvent.change(screen.getByPlaceholderText(/cek suhu chiller/), { target: { value: "Cek suhu chiller" } });
    fireEvent.submit(screen.getByRole("button", { name: "Tambahkan Checklist" }).closest("form"));
    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/workforce/tasks",
      expect.objectContaining({ method: "POST" }),
    ));

    fireEvent.click(screen.getByRole("button", { name: "Setujui" }));
    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/workforce/time-off",
      expect.objectContaining({ method: "PATCH" }),
    ));
  });

  it("shows employee access management only to owner", async () => {
    auth.user = { id: 1, name: "Owner Tropical", role: "admin" };
    render(<WorkforcePage />);
    await waitFor(() => expect(screen.getByText("Schedule Builder")).not.toBeNull());
    const link = screen.getByText(/Kelola akses karyawan/);
    expect(link.getAttribute("href")).toBe("/users");
  });
});
