export interface SSEOptions {
  /** milliseconds before attempting to reconnect */
  retry?: number;
  /** called with a message when the connection is lost */
  onError?: (msg: string) => void;
  /** called when a new connection opens */
  onOpen?: () => void;
}

export function createEventSource(
  url: string,
  setup: (es: EventSource) => void,
  opts: SSEOptions = {}
) {
  let es: EventSource | null = null;
  let closed = false;
  const retry = opts.retry ?? 5000;

  function connect() {
    // Include credentials so auth cookies are sent to protected SSE endpoints
    es = new EventSource(url, { withCredentials: true });
    setup(es!);
    es.onopen = () => {
      opts.onOpen?.();
    };
    es.onerror = () => {
      if (closed) return;
      opts.onError?.("Lost connection, retrying…");
      es?.close();
      setTimeout(connect, retry);
    };
  }

  connect();

  return {
    close() {
      closed = true;
      es?.close();
    }
  };
}
