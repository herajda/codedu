import type { PageLoad } from './$types';
import { redirect } from '@sveltejs/kit';

export const load: PageLoad = async ({ fetch }) => {
  const r = await fetch('/api/me');
  if (!r.ok) throw redirect(307, '/login');
  const me = await r.json();
  if (me.role !== 'admin') throw redirect(307, '/dashboard');
  return {};
};
