"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { usePathname } from "next/navigation";
import { api, API_URL, sessionUser, token } from "../lib/api";

// Konstanta untuk nilai yang sering digunakan
const MESSAGE_LIMIT_INITIAL = 100;
const MESSAGE_LIMIT_MAX = 200;
const RETRY_DELAY_MS = 1500;
const STREAM_BUFFER_FLUSH_DELAY = 0; // Tidak digunakan, hanya dokumentasi
const SSE_EVENT_PREFIX = "data:";

// Helper untuk membandingkan user ID
export function isOwnChatMessage(message, currentUser) {
  const currentId = currentUser?.sub ?? currentUser?.id;
  return currentId != null && String(message?.user_id) === String(currentId);
}

// Helper untuk parsing satu event SSE menjadi objek JSON
function parseSSEEvent(eventText) {
  const data = eventText
    .split("\n")
    .filter((line) => line.startsWith(SSE_EVENT_PREFIX))
    .map((line) => line.slice(SSE_EVENT_PREFIX.length).trim())
    .join("\n");

  if (!data) return null;

  try {
    return JSON.parse(data);
  } catch {
    // Abaikan event yang rusak, pertahankan koneksi
    return null;
  }
}

const LiveChatContext = createContext(null);

export default function LiveChatProvider({ children }) {
  const pathname = usePathname();
  const authenticated = pathname !== "/login" && Boolean(token());
  const [messages, setMessages] = useState([]);
  const [currentUser, setCurrentUser] = useState(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  // Append message dengan deduplikasi dan batas maksimal riwayat
  const appendMessage = useCallback((message) => {
    setMessages((rows) => {
      if (!message || rows.some((row) => row.id === message.id)) return rows;
      return [...rows, message].slice(-MESSAGE_LIMIT_MAX);
    });
  }, []);

  // Kirim pesan baru
  const sendMessage = useCallback(async (body) => {
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
      setError(e.message || "Gagal mengirim pesan");
      throw e;
    } finally {
      setSending(false);
    }
  }, [sending, appendMessage]);

  useEffect(() => {
    let active = true;
    let stopped = false;
    let retryTimer = null;
    let controller = null;

    // Set currentUser hanya jika authenticated
    if (authenticated) {
      const user = sessionUser();
      setCurrentUser(user);
    } else {
      setCurrentUser(null);
      setMessages([]);
      setConnected(false);
      setError("");
      return () => {
        active = false;
        stopped = true;
      };
    }

    // Muat riwayat pesan
    api(`/api/chat/messages?limit=${MESSAGE_LIMIT_INITIAL}`)
      .then((rows) => {
        if (active) setMessages(Array.isArray(rows) ? rows : []);
      })
      .catch((e) => {
        if (active) setError(e.message || "Gagal memuat riwayat pesan");
      });

    async function connect() {
      if (stopped) return;

      controller = new AbortController();
      const authToken = token();

      try {
        const response = await fetch(`${API_URL}/api/chat/stream`, {
          headers: { Authorization: `Bearer ${authToken}` },
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

        while (true) {
          if (stopped) break;

          const { done, value } = await reader.read();
          if (done || stopped) break;

          buffer += decoder.decode(value, { stream: true });
          const events = buffer.split("\n\n");
          buffer = events.pop() || "";

          for (const event of events) {
            const parsed = parseSSEEvent(event);
            if (parsed) appendMessage(parsed);
          }
        }

        // Flush sisa buffer saat stream selesai
        if (buffer) {
          const parsed = parseSSEEvent(buffer);
          if (parsed) appendMessage(parsed);
        }
      } catch (e) {
        if (!stopped && active && e.name !== "AbortError") {
          setError("Koneksi chat sedang disambungkan ulang...");
        }
      } finally {
        if (!stopped && active) {
          setConnected(false);
          retryTimer = window.setTimeout(connect, RETRY_DELAY_MS);
        }
      }
    }

    connect();

    return () => {
      active = false;
      stopped = true;
      if (retryTimer) window.clearTimeout(retryTimer);
      controller?.abort();
    };
  }, [authenticated, appendMessage]);

  const value = useMemo(
    () => ({
      messages,
      currentUser,
      connected,
      error,
      sending,
      sendMessage,
    }),
    [messages, currentUser, connected, error, sending, sendMessage]
  );

  return (
    <LiveChatContext.Provider value={value}>
      {children}
    </LiveChatContext.Provider>
  );
}

export function useLiveChat() {
  const context = useContext(LiveChatContext);
  if (!context) {
    throw new Error("useLiveChat harus digunakan di dalam LiveChatProvider");
  }
  return context;
}