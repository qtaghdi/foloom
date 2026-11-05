import { afterEach, expect, it, vi } from 'vitest';
import { documentEvents } from '../src/transport/document-events';
afterEach(() => vi.unstubAllGlobals());
it('pauses hidden tab events and replays from the last received event without leaking listeners', () => {
  const parent = {};
  let visibility: (event: any) => void = () => {};
  const remove = vi.fn();
  vi.stubGlobal('window', { parent, location: { origin: 'http://localhost', search: '?document=first' },
    addEventListener: (_name: string, listener: any) => { visibility = listener; }, removeEventListener: remove });
  const sources: any[] = [];
  class Source {
    listeners: Record<string, (event: any) => void> = {};
    close = vi.fn(); onerror: any;
    constructor(readonly url: string) { sources.push(this); }
    addEventListener(type: string, listener: any) { this.listeners[type] = listener; }
  }
  vi.stubGlobal('EventSource', Source);
  const events = documentEvents('/api/events?after=0&document=first');
  const receive = vi.fn(); events.addEventListener('workbook', receive);
  sources[0].listeners.workbook({ lastEventId: '12', data: '{}' });
  const notify = (active: boolean, origin = 'http://localhost') => visibility({ origin, source: parent, data: { type: 'workspace:visibility', active } });
  notify(false, 'https://evil.example'); expect(sources[0].close).not.toHaveBeenCalled();
  notify(false); expect(sources[0].close).toHaveBeenCalledOnce();
  sources[0].listeners.workbook({ lastEventId: '13', data: '{}' }); expect(receive).toHaveBeenCalledOnce();
  notify(true); expect(sources[1].url).toContain('after=12'); expect(sources[1].url).toContain('document=first');
  sources[1].listeners.workbook({ lastEventId: '14', data: '{}' }); expect(receive).toHaveBeenCalledTimes(2);
  events.close(); expect(sources[1].close).toHaveBeenCalledOnce(); expect(remove).toHaveBeenCalledOnce();
});
