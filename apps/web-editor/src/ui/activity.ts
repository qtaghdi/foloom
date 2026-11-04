import { createSignal } from 'solid-js';

export interface ActivityEntry {
  id: string; actor: 'ai' | 'human' | 'system'; state: 'pending' | 'committed' | 'failed' | 'uncertain';
  title: string; location?: string; revision?: number; observedAt?: number;
}

// This bounded UI log describes this connection only. It is never a disk history
// or a receipt for a write whose server result has not been confirmed.
export function createActivityModel() {
  const [entries, setEntries] = createSignal<ActivityEntry[]>([]);
  const [open, setOpen] = createSignal(typeof window !== 'undefined' && window.matchMedia('(min-width: 1100px)').matches);
  const record = (entry: ActivityEntry) => setEntries((previous) => {
    const existing = previous.find((item) => item.id === entry.id);
    return [{ ...entry, observedAt: existing?.observedAt ?? entry.observedAt ?? Date.now() }, ...previous.filter((item) => item.id !== entry.id)].slice(0, 60);
  });
  const toggle = () => {
    const next = !open(); setOpen(next);
    queueMicrotask(() => {
      if (next) document.getElementById('document-activity-heading')?.focus();
      else document.querySelector<HTMLButtonElement>('[data-activity-toggle]')?.focus();
    });
  };
  const markUncertain = () => setEntries((previous) => previous.map((entry) => entry.state === 'pending' ? { ...entry, state: 'uncertain' } : entry));
  return { entries, open, record, toggle, markUncertain };
}
