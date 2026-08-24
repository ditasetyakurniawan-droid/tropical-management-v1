"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { api, clearSession, setSession, token } from "../lib/api";
import EnterpriseLoader from "./EnterpriseLoader";

// Constants
const IDLE_TIMEOUT_MS = 15 * 60 * 1000;
const ACTIVITY_EVENTS = ["click", "keydown", "scroll", "touchstart"];
const SESSION_END_REASONS = {
  IDLE: "idle",
  EXPIRED: "expired",
  SIGNED_OUT: "signed-out",
};

export default function SessionGate({ children }) {
  const pathname = usePathname();
  const [ready, setReady] = useState(pathname === "/login");

  // Refs untuk lifecycle dan timer agar tidak terpengaruh closure
  const isMountedRef = useRef(true);
  const expiryTimerRef = useRef(null);
  const idleTimerRef = useRef(null);

  const clearTimers = () => {
    if (expiryTimerRef.current) {
      window.clearTimeout(expiryTimerRef.current);
      expiryTimerRef.current = null;
    }
    if (idleTimerRef.current) {
      window.clearTimeout(idleTimerRef.current);
      idleTimerRef.current = null;
    }
  };

  const removeActivityListeners = () => {
    ACTIVITY_EVENTS.forEach((eventName) => {
      window.removeEventListener(eventName, resetIdleTimer);
    });
  };

  // Definisikan resetIdleTimer di luar effect agar bisa dipakai oleh removeActivityListeners
  const resetIdleTimer = () => {
    if (!isMountedRef.current) return;
    if (idleTimerRef.current) {
      window.clearTimeout(idleTimerRef.current);
    }
    idleTimerRef.current = window.setTimeout(() => {
      endSession(SESSION_END_REASONS.IDLE);
    }, IDLE_TIMEOUT_MS);
  };

  // endSession juga di luar effect, tapi memerlukan akses ke fungsi di atas
  const endSession = (reason) => {
    if (!isMountedRef.current) return;
    isMountedRef.current = false;
    clearTimers();
    removeActivityListeners();
    clearSession();
    window.location.replace(`/login?reason=${reason}`);
  };

  useEffect(() => {
    // Set mounted flag on mount and cleanup on unmount
    isMountedRef.current = true;

    // Jika sedang di halaman login, tidak perlu validasi
    if (pathname === "/login") {
      setReady(true);
      return () => {
        isMountedRef.current = false;
        clearTimers();
        removeActivityListeners();
      };
    }

    // Reset state dan cek token
    setReady(false);
    const jwt = token();

    if (!jwt) {
      endSession(SESSION_END_REASONS.SIGNED_OUT);
      return () => {
        isMountedRef.current = false;
      };
    }

    // Validasi sesi ke server
    api("/api/auth/me")
      .then((claims) => {
        if (!isMountedRef.current) return;

        const expiresAt = Number(claims.exp) * 1000;
        const remaining = expiresAt - Date.now();

        if (!Number.isFinite(expiresAt) || remaining <= 0) {
          endSession(SESSION_END_REASONS.EXPIRED);
          return;
        }

        setSession(jwt, claims);

        // Atur timer kedaluwarsa berdasarkan masa berlaku token
        expiryTimerRef.current = window.setTimeout(() => {
          endSession(SESSION_END_REASONS.EXPIRED);
        }, remaining);

        // Pasang listener aktivitas untuk reset idle timer
        ACTIVITY_EVENTS.forEach((eventName) => {
          const passive = eventName === "scroll" || eventName === "touchstart";
          window.addEventListener(eventName, resetIdleTimer, passive ? { passive: true } : undefined);
        });

        // Set idle timer awal
        resetIdleTimer();
        setReady(true);
      })
      .catch(() => {
        if (isMountedRef.current) {
          endSession(SESSION_END_REASONS.SIGNED_OUT);
        }
      });

    // Cleanup utama
    return () => {
      isMountedRef.current = false;
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