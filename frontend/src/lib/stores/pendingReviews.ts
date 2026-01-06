import { writable } from 'svelte/store';
import { apiJSON } from '$lib/api';

export const pendingReviewCount = writable<number>(0);

let loadingPromise: Promise<void> | null = null;

export async function loadPendingReviewCount(): Promise<void> {
    // Prevent concurrent loads
    if (loadingPromise) return loadingPromise;

    loadingPromise = (async () => {
        try {
            const data = await apiJSON('/api/pending-reviews/count');
            pendingReviewCount.set(data.count ?? 0);
        } catch {
            // Silently fail - user might not be a teacher
            pendingReviewCount.set(0);
        } finally {
            loadingPromise = null;
        }
    })();

    return loadingPromise;
}

export function setPendingReviewCount(n: number) {
    pendingReviewCount.set(n);
}

export function decrementPendingReviewCount() {
    pendingReviewCount.update((n) => Math.max(0, n - 1));
}
