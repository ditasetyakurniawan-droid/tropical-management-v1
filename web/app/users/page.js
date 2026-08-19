"use client";

import { useEffect, useState } from "react";
import { api } from "../../lib/api";

export default function Users() {
  const [users, setUsers] = useState([]);
  const [form, setForm] = useState({ name: "", email: "", password: "", role: "staff" });
  const [error, setError] = useState("");
  const load = () => api("/api/users").then(setUsers);
  useEffect(() => { load().catch((e) => setError(e.message)); }, []);

  async function submit(event) {
    event.preventDefault(); setError("");
    try { await api("/api/users", { method: "POST", body: JSON.stringify(form) }); setForm({ name: "", email: "", password: "", role: "staff" }); await load(); }
    catch (e) { setError(e.message); }
  }

  async function patchUser(user, patch) {
    const payload = { id: user.id, name: user.name, role: user.role, active: user.active, ...patch };
    setUsers((rows) => rows.map((x) => x.id === user.id ? { ...x, ...patch } : x));
    try { await api("/api/users", { method: "PATCH", body: JSON.stringify(payload) }); await load(); }
    catch (e) { setError(e.message); await load().catch(() => {}); }
  }

  return (
    <main className="page-wrap">
      <div><p className="eyebrow">Identity & access</p><h1 className="page-title mt-3">Role Management</h1><div className="gold-rule mt-5" /><p className="page-subtitle mt-5">Control operational access with explicit roles and account activation status.</p></div>
      {error && <div className="toast-error mt-5">{error}</div>}

      <section className="mt-7 grid gap-5 xl:grid-cols-[390px_1fr]">
        <form onSubmit={submit} className="card p-6 md:p-7">
          <p className="section-label">Create account</p><h2 className="mt-2 text-2xl font-black text-emerald-950">New team member</h2>
          <div className="mt-5 space-y-3"><input className="input" placeholder="Full name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /><input className="input" placeholder="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /><input className="input" placeholder="Password · min 8 chars" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} /><select className="input" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}><option value="staff">Staff</option><option value="auditor">Auditor</option><option value="admin">Admin</option></select><button className="btn w-full">Create User</button></div>
          <div className="mt-6 rounded-2xl border border-amber-200/60 bg-amber-50/60 p-4 text-xs leading-6 text-amber-900"><strong>Access model:</strong> Admin manages users, Auditor owns compliance workflows, Staff records operational sales. Read access remains available to authenticated roles.</div>
        </form>

        <div className="card overflow-hidden">
          <div className="flex items-center justify-between border-b border-emerald-950/8 p-5"><div><p className="section-label">User directory</p><h2 className="mt-1 text-xl font-black text-emerald-950">{users.length} accounts</h2></div></div>
          <div className="divide-y divide-emerald-950/7">{users.length ? users.map((user) => <div key={user.id} className="grid gap-4 p-5 md:grid-cols-[1fr_170px_120px] md:items-center"><div><div className="flex flex-wrap items-center gap-2"><strong className="text-emerald-950">{user.name}</strong><span className={`status-pill ${user.active ? "status-verified" : "status-closed"}`}>{user.active ? "active" : "inactive"}</span></div><p className="mt-1 text-sm text-slate-500">{user.email}</p></div><select className="input" value={user.role} onChange={(e) => patchUser(user, { role: e.target.value })}><option value="staff">Staff</option><option value="auditor">Auditor</option><option value="admin">Admin</option></select><button className={`btn ${user.active ? "btn-secondary" : ""}`} type="button" onClick={() => patchUser(user, { active: !user.active })}>{user.active ? "Deactivate" : "Activate"}</button></div>) : <div className="empty-state m-5">No users available.</div>}</div>
        </div>
      </section>
    </main>
  );
}
