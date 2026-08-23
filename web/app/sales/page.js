"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "../../lib/api";
import { channelLabel } from "../../lib/labels";

const channels = ["dine-in", "takeaway", "gofood", "grabfood", "shopeefood", "website", "corporate"];

export default function Sales() {
  const [list, setList] = useState([]);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ business_date: new Date().toISOString().slice(0, 10), orders: 1, revenue: 0, channel: "dine-in" });
  const load = () => api("/api/sales").then(setList);
  useEffect(() => { load().catch((e) => setError(e.message)); }, []);

  const today = new Date().toISOString().slice(0, 10);
  const todayRows = useMemo(() => list.filter((x) => x.business_date === today), [list, today]);
  const todayRevenue = todayRows.reduce((sum, x) => sum + Number(x.revenue || 0), 0);
  const todayOrders = todayRows.reduce((sum, x) => sum + Number(x.orders || 0), 0);

  async function submit(event) {
    event.preventDefault(); setError("");
    try { await api("/api/sales", { method: "POST", body: JSON.stringify({ ...form, orders: Number(form.orders), revenue: Number(form.revenue) }) }); await load(); }
    catch (e) { setError(e.message); }
  }

  return (
    <main className="page-wrap">
      <div className="grid gap-6 lg:grid-cols-[1fr_auto] lg:items-end">
        <div><p className="eyebrow">Operasional Pendapatan</p><h1 className="page-title mt-3">Analitik Penjualan</h1><div className="gold-rule mt-5" /><p className="page-subtitle mt-5">Catat pendapatan harian restoran berdasarkan kanal dan pantau metrik operasional terbaru dari pusat kontrol.</p></div>
        <div className="grid grid-cols-2 gap-3"><div className="card p-4 text-center"><p className="text-[9px] font-black uppercase tracking-[.13em] text-slate-400">Pendapatan Hari Ini</p><p className="mt-2 text-xl font-black text-emerald-950">Rp {todayRevenue.toLocaleString("id-ID")}</p></div><div className="card p-4 text-center"><p className="text-[9px] font-black uppercase tracking-[.13em] text-slate-400">Pesanan</p><p className="mt-2 text-2xl font-black text-emerald-950">{todayOrders}</p></div></div>
      </div>
      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-7 grid gap-5 xl:grid-cols-[390px_1fr]">
        <form onSubmit={submit} className="card p-6 md:p-7">
          <p className="section-label">Input Penjualan Harian</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Catat pendapatan per kanal</h2>
          <div className="mt-5 space-y-3"><label className="block text-xs font-bold text-emerald-950">Tanggal Transaksi<input className="input mt-2" type="date" value={form.business_date} onChange={(e) => setForm({ ...form, business_date: e.target.value })} /></label><label className="block text-xs font-bold text-emerald-950">Pesanan<input className="input mt-2" type="number" min="0" value={form.orders} onChange={(e) => setForm({ ...form, orders: e.target.value })} /></label><label className="block text-xs font-bold text-emerald-950">Pendapatan<input className="input mt-2" type="number" min="0" value={form.revenue} onChange={(e) => setForm({ ...form, revenue: e.target.value })} /></label><label className="block text-xs font-bold text-emerald-950">Kanal<select className="input mt-2" value={form.channel} onChange={(e) => setForm({ ...form, channel: e.target.value })}>{channels.map((x) => <option key={x} value={x}>{channelLabel(x)}</option>)}</select></label><button className="btn w-full">Simpan Penjualan</button></div>
        </form>

        <div className="card overflow-hidden">
          <div className="flex items-center justify-between border-b border-emerald-950/8 p-5"><div><p className="section-label">Buku Pendapatan</p><h2 className="mt-1 text-xl font-black text-emerald-950">Entri Terbaru</h2></div></div>
          <div className="divide-y divide-emerald-950/7">{list.length ? list.map((x) => <div key={x.id} className="grid gap-3 p-5 sm:grid-cols-[130px_1fr_1fr_auto] sm:items-center"><strong className="text-emerald-950">{x.business_date}</strong><span className="text-sm text-slate-500">{x.orders} pesanan</span><span className="text-lg font-black text-emerald-700">Rp {Number(x.revenue).toLocaleString("id-ID")}</span><span className="status-pill status-verified">{channelLabel(x.channel)}</span></div>) : <div className="empty-state m-5">Belum ada entri penjualan.</div>}</div>
        </div>
      </section>
    </main>
  );
}
