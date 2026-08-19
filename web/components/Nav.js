"use client";

import Link from "next/link";
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
  if (pathname === "/login") return null;
  return (
    <header className="sticky top-0 z-20 border-b border-emerald-950/10 bg-white/90 backdrop-blur">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-5 py-4">
        <Link href="/" className="font-black tracking-tight text-emerald-950">TROPICAL<span className="text-amber-500">.</span></Link>
        <nav className="flex max-w-[70vw] gap-2 overflow-x-auto text-sm font-semibold">
          {links.map(([href, label]) => <Link key={href} href={href} className={`rounded-full px-3 py-2 whitespace-nowrap ${pathname === href ? "bg-emerald-950 text-white" : "text-emerald-950 hover:bg-emerald-50"}`}>{label}</Link>)}
        </nav>
      </div>
    </header>
  );
}
