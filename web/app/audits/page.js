"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "../../lib/api";
import { severityLabel, statusLabel } from "../../lib/labels";

const statuses = ["open", "in_progress", "resolved", "verified", "closed"];

export default function Audits() {
  const [audits, setAudits] = useState([]);
  const [issues, setIssues] = useState([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ restaurant: "Tropical Main", auditor: "Auditor Internal", cleanliness: 90, sop: 90, food_quality: 90, notes: "" });
  const [issue, setIssue] = useState({ audit_id: 0, title: "", severity: "medium", status: "open", assigned_to: "", due_date: "", corrective_action: "" });

  const load = () => Promise.all([api("/api/audits").then(setAudits), api("/api/issues").then(setIssues)]);
  useEffect(() => { load().catch((e) => setError(e.message)); }, []);

  const openIssues = useMemo(() => issues.filter((x) => x.status !== "closed"), [issues]);
  const overdue = useMemo(() => openIssues.filter((x) => x.due_date && new Date(`${x.due_date}T23:59:59`) < new Date() && !["verified", "closed"].includes(x.status)), [openIssues]);
  const average = audits.length ? audits.reduce((sum, x) => sum + Number(x.score || 0), 0) / audits.length : 0;

  async function submitAudit(event) {
    event.preventDefault(); setError(""); setSaving(true);
    try {
      await api("/api/audits", { method: "POST", body: JSON.stringify({ ...form, cleanliness: Number(form.cleanliness), sop: Number(form.sop), food_quality: Number(form.food_quality) }) });
      setForm({ ...form, notes: "" });
      await load();
    } catch (e) { setError(e.message); } finally { setSaving(false); }
  }

  async function submitIssue(event) {
    event.preventDefault(); setError(""); setSaving(true);
    try {
      await api("/api/issues", { method: "POST", body: JSON.stringify({ ...issue, audit_id: Number(issue.audit_id) }) });
      setIssue({ audit_id: 0, title: "", severity: "medium", status: "open", assigned_to: "", due_date: "", corrective_action: "" });
      await load();
    } catch (e) { setError(e.message); } finally { setSaving(false); }
  }

  async function updateFinding(current, patch) {
    const next = { ...current, ...patch };
    setIssues((rows) => rows.map((x) => x.id === current.id ? next : x));
    try {
      await api("/api/issues", { method: "PATCH", body: JSON.stringify(next) });
      await load();
    } catch (e) {
      setError(e.message);
      await load().catch(() => {});
    }
  }

  return (
    <main className="page-wrap">
      <div className="grid gap-7 lg:grid-cols-[1fr_auto] lg:items-end">
        <div><p className="eyebrow">Kualitas & Kepatuhan</p><h1 className="page-title mt-3">Audit Internal</h1><div className="gold-rule mt-5" /><p className="page-subtitle mt-5">Ubah hasil checklist menjadi tindakan korektif yang jelas dengan tingkat risiko, PIC, tenggat, dan alur verifikasi.</p></div>
        <div className="grid grid-cols-3 gap-2">
          {[["Rata-rata", `${average.toFixed(1)}%`], ["Aktif", openIssues.length], ["Terlambat", overdue.length]].map(([label, value]) => <div key={label} className="card min-w-[94px] p-4 text-center"><p className="text-[9px] font-black uppercase tracking-[.13em] text-slate-400">{label}</p><p className="mt-2 text-xl font-black text-emerald-950">{value}</p></div>)}
        </div>
      </div>

      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-7 grid gap-5 xl:grid-cols-2">
        <form onSubmit={submitAudit} className="card p-6 md:p-7">
          <div className="mb-5"><p className="section-label">01 - Checklist Audit</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Catat skor kepatuhan</h2></div>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="text-xs font-bold text-emerald-950">Restoran<input className="input mt-2" value={form.restaurant} onChange={(e) => setForm({ ...form, restaurant: e.target.value })} /></label>
            <label className="text-xs font-bold text-emerald-950">Auditor<input className="input mt-2" value={form.auditor} onChange={(e) => setForm({ ...form, auditor: e.target.value })} /></label>
          </div>
          <div className="mt-4 grid grid-cols-3 gap-3">
            {[["cleanliness", "Kebersihan"], ["sop", "SOP"], ["food_quality", "Kualitas Makanan"]].map(([key, label]) => <label key={key} className="text-xs font-bold text-emerald-950">{label}<input className="input mt-2" type="number" min="0" max="100" value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })} /></label>)}
          </div>
          <label className="mt-4 block text-xs font-bold text-emerald-950">Catatan<textarea className="input mt-2" value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} placeholder="Observasi, bukti, dan konteks audit..." /></label>
          <button className="btn mt-5 w-full" disabled={saving}>Simpan Audit</button>
        </form>

        <form onSubmit={submitIssue} className="card p-6 md:p-7">
          <div className="mb-5"><p className="section-label">02 - Tindakan Korektif</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Buat temuan</h2></div>
          <select className="input" value={issue.audit_id} onChange={(e) => setIssue({ ...issue, audit_id: e.target.value })}><option value="0">Temuan umum / tanpa tautan audit</option>{audits.map((a) => <option key={a.id} value={a.id}>Audit #{a.id} — {a.restaurant}</option>)}</select>
          <input className="input mt-3" placeholder="Judul temuan" value={issue.title} onChange={(e) => setIssue({ ...issue, title: e.target.value })} />
          <div className="mt-3 grid grid-cols-2 gap-3">
            <select className="input" value={issue.severity} onChange={(e) => setIssue({ ...issue, severity: e.target.value })}><option value="low">Rendah</option><option value="medium">Sedang</option><option value="high">Tinggi</option><option value="critical">Kritis</option></select>
            <input className="input" type="date" value={issue.due_date} onChange={(e) => setIssue({ ...issue, due_date: e.target.value })} />
          </div>
          <input className="input mt-3" placeholder="PIC yang ditugaskan" value={issue.assigned_to} onChange={(e) => setIssue({ ...issue, assigned_to: e.target.value })} />
          <textarea className="input mt-3" placeholder="Rencana tindakan korektif" value={issue.corrective_action} onChange={(e) => setIssue({ ...issue, corrective_action: e.target.value })} />
          <button className="btn mt-4 w-full" disabled={saving}>Buat Temuan</button>
        </form>
      </section>

      <section className="mt-6 grid gap-5 xl:grid-cols-[.8fr_1.2fr]">
        <div className="card overflow-hidden">
          <div className="border-b border-emerald-950/8 p-5"><p className="section-label">Audit Terbaru</p></div>
          <div className="divide-y divide-emerald-950/7">
            {audits.length ? audits.slice(0, 12).map((a) => <div key={a.id} className="p-5"><div className="flex items-start justify-between gap-4"><div><strong className="text-emerald-950">{a.restaurant}</strong><p className="mt-1 text-xs text-slate-500">{a.auditor} · {new Date(a.created_at).toLocaleDateString("id-ID")}</p></div><span className="text-xl font-black text-emerald-700">{Number(a.score).toFixed(1)}%</span></div><div className="progress-track mt-3"><div className="progress-value" style={{ width: `${a.score}%` }} /></div></div>) : <div className="empty-state m-5">Belum ada catatan audit.</div>}
          </div>
        </div>

        <div className="card overflow-hidden">
          <div className="flex items-center justify-between border-b border-emerald-950/8 p-5"><p className="section-label">Pelacak Temuan</p><span className="text-xs font-bold text-slate-400">{openIssues.length} aktif</span></div>
          <div className="divide-y divide-emerald-950/7">
            {issues.length ? issues.map((x) => <FindingRow key={x.id} issue={x} onUpdate={updateFinding} />) : <div className="empty-state m-5">Belum ada temuan.</div>}
          </div>
        </div>
      </section>
    </main>
  );
}

function FindingRow({ issue, onUpdate }) {
  const [draft, setDraft] = useState(issue);
  useEffect(() => setDraft(issue), [issue]);

  return (
    <article className="p-5 md:p-6">
      {/* Header is deliberately isolated from editable controls so badges can never overlap inputs/selects. */}
      <header className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className={`status-pill severity-${issue.severity}`}>{severityLabel(issue.severity)}</span>
          <span className={`status-pill status-${issue.status}`}>{statusLabel(issue.status)}</span>
          {issue.audit_id > 0 && <span className="text-[10px] font-black tracking-wide text-slate-400">AUDIT #{issue.audit_id}</span>}
        </div>
        <h3 className="mt-3 break-words text-lg font-black leading-snug text-emerald-950">{issue.title || "Temuan tanpa judul"}</h3>
      </header>

      <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-[180px_minmax(0,1fr)_180px]">
        <label className="block min-w-0 text-[10px] font-black uppercase tracking-[.12em] text-slate-400">
          Status Alur
          <select className="input mt-2 block w-full" value={draft.status} onChange={(e) => setDraft({ ...draft, status: e.target.value })}>
            {statuses.map((s) => <option key={s} value={s}>{statusLabel(s)}</option>)}
          </select>
        </label>

        <label className="block min-w-0 text-[10px] font-black uppercase tracking-[.12em] text-slate-400">
          PIC yang ditugaskan
          <input className="input mt-2 block w-full" placeholder="Penanggung jawab" value={draft.assigned_to || ""} onChange={(e) => setDraft({ ...draft, assigned_to: e.target.value })} />
        </label>

        <label className="block min-w-0 text-[10px] font-black uppercase tracking-[.12em] text-slate-400 md:col-span-2 xl:col-span-1">
          Tenggat
          <input className="input mt-2 block w-full" type="date" value={draft.due_date || ""} onChange={(e) => setDraft({ ...draft, due_date: e.target.value })} />
        </label>
      </div>

      <label className="mt-4 block text-[10px] font-black uppercase tracking-[.12em] text-slate-400">
        Tindakan korektif / catatan verifikasi
        <textarea className="input mt-2 block w-full" placeholder="Jelaskan tindakan korektif atau bukti verifikasi" value={draft.corrective_action || ""} onChange={(e) => setDraft({ ...draft, corrective_action: e.target.value })} />
      </label>

      <footer className="mt-4 flex flex-col gap-3 border-t border-emerald-950/7 pt-4 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-[11px] text-slate-400">Tenggat: {draft.due_date || "belum ditetapkan"}</p>
        <button className="btn btn-secondary px-4 py-2 text-xs" type="button" onClick={() => onUpdate(issue, draft)}>Simpan Alur</button>
      </footer>
    </article>
  );
}
