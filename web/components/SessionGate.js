"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { api, token } from "../lib/api";

export default function SessionGate({ children }) {
  const pathname = usePathname();
  const [ready, setReady] = useState(pathname === "/login");

  useEffect(() => {
    if (pathname === "/login") {
      setReady(true);
      return;
    }

    if (!token()) {
      window.location.replace("/login");
      return;
    }

    api("/api/auth/me")
      .then((claims) => {
        localStorage.setItem("tropical_user", JSON.stringify(claims));
        setReady(true);
      })
      .catch(() => {
        localStorage.removeItem("tropical_token");
        localStorage.removeItem("tropical_user");
        window.location.replace("/login");
      });
  }, [pathname]);

  if (!ready) {
    return (
      <div className="session-loading">
        <div className="luxury-spinner" />
        <p>Preparing your operations workspace...</p>
      </div>
    );
  }

  return children;
}
