import { writable } from 'svelte/store';

/** Number of unread messages not yet viewed in the Messages page */
export const unreadMessages = writable(0);

export const incrementUnread = () => unreadMessages.update(n => n + 1);
export const resetUnread = () => unreadMessages.set(0);
