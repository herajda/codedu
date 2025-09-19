import type { LayoutLoad } from './$types';
import { redirect } from '@sveltejs/kit';

export const load: LayoutLoad = async ({ fetch, url }) => {
  const path = url.pathname;

  const publicPrefixes = ['/login', '/register', '/forgot-password', '/reset-password'];

  // Allow unauthenticated access to public auth routes
  if (publicPrefixes.some((prefix) => path.startsWith(prefix))) {
    return {};
  }

  const r = await fetch('/api/me');
  if (!r.ok) {
    throw redirect(307, '/login');
  }

  return {};
};

