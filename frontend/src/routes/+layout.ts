
import type { LayoutLoad } from './$types';
import { redirect } from '@sveltejs/kit';

export const load: LayoutLoad = async ({ fetch, url, data }) => {
  const path = url.pathname;

  const publicPrefixes = ['/login', '/register', '/forgot-password', '/reset-password', '/verify-email'];

  if (publicPrefixes.some((prefix) => path.startsWith(prefix))) {
    return { ...data };
  }

  const r = await fetch('/api/me');
  if (!r.ok) {
    throw redirect(307, '/login');
  }

  return { ...data };
};
