import { writable } from 'svelte/store';

/** controls whether the sidebar is open on small screens */
export const sidebarOpen = writable(false);
