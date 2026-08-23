"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { roleLabel } from "../lib/labels";
import { isOwnChatMessage, useLiveChat } from "./LiveChatProvider";

function initials(name = "User") {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("") || "U";
}

export default function FloatingChat() {
  const pathname = usePathname();
  const { messages, currentUser, connected, error, sending, sendMessage } = useLiveChat();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const [unread, setUnread] = useState(0);
  const lastMessageId = useRef(null);
  const bottomRef = useRef(null);

  useEffect(() => {
    if (!messages.length) return;
    const latest = messages[messages.length - 1];

    if (lastMessageId.current == null) {
      lastMessageId.current = latest.id;
      return;
    }

    if (latest.id !== lastMessageId.current) {
      if (!open && !isOwnChatMessage(latest, currentUser)) {
        setUnread((value) => Math.min(value + 1, 99));
      }
      lastMessageId.current = latest.id;
    }
  }, [messages, open, currentUser]);

  useEffect(() => {
    if (!open) return;
    setUnread(0);
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [open, messages.length]);

  async function submit(event) {
    event.preventDefault();
    const body = draft.trim();
    if (!body || sending) return;

    try {
      await sendMessage(body);
      setDraft("");
    } catch {
      // Error is rendered from the shared live chat state.
    }
  }

  if (pathname === "/login" || !currentUser) return null;

  return (
    <div className="floating-chat-shell">
      {open && (
        <section className="floating-chat-panel" aria-label="Chat tim">
          <header className="floating-chat-header">
            <div>
              <div className="flex items-center gap-2">
                <strong>Chat Tim</strong>
                <span className={`floating-chat-status ${connected ? "online" : ""}`}>{connected ? "Aktif" : "Menghubungkan"}</span>
              </div>
              <p>{currentUser.name || "Pengguna Tropical"} - {roleLabel(currentUser.role)}</p>
            </div>
            <button type="button" className="floating-chat-close" onClick={() => setOpen(false)} aria-label="Tutup chat">x</button>
          </header>

          {error && <div className="floating-chat-error">{error}</div>}

          <div className="floating-chat-feed" aria-live="polite">
            {messages.length === 0 ? (
              <div className="floating-chat-empty">
                <strong>Belum ada percakapan.</strong>
                <span>Kirim pesan pertama untuk tim.</span>
              </div>
            ) : messages.slice(-60).map((message) => {
              const own = isOwnChatMessage(message, currentUser);
              return (
                <article key={message.id} className={`floating-chat-message ${own ? "own" : ""}`}>
                  <div className="floating-chat-avatar">{initials(message.user_name)}</div>
                  <div className="floating-chat-bubble">
                    <div className="floating-chat-meta">
                      <strong>{own ? "Anda" : message.user_name}</strong>
                      <span>{roleLabel(message.role)}</span>
                      <time>{new Date(message.created_at).toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" })}</time>
                    </div>
                    <p>{message.body}</p>
                  </div>
                </article>
              );
            })}
            <div ref={bottomRef} />
          </div>

          <form className="floating-chat-compose" onSubmit={submit}>
            <input
              className="input"
              maxLength={1000}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Tulis pesan untuk tim..."
              aria-label="Pesan chat"
            />
            <button className="btn" disabled={sending || !draft.trim()}>{sending ? "Mengirim..." : "Kirim"}</button>
          </form>

          <footer className="floating-chat-footer">
            <span>Pesan tersinkron secara real-time.</span>
            {pathname !== "/chat" && <Link href="/chat">Buka chat penuh</Link>}
          </footer>
        </section>
      )}

      <button
        type="button"
        className={`floating-chat-launcher ${open ? "open" : ""}`}
        onClick={() => setOpen((value) => !value)}
        aria-label={open ? "Tutup chat tim" : "Buka chat tim"}
        title="Chat Tim"
      >
        <span className="floating-chat-emoji" aria-hidden="true">{"\uD83D\uDCAC"}</span>
        <span className="floating-chat-launcher-label">Chat</span>
        {unread > 0 && <span className="floating-chat-unread">{unread > 9 ? "9+" : unread}</span>}
      </button>
    </div>
  );
}
