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

export function isOwnChatMessage(message, currentUser) {
  const currentId = currentUser?.sub ?? currentUser?.id;
  if (currentId == null) return false;
  return String(message?.user_id) === String(currentId);
}

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
    return null;
  }
}

// ===== Helper di luar komponen =====

async function loadHistory({ appendMessage, onError, isActive }) {
  try {
    const rows = await api(`${HISTORY_ENDPOINT}?limit=${MESSAGE_LIMIT_INITIAL}`);
    if (!isActive()) return;
    const history = Array.isArray(rows) ? rows : [];
    appendMessage(history);
  } catch (err) {
    if (isActive()) onError(err?.message || "Gagal memuat riwayat pesan");
  }
}

async function openStream() {
  const authToken = token();
  const response = await fetch(`${API_URL}${STREAM_ENDPOINT}`, {
    headers: authToken ? { Authorization: `Bearer ${authToken}` } : undefined,
    cache: "no-store",
  });
  if (!response.ok || !response.body) {
    throw new Error(`HTTP ${response.status}`);
  }
  return response.body.getReader();
}

async function consumeStream({ reader, appendMessage, isActive, isStopped }) {
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    if (isStopped() || !isActive()) {
      reader.cancel();
      break;
    }

    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const events = buffer.split("\n\n");
    buffer = events.pop() || "";

    for (const event of events) {
      const parsed = parseSSEEvent(event);
      if (parsed) appendMessage([parsed]);
    }
  }

  if (buffer && isActive() && !isStopped()) {
    const parsed = parseSSEEvent(buffer);
    if (parsed) appendMessage([parsed]);
  }
}

async function connectChatStream({
  appendMessage,
  onConnected,
  onDisconnected,
  onError,
  isActive,
  isStopped,
}) {
  try {
    const reader = await openStream();
    if (!isActive()) return;

    onConnected();
    await consumeStream({ reader, appendMessage, isActive, isStopped });
  } catch (err) {
    if (isActive() && !isStopped() && err?.name !== "AbortError") {
      onError("Koneksi chat sedang disambungkan ulang...");
    }
  } finally {
    if (isActive() && !isStopped()) {
      onDisconnected();
      window.setTimeout(
        () =>
          connectChatStream({
            appendMessage,
            onConnected,
            onDisconnected,
            onError,
            isActive,
            isStopped,
          }),
        RETRY_DELAY_MS
      );
    }
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
  const activeRef = useRef(true);
  const stoppedRef = useRef(false);

  const appendMessage = useCallback((incoming) => {
    setMessages((rows) => {
      const valid = incoming.filter(
        (msg) => msg?.id && !rows.some((row) => row.id === msg.id)
      );
      return [...rows, ...valid].slice(-MESSAGE_LIMIT_MAX);
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
        appendMessage([saved]);
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
    activeRef.current = true;
    stoppedRef.current = false;

    if (!authenticated) {
      setCurrentUser(null);
      setMessages([]);
      setConnected(false);
      setError("");
      return () => {
        activeRef.current = false;
        stoppedRef.current = true;
      };
    }

    setCurrentUser(sessionUser());

    loadHistory({
      appendMessage,
      onError: setError,
      isActive: () => activeRef.current,
    });

    connectChatStream({
      appendMessage,
      onConnected: () => setConnected(true),
      onDisconnected: () => setConnected(false),
      onError: setError,
      isActive: () => activeRef.current,
      isStopped: () => stoppedRef.current,
    });

    return () => {
      activeRef.current = false;
      stoppedRef.current = true;
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