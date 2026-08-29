import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ user: null, api: vi.fn() }));

vi.mock("../lib/api", () => ({
  api: (...args) => auth.api(...args),
  sessionUser: () => auth.user,
}));

import Inventory from "../app/inventory/page";

function apiFixture(path, options = {}) {
  if (path === "/api/inventory" && !options.method) {
    return Promise.resolve([{ id: 1, sku: "BEEF-01", name: "Beef Striploin", unit: "kg", stock: 4, reorder_level: 5, supplier_id: 7 }]);
  }
  if (path === "/api/suppliers" && !options.method) {
    return Promise.resolve([{ id: 7, name: "Supplier Daging", contact: "sales@example.test", phone: "0812" }]);
  }
  if (path === "/api/inventory/movements" && !options.method) return Promise.resolve([]);
  if (options.method) return Promise.resolve({ status: "ok" });
  return Promise.resolve([]);
}

beforeEach(() => {
  auth.api.mockReset();
  auth.api.mockImplementation(apiFixture);
});

describe("inventory role separation", () => {
  it("keeps PIC focused on operational stock adjustments", async () => {
    auth.user = { id: 2, name: "PIC Malam", role: "auditor" };
    render(<Inventory />);

    await waitFor(() => expect(screen.getByText("Beef Striploin")).not.toBeNull());
    expect(screen.getByText(/Mode PIC/)).not.toBeNull();
    expect(screen.getByText("Sesuaikan inventaris")).not.toBeNull();
    expect(screen.queryByText("Buat master stok")).toBeNull();
    expect(screen.queryByText("Daftarkan pemasok")).toBeNull();

    fireEvent.change(screen.getByDisplayValue("Pilih item"), { target: { value: "1" } });
    fireEvent.change(screen.getByPlaceholderText("+10 penerimaan / -3 pemakaian"), { target: { value: "-1" } });
    fireEvent.submit(screen.getByRole("button", { name: "Simpan Pergerakan" }).closest("form"));

    await waitFor(() => expect(auth.api).toHaveBeenCalledWith(
      "/api/inventory/adjust",
      expect.objectContaining({ method: "POST" }),
    ));
  });

  it("keeps inventory master data controls owner-only", async () => {
    auth.user = { id: 1, name: "Owner Tropical", role: "admin" };
    render(<Inventory />);

    await waitFor(() => expect(screen.getByText("Beef Striploin")).not.toBeNull());
    expect(screen.getByText("Buat master stok")).not.toBeNull();
    expect(screen.getByText("Daftarkan pemasok")).not.toBeNull();
    expect(screen.queryByText(/Mode PIC/)).toBeNull();
  });
});

describe("inventory owner operations", () => {
  it("covers owner inventory master workflows", async () => {
    auth.user = { id: 1, name: "Owner Tropical", role: "admin" };
    render(<Inventory />);

    await waitFor(() =>
      expect(screen.getByText("Beef Striploin")).not.toBeNull()
    );

    // Create inventory master item.
    fireEvent.change(screen.getByPlaceholderText("SKU"), {
      target: { value: "CHICKEN-01" },
    });
    fireEvent.change(screen.getByPlaceholderText("Nama item"), {
      target: { value: "Chicken Breast" },
    });
    fireEvent.change(screen.getByPlaceholderText("Satuan"), {
      target: { value: "kg" },
    });
    fireEvent.change(screen.getByPlaceholderText("Stok awal"), {
      target: { value: "12" },
    });
    fireEvent.change(
      screen.getByPlaceholderText("Batas pemesanan ulang"),
      {
        target: { value: "4" },
      },
    );

    fireEvent.submit(
      screen.getByRole("button", { name: "Buat Item" }).closest("form"),
    );

    await waitFor(() =>
      expect(auth.api).toHaveBeenCalledWith(
        "/api/inventory",
        expect.objectContaining({
          method: "POST",
          body: expect.any(String),
        }),
      ),
    );

    const itemCreateCall = auth.api.mock.calls.find(
      ([path, options]) =>
        path === "/api/inventory" && options?.method === "POST",
    );

    expect(JSON.parse(itemCreateCall[1].body)).toEqual(
      expect.objectContaining({
        sku: "CHICKEN-01",
        name: "Chicken Breast",
        unit: "kg",
        stock: 12,
        reorder_level: 4,
      }),
    );

    // Create supplier master.
    fireEvent.change(screen.getByPlaceholderText("Nama pemasok"), {
      target: { value: "Supplier Ayam Temanggung" },
    });
    fireEvent.change(screen.getByPlaceholderText("Kontak / email"), {
      target: { value: "sales@ayam.test" },
    });
    fireEvent.change(screen.getByPlaceholderText("Telepon"), {
      target: { value: "081234567890" },
    });

    fireEvent.submit(
      screen
        .getByRole("button", { name: "Tambah Pemasok" })
        .closest("form"),
    );

    await waitFor(() =>
      expect(auth.api).toHaveBeenCalledWith(
        "/api/suppliers",
        expect.objectContaining({
          method: "POST",
          body: expect.any(String),
        }),
      ),
    );

    const supplierCreateCall = auth.api.mock.calls.find(
      ([path, options]) =>
        path === "/api/suppliers" && options?.method === "POST",
    );

    expect(JSON.parse(supplierCreateCall[1].body)).toEqual({
      name: "Supplier Ayam Temanggung",
      contact: "sales@ayam.test",
      phone: "081234567890",
    });
  });
});
