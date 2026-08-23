"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { roleLabel } from "../../lib/labels";
import { isOwnChatMessage, useLiveChat } from "../../components/LiveChatProvider";

const palette = ["#0f766e", "#b45309", "#7c3aed", "#be123c", "#0369a1", "#4d7c0f", "#c2410c", "#6d28d9", "#0f766e", "#9f1239"];

function accentFor(name = "") {
  let hash = 0;
  for (const char of name) hash = ((hash << 5) - hash + char.charCodeAt(0)) | 0;
  return palette[Math.abs(hash) % palette.length];
}

function initials(name = "User") {
  return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join("") || "U";
}

export default function ChatPage() {
  const { messages, currentUser, connected, error, sending, sendMessage } = useLiveChat();
  const [draft, setDraft] = useState("");
  const bottomRef = useRef(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages.length]);

  const participantCount = useMemo(
    () => new Set(messages.slice(-100).map((message) => message.user_id)).size,
    [messages],
  );

  async function send(event) {
    event.preventDefault();
    const body = draft.trim();
    if (!body || sending) return;

    try {
      await sendMessage(body);
      setDraft("");
    } catch {
      // Shared provider renders the request error.
    }
  }

  return (
    <main className="page-wrap">
      <section className="mb-7 grid gap-6 lg:grid-cols-[1fr_auto] lg:items-end">
        <div>
          <p className="eyebrow">Komunikasi Tim</p>
          <h1 className="page-title mt-3">Chat Tim Langsung</h1>
          <div className="gold-rule mt-5" />
          <p className="page-subtitle mt-5">Satu ruang percakapan untuk seluruh pengguna Tropical yang sudah terautentikasi. Identitas dan peran berasal dari sesi login yang terverifikasi.</p>
        </div>
        <div className="card flex gap-6 px-5 py-4">
          <div><p className="section-label">Peserta</p><p className="mt-1 text-2xl font-black text-emerald-950">{participantCount}</p></div>
          <div><p className="section-label">Peran Anda</p><p className="mt-1 text-sm font-black text-emerald-950">{roleLabel(currentUser?.role)}</p></div>
        </div>
      </section>

      {error && <div className="toast-error mb-4">{error}</div>}

      <section className="card chat-card">
        <header className="chat-head">
          <div>
            <p className="section-label">Ruang Umum</p>
            <h2 className="mt-1 text-2xl font-black text-emerald-950">Ruang Operasional Restoran</h2>
          </div>
          <span className="live-indicator">{connected ? "AKTIF" : "MENGHUBUNGKAN"}</span>
        </header>

        <div className="chat-feed" aria-live="polite">
          {messages.length === 0 ? (
            <div className="chat-empty"><strong className="block text-lg text-emerald-950">Belum ada percakapan.</strong><span>Mulai pesan pertama untuk tim. Semua pengguna yang login akan menerima pesan baru secara real-time.</span></div>
          ) : messages.map((message) => {
            const own = isOwnChatMessage(message, currentUser);
            const accent = accentFor(message.user_name);
            return (
              <article key={message.id} className={`chat-message ${own ? "own" : ""}`}>
                <div className="chat-avatar">{initials(message.user_name)}</div>
                <div className="chat-bubble">
                  <div className="chat-user-line">
                    <span className="chat-user-name" style={{ color: own ? "#f7dda0" : accent }}>{own ? "Anda" : message.user_name}</span>
                    <span className="chat-role">{roleLabel(message.role)}</span>
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
          <input className="input" maxLength={1000} value={draft} onChange={(e) => setDraft(e.target.value)} placeholder={`Tulis pesan sebagai ${currentUser?.name || "pengguna"}...`} />
          <button className="btn px-6" disabled={sending || !draft.trim()}>{sending ? "Mengirim..." : "Kirim Pesan"}</button>
        </form>
      </section>
    </main>
  );
}
