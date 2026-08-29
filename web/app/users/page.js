"use client";

import { useEffect, useState } from "react";
import { api, clearSession } from "../../lib/api";

export default function Users() {
  const [users, setUsers] = useState([]);
  const [form, setForm] = useState({ name: "", email: "", password: "", role: "staff" });
  const [passwordForm, setPasswordForm] = useState({ current: "", next: "", confirm: "" });
  const [passwordBusy, setPasswordBusy] = useState(false);
  const [passwordMessage, setPasswordMessage] = useState("");
  const [error, setError] = useState("");

  const load = () => api("/api/users").then(setUsers);
  useEffect(() => { load().catch((e) => setError(e.message)); }, []);

  async function submit(event) {
    event.preventDefault();
    setError("");
    try {
      await api("/api/users", { method: "POST", body: JSON.stringify(form) });
      setForm({ name: "", email: "", password: "", role: "staff" });
      await load();
    } catch (e) {
      setError(e.message);
    }
  }

  async function patchUser(user, patch) {
    const payload = { id: user.id, name: user.name, role: user.role, active: user.active, ...patch };
    setUsers((rows) => rows.map((x) => x.id === user.id ? { ...x, ...patch } : x));
    try {
      await api("/api/users", { method: "PATCH", body: JSON.stringify(payload) });
      await load();
    } catch (e) {
      setError(e.message);
      await load().catch(() => {});
    }
  }

  async function changePassword(event) {
    event.preventDefault();
    setError("");
    setPasswordMessage("");

    if (passwordForm.next.length < 12) {
      setError("Kata sandi baru minimal 12 karakter.");
      return;
    }
    if (passwordForm.next !== passwordForm.confirm) {
      setError("Konfirmasi kata sandi baru tidak sama.");
      return;
    }

    setPasswordBusy(true);
    try {
      await api("/api/auth/change-password", {
        method: "POST",
        body: JSON.stringify({
          current_password: passwordForm.current,
          new_password: passwordForm.next,
        }),
      });
      setPasswordMessage("Kata sandi diperbarui. Sesi akan ditutup dengan aman...");
      setPasswordForm({ current: "", next: "", confirm: "" });
      window.setTimeout(() => {
        clearSession();
        window.location.replace("/login?reason=password-changed");
      }, 700);
    } catch (e) {
      setError(e.message);
    } finally {
      setPasswordBusy(false);
    }
  }

  return (
    <main className="page-wrap">
      <div>
        <p className="eyebrow">Identitas & Akses</p>
        <h1 className="page-title mt-3">Manajemen Peran</h1>
        <div className="gold-rule mt-5" />
        <p className="page-subtitle mt-5">Atur akses operasional dengan peran yang jelas, status akun, dan perubahan kata sandi yang aman.</p>
      </div>

      {error && <div className="toast-error mt-5">{error}</div>}
      {passwordMessage && <div className="mt-5 rounded-2xl border border-emerald-200/70 bg-emerald-50/80 px-4 py-3 text-sm font-bold text-emerald-900">{passwordMessage}</div>}

      <section className="mt-7 grid gap-5 xl:grid-cols-[390px_1fr]">
        <form onSubmit={changePassword} className="card p-6 md:p-7">
          <p className="section-label">Keamanan Akun</p>
          <h2 className="mt-2 text-2xl font-black text-emerald-950">Ubah kata sandi saya</h2>
          <p className="mt-2 text-sm leading-6 text-slate-500">Kata sandi saat ini akan diverifikasi sebelum kata sandi baru disimpan. Setelah berhasil, Anda akan diminta login kembali.</p>
          <div className="mt-5 space-y-3">
            <input className="input" type="password" placeholder="Kata sandi saat ini" autoComplete="current-password" value={passwordForm.current} onChange={(e) => setPasswordForm({ ...passwordForm, current: e.target.value })} required />
            <input className="input" type="password" placeholder="Kata sandi baru - minimal 12 karakter" autoComplete="new-password" value={passwordForm.next} onChange={(e) => setPasswordForm({ ...passwordForm, next: e.target.value })} required />
            <input className="input" type="password" placeholder="Konfirmasi kata sandi baru" autoComplete="new-password" value={passwordForm.confirm} onChange={(e) => setPasswordForm({ ...passwordForm, confirm: e.target.value })} required />
            <button className="btn w-full" disabled={passwordBusy}>{passwordBusy ? "Memperbarui dengan aman..." : "Perbarui Kata Sandi"}</button>
          </div>
        </form>

        <div className="card p-6 md:p-7">
          <p className="section-label">Kebijakan Sesi</p>
          <h2 className="mt-2 text-2xl font-black text-emerald-950">Timeout tidak aktif 15 menit</h2>
          <div className="mt-5 grid gap-3 md:grid-cols-3">
            <div className="rounded-2xl border border-emerald-950/8 bg-white/70 p-4"><strong className="text-sm text-emerald-950">Penyimpanan sesi saja</strong><p className="mt-2 text-xs leading-5 text-slate-500">Autentikasi tidak lagi disimpan secara permanen di local storage browser.</p></div>
            <div className="rounded-2xl border border-emerald-950/8 bg-white/70 p-4"><strong className="text-sm text-emerald-950">Perlindungan tidak aktif</strong><p className="mt-2 text-xs leading-5 text-slate-500">Aktivitas klik, keyboard, scroll, sentuh, dan navigasi akan mereset timer 15 menit.</p></div>
            <div className="rounded-2xl border border-emerald-950/8 bg-white/70 p-4"><strong className="text-sm text-emerald-950">Maksimal 30 menit</strong><p className="mt-2 text-xs leading-5 text-slate-500">JWT yang divalidasi API berakhir setelah 30 menit walaupun pengguna tetap aktif.</p></div>
          </div>
        </div>
      </section>

      <section className="mt-5 grid gap-5 xl:grid-cols-[390px_1fr]">
        <form onSubmit={submit} className="card p-6 md:p-7">
          <p className="section-label">Buat Akun</p>
          <h2 className="mt-2 text-2xl font-black text-emerald-950">Anggota tim baru</h2>
          <div className="mt-5 space-y-3">
            <input className="input" placeholder="Nama lengkap" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            <input className="input" placeholder="Email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
            <input className="input" placeholder="Kata sandi - minimal 12 karakter" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            <select className="input" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
              <option value="staff">Karyawan</option>
              <option value="auditor">PIC</option>
              <option value="admin">Owner</option>
            </select>
            <button className="btn w-full">Buat Pengguna</button>
          </div>
          <div className="mt-6 rounded-2xl border border-amber-200/60 bg-amber-50/60 p-4 text-xs leading-6 text-amber-900"><strong>Model akses:</strong> Owner mengelola akun dan melihat seluruh bisnis. PIC mengatur shift, approval, kualitas, stok, dan operasional. Karyawan hanya melihat ruang kerja pribadi, penjualan sesuai tugas, dan chat tim.</div>
        </form>

        <div className="card overflow-hidden">
          <div className="flex items-center justify-between border-b border-emerald-950/8 p-5">
            <div><p className="section-label">Direktori Pengguna</p><h2 className="mt-1 text-xl font-black text-emerald-950">{users.length} akun</h2></div>
          </div>
          <div className="divide-y divide-emerald-950/7">
            {users.length ? users.map((user) => (
              <div key={user.id} className="grid gap-4 p-5 md:grid-cols-[1fr_170px_120px] md:items-center">
                <div>
                  <div className="flex flex-wrap items-center gap-2"><strong className="text-emerald-950">{user.name}</strong><span className={`status-pill ${user.active ? "status-verified" : "status-closed"}`}>{user.active ? "aktif" : "nonaktif"}</span></div>
                  <p className="mt-1 text-sm text-slate-500">{user.email}</p>
                </div>
                <select className="input" value={user.role} onChange={(e) => patchUser(user, { role: e.target.value })}>
                  <option value="staff">Karyawan</option><option value="auditor">PIC</option><option value="admin">Owner</option>
                </select>
                <button className={`btn ${user.active ? "btn-secondary" : ""}`} type="button" onClick={() => patchUser(user, { active: !user.active })}>{user.active ? "Nonaktifkan" : "Aktifkan"}</button>
              </div>
            )) : <div className="empty-state m-5">Belum ada pengguna.</div>}
          </div>
        </div>
      </section>
    </main>
  );
}
