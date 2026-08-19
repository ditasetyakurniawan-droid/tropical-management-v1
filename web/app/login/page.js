"use client";

import { useState } from "react";
import { API_URL } from "../../lib/api";

export default function Login() {
  const [email, setEmail] = useState("admin@tropical.local");
  const [password, setPassword] = useState("ChangeThis123!");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event) {
    event.preventDefault();
    setError("");
    setLoading(true);
    try {
      const response = await fetch(`${API_URL}/api/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      const data = await response.json();
      if (!response.ok) {
        setError(data.error || "Login gagal");
        return;
      }
      localStorage.setItem("tropical_token", data.token);
      localStorage.setItem("tropical_user", JSON.stringify(data.user));
      window.location.href = "/";
    } catch {
      setError("API tidak dapat dihubungi. Pastikan docker compose sedang berjalan.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="relative z-10 grid min-h-screen place-items-center px-5 py-10">
      <div className="grid w-full max-w-5xl overflow-hidden rounded-[36px] border border-emerald-950/10 bg-white/80 shadow-[0_40px_120px_rgba(5,44,34,.18)] backdrop-blur-xl lg:grid-cols-[1.08fr_.92fr]">
        <section className="hero-luxury hidden min-h-[610px] rounded-none border-0 p-12 lg:flex lg:flex-col lg:justify-between">
          <div>
            <span className="hero-badge">Restaurant Intelligence Suite</span>
            <h1 className="mt-7 max-w-lg text-6xl font-black leading-[.94] tracking-[-.06em]">Operate with clarity. Lead with standards.</h1>
            <p className="mt-6 max-w-md text-sm leading-7 text-emerald-50/75">Sales, compliance, stock health and corrective actions united in one calm operational command center.</p>
          </div>
          <div className="grid grid-cols-3 gap-3">
            {["Sales Intelligence", "Internal Audit", "Inventory Control"].map((label) => (
              <div key={label} className="rounded-2xl border border-white/10 bg-white/5 p-4 text-xs font-bold text-emerald-50/80 backdrop-blur">
                <div className="mb-3 h-1 w-8 rounded-full bg-amber-300/80" />{label}
              </div>
            ))}
          </div>
        </section>

        <section className="flex min-h-[610px] items-center p-7 md:p-12">
          <form onSubmit={submit} className="w-full">
            <div className="mb-10">
              <div className="mb-5 flex items-center gap-3">
                <span className="brand-mark"><span /></span>
                <span className="font-black tracking-tight text-emerald-950">TROPICAL<span className="text-amber-500">.</span></span>
              </div>
              <p className="eyebrow">Secure operations access</p>
              <h2 className="mt-3 text-4xl font-black tracking-[-.045em] text-emerald-950">Welcome back.</h2>
              <p className="mt-3 text-sm leading-6 text-slate-500">Sign in to your restaurant management and internal audit workspace.</p>
            </div>

            <div className="space-y-4">
              <label className="block text-xs font-black uppercase tracking-[.12em] text-emerald-950/70">Email
                <input className="input mt-2" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="Email" autoComplete="username" />
              </label>
              <label className="block text-xs font-black uppercase tracking-[.12em] text-emerald-950/70">Password
                <input className="input mt-2" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password" autoComplete="current-password" />
              </label>
              {error && <div className="toast-error">{error}</div>}
              <button className="btn mt-2 w-full" disabled={loading}>{loading ? "Signing in..." : "Enter Control Center"}</button>
            </div>

            <p className="mt-7 text-center text-[11px] leading-5 text-slate-400">Local bootstrap credentials are development-only. Production credentials will be Vault-injected.</p>
          </form>
        </section>
      </div>
    </main>
  );
}
