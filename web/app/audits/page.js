"use client";

import { useEffect, useState } from "react";
import { api } from "../../lib/api";

export default function Audits() {
  const [audits, setAudits] = useState([]);
  const [issues, setIssues] = useState([]);
  const [form, setForm] = useState({
    restaurant: "Tropical Main",
    auditor: "Internal Auditor",
    cleanliness: 90,
    sop: 90,
    food_quality: 90,
    notes: "",
  });
  const [issue, setIssue] = useState({
    audit_id: 0,
    title: "",
    severity: "medium",
    status: "open",
    assigned_to: "",
  });

  // Reload both bounded contexts after a mutation so the screen always reflects backend state.
  const load = () => Promise.all([
    api("/api/audits").then(setAudits),
    api("/api/issues").then(setIssues),
  ]);

  useEffect(() => { load().catch(() => {}); }, []);

  async function submitAudit(event) {
    event.preventDefault();
    await api("/api/audits", {
      method: "POST",
      body: JSON.stringify({
        ...form,
        cleanliness: Number(form.cleanliness),
        sop: Number(form.sop),
        food_quality: Number(form.food_quality),
      }),
    });
    await load();
  }

  async function submitIssue(event) {
    event.preventDefault();
    await api("/api/issues", {
      method: "POST",
      body: JSON.stringify({ ...issue, audit_id: Number(issue.audit_id) }),
    });
    setIssue({ ...issue, audit_id: 0, title: "", assigned_to: "" });
    await load();
  }

  return (
    <main className="mx-auto max-w-7xl p-5 md:p-8">
      <h1 className="text-4xl font-black text-emerald-950">Internal Audit</h1>
      <p className="mt-2 text-slate-500">Checklist compliance and corrective action tracking in one workspace.</p>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <form onSubmit={submitAudit} className="card space-y-4 p-6">
          <h2 className="text-xl font-black">New checklist</h2>
          {["restaurant", "auditor", "cleanliness", "sop", "food_quality", "notes"].map((key) => (
            <label key={key} className="block text-sm font-bold capitalize">
              {key.replace("_", " ")}
              <input
                className="input mt-1"
                type={["cleanliness", "sop", "food_quality"].includes(key) ? "number" : "text"}
                value={form[key]}
                onChange={(e) => setForm({ ...form, [key]: e.target.value })}
              />
            </label>
          ))}
          <button className="btn w-full">Submit audit</button>
        </form>

        <form onSubmit={submitIssue} className="card space-y-4 p-6">
          <h2 className="text-xl font-black">Create finding</h2>
          <select className="input" value={issue.audit_id} onChange={(e) => setIssue({ ...issue, audit_id: e.target.value })}>
            <option value="0">General issue / no audit link</option>
            {audits.map((a) => <option key={a.id} value={a.id}>Audit #{a.id} — {a.restaurant}</option>)}
          </select>
          <input className="input" placeholder="Issue title" value={issue.title} onChange={(e) => setIssue({ ...issue, title: e.target.value })} />
          <select className="input" value={issue.severity} onChange={(e) => setIssue({ ...issue, severity: e.target.value })}>
            <option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="critical">Critical</option>
          </select>
          <input className="input" placeholder="Assigned to" value={issue.assigned_to} onChange={(e) => setIssue({ ...issue, assigned_to: e.target.value })} />
          <button className="btn w-full">Create issue</button>
        </form>
      </div>

      <div className="mt-6 grid gap-6 xl:grid-cols-2">
        <section className="card overflow-hidden">
          <div className="border-b p-5 font-black">Recent audits</div>
          <div className="divide-y">
            {audits.map((a) => (
              <div key={a.id} className="grid gap-2 p-5 md:grid-cols-4">
                <strong>{a.restaurant}</strong><span>{a.auditor}</span>
                <span className="font-black text-emerald-700">{Number(a.score).toFixed(1)}%</span>
                <span className="text-slate-500">{new Date(a.created_at).toLocaleDateString()}</span>
              </div>
            ))}
          </div>
        </section>

        <section className="card overflow-hidden">
          <div className="border-b p-5 font-black">Issue tracker</div>
          <div className="divide-y">
            {issues.map((x) => (
              <div key={x.id} className="flex items-start justify-between gap-4 p-5">
                <div><strong>{x.title}</strong><p className="mt-1 text-sm text-slate-500">{x.assigned_to || "Unassigned"} · {x.status}</p></div>
                <span className="rounded-full bg-amber-100 px-3 py-1 text-xs font-black uppercase text-amber-800">{x.severity}</span>
              </div>
            ))}
          </div>
        </section>
      </div>
    </main>
  );
}
