import type { PageLoad } from './$types';
import { redirect } from '@sveltejs/kit';

export const load: PageLoad = async ({ fetch }) => {
  const r = await fetch('/api/me');
  if (r.ok) {
    throw redirect(307, '/dashboard');
  }
  throw redirect(307, '/login');
};
