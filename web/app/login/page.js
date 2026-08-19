"use client";
import { useState } from "react";
import { API_URL } from "../../lib/api";

export default function Login() {
  const [email,setEmail]=useState("admin@tropical.local");
  const [password,setPassword]=useState("ChangeThis123!");
  const [error,setError]=useState("");
  async function submit(e){e.preventDefault();setError("");const res=await fetch(`${API_URL}/api/auth/login`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email,password})});const data=await res.json();if(!res.ok){setError(data.error||"Login gagal");return;}localStorage.setItem("tropical_token",data.token);window.location.href="/";}
  return <main className="grid min-h-screen place-items-center p-5"><form onSubmit={submit} className="card w-full max-w-md p-8"><div className="mb-8"><div className="text-xs font-black uppercase tracking-[.3em] text-amber-500">Tropical Operations</div><h1 className="mt-2 text-4xl font-black text-emerald-950">Welcome back.</h1><p className="mt-2 text-slate-500">Restaurant Management & Internal Audit</p></div><div className="space-y-4"><input className="input" value={email} onChange={e=>setEmail(e.target.value)} placeholder="Email"/><input className="input" type="password" value={password} onChange={e=>setPassword(e.target.value)} placeholder="Password"/>{error&&<p className="text-sm text-red-600">{error}</p>}<button className="btn w-full">Sign in</button></div></form></main>;
}
