"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "../../lib/api";

const statuses = ["open", "in_progress", "resolved", "verified", "closed"];

export default function Audits() {
  const [audits, setAudits] = useState([]);
  const [issues, setIssues] = useState([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ restaurant: "Tropical Main", auditor: "Internal Auditor", cleanliness: 90, sop: 90, food_quality: 90, notes: "" });
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
        <div><p className="eyebrow">Quality & compliance</p><h1 className="page-title mt-3">Internal Audit</h1><div className="gold-rule mt-5" /><p className="page-subtitle mt-5">Turn checklist results into owned corrective actions with severity, PIC, due date and verification workflow.</p></div>
        <div className="grid grid-cols-3 gap-2">
          {[["Avg score", `${average.toFixed(1)}%`], ["Active", openIssues.length], ["Overdue", overdue.length]].map(([label, value]) => <div key={label} className="card min-w-[94px] p-4 text-center"><p className="text-[9px] font-black uppercase tracking-[.13em] text-slate-400">{label}</p><p className="mt-2 text-xl font-black text-emerald-950">{value}</p></div>)}
        </div>
      </div>

      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-7 grid gap-5 xl:grid-cols-2">
        <form onSubmit={submitAudit} className="card p-6 md:p-7">
          <div className="mb-5"><p className="section-label">01 · Audit checklist</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Record compliance score</h2></div>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="text-xs font-bold text-emerald-950">Restaurant<input className="input mt-2" value={form.restaurant} onChange={(e) => setForm({ ...form, restaurant: e.target.value })} /></label>
            <label className="text-xs font-bold text-emerald-950">Auditor<input className="input mt-2" value={form.auditor} onChange={(e) => setForm({ ...form, auditor: e.target.value })} /></label>
          </div>
          <div className="mt-4 grid grid-cols-3 gap-3">
            {[["cleanliness", "Cleanliness"], ["sop", "SOP"], ["food_quality", "Food Quality"]].map(([key, label]) => <label key={key} className="text-xs font-bold text-emerald-950">{label}<input className="input mt-2" type="number" min="0" max="100" value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })} /></label>)}
          </div>
          <label className="mt-4 block text-xs font-bold text-emerald-950">Notes<textarea className="input mt-2" value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} placeholder="Observations, evidence notes, context..." /></label>
          <button className="btn mt-5 w-full" disabled={saving}>Submit Audit</button>
        </form>

        <form onSubmit={submitIssue} className="card p-6 md:p-7">
          <div className="mb-5"><p className="section-label">02 · Corrective action</p><h2 className="mt-2 text-2xl font-black text-emerald-950">Create finding</h2></div>
          <select className="input" value={issue.audit_id} onChange={(e) => setIssue({ ...issue, audit_id: e.target.value })}><option value="0">General issue / no audit link</option>{audits.map((a) => <option key={a.id} value={a.id}>Audit #{a.id} — {a.restaurant}</option>)}</select>
          <input className="input mt-3" placeholder="Finding title" value={issue.title} onChange={(e) => setIssue({ ...issue, title: e.target.value })} />
          <div className="mt-3 grid grid-cols-2 gap-3">
            <select className="input" value={issue.severity} onChange={(e) => setIssue({ ...issue, severity: e.target.value })}><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="critical">Critical</option></select>
            <input className="input" type="date" value={issue.due_date} onChange={(e) => setIssue({ ...issue, due_date: e.target.value })} />
          </div>
          <input className="input mt-3" placeholder="Assigned PIC" value={issue.assigned_to} onChange={(e) => setIssue({ ...issue, assigned_to: e.target.value })} />
          <textarea className="input mt-3" placeholder="Corrective action plan" value={issue.corrective_action} onChange={(e) => setIssue({ ...issue, corrective_action: e.target.value })} />
          <button className="btn mt-4 w-full" disabled={saving}>Create Finding</button>
        </form>
      </section>

      <section className="mt-6 grid gap-5 xl:grid-cols-[.8fr_1.2fr]">
        <div className="card overflow-hidden">
          <div className="border-b border-emerald-950/8 p-5"><p className="section-label">Recent audits</p></div>
          <div className="divide-y divide-emerald-950/7">
            {audits.length ? audits.slice(0, 12).map((a) => <div key={a.id} className="p-5"><div className="flex items-start justify-between gap-4"><div><strong className="text-emerald-950">{a.restaurant}</strong><p className="mt-1 text-xs text-slate-500">{a.auditor} · {new Date(a.created_at).toLocaleDateString("id-ID")}</p></div><span className="text-xl font-black text-emerald-700">{Number(a.score).toFixed(1)}%</span></div><div className="progress-track mt-3"><div className="progress-value" style={{ width: `${a.score}%` }} /></div></div>) : <div className="empty-state m-5">No audit records yet.</div>}
          </div>
        </div>

        <div className="card overflow-hidden">
          <div className="flex items-center justify-between border-b border-emerald-950/8 p-5"><p className="section-label">Issue tracker</p><span className="text-xs font-bold text-slate-400">{openIssues.length} active</span></div>
          <div className="divide-y divide-emerald-950/7">
            {issues.length ? issues.map((x) => <FindingRow key={x.id} issue={x} onUpdate={updateFinding} />) : <div className="empty-state m-5">No findings yet.</div>}
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
    <div className="p-5">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2"><span className={`status-pill severity-${issue.severity}`}>{issue.severity}</span><span className={`status-pill status-${issue.status}`}>{issue.status.replace("_", " ")}</span>{issue.audit_id > 0 && <span className="text-[10px] font-black text-slate-400">AUDIT #{issue.audit_id}</span>}</div>
          <h3 className="mt-3 text-lg font-black text-emerald-950">{issue.title}</h3>
        </div>
        <select className="input lg:w-40" value={draft.status} onChange={(e) => setDraft({ ...draft, status: e.target.value })}>{statuses.map((s) => <option key={s} value={s}>{s.replace("_", " ")}</option>)}</select>
      </div>
      <div className="mt-4 grid gap-3 md:grid-cols-[1fr_170px]">
        <input className="input" placeholder="Assigned PIC" value={draft.assigned_to || ""} onChange={(e) => setDraft({ ...draft, assigned_to: e.target.value })} />
        <input className="input" type="date" value={draft.due_date || ""} onChange={(e) => setDraft({ ...draft, due_date: e.target.value })} />
      </div>
      <textarea className="input mt-3" placeholder="Corrective action / verification note" value={draft.corrective_action || ""} onChange={(e) => setDraft({ ...draft, corrective_action: e.target.value })} />
      <div className="mt-3 flex items-center justify-between gap-3"><p className="text-[11px] text-slate-400">Due {draft.due_date || "not set"}</p><button className="btn btn-secondary px-4 py-2 text-xs" type="button" onClick={() => onUpdate(issue, draft)}>Save workflow</button></div>
    </div>
  );
}
