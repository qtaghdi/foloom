import { listDOCXRecovery, recoveryEntrySchema, recoveryRoute, removeDOCXRecovery, type RecoveryEntry } from '../transport/docx-recovery';

type Mode = 'load' | 'enable' | 'update' | 'delete';
export type RecoveryStatus = { entry?: RecoveryEntry; busy?: boolean; error?: boolean };

// Recovery writes are independent of source Save and serialized per editor.
// An unconfirmed checkpoint cannot replace the last confirmed receipt in UI.
export class DraftRecovery {
  private entry?: RecoveryEntry;
  private pending?: Promise<void>;
  private timer?: ReturnType<typeof setTimeout>;
  private lastBody?: string;
  private disposed = false;
  private loaded = false;

  constructor(
    private id: string,
    private payload: () => string | undefined,
    private changed: (status: RecoveryStatus) => void,
  ) {}

  async load(): Promise<void> {
    if (this.loaded || this.disposed || this.pending) return;
    const baseline = this.payload();
    await this.run('load', async () => {
      this.entry = (await listDOCXRecovery()).find(entry => entry.id === this.id);
      this.lastBody = baseline;
      this.loaded = true;
    });
  }

  schedule(): void {
    this.cancelTimer();
    if (!this.entry || this.disposed) return;
    this.timer = setTimeout(() => { void this.flush(); }, 650);
  }

  private cancelTimer(): void {
    if (this.timer !== undefined) clearTimeout(this.timer);
    this.timer = undefined;
  }

  async flush(): Promise<void> {
    this.cancelTimer();
    await this.pending;
    if (!this.entry || this.disposed) return;
    const body = this.payload();
    if (body === undefined || body === this.lastBody) return;
    await this.checkpoint('update', body);
  }

  async enable(): Promise<void> {
    if (this.disposed || this.pending) return;
    const body = this.payload();
    if (body !== undefined) await this.checkpoint('enable', body);
  }

  async remove(): Promise<void> {
    this.cancelTimer();
    if (this.disposed || this.pending || !this.entry) return;
    await this.run('delete', async () => {
      if (!(await removeDOCXRecovery(this.id)).ok) throw new Error('DOCX recovery removal failed');
      this.entry = undefined;
      this.lastBody = undefined;
    });
  }

  private checkpoint(mode: 'enable' | 'update', body: string): Promise<void> {
    return this.run(mode, async () => {
      const response = await fetch(recoveryRoute, {
        method: mode === 'enable' ? 'POST' : 'PUT',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body, signal: AbortSignal.timeout(4000),
      });
      if (response.status === 404 && mode === 'update') {
        this.entry = undefined;
        this.lastBody = undefined;
      }
      if (!response.ok) throw new Error('DOCX recovery checkpoint failed');
      const entry = recoveryEntrySchema.parse(await response.json());
      if (entry.id !== this.id) throw new Error('DOCX recovery identity mismatch');
      this.entry = entry;
      this.lastBody = body;
    });
  }

  private run(mode: Mode, action: () => Promise<void>): Promise<void> {
    if (this.pending) return this.pending;
    // Editing stays available during background checkpoints.
    if (mode !== 'update') this.changed({ entry: this.entry, busy: true });
    const run = action().then(
      () => { if (!this.disposed) this.changed({ entry: this.entry }); },
      () => { if (!this.disposed) this.changed({ entry: this.entry, error: true }); },
    ).finally(() => { this.pending = undefined; });
    this.pending = run;
    return run;
  }

  dispose(): void {
    this.cancelTimer();
    // The browser limits keepalive uploads. Never send an older checkpoint
    // concurrently with a newer one that has already reached the server.
    const body = this.entry && !this.pending ? this.payload() : undefined;
    if (body && body !== this.lastBody && new TextEncoder().encode(body).length <= 60 * 1024) {
      void fetch(recoveryRoute, {
        method: 'PUT', credentials: 'same-origin', keepalive: true,
        headers: { 'Content-Type': 'application/json' }, body,
      }).catch(() => {});
    }
    this.disposed = true;
  }
}
