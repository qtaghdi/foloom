import { For, Show, createSignal } from 'solid-js';
import { locale, t } from '../i18n';
import { listDOCXRecovery, removeDOCXRecovery, type RecoveryEntry } from '../transport/docx-recovery';
import { buttonClass, quietButtonClass } from '../ui/editor-chrome';

export function RecoveryPanel(props: { full: boolean; onOpen: (id: string) => Promise<boolean> }) {
  const [entries, setEntries] = createSignal<RecoveryEntry[]>([]);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal(false);
  const [confirm, setConfirm] = createSignal('');
  const load = async () => {
    if (busy()) return;
    setBusy(true); setError(false);
    try { setEntries(await listDOCXRecovery()); }
    catch { setError(true); }
    finally { setBusy(false); }
  };
  const remove = async (id: string) => {
    setBusy(true); setError(false); setConfirm('');
    try {
      if (!(await removeDOCXRecovery(id)).ok) throw new Error('DOCX recovery removal failed');
      setEntries(items => items.filter(item => item.id !== id));
    } catch { setError(true); }
    finally { setBusy(false); }
  };
  return <details onToggle={event => { if (event.currentTarget.open) void load(); }}>
    <summary class={quietButtonClass}>{t('docxRecoveryCopies')}</summary>
    <div class="absolute right-4 top-full z-50 mt-2 max-h-[65vh] w-[min(30rem,calc(100vw-2rem))] overflow-auto rounded-lg border border-slate-200 bg-white p-3 text-xs shadow-lg">
      <p class="leading-5">{t('docxRecoveryListHint')}</p>
      <Show when={error()}><p role="alert" class="py-2 text-red-700">{t('docxRecoveryFailed')}</p></Show>
      <button type="button" class={quietButtonClass} disabled={busy()} onClick={() => void load()}>{t('docxRecoveryRefresh')}</button>
      <Show when={entries().length || busy() || error()} fallback={<p>{t('docxRecoveryEmpty')}</p>}>
        <ul class="space-y-2"><For each={entries()}>{entry => <li class="rounded border border-slate-200 p-2">
          <p class="break-all font-medium">{entry.name}</p>
          <p>{new Intl.DateTimeFormat(locale(), { dateStyle: 'short', timeStyle: 'short' }).format(new Date(entry.updatedAt))} · {t(entry.hasDraft ? 'docxRecoveryDraft' : 'docxRecoveryCommitted')}</p>
          <div class="flex flex-wrap gap-1">
            <button type="button" class={buttonClass} disabled={busy() || props.full} onClick={async () => { setBusy(true); try { if (await props.onOpen(entry.id)) setConfirm(''); } finally { setBusy(false); } }}>{t('docxRecoveryOpen')}</button>
            <Show when={confirm() === entry.id} fallback={<button type="button" class={quietButtonClass} disabled={busy()} onClick={() => setConfirm(entry.id)}>{t('docxRecoveryDelete')}</button>}>
              <span class="self-center">{t('docxRecoveryConfirmDelete')}</span>
              <button type="button" class={buttonClass} disabled={busy()} onClick={() => void remove(entry.id)}>{t('docxRecoveryDelete')}</button>
              <button type="button" class={quietButtonClass} onClick={() => setConfirm('')}>{t('cancel')}</button>
            </Show>
          </div>
        </li>}</For></ul>
      </Show>
    </div>
  </details>;
}
