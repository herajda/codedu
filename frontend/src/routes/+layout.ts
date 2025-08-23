import type { LayoutLoad } from './$types';
import { redirect } from '@sveltejs/kit';

export const load: LayoutLoad = async ({ fetch, url }) => {
  const path = url.pathname;

  // Allow unauthenticated access only to login and register routes
  if (path.startsWith('/login') || path.startsWith('/register')) {
    return {};
  }

  const r = await fetch('/api/me');
  if (!r.ok) {
    throw redirect(307, '/login');
  }

  return {};
};


