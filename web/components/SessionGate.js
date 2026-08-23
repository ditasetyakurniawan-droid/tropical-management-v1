"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { api, clearSession, setSession, token } from "../lib/api";
import EnterpriseLoader from "./EnterpriseLoader";

const IDLE_TIMEOUT_MS = 15 * 60 * 1000;
const ACTIVITY_EVENTS = ["click", "keydown", "scroll", "touchstart"];

export default function SessionGate({ children }) {
  const pathname = usePathname();
  const [ready, setReady] = useState(pathname === "/login");

  useEffect(() => {
    let expiryTimer;
    let idleTimer;
    let active = true;

    const clearTimers = () => {
      if (expiryTimer) window.clearTimeout(expiryTimer);
      if (idleTimer) window.clearTimeout(idleTimer);
    };

    const removeActivityListeners = () => {
      ACTIVITY_EVENTS.forEach((eventName) => {
        window.removeEventListener(eventName, resetIdleTimer);
      });
    };

    const endSession = (reason) => {
      if (!active) return;
      active = false;
      clearTimers();
      removeActivityListeners();
      clearSession();
      window.location.replace(`/login?reason=${reason}`);
    };

    function resetIdleTimer() {
      if (!active) return;
      if (idleTimer) window.clearTimeout(idleTimer);
      idleTimer = window.setTimeout(() => endSession("idle"), IDLE_TIMEOUT_MS);
    }

    if (pathname === "/login") {
      setReady(true);
      return () => {
        active = false;
        clearTimers();
        removeActivityListeners();
      };
    }

    setReady(false);
    const jwt = token();
    if (!jwt) {
      endSession("signed-out");
      return () => {};
    }

    api("/api/auth/me")
      .then((claims) => {
        if (!active) return;

        const expiresAt = Number(claims.exp) * 1000;
        const remaining = expiresAt - Date.now();

        if (!Number.isFinite(expiresAt) || remaining <= 0) {
          endSession("expired");
          return;
        }

        setSession(jwt, claims);

        expiryTimer = window.setTimeout(() => {
          endSession("expired");
        }, remaining);

        ACTIVITY_EVENTS.forEach((eventName) => {
          const passive = eventName === "scroll" || eventName === "touchstart";
          window.addEventListener(eventName, resetIdleTimer, passive ? { passive: true } : undefined);
        });

        // Route navigation also resets the idle window because this effect runs on pathname changes.
        resetIdleTimer();
        setReady(true);
      })
      .catch(() => {
        endSession("signed-out");
      });

    return () => {
      active = false;
      clearTimers();
      removeActivityListeners();
    };
  }, [pathname]);

  if (!ready) {
    return (
      <EnterpriseLoader
        message="Menyiapkan ruang kerja operasional"
        detail="Memvalidasi sesi aman dan hak akses pengguna"
      />
    );
  }

  return children;
}
