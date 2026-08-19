"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";

const links = [
  ["/", "Dashboard"],
  ["/audits", "Audit"],
  ["/inventory", "Inventory"],
  ["/sales", "Sales"],
  ["/users", "Users"],
];

export default function Nav() {
  const pathname = usePathname();
  const [user, setUser] = useState(null);

  useEffect(() => {
    try {
      const raw = localStorage.getItem("tropical_user");
      if (raw) setUser(JSON.parse(raw));
    } catch {
      setUser(null);
    }
  }, [pathname]);

  if (pathname === "/login") return null;

  function logout() {
    localStorage.removeItem("tropical_token");
    localStorage.removeItem("tropical_user");
    window.location.replace("/login");
  }

  return (
    <header className="luxury-nav">
      <div className="mx-auto flex max-w-7xl items-center gap-4 px-4 py-3 md:px-6">
        <Link href="/" className="brand-lockup" aria-label="Tropical Management home">
          <span className="brand-mark"><span /></span>
          <span>TROPICAL<strong>.</strong></span>
        </Link>

        <nav className="nav-scroll ml-auto flex items-center gap-1 overflow-x-auto text-sm font-semibold">
          {links.map(([href, label]) => (
            <Link
              key={href}
              href={href}
              className={`luxury-nav-link ${pathname === href ? "active" : ""}`}
            >
              {label}
            </Link>
          ))}
        </nav>

        <div className="hidden items-center gap-3 border-l border-emerald-950/10 pl-4 md:flex">
          <div className="text-right leading-tight">
            <p className="text-xs font-black text-emerald-950">{user?.name || "Tropical User"}</p>
            <p className="text-[10px] font-bold uppercase tracking-[.18em] text-amber-600">{user?.role || "session"}</p>
          </div>
          <button onClick={logout} className="logout-button" type="button">Logout</button>
        </div>
      </div>
    </header>
  );
}
