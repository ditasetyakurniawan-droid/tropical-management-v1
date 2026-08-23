"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import { api, API_URL, sessionUser, token } from "../lib/api";

const LiveChatContext = createContext(null);

export function isOwnChatMessage(message, currentUser) {
  const currentId = currentUser?.sub ?? currentUser?.id;
  return currentId != null && String(message?.user_id) === String(currentId);
}

export default function LiveChatProvider({ children }) {
  const pathname = usePathname();
  const authenticated = pathname !== "/login" && Boolean(token());
  const [messages, setMessages] = useState([]);
  const [currentUser, setCurrentUser] = useState(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  const appendMessage = (message) => {
    setMessages((rows) => {
      if (!message || rows.some((row) => row.id === message.id)) return rows;
      return [...rows, message].slice(-200);
    });
  };

  useEffect(() => {
    let active = true;
    let stopped = false;
    let retryTimer;
    let controller;

    const user = authenticated ? sessionUser() : null;
    setCurrentUser(user);

    if (!authenticated) {
      setMessages([]);
      setConnected(false);
      setError("");
      return () => {
        active = false;
        stopped = true;
      };
    }

    api("/api/chat/messages?limit=100")
      .then((rows) => {
        if (active) setMessages(Array.isArray(rows) ? rows : []);
      })
      .catch((e) => {
        if (active) setError(e.message);
      });

    async function connect() {
      if (stopped) return;
      controller = new AbortController();

      try {
        const response = await fetch(`${API_URL}/api/chat/stream`, {
          headers: { Authorization: `Bearer ${token()}` },
          cache: "no-store",
          signal: controller.signal,
        });

        if (!response.ok || !response.body) {
          throw new Error(`HTTP ${response.status}`);
        }

        if (active) {
          setConnected(true);
          setError("");
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";

        while (!stopped) {
          const { done, value } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const events = buffer.split("\n\n");
          buffer = events.pop() || "";

          for (const event of events) {
            const data = event
              .split("\n")
              .filter((line) => line.startsWith("data:"))
              .map((line) => line.slice(5).trim())
              .join("\n");

            if (!data) continue;
            try {
              appendMessage(JSON.parse(data));
            } catch {
              // Abaikan event stream yang rusak dan pertahankan koneksi.
            }
          }
        }
      } catch (e) {
        if (!stopped && active && e.name !== "AbortError") {
          setError("Koneksi chat sedang disambungkan ulang...");
        }
      } finally {
        if (!stopped && active) {
          setConnected(false);
          retryTimer = window.setTimeout(connect, 1500);
        }
      }
    }

    connect();

    return () => {
      active = false;
      stopped = true;
      window.clearTimeout(retryTimer);
      controller?.abort();
    };
  }, [authenticated]);

  async function sendMessage(body) {
    const value = String(body || "").trim();
    if (!value || sending) return null;

    setSending(true);
    setError("");
    try {
      const saved = await api("/api/chat/messages", {
        method: "POST",
        body: JSON.stringify({ body: value }),
      });
      appendMessage(saved);
      return saved;
    } catch (e) {
      setError(e.message);
      throw e;
    } finally {
      setSending(false);
    }
  }

  const value = useMemo(() => ({
    messages,
    currentUser,
    connected,
    error,
    sending,
    sendMessage,
  }), [messages, currentUser, connected, error, sending]);

  return <LiveChatContext.Provider value={value}>{children}</LiveChatContext.Provider>;
}

export function useLiveChat() {
  const context = useContext(LiveChatContext);
  if (!context) throw new Error("useLiveChat harus digunakan di dalam LiveChatProvider");
  return context;
}
