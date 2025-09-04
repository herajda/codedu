import type { HandleFetch } from '@sveltejs/kit';

// Ensure SSR fetch('/api/...') talks to the backend container and forwards cookies
export const handleFetch: HandleFetch = async ({ event, request, fetch }) => {
  const url = new URL(request.url);

  // Only rewrite API calls; let everything else pass through
  if (url.pathname.startsWith('/api/')) {
    const base = (process.env.SSR_API_BASE ?? 'http://backend:8080').replace(/\/$/, '');
    const target = base + url.pathname + url.search;

    // Clone headers and ensure cookies from the incoming request are forwarded
    const headers = new Headers(request.headers);
    const cookie = event.request.headers.get('cookie');
    if (cookie) headers.set('cookie', cookie);

    const init: RequestInit = { method: request.method, headers };
    if (request.method !== 'GET' && request.method !== 'HEAD') {
      // @ts-expect-error - body is allowed for non-GET/HEAD and may be a ReadableStream
      init.body = request.body as any;
    }

    return fetch(target, init);
  }

  return fetch(request);
};

