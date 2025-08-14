import { writable } from 'svelte/store';

export const unreadMessages = writable<number>(0);

export function incrementUnreadMessages() {
  unreadMessages.update((n) => n + 1);
}

export function resetUnreadMessages() {
  unreadMessages.set(0);
}


