"use client";

import { useEffect, useState } from "react";
import { API_URL, clearSession, setSession } from "../../lib/api";

export default function Login() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [reason, setReason] = useState("");

  useEffect(() => {
    clearSession();
    setReason(new URLSearchParams(window.location.search).get("reason") || "");
  }, []);

  async function submit(event) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const response = await fetch(`${API_URL}/api/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
        cache: "no-store",
      });
      const data = await response.json();
      if (!response.ok) {
        setError(data.error || "Login gagal");
        return;
      }
      setSession(data.token, data.user);
      window.location.replace("/");
    } catch {
      setError("API tidak dapat dihubungi. Pastikan backend sedang berjalan.");
    } finally {
      setLoading(false);
    }
  }

  const idle = reason === "idle";
  const expired = reason === "expired";
  const passwordChanged = reason === "password-changed";

  return (
    <main className="relative z-10 grid min-h-screen place-items-center px-5 py-10">
      <div className="grid w-full max-w-5xl overflow-hidden rounded-[36px] border border-emerald-950/10 bg-white/80 shadow-[0_40px_120px_rgba(5,44,34,.18)] backdrop-blur-xl lg:grid-cols-[1.08fr_.92fr]">
        <section className="hero-luxury hidden min-h-[610px] rounded-none border-0 p-12 lg:flex lg:flex-col lg:justify-between">
          <div>
            <span className="hero-badge">Sistem Intelijen Restoran</span>
            <h1 className="mt-7 max-w-lg text-6xl font-black leading-[.94] tracking-[-.06em]">Kelola dengan jelas. Pimpin dengan standar.</h1>
            <p className="mt-6 max-w-md text-sm leading-7 text-emerald-50/75">Penjualan, kepatuhan, kesehatan stok, dan tindakan korektif dalam satu pusat kendali operasional yang ringkas.</p>
          </div>
          <div className="grid grid-cols-3 gap-3">
            {["Analitik Penjualan", "Audit Internal", "Kontrol Inventaris"].map((label) => (
              <div key={label} className="rounded-2xl border border-white/10 bg-white/5 p-4 text-xs font-bold text-emerald-50/80 backdrop-blur">
                <div className="mb-3 h-1 w-8 rounded-full bg-amber-300/80" />{label}
              </div>
            ))}
          </div>
        </section>

        <section className="flex min-h-[610px] items-center p-7 md:p-12">
          <form onSubmit={submit} className="w-full" autoComplete="on">
            <div className="mb-10">
              <div className="mb-5 flex items-center gap-3">
                <span className="brand-mark"><span /></span>
                <span className="font-black tracking-tight text-emerald-950">TROPICAL<span className="text-amber-500">.</span></span>
              </div>
              <p className="eyebrow">Akses Operasional Aman</p>
              <h2 className="mt-3 text-4xl font-black tracking-[-.045em] text-emerald-950">Selamat datang kembali.</h2>
              <p className="mt-3 text-sm leading-6 text-slate-500">Masuk ke ruang kerja manajemen restoran dan audit internal Anda.</p>
            </div>

            <div className="space-y-4">
              {idle && <div className="rounded-2xl border border-amber-200/70 bg-amber-50/80 px-4 py-3 text-sm font-bold text-amber-900">Sesi berakhir setelah 15 menit tidak ada aktivitas. Silakan masuk kembali.</div>}
              {expired && <div className="rounded-2xl border border-amber-200/70 bg-amber-50/80 px-4 py-3 text-sm font-bold text-amber-900">Batas maksimum sesi 30 menit telah berakhir. Silakan masuk kembali.</div>}
              {passwordChanged && <div className="rounded-2xl border border-emerald-200/70 bg-emerald-50/80 px-4 py-3 text-sm font-bold text-emerald-900">Kata sandi berhasil diperbarui. Silakan masuk kembali dengan kata sandi baru.</div>}
              <label className="block text-xs font-black uppercase tracking-[.12em] text-emerald-950/70">Email
                <input className="input mt-2" type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="admin@tropical.local" autoComplete="username" autoFocus required />
              </label>
              <label className="block text-xs font-black uppercase tracking-[.12em] text-emerald-950/70">Kata Sandi
                <input className="input mt-2" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Masukkan kata sandi" autoComplete="current-password" required />
              </label>
              {error && <div className="toast-error">{error}</div>}
              <button className="btn mt-2 w-full" disabled={loading}>
                {loading ? <span className="inline-flex items-center justify-center gap-2"><span className="button-spinner" />Memproses login...</span> : "Masuk ke Pusat Kontrol"}
              </button>
            </div>

            <p className="mt-7 text-center text-[11px] leading-5 text-slate-400">Kredensial sesi hanya disimpan selama sesi browser ini. Sesi akan berakhir setelah 15 menit tidak aktif atau maksimal 30 menit sejak login.</p>
          </form>
        </section>
      </div>
    </main>
  );
}
