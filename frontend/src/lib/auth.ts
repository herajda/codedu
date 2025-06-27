import { writable } from 'svelte/store';
import { apiFetch } from '$lib/api';
import { browser } from '$app/environment';

export type User = { id: number; role: string } | null;

function createAuth() {
  // Always start with null on both server and client…
  const { subscribe, set } = writable<User>(null);

  /** Called from Login.svelte after successful auth */
  function login(id: number, role: string) {
    const user = { id, role };
    if (browser) {
      localStorage.setItem('user', JSON.stringify(user));
    }
    set(user);
  }

  /** Log out everywhere */
  function logout() {
    if (browser) {
      localStorage.removeItem('user');
    }
    set(null);
  }

  /** Run once at start-up: restore role/id when only a token is stored */
  async function init() {
    if (!browser) return;            // don’t do anything on server

    // read from localStorage
    const raw = localStorage.getItem('user');
    const cur: User = raw ? JSON.parse(raw) : null;
    if (!cur) return;

    const r = await apiFetch('/api/me');
    if (r.ok) {
      const me = await r.json();
      login(me.id, me.role);
    } else {
      logout();
    }
  }

  return { subscribe, login, logout, init };
}

export const auth = createAuth();
