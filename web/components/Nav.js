"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import { clearSession, sessionUser } from "../lib/api";
import { roleLabel } from "../lib/labels";
import { navigationForRole, personaForRole } from "../lib/roles";

export default function Nav() {
  const pathname = usePathname();
  const [user, setUser] = useState(null);

  useEffect(() => {
    setUser(sessionUser());
  }, [pathname]);

  const links = useMemo(() => navigationForRole(user?.role || "staff"), [user?.role]);
  const persona = personaForRole(user?.role);

  if (pathname === "/login") return null;

  function logout() {
    clearSession();
    window.location.replace("/login");
  }

  return (
    <header className="luxury-nav">
      <div className="mx-auto flex max-w-7xl items-center gap-4 px-4 py-3 md:px-6">
        <Link href="/" className="brand-lockup" aria-label="Beranda Tropical Steak House">
          <span className="brand-mark"><span /></span>
          <span className="brand-copy">
            <span className="brand-title">TROPICAL<strong>.</strong></span>
            <span className="brand-subtitle">STEAK HOUSE · INTERNAL OS</span>
          </span>
        </Link>

        <nav className="nav-scroll ml-auto flex items-center gap-1 overflow-x-auto text-sm font-semibold" aria-label={`Navigasi ${persona}`}>
          {links.map(({ href, label }) => (
            <Link key={href} href={href} className={`luxury-nav-link ${pathname === href ? "active" : ""}`}>
              {label}
            </Link>
          ))}
        </nav>

        <div className="hidden items-center gap-3 border-l border-emerald-950/10 pl-4 md:flex">
          <div className="text-right leading-tight">
            <p className="text-sm font-black text-emerald-950">{user?.name || "Tim Tropical"}</p>
            <p className="text-[11px] font-black uppercase tracking-[.16em] text-amber-600">{roleLabel(user?.role || "staff")}</p>
          </div>
          <button onClick={logout} className="logout-button" type="button">Keluar</button>
        </div>
      </div>
    </header>
  );
}
