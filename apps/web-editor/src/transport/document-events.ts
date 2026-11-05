import { z } from 'zod';

const visibilitySchema = z.object({ type: z.literal('workspace:visibility'), active: z.boolean() });

// A local HTTP/1 browser has a small per-origin connection pool. Keep only the
// selected editor's SSE connected and replay from the last received event when
// returning to a tab. The controller still owns and closes its subscription.
export function documentEvents(path: string): Pick<EventSource, 'addEventListener' | 'onerror' | 'close'> {
  if (typeof window === 'undefined' || window.parent === window || !new URLSearchParams(window.location.search).has('document')) return new EventSource(path);
  let source: EventSource | undefined;
  let active = true;
  let closed = false;
  let after: string | undefined;
  let onerror: EventSource['onerror'] = null;
  const listeners: Array<{ type: string; listener: EventListenerOrEventListenerObject }> = [];
  const connect = () => {
    if (closed || !active || source) return;
    const url = new URL(path, window.location.origin);
    if (after) url.searchParams.set('after', after);
    const connection = new EventSource(`${url.pathname}${url.search}`);
    source = connection;
    for (const { type, listener } of listeners) bind(connection, type, listener);
    connection.onerror = event => { if (source === connection) onerror?.call(connection, event); };
  };
  const bind = (connection: EventSource, type: string, listener: EventListenerOrEventListenerObject) => {
    connection.addEventListener(type, event => {
      if (source !== connection || closed) return;
      const id = (event as MessageEvent).lastEventId;
      if (/^[0-9]+$/.test(id)) after = id;
      if (typeof listener === 'function') listener.call(connection, event);
      else listener.handleEvent(event);
    });
  };
  const visibility = (event: MessageEvent) => {
    if (event.origin !== window.location.origin || event.source !== window.parent) return;
    const parsed = visibilitySchema.safeParse(event.data);
    if (!parsed.success) return;
    active = parsed.data.active;
    if (active) connect();
    else { source?.close(); source = undefined; }
  };
  window.addEventListener('message', visibility);
  connect();
  return {
    addEventListener(type: string, listener: EventListenerOrEventListenerObject) { listeners.push({ type, listener }); if (source) bind(source, type, listener); },
    get onerror() { return onerror; },
    set onerror(handler) { onerror = handler; },
    close() { closed = true; source?.close(); source = undefined; window.removeEventListener('message', visibility); },
  } as Pick<EventSource, 'addEventListener' | 'onerror' | 'close'>;
}
