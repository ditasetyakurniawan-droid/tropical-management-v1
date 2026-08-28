import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const navState = vi.hoisted(() => ({ pathname: "/" }));
const chatApi = vi.hoisted(() => ({
  api: vi.fn(),
  sessionUser: vi.fn(),
  token: vi.fn(),
}));

vi.mock("next/navigation", () => ({ usePathname: () => navState.pathname }));
vi.mock("../lib/api", () => ({
  API_URL: "http://api.local",
  api: chatApi.api,
  sessionUser: chatApi.sessionUser,
  token: chatApi.token,
}));

import LiveChatProvider, {
  connectChatStream,
  consumeStream,
  isOwnChatMessage,
  loadHistory,
  openStream,
  parseSSEEvent,
  useLiveChat,
} from "../components/LiveChatProvider";

function Probe() {
  const chat = useLiveChat();
  return (
    <div>
      <span data-testid="message-count">{chat.messages.length}</span>
      <span data-testid="current-user">{chat.currentUser?.name || "none"}</span>
      <span data-testid="chat-error">{chat.error || "none"}</span>
      <button type="button" onClick={() => chat.sendMessage(" hello team ")}>send</button>
    </div>
  );
}

beforeEach(() => {
  navState.pathname = "/";
  chatApi.api.mockReset();
  chatApi.sessionUser.mockReset();
  chatApi.token.mockReset();
  globalThis.fetch = vi.fn(() => new Promise(() => {}));
});

describe("LiveChatProvider helpers", () => {
  it("identifies messages owned by the current user", () => {
    expect(isOwnChatMessage({ user_id: 7 }, { id: 7 })).toBe(true);
    expect(isOwnChatMessage({ user_id: "8" }, { sub: 8 })).toBe(true);
    expect(isOwnChatMessage({ user_id: 9 }, { id: 7 })).toBe(false);
    expect(isOwnChatMessage({ user_id: 7 }, null)).toBe(false);
  });



  it("loads history defensively", async () => {
    const appendMessage = vi.fn();
    const onError = vi.fn();
    chatApi.api.mockResolvedValueOnce([{ id: 1 }]);
    await loadHistory({ appendMessage, onError, isActive: () => true });
    expect(appendMessage).toHaveBeenCalledWith([{ id: 1 }]);
    expect(onError).not.toHaveBeenCalled();

    chatApi.api.mockRejectedValueOnce(new Error("history down"));
    await loadHistory({ appendMessage, onError, isActive: () => true });
    expect(onError).toHaveBeenCalledWith("history down");

    chatApi.api.mockResolvedValueOnce([{ id: 2 }]);
    await loadHistory({ appendMessage, onError, isActive: () => false });
    expect(appendMessage).not.toHaveBeenCalledWith([{ id: 2 }]);
  });

  it("opens the authenticated SSE stream and rejects bad upstream responses", async () => {
    chatApi.token.mockReturnValue("jwt-stream");
    const reader = { read: vi.fn(), cancel: vi.fn() };
    globalThis.fetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      body: { getReader: () => reader },
    });
    await expect(openStream()).resolves.toBe(reader);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "http://api.local/api/chat/stream",
      expect.objectContaining({ headers: { Authorization: "Bearer jwt-stream" } }),
    );

    globalThis.fetch.mockResolvedValueOnce({ ok: false, status: 503, body: null });
    await expect(openStream()).rejects.toThrow("HTTP 503");
  });

  it("consumes complete and buffered SSE events", async () => {
    const encoder = new TextEncoder();
    const reader = {
      cancel: vi.fn(),
      read: vi.fn()
        .mockResolvedValueOnce({ done: false, value: encoder.encode('data: {"id":1}\n\ndata: {"id":2}') })
        .mockResolvedValueOnce({ done: true }),
    };
    const appendMessage = vi.fn();
    await consumeStream({
      reader,
      appendMessage,
      isActive: () => true,
      isStopped: () => false,
    });
    expect(appendMessage).toHaveBeenNthCalledWith(1, [{ id: 1 }]);
    expect(appendMessage).toHaveBeenNthCalledWith(2, [{ id: 2 }]);
  });

  it("cancels stream consumption after shutdown", async () => {
    const reader = { cancel: vi.fn(), read: vi.fn() };
    await consumeStream({
      reader,
      appendMessage: vi.fn(),
      isActive: () => true,
      isStopped: () => true,
    });
    expect(reader.cancel).toHaveBeenCalledTimes(1);
    expect(reader.read).not.toHaveBeenCalled();
  });

  it("marks successful stream lifecycle transitions", async () => {
    const reader = { cancel: vi.fn(), read: vi.fn().mockResolvedValue({ done: true }) };
    globalThis.fetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      body: { getReader: () => reader },
    });
    const onConnected = vi.fn();
    const onDisconnected = vi.fn();
    const onError = vi.fn();
    const timer = vi.spyOn(window, "setTimeout").mockImplementation(() => 1);

    await connectChatStream({
      appendMessage: vi.fn(),
      onConnected,
      onDisconnected,
      onError,
      isActive: () => true,
      isStopped: () => false,
    });

    expect(onConnected).toHaveBeenCalledTimes(1);
    expect(onDisconnected).toHaveBeenCalledTimes(1);
    expect(onError).not.toHaveBeenCalled();
    expect(timer).toHaveBeenCalled();
    timer.mockRestore();
  });

  it("parses valid SSE data and rejects malformed events", () => {
    expect(parseSSEEvent('data: {"id":1,"body":"hello"}\n\n')).toEqual({ id: 1, body: "hello" });
    expect(parseSSEEvent("event: ping\n\n")).toBeNull();
    expect(parseSSEEvent("data: not-json\n\n")).toBeNull();
    expect(parseSSEEvent("")).toBeNull();
  });
});

describe("LiveChatProvider", () => {
  it("stays idle on the login route", () => {
    navState.pathname = "/login";
    chatApi.token.mockReturnValue("");

    render(<LiveChatProvider><Probe /></LiveChatProvider>);

    expect(screen.getByTestId("message-count").textContent).toBe("0");
    expect(screen.getByTestId("current-user").textContent).toBe("none");
    expect(chatApi.api).not.toHaveBeenCalled();
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("loads chat history for an authenticated user", async () => {
    chatApi.token.mockReturnValue("jwt-chat");
    chatApi.sessionUser.mockReturnValue({ id: 7, name: "Tropical Admin", role: "admin" });
    chatApi.api.mockResolvedValueOnce([
      { id: 1, user_id: 7, body: "hello" },
      { id: 2, user_id: 8, body: "world" },
    ]);

    render(<LiveChatProvider><Probe /></LiveChatProvider>);

    await waitFor(() => expect(screen.getByTestId("message-count").textContent).toBe("2"));
    expect(screen.getByTestId("current-user").textContent).toBe("Tropical Admin");
    expect(chatApi.api).toHaveBeenCalledWith("/api/chat/messages?limit=100");
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "http://api.local/api/chat/stream",
      expect.objectContaining({ headers: { Authorization: "Bearer jwt-chat" }, cache: "no-store" }),
    );
  });

  it("surfaces history errors without breaking the provider", async () => {
    chatApi.token.mockReturnValue("jwt-chat");
    chatApi.sessionUser.mockReturnValue({ id: 7, name: "Admin" });
    chatApi.api.mockRejectedValueOnce(new Error("history unavailable"));

    render(<LiveChatProvider><Probe /></LiveChatProvider>);

    await waitFor(() => expect(screen.getByTestId("chat-error").textContent).toBe("history unavailable"));
  });

  it("posts a trimmed message and appends the saved result", async () => {
    chatApi.token.mockReturnValue("jwt-chat");
    chatApi.sessionUser.mockReturnValue({ id: 7, name: "Admin" });
    chatApi.api
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce({ id: 3, user_id: 7, body: "hello team" });

    render(<LiveChatProvider><Probe /></LiveChatProvider>);
    await waitFor(() => expect(chatApi.api).toHaveBeenCalledWith("/api/chat/messages?limit=100"));

    fireEvent.click(screen.getByRole("button", { name: "send" }));

    await waitFor(() => expect(screen.getByTestId("message-count").textContent).toBe("1"));
    expect(chatApi.api).toHaveBeenCalledWith("/api/chat/messages", {
      method: "POST",
      body: JSON.stringify({ body: "hello team" }),
    });
  });
});
