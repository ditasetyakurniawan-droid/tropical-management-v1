"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { api, API_URL, token } from "../../lib/api";

const palette = ["#0f766e", "#b45309", "#7c3aed", "#be123c", "#0369a1", "#4d7c0f", "#c2410c", "#6d28d9", "#0f766e", "#9f1239"];

function accentFor(name = "") {
  let hash = 0;
  for (const char of name) hash = ((hash << 5) - hash + char.charCodeAt(0)) | 0;
  return palette[Math.abs(hash) % palette.length];
}

function initials(name = "User") {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join("") || "U";
}

function readCurrentUser() {
  try {
    return JSON.parse(localStorage.getItem("tropical_user") || "null");
  } catch {
    return null;
  }
}

export default function ChatPage() {
  const [messages, setMessages] = useState([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");
  const [currentUser, setCurrentUser] = useState(null);
  const bottomRef = useRef(null);

  const appendMessage = (message) => {
    setMessages((rows) => {
      if (rows.some((row) => row.id === message.id)) return rows;
      return [...rows, message].slice(-200);
    });
  };

  useEffect(() => {
    setCurrentUser(readCurrentUser());
    api("/api/chat/messages?limit=100")
      .then(setMessages)
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    let stopped = false;
    let retryTimer;
    const controller = new AbortController();

    async function connect() {
      if (stopped) return;
      try {
        const response = await fetch(`${API_URL}/api/chat/stream`, {
          headers: { Authorization: `Bearer ${token()}` },
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok || !response.body) throw new Error(`Live stream HTTP ${response.status}`);
        setConnected(true);
        setError("");
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
            const data = event.split("\n").filter((line) => line.startsWith("data:"))
              .map((line) => line.slice(5).trim()).join("\n");
            if (!data) continue;
            try { appendMessage(JSON.parse(data)); } catch { /* ignore malformed event */ }
          }
        }
      } catch (e) {
        if (!stopped && e.name !== "AbortError") setError("Live connection reconnecting...");
      } finally {
        if (!stopped) {
          setConnected(false);
          retryTimer = setTimeout(connect, 1500);
        }
      }
    }

    connect();
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      controller.abort();
    };
  }, []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages.length]);

  const participantCount = useMemo(() => new Set(messages.slice(-100).map((message) => message.user_id)).size, [messages]);

  async function send(event) {
    event.preventDefault();
    const body = draft.trim();
    if (!body || sending) return;
    setSending(true);
    setError("");
    try {
      const saved = await api("/api/chat/messages", { method: "POST", body: JSON.stringify({ body }) });
      appendMessage(saved);
      setDraft("");
    } catch (e) {
      setError(e.message);
    } finally {
      setSending(false);
    }
  }

  return (
    <main className="page-wrap">
      <section className="mb-7 grid gap-6 lg:grid-cols-[1fr_auto] lg:items-end">
        <div>
          <p className="eyebrow">Team communication</p>
          <h1 className="page-title mt-3">General Live Chat</h1>
          <div className="gold-rule mt-5" />
          <p className="page-subtitle mt-5">One shared room for every authenticated Tropical user. Identity and role come from the verified session, not from editable chat fields.</p>
        </div>
        <div className="card flex gap-6 px-5 py-4">
          <div><p className="section-label">Participants</p><p className="mt-1 text-2xl font-black text-emerald-950">{participantCount}</p></div>
          <div><p className="section-label">Session</p><p className="mt-1 text-sm font-black capitalize text-emerald-950">{currentUser?.role || "—"}</p></div>
        </div>
      </section>

      {error && <div className="toast-error mb-4">{error}</div>}

      <section className="card chat-card">
        <header className="chat-head">
          <div>
            <p className="section-label">General room</p>
            <h2 className="mt-1 text-2xl font-black text-emerald-950">Restaurant Operations Lounge</h2>
          </div>
          <span className="live-indicator">{connected ? "LIVE" : "RECONNECTING"}</span>
        </header>

        <div className="chat-feed" aria-live="polite">
          {messages.length === 0 ? (
            <div className="chat-empty"><strong className="block text-lg text-emerald-950">Belum ada percakapan.</strong><span>Mulai pesan pertama untuk tim. Semua user yang login akan menerima pesan baru secara real-time.</span></div>
          ) : messages.map((message) => {
            const own = String(message.user_id) === String(currentUser?.sub);
            const accent = accentFor(message.user_name);
            return (
              <article key={message.id} className={`chat-message ${own ? "own" : ""}`}>
                <div className="chat-avatar">{initials(message.user_name)}</div>
                <div className="chat-bubble">
                  <div className="chat-user-line">
                    <span className="chat-user-name" style={{ color: own ? "#f7dda0" : accent }}>{message.user_name}</span>
                    <span className="chat-role">{message.role}</span>
                    <time className="chat-time">{new Date(message.created_at).toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" })}</time>
                  </div>
                  <p className="chat-body">{message.body}</p>
                </div>
              </article>
            );
          })}
          <div ref={bottomRef} />
        </div>

        <form className="chat-compose" onSubmit={send}>
          <input className="input" maxLength={1000} value={draft} onChange={(e) => setDraft(e.target.value)} placeholder={`Tulis pesan sebagai ${currentUser?.name || "user"}...`} />
          <button className="btn px-6" disabled={sending || !draft.trim()}>{sending ? "Sending..." : "Send message"}</button>
        </form>
      </section>
    </main>
  );
}
