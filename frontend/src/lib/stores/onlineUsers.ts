import { writable } from 'svelte/store';
import { apiFetch } from '$lib/api';

export type OnlineUser = {
	id: string;
	name: string;
	avatar: string;
	email: string;
};

function createOnlineUsersStore() {
	const { subscribe, set, update } = writable<OnlineUser[]>([]);

	async function loadOnlineUsers() {
		try {
			const response = await apiFetch('/api/online-users');
			if (response.ok) {
				const users = await response.json();
				set(users);
			}
		} catch (error) {
			console.error('Failed to load online users:', error);
		}
	}

	async function markOnline() {
		try {
			await apiFetch('/api/presence', { method: 'POST' });
		} catch (error) {
			console.error('Failed to mark online:', error);
		}
	}

	async function updateLastSeen() {
		try {
			await apiFetch('/api/presence', { method: 'PUT' });
		} catch (error) {
			console.error('Failed to update last seen:', error);
		}
	}

	async function markOffline() {
		try {
			await apiFetch('/api/presence', { method: 'DELETE' });
		} catch (error) {
			console.error('Failed to mark offline:', error);
		}
	}

	function isUserOnline(userId: number): boolean {
		let online = false;
		update(users => {
			online = users.some(user => user.id === userId);
			return users;
		});
		return online;
	}

	return {
		subscribe,
		loadOnlineUsers,
		markOnline,
		updateLastSeen,
		markOffline,
		isUserOnline
	};
}

export const onlineUsers = createOnlineUsersStore();
