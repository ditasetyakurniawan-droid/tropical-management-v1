"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import EnterpriseLoader from "./EnterpriseLoader";

const MINIMUM_VISIBLE_MS = 260;
const SAFETY_TIMEOUT_MS = 5000;

export default function RouteTransition({ children }) {
  const pathname = usePathname();
  const [navigating, setNavigating] = useState(false);
  const navigationStartedAt = useRef(0);
  const previousPathname = useRef(pathname);
  const safetyTimer = useRef(null);

  useEffect(() => {
    function beginNavigation(event) {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

      const anchor = event.target.closest?.("a[href]");
      if (!anchor || anchor.target === "_blank" || anchor.hasAttribute("download")) return;

      const rawHref = anchor.getAttribute("href");
      if (!rawHref || rawHref.startsWith("#") || rawHref.startsWith("mailto:") || rawHref.startsWith("tel:")) return;

      let destination;
      try {
        destination = new URL(anchor.href, window.location.href);
      } catch {
        return;
      }

      if (destination.origin !== window.location.origin) return;

      const current = `${window.location.pathname}${window.location.search}`;
      const next = `${destination.pathname}${destination.search}`;
      if (current === next) return;

      navigationStartedAt.current = Date.now();
      setNavigating(true);

      window.clearTimeout(safetyTimer.current);
      safetyTimer.current = window.setTimeout(() => setNavigating(false), SAFETY_TIMEOUT_MS);
    }

    document.addEventListener("click", beginNavigation, true);
    return () => {
      document.removeEventListener("click", beginNavigation, true);
      window.clearTimeout(safetyTimer.current);
    };
  }, []);

  useEffect(() => {
    if (previousPathname.current === pathname) return;
    previousPathname.current = pathname;

    const elapsed = Date.now() - navigationStartedAt.current;
    const remaining = Math.max(MINIMUM_VISIBLE_MS - elapsed, 80);
    const finishTimer = window.setTimeout(() => {
      setNavigating(false);
      window.clearTimeout(safetyTimer.current);
    }, remaining);

    return () => window.clearTimeout(finishTimer);
  }, [pathname]);

  return (
    <>
      <div className={`route-progress ${navigating ? "route-progress-active" : ""}`} aria-hidden="true"><span /></div>
      {navigating && (
        <div className="route-loading-overlay">
          <EnterpriseLoader
            embedded
            message="Berpindah tampilan operasional"
            detail="Memuat halaman berikutnya tanpa memutus sesi Anda"
          />
        </div>
      )}
      <div key={pathname} className="route-page-enter">{children}</div>
    </>
  );
}
