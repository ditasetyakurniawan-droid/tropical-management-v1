"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { api, sessionUser } from "../lib/api";
import { roleLabel, stationLabel } from "../lib/labels";
import { formatDate, isoDate } from "../lib/workforce";

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
  const user = sessionUser();
  if (user?.role === "staff") return <EmployeeHome user={user} />;
  return <ManagerHome user={user} />;
}

function EmployeeHome({ user }) {
  const [summary, setSummary] = useState(null);
  const [shifts, setShifts] = useState([]);
  const [tasks, setTasks] = useState([]);
  const [error, setError] = useState("");
  const today = isoDate();

  useEffect(() => {
    Promise.all([
      api("/api/workforce/summary"),
      api(`/api/workforce/shifts?from=${today}&to=${today}`),
      api(`/api/workforce/tasks?from=${today}&to=${today}`),
    ]).then(([nextSummary, nextShifts, nextTasks]) => {
      setSummary(nextSummary);
      setShifts(nextShifts || []);
      setTasks(nextTasks || []);
    }).catch((e) => setError(e.message));
  }, [today]);

  const shift = shifts[0];
  const openTasks = tasks.filter((row) => row.status !== "done");

  return (
    <main className="page-wrap">
      <section className="ops-hero">
        <div className="relative z-10 max-w-3xl">
          <span className="hero-badge">Tropical Steak House · Employee Home</span>
          <h1 className="mt-5 text-4xl font-black tracking-[-.05em] text-white md:text-6xl">Selamat bekerja, {user?.name || "Tim Tropical"}.</h1>
          <p className="mt-5 max-w-2xl text-sm leading-7 text-emerald-50/75">Beranda karyawan hanya menampilkan hal yang Anda perlukan untuk shift hari ini. Jadwal, kehadiran, checklist, dan komunikasi tim.</p>
          <div className="mt-6 flex flex-wrap gap-2"><span className="hero-badge">{roleLabel(user?.role)}</span>{shift && <span className="hero-badge">{stationLabel(shift.station)} · {shift.start_time}–{shift.end_time}</span>}</div>
        </div>
      </section>

      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-5 grid gap-4 md:grid-cols-3">
        <Metric label="Shift Hari Ini" value={summary?.shifts_today || 0} detail={shift ? `${formatDate(shift.shift_date)} · ${stationLabel(shift.station)}` : "Belum ada shift terjadwal"} accent="#f3d886" />
        <Metric label="Status Kehadiran" value={summary?.on_duty ? "On Duty" : "Belum Masuk"} detail="Clock in/out dari ruang kerja shift" accent="#c6dc73" />
        <Metric label="Checklist Terbuka" value={summary?.open_tasks || 0} detail="tugas shift yang perlu diselesaikan" accent="#79a982" />
      </section>

      <section className="mt-5 grid gap-5 lg:grid-cols-[1.1fr_.9fr]">
        <div className="card p-6 md:p-7">
          <div className="flex items-center justify-between gap-4"><div><p className="eyebrow">Fokus Hari Ini</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Checklist shift</h2></div><Link href="/workforce" className="text-xs font-black text-amber-700">Buka Hari & Shift →</Link></div>
          <div className="mt-5 space-y-3">
            {openTasks.length ? openTasks.slice(0, 5).map((task) => <div key={task.id} className="ops-history"><div><p className="font-black text-emerald-950">{task.title}</p><p className="text-xs text-slate-500">{stationLabel(task.station)} · {task.assigned_to_name}</p></div><span className="ops-pill">{task.priority}</span></div>) : <div className="ops-empty">Tidak ada checklist terbuka hari ini.</div>}
          </div>
        </div>
        <div className="card p-6 md:p-7">
          <p className="eyebrow">Quick Access</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Kerja tanpa banyak klik</h2>
          <div className="mt-5 grid gap-3"><Link className="quick-action" href="/workforce"><strong>Hari & Shift</strong><span>Clock in/out, jadwal, izin, checklist</span></Link><Link className="quick-action" href="/sales"><strong>Penjualan</strong><span>Catat transaksi operasional sesuai akses</span></Link><Link className="quick-action" href="/chat"><strong>Chat Tim</strong><span>Koordinasi langsung antar shift</span></Link></div>
        </div>
      </section>
    </main>
  );
}

function ManagerHome({ user }) {
  const [data, setData] = useState(null);
  const [sales, setSales] = useState([]);
  const [audits, setAudits] = useState([]);
  const [issues, setIssues] = useState([]);
  const [inventory, setInventory] = useState([]);
  const [workforce, setWorkforce] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      api("/api/dashboard"),
      api("/api/sales"),
      api("/api/audits"),
      api("/api/issues"),
      api("/api/inventory"),
      api("/api/workforce/summary"),
    ])
      .then(([dashboard, salesRows, auditRows, issueRows, inventoryRows, workforceRows]) => {
        setData(dashboard); setSales(salesRows); setAudits(auditRows); setIssues(issueRows); setInventory(inventoryRows); setWorkforce(workforceRows);
      })
      .catch((e) => setError(e.message));
  }, []);

  const recentSales = useMemo(() => sales.slice(0, 7).reverse(), [sales]);
  const maxRevenue = Math.max(...recentSales.map((x) => Number(x.revenue || 0)), 1);
  const recentAudits = audits.slice(0, 5);
  const lowStock = inventory.filter((x) => Number(x.stock) <= Number(x.reorder_level)).slice(0, 5);
  const critical = issues.filter((x) => x.severity === "critical" && x.status !== "closed").length;
  const high = issues.filter((x) => x.severity === "high" && x.status !== "closed").length;
  const ownerMode = user?.role === "admin";

  return (
    <main className="page-wrap">
      <section className="hero-luxury p-7 md:p-10 lg:p-12">
        <div className="relative z-10 grid gap-10 lg:grid-cols-[1.25fr_.75fr] lg:items-end">
          <div><span className="hero-badge">{ownerMode ? "Owner Intelligence" : "PIC Shift Command"} · Tropical Steak House</span><h1 className="mt-6 max-w-3xl text-5xl font-black leading-[.95] tracking-[-.06em] md:text-7xl">{ownerMode ? "Pusat Kendali Bisnis" : "Kontrol Operasional Shift"}</h1><p className="mt-6 max-w-xl text-sm leading-7 text-emerald-50/72">{ownerMode ? "Pantau pendapatan, kualitas, stok, dan kesiapan tim dalam satu tampilan eksekutif." : "Fokus pada kesiapan tim, kualitas layanan, stok, dan tindak lanjut operasional yang perlu ditangani hari ini."}</p></div>
          <div className="grid grid-cols-2 gap-3"><div className="rounded-3xl border border-white/10 bg-white/7 p-5 backdrop-blur"><p className="text-[10px] font-black uppercase tracking-[.15em] text-amber-200">Tim on duty</p><p className="mt-2 text-3xl font-black">{workforce?.on_duty || 0}</p></div><div className="rounded-3xl border border-white/10 bg-white/7 p-5 backdrop-blur"><p className="text-[10px] font-black uppercase tracking-[.15em] text-amber-200">Approval izin</p><p className="mt-2 text-3xl font-black">{workforce?.pending_time_off || 0}</p></div></div>
        </div>
      </section>

      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Metric label="Penjualan Hari Ini" value={`Rp ${(data?.sales_today || 0).toLocaleString("id-ID")}`} detail={`${data?.orders_today || 0} pesanan tercatat`} accent="#f3d886" />
        <Metric label="Skor Audit" value={`${Number(data?.audit_score || 0).toFixed(1)}%`} detail={`${data?.open_issues || 0} temuan aktif`} accent="#c6dc73" />
        <Metric label="Peringatan Stok" value={data?.inventory_alerts || 0} detail={`${data?.total_items || 0} item dipantau`} accent="#79a982" />
        <Metric label="Checklist Shift" value={workforce?.open_tasks || 0} detail={`${workforce?.shifts_today || 0} shift terjadwal hari ini`} accent="#d7ad55" />
      </section>

      <section className="mt-5 grid gap-5 xl:grid-cols-[1.1fr_.9fr]">
        <div className="card p-6 md:p-7"><div className="flex items-end justify-between gap-4"><div><p className="eyebrow">Pergerakan Pendapatan</p><h2 className="mt-2 text-2xl font-black tracking-tight text-emerald-950">Kinerja penjualan terbaru</h2></div><Link href="/sales" className="text-xs font-black text-amber-700">Buka Penjualan →</Link></div><div className="mt-7 mini-chart">{recentSales.length ? recentSales.map((x) => <div key={x.id} className="group flex h-full flex-1 flex-col justify-end gap-2" title={`${x.business_date}: Rp ${Number(x.revenue).toLocaleString("id-ID")}`}><div className="mini-bar" style={{ height: `${Math.max((Number(x.revenue) / maxRevenue) * 100, 8)}%` }} /><span className="truncate text-center text-[9px] font-bold text-slate-400">{String(x.business_date).slice(5)}</span></div>) : <div className="empty-state w-full">Tambahkan data penjualan untuk menampilkan tren pendapatan.</div>}</div></div>
        <div className="card p-6 md:p-7"><div className="flex items-end justify-between gap-4"><div><p className="eyebrow">People Operations</p><h2 className="mt-2 text-2xl font-black tracking-tight text-emerald-950">Kesiapan tim hari ini</h2></div><Link href="/workforce" className="text-xs font-black text-amber-700">Buka Tim & Shift →</Link></div><div className="mt-6 grid grid-cols-2 gap-3"><div className="rounded-2xl border border-emerald-950/8 bg-white/55 p-4"><p className="eyebrow">Terjadwal</p><p className="mt-3 text-3xl font-black text-emerald-950">{workforce?.shifts_today || 0}</p></div><div className="rounded-2xl border border-emerald-950/8 bg-white/55 p-4"><p className="eyebrow">On Duty</p><p className="mt-3 text-3xl font-black text-emerald-950">{workforce?.on_duty || 0}</p></div></div><p className="mt-4 text-xs leading-5 text-slate-500">PIC dapat menerbitkan shift, approval izin, memantau attendance, dan membagi checklist per station.</p></div>
      </section>

      <section className="mt-5 grid gap-5 lg:grid-cols-3">
        <div className="card p-6 md:p-7"><div className="flex items-center justify-between gap-4"><div><p className="eyebrow">Tindakan Korektif</p><h2 className="mt-2 text-xl font-black text-emerald-950">Risiko terbuka</h2></div><span className="status-pill status-open">{issues.filter((x) => x.status !== "closed").length} aktif</span></div><div className="mt-5 grid grid-cols-2 gap-3">{[["Kritis", critical, "severity-critical"], ["Tinggi", high, "severity-high"]].map(([label, count, cls]) => <div key={label} className="rounded-2xl border border-emerald-950/8 bg-white/55 p-4"><span className={`status-pill ${cls}`}>{label}</span><p className="mt-3 text-3xl font-black text-emerald-950">{count}</p></div>)}</div></div>
        <div className="card p-6 md:p-7"><div className="flex items-center justify-between gap-4"><div><p className="eyebrow">Kesehatan Inventaris</p><h2 className="mt-2 text-xl font-black text-emerald-950">Perlu perhatian</h2></div><Link href="/inventory" className="text-xs font-black text-amber-700">Kelola →</Link></div><div className="mt-5 space-y-3">{lowStock.length ? lowStock.map((x) => <div key={x.id} className="flex items-center justify-between rounded-2xl border border-red-100 bg-red-50/55 px-4 py-3"><div><p className="font-black text-emerald-950">{x.name}</p><p className="text-xs text-slate-500">Pesan ulang {x.reorder_level} {x.unit}</p></div><span className="font-black text-red-700">{x.stock} {x.unit}</span></div>) : <div className="empty-state">Stok masih di atas batas pemesanan ulang.</div>}</div></div>
        <div className="card p-6 md:p-7"><div className="flex items-end justify-between gap-4"><div><p className="eyebrow">Kualitas Audit</p><h2 className="mt-2 text-xl font-black text-emerald-950">Audit terbaru</h2></div><Link href="/audits" className="text-xs font-black text-amber-700">Buka →</Link></div><div className="mt-5 space-y-4">{recentAudits.length ? recentAudits.slice(0, 4).map((x) => <div key={x.id}><div className="mb-2 flex items-center justify-between gap-3 text-xs"><span className="truncate font-bold text-emerald-950">{x.restaurant}</span><span className="font-black text-emerald-700">{Number(x.score).toFixed(1)}%</span></div><div className="progress-track"><div className="progress-value" style={{ width: `${Math.max(0, Math.min(100, Number(x.score)))}%` }} /></div></div>) : <div className="empty-state">Belum ada audit terbaru.</div>}</div></div>
      </section>
    </main>
  );
}
