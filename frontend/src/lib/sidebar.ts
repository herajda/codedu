import { writable } from 'svelte/store';

/** controls whether the sidebar is open on small screens */
export const sidebarOpen = writable(false);

/** controls whether the sidebar is collapsed on large screens */
export const sidebarCollapsed = writable(false);
