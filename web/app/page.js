"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api } from "../lib/api";

function Metric({ label, value, detail, accent }) {
  return (
    <div className="card metric card-hover p-6 md:p-7">
      <div className="relative z-10">
        <div className="flex items-center justify-between gap-4">
          <p className="text-xs font-black uppercase tracking-[.14em] text-emerald-100/80">{label}</p>
          <span className="h-2.5 w-2.5 rounded-full" style={{ background: accent, boxShadow: `0 0 0 5px ${accent}20` }} />
        </div>
        <p className="kpi-number mt-4 text-4xl font-black md:text-5xl">{value}</p>
        <p className="mt-3 text-xs font-semibold text-emerald-100/70">{detail}</p>
      </div>
    </div>
  );
}

export default function Dashboard() {
  const [data, setData] = useState(null);
  const [sales, setSales] = useState([]);
  const [audits, setAudits] = useState([]);
  const [issues, setIssues] = useState([]);
  const [inventory, setInventory] = useState([]);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      api("/api/dashboard"),
      api("/api/sales"),
      api("/api/audits"),
      api("/api/issues"),
      api("/api/inventory"),
    ])
      .then(([dashboard, salesRows, auditRows, issueRows, inventoryRows]) => {
        setData(dashboard);
        setSales(salesRows);
        setAudits(auditRows);
        setIssues(issueRows);
        setInventory(inventoryRows);
      })
      .catch((e) => setError(e.message));
  }, []);

  const recentSales = useMemo(() => sales.slice(0, 7).reverse(), [sales]);
  const maxRevenue = Math.max(...recentSales.map((x) => Number(x.revenue || 0)), 1);
  const recentAudits = audits.slice(0, 5);
  const lowStock = inventory.filter((x) => Number(x.stock) <= Number(x.reorder_level)).slice(0, 5);
  const critical = issues.filter((x) => x.severity === "critical" && x.status !== "closed").length;
  const high = issues.filter((x) => x.severity === "high" && x.status !== "closed").length;

  return (
    <main className="page-wrap">
      <section className="hero-luxury p-7 md:p-10 lg:p-12">
        <div className="relative z-10 grid gap-10 lg:grid-cols-[1.25fr_.75fr] lg:items-end">
          <div>
            <span className="hero-badge">Live restaurant intelligence</span>
            <h1 className="mt-6 max-w-3xl text-5xl font-black leading-[.95] tracking-[-.06em] md:text-7xl">Restaurant Control Center</h1>
            <p className="mt-6 max-w-xl text-sm leading-7 text-emerald-50/72">A single operational view for revenue, audit discipline, findings and stock risk—designed for fast managerial decisions.</p>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="rounded-3xl border border-white/10 bg-white/7 p-5 backdrop-blur">
              <p className="text-[10px] font-black uppercase tracking-[.15em] text-amber-200">Critical findings</p>
              <p className="mt-2 text-3xl font-black">{critical}</p>
            </div>
            <div className="rounded-3xl border border-white/10 bg-white/7 p-5 backdrop-blur">
              <p className="text-[10px] font-black uppercase tracking-[.15em] text-amber-200">High findings</p>
              <p className="mt-2 text-3xl font-black">{high}</p>
            </div>
          </div>
        </div>
      </section>

      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-5 grid gap-4 md:grid-cols-3">
        <Metric label="Sales Today" value={`Rp ${(data?.sales_today || 0).toLocaleString("id-ID")}`} detail={`${data?.orders_today || 0} orders recorded`} accent="#f3d886" />
        <Metric label="Audit Score" value={`${Number(data?.audit_score || 0).toFixed(1)}%`} detail={`${data?.open_issues || 0} active findings`} accent="#c6dc73" />
        <Metric label="Inventory Alerts" value={data?.inventory_alerts || 0} detail={`${data?.total_items || 0} tracked stock items`} accent="#79a982" />
      </section>

      <section className="mt-5 grid gap-5 xl:grid-cols-[1.1fr_.9fr]">
        <div className="card p-6 md:p-7">
          <div className="flex items-end justify-between gap-4">
            <div>
              <p className="eyebrow">Revenue pulse</p>
              <h2 className="mt-2 text-2xl font-black tracking-tight text-emerald-950">Recent sales performance</h2>
            </div>
            <Link href="/sales" className="text-xs font-black text-amber-700">Open Sales →</Link>
          </div>
          <div className="mt-7 mini-chart">
            {recentSales.length ? recentSales.map((x) => (
              <div key={x.id} className="group flex h-full flex-1 flex-col justify-end gap-2" title={`${x.business_date}: Rp ${Number(x.revenue).toLocaleString("id-ID")}`}>
                <div className="mini-bar" style={{ height: `${Math.max((Number(x.revenue) / maxRevenue) * 100, 8)}%` }} />
                <span className="truncate text-center text-[9px] font-bold text-slate-400">{String(x.business_date).slice(5)}</span>
              </div>
            )) : <div className="empty-state w-full">Add sales entries to reveal the revenue trend.</div>}
          </div>
        </div>

        <div className="card p-6 md:p-7">
          <div className="flex items-end justify-between gap-4">
            <div>
              <p className="eyebrow">Compliance pulse</p>
              <h2 className="mt-2 text-2xl font-black tracking-tight text-emerald-950">Latest audit quality</h2>
            </div>
            <Link href="/audits" className="text-xs font-black text-amber-700">Open Audit →</Link>
          </div>
          <div className="mt-6 space-y-4">
            {recentAudits.length ? recentAudits.map((x) => (
              <div key={x.id}>
                <div className="mb-2 flex items-center justify-between gap-3 text-xs">
                  <span className="truncate font-bold text-emerald-950">{x.restaurant}</span>
                  <span className="font-black text-emerald-700">{Number(x.score).toFixed(1)}%</span>
                </div>
                <div className="progress-track"><div className="progress-value" style={{ width: `${Math.max(0, Math.min(100, Number(x.score)))}%` }} /></div>
              </div>
            )) : <div className="empty-state">Submit an audit to reveal compliance quality.</div>}
          </div>
        </div>
      </section>

      <section className="mt-5 grid gap-5 lg:grid-cols-2">
        <div className="card p-6 md:p-7">
          <div className="flex items-center justify-between gap-4">
            <div><p className="eyebrow">Corrective actions</p><h2 className="mt-2 text-xl font-black text-emerald-950">Open finding risk</h2></div>
            <span className="status-pill status-open">{issues.filter((x) => x.status !== "closed").length} active</span>
          </div>
          <div className="mt-5 grid grid-cols-2 gap-3">
            {[["Critical", critical, "severity-critical"], ["High", high, "severity-high"]].map(([label, count, cls]) => (
              <div key={label} className="rounded-2xl border border-emerald-950/8 bg-white/55 p-4">
                <span className={`status-pill ${cls}`}>{label}</span>
                <p className="mt-3 text-3xl font-black text-emerald-950">{count}</p>
              </div>
            ))}
          </div>
        </div>

        <div className="card p-6 md:p-7">
          <div className="flex items-center justify-between gap-4">
            <div><p className="eyebrow">Inventory health</p><h2 className="mt-2 text-xl font-black text-emerald-950">Items requiring attention</h2></div>
            <Link href="/inventory" className="text-xs font-black text-amber-700">Manage →</Link>
          </div>
          <div className="mt-5 space-y-3">
            {lowStock.length ? lowStock.map((x) => (
              <div key={x.id} className="flex items-center justify-between rounded-2xl border border-red-100 bg-red-50/55 px-4 py-3">
                <div><p className="font-black text-emerald-950">{x.name}</p><p className="text-xs text-slate-500">Reorder at {x.reorder_level} {x.unit}</p></div>
                <span className="font-black text-red-700">{x.stock} {x.unit}</span>
              </div>
            )) : <div className="empty-state">Inventory is currently above configured reorder levels.</div>}
          </div>
        </div>
      </section>
    </main>
  );
}
