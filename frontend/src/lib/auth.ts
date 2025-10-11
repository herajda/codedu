
import { writable } from "svelte/store";
import { apiFetch } from "$lib/api";
import { browser } from "$app/environment";
import { onlineUsers } from "./stores/onlineUsers";

export type User = {
  id: string;
  role: string;
  name?: string | null;
  avatar?: string | null;
  email?: string | null;
  email_verified?: boolean | null;
  bk_uid?: string | null;
  theme?: "light" | "dark" | null;
  email_notifications?: boolean | null;
  email_message_digest?: boolean | null;
} | null;

function createAuth() {
  // Always start with null on both server and client…
  const { subscribe, set } = writable<User>(null);

  /** Called from Login.svelte after successful auth */
  function login(
    id: number,
    role: string,
    name?: string | null,
    avatar?: string | null,
    bk_uid?: string | null,
    email?: string | null,
    emailVerified?: boolean | null,
    theme?: "light" | "dark" | null,
    emailNotifications?: boolean | null,
    emailMessageDigest?: boolean | null,
  ) {
    set({
      id,
      role,
      name,
      avatar,
      bk_uid,
      email,
      email_verified: emailVerified ?? null,
      theme,
      email_notifications: emailNotifications ?? true,
      email_message_digest: emailMessageDigest ?? true,
    });
    // Mark user as online
    onlineUsers.markOnline();
  }

  /** Log out everywhere */
  async function logout() {
    // Mark user as offline BEFORE clearing auth so the request is authorized
    await onlineUsers.markOffline();
    if (browser) {
      try {
        await apiFetch("/api/logout", { method: "POST" });
      } catch {
        // ignore errors
      }
      try {
        localStorage.removeItem("cg-msg-key");
      } catch {}
    }
    set(null);
  }

  /** Run once at start-up: restore role/id when only a token is stored */
  async function init() {
    if (!browser) return; // don’t do anything on server

    try {
      const r = await apiFetch("/api/me");
      if (r.ok) {
        const me = await r.json();
        login(
          me.id,
          me.role,
          me.name ?? null,
          me.avatar ?? null,
          me.bk_uid ?? null,
          me.email ?? null,
          me.email_verified ?? null,
          me.theme ?? null,
          me.email_notifications ?? true,
          me.email_message_digest ?? true,
        );
      } else if (r.status === 401) {
        set(null);
      } else {
        set(null);
      }
    } catch {
      set(null);
    }
  }

  return { subscribe, login, logout, init };
}

export const auth = createAuth();
