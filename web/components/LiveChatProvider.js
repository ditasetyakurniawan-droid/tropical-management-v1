"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { usePathname } from "next/navigation";
import { api, API_URL, sessionUser, token } from "../lib/api";

const MESSAGE_LIMIT_INITIAL = 100;
const MESSAGE_LIMIT_MAX = 200;
const RETRY_DELAY_MS = 1500;
const SSE_EVENT_PREFIX = "data:";
const HISTORY_ENDPOINT = "/api/chat/messages";
const STREAM_ENDPOINT = "/api/chat/stream";

// Helper untuk membandingkan user ID
export function isOwnChatMessage(message, currentUser) {
  const currentId = currentUser?.sub ?? currentUser?.id;

  if (currentId == null) return false;
  return String(message?.user_id) === String(currentId);
}

// Helper untuk parsing satu event SSE menjadi objek JSON
function parseSSEEvent(eventText) {
  if (!eventText) return null;

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

  const sendingRef = useRef(false);

  const appendMessage = useCallback((message) => {
    setMessages((rows) => {
      if (!message?.id || rows.some((row) => row.id === message.id)) {
        return rows;
      }

      return [...rows, message].slice(-MESSAGE_LIMIT_MAX);
    });
  }, []);

  const sendMessage = useCallback(
    async (body) => {
      const value = String(body ?? "").trim();

      if (!value || sendingRef.current) return null;

      sendingRef.current = true;
      setSending(true);
      setError("");

      try {
        const saved = await api(HISTORY_ENDPOINT, {
          method: "POST",
          body: JSON.stringify({ body: value }),
        });

        appendMessage(saved);
        return saved;
      } catch (err) {
        setError(err?.message || "Gagal mengirim pesan");
        throw err;
      } finally {
        sendingRef.current = false;
        setSending(false);
      }
    },
    [appendMessage]
  );

  useEffect(() => {
    let active = true;
    let stopped = false;
    let retryTimer = null;
    let streamController = null;

    if (!authenticated) {
      setCurrentUser(null);
      setMessages([]);
      setConnected(false);
      setError("");

      return () => {
        active = false;
        stopped = true;
      };
    }

    setCurrentUser(sessionUser());

    api(`${HISTORY_ENDPOINT}?limit=${MESSAGE_LIMIT_INITIAL}`)
      .then((rows) => {
        if (!active) return;

        const history = Array.isArray(rows) ? rows : [];
        setMessages(history.slice(-MESSAGE_LIMIT_MAX));
      })
      .catch((err) => {
        if (active) {
          setError(err?.message || "Gagal memuat riwayat pesan");
        }
      });

    async function connectToStream() {
      if (stopped) return;

      streamController = new AbortController();
      const authToken = token();

      try {
        const response = await fetch(`${API_URL}${STREAM_ENDPOINT}`, {
          headers: authToken
            ? { Authorization: `Bearer ${authToken}` }
            : undefined,
          cache: "no-store",
          signal: streamController.signal,
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

        while (!stopped && active) {
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

        if (buffer && active && !stopped) {
          const parsed = parseSSEEvent(buffer);
          if (parsed) appendMessage(parsed);
        }
      } catch (err) {
        if (active && !stopped && err?.name !== "AbortError") {
          setError("Koneksi chat sedang disambungkan ulang...");
        }
      } finally {
        if (active && !stopped) {
          setConnected(false);
          retryTimer = window.setTimeout(connectToStream, RETRY_DELAY_MS);
        }
      }
    }

    connectToStream();

    return () => {
      active = false;
      stopped = true;

      if (retryTimer) window.clearTimeout(retryTimer);
      streamController?.abort();
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