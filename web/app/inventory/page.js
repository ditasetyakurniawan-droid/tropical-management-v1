"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "../../lib/api";

export default function Inventory() {
  const [items, setItems] = useState([]);
  const [suppliers, setSuppliers] = useState([]);
  const [movements, setMovements] = useState([]);
  const [error, setError] = useState("");
  const [itemForm, setItemForm] = useState({ sku: "", name: "", unit: "pcs", stock: 0, reorder_level: 5, supplier_id: 0 });
  const [supplierForm, setSupplierForm] = useState({ name: "", contact: "", phone: "" });
  const [adjustment, setAdjustment] = useState({ item_id: 0, delta: 1, reason: "Receiving" });

  const load = () => Promise.all([
    api("/api/inventory").then(setItems),
    api("/api/suppliers").then(setSuppliers),
    api("/api/inventory/movements").then(setMovements),
  ]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, []);

  const alerts = useMemo(() => items.filter((x) => Number(x.stock) <= Number(x.reorder_level)), [items]);

  async function createItem(event) {
    event.preventDefault(); setError("");
    try {
      await api("/api/inventory", { method: "POST", body: JSON.stringify({ ...itemForm, stock: Number(itemForm.stock), reorder_level: Number(itemForm.reorder_level), supplier_id: Number(itemForm.supplier_id) }) });
      setItemForm({ ...itemForm, sku: "", name: "", stock: 0 }); await load();
    } catch (e) { setError(e.message); }
  }

  async function createSupplier(event) {
    event.preventDefault(); setError("");
    try { await api("/api/suppliers", { method: "POST", body: JSON.stringify(supplierForm) }); setSupplierForm({ name: "", contact: "", phone: "" }); await load(); }
    catch (e) { setError(e.message); }
  }

  async function adjustStock(event) {
    event.preventDefault(); setError("");
    try { await api("/api/inventory/adjust", { method: "POST", body: JSON.stringify({ ...adjustment, item_id: Number(adjustment.item_id), delta: Number(adjustment.delta) }) }); await load(); }
    catch (e) { setError(e.message); }
  }

  return (
    <main className="page-wrap">
      <div className="grid gap-6 lg:grid-cols-[1fr_auto] lg:items-end">
        <div><p className="eyebrow">Procurement & stock</p><h1 className="page-title mt-3">Inventory Control</h1><div className="gold-rule mt-5" /><p className="page-subtitle mt-5">Track suppliers, stock thresholds and every inventory movement from one controlled workspace.</p></div>
        <div className="grid grid-cols-2 gap-3"><div className="card p-4 text-center"><p className="text-[9px] font-black uppercase tracking-[.13em] text-slate-400">Tracked</p><p className="mt-2 text-2xl font-black text-emerald-950">{items.length}</p></div><div className="card p-4 text-center"><p className="text-[9px] font-black uppercase tracking-[.13em] text-slate-400">Alerts</p><p className="mt-2 text-2xl font-black text-red-600">{alerts.length}</p></div></div>
      </div>
      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-7 grid gap-5 xl:grid-cols-3">
        <form onSubmit={createItem} className="card p-6">
          <p className="section-label">New inventory item</p><h2 className="mt-2 text-xl font-black text-emerald-950">Create stock master</h2>
          <div className="mt-5 space-y-3">
            <input className="input" placeholder="SKU" value={itemForm.sku} onChange={(e) => setItemForm({ ...itemForm, sku: e.target.value })} />
            <input className="input" placeholder="Item name" value={itemForm.name} onChange={(e) => setItemForm({ ...itemForm, name: e.target.value })} />
            <div className="grid grid-cols-2 gap-3"><input className="input" placeholder="Unit" value={itemForm.unit} onChange={(e) => setItemForm({ ...itemForm, unit: e.target.value })} /><input className="input" type="number" min="0" placeholder="Initial stock" value={itemForm.stock} onChange={(e) => setItemForm({ ...itemForm, stock: e.target.value })} /></div>
            <input className="input" type="number" min="0" placeholder="Reorder level" value={itemForm.reorder_level} onChange={(e) => setItemForm({ ...itemForm, reorder_level: e.target.value })} />
            <select className="input" value={itemForm.supplier_id} onChange={(e) => setItemForm({ ...itemForm, supplier_id: e.target.value })}><option value="0">No supplier</option>{suppliers.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}</select>
            <button className="btn w-full">Create Item</button>
          </div>
        </form>

        <form onSubmit={createSupplier} className="card p-6">
          <p className="section-label">Supplier directory</p><h2 className="mt-2 text-xl font-black text-emerald-950">Register supplier</h2>
          <div className="mt-5 space-y-3"><input className="input" placeholder="Supplier name" value={supplierForm.name} onChange={(e) => setSupplierForm({ ...supplierForm, name: e.target.value })} /><input className="input" placeholder="Contact person / email" value={supplierForm.contact} onChange={(e) => setSupplierForm({ ...supplierForm, contact: e.target.value })} /><input className="input" placeholder="Phone" value={supplierForm.phone} onChange={(e) => setSupplierForm({ ...supplierForm, phone: e.target.value })} /><button className="btn btn-gold w-full">Add Supplier</button></div>
          <div className="mt-5 space-y-2">{suppliers.slice(0, 5).map((x) => <div key={x.id} className="rounded-2xl border border-emerald-950/8 bg-white/55 p-3"><p className="font-black text-emerald-950">{x.name}</p><p className="text-xs text-slate-500">{x.contact || "No contact"} · {x.phone || "No phone"}</p></div>)}</div>
        </form>

        <form onSubmit={adjustStock} className="card p-6">
          <p className="section-label">Stock movement</p><h2 className="mt-2 text-xl font-black text-emerald-950">Adjust inventory</h2>
          <div className="mt-5 space-y-3"><select className="input" value={adjustment.item_id} onChange={(e) => setAdjustment({ ...adjustment, item_id: e.target.value })}><option value="0">Select item</option>{items.map((x) => <option key={x.id} value={x.id}>{x.name} · {x.stock} {x.unit}</option>)}</select><input className="input" type="number" step="0.01" value={adjustment.delta} onChange={(e) => setAdjustment({ ...adjustment, delta: e.target.value })} placeholder="+10 receiving / -3 usage" /><input className="input" value={adjustment.reason} onChange={(e) => setAdjustment({ ...adjustment, reason: e.target.value })} placeholder="Reason" /><p className="text-[11px] leading-5 text-slate-400">Positive values add stock. Negative values record consumption, waste or correction. Every adjustment is written to the movement ledger.</p><button className="btn w-full">Post Movement</button></div>
        </form>
      </section>

      <section className="mt-6 grid gap-5 xl:grid-cols-[1.1fr_.9fr]">
        <div className="card p-6">
          <div className="flex items-center justify-between"><div><p className="eyebrow">Stock health</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Inventory master</h2></div><span className="status-pill status-open">{alerts.length} low</span></div>
          <div className="mt-5 grid gap-3 sm:grid-cols-2">{items.length ? items.map((x) => { const low = Number(x.stock) <= Number(x.reorder_level); const supplier = suppliers.find((s) => s.id === x.supplier_id); return <div key={x.id} className="card card-hover p-5"><div className="flex items-start justify-between gap-4"><div><p className="text-[10px] font-black uppercase tracking-[.12em] text-slate-400">{x.sku}</p><h3 className="mt-1 text-lg font-black text-emerald-950">{x.name}</h3><p className="mt-1 text-xs text-slate-500">{supplier?.name || "No supplier assigned"}</p></div><span className={`status-pill ${low ? "severity-critical" : "status-verified"}`}>{low ? "reorder" : "healthy"}</span></div><p className="mt-5 text-3xl font-black tracking-tight text-emerald-950">{x.stock} <span className="text-sm font-bold text-slate-400">{x.unit}</span></p><p className="mt-2 text-xs text-slate-500">Reorder threshold: {x.reorder_level} {x.unit}</p></div>; }) : <div className="empty-state sm:col-span-2">No inventory items yet.</div>}</div>
        </div>

        <div className="card overflow-hidden">
          <div className="border-b border-emerald-950/8 p-5"><p className="section-label">Movement ledger</p></div>
          <div className="max-h-[520px] divide-y divide-emerald-950/7 overflow-auto">{movements.length ? movements.map((x) => <div key={x.id} className="flex items-center justify-between gap-4 p-4"><div><p className="font-black text-emerald-950">{x.item_name}</p><p className="mt-1 text-xs text-slate-500">{x.reason} · {new Date(x.created_at).toLocaleString("id-ID")}</p></div><span className={`text-lg font-black ${Number(x.delta) >= 0 ? "text-emerald-700" : "text-red-600"}`}>{Number(x.delta) >= 0 ? "+" : ""}{x.delta}</span></div>) : <div className="empty-state m-5">No stock movements yet.</div>}</div>
        </div>
      </section>
    </main>
  );
}
