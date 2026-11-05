import { z } from 'zod';
import { errorResponseSchema } from './schemas';

export interface DocumentCheckpoint { id: string; revision: number; name: string }
export interface DownloadedFile { blob: Blob; name: string }
export const savedReceiptSchema = z.object({ id: z.string(), revision: z.number().int().nonnegative(), name: z.string() });
export class DocumentFileError extends Error {
  constructor(message: string, readonly status: number) { super(message); }
}
function matches(receipt: DocumentCheckpoint, checkpoint: DocumentCheckpoint): void {
  if (receipt.id !== checkpoint.id || receipt.revision !== checkpoint.revision || receipt.name !== checkpoint.name) throw new DocumentFileError('document revision conflict during file preparation', 409);
}
async function requireOK(response: Response): Promise<void> {
  if (response.ok) return;
  let message = 'document file request failed';
  try { const error = errorResponseSchema.safeParse(await response.json()); if (error.success) message = error.data.error; } catch { /* Preserve a useful failure if the response is not JSON. */ }
  throw new DocumentFileError(message, response.status);
}
export async function confirmSavedDocument(checkpoint: DocumentCheckpoint): Promise<void> {
  const query = new URLSearchParams({ document: checkpoint.id });
  const response = await fetch(`/api/save?${query}`, { method: 'POST', credentials: 'same-origin', cache: 'no-store', redirect: 'error', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ sessionId: checkpoint.id, baseRevision: checkpoint.revision }) });
  await requireOK(response);
  matches(savedReceiptSchema.parse(await response.json()), checkpoint);
}
export async function downloadSavedDocument(checkpoint: DocumentCheckpoint, format: 'xlsx' | 'docx'): Promise<DownloadedFile> {
  const query = new URLSearchParams({ document: checkpoint.id, session: checkpoint.id, revision: String(checkpoint.revision) });
  const response = await fetch(`/api/download?${query}`, { credentials: 'same-origin', cache: 'no-store', redirect: 'error' });
  await requireOK(response);
  const mimeType = format === 'xlsx' ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' : 'application/vnd.openxmlformats-officedocument.wordprocessingml.document';
  if (response.headers.get('X-Document-ID') !== checkpoint.id || response.headers.get('X-Document-Revision') !== String(checkpoint.revision)) throw new DocumentFileError('document revision conflict during download', 409);
  if (response.headers.get('Content-Type')?.split(';')[0] !== mimeType) throw new DocumentFileError('unexpected document download content type', 502);
  const blob = await response.blob();
  if (!blob.size || blob.size > (format === 'xlsx' ? 100 : 32) * 1024 * 1024) throw new DocumentFileError('document download exceeds file size limits', 413);
  return { blob, name: checkpoint.name };
}
// Hand the checked bytes to the browser. A started download does not imply that
// the user has completed the browser's save dialog.
export function startBrowserDownload(file: DownloadedFile): void {
  const url = URL.createObjectURL(file.blob);
  try {
    // Start workspace downloads from its top-level document so embedded browser
    // download handlers receive the action from the visible workspace surface.
    const owner = window.frameElement?.hasAttribute('data-document-editor') ? window.parent.document : document;
    const link = owner.createElement('a');
    link.href = url; link.download = file.name;
    owner.body.append(link); link.click(); link.remove();
  } finally { window.setTimeout(() => URL.revokeObjectURL(url), 60_000); }
}
