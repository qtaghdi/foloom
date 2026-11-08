import { z } from 'zod';

export const recoveryRoute = '/api/documents/docx-recovery';
const recoveryID = z.string().regex(/^[a-f0-9]{32}$/);
export const recoveryEntrySchema = z.object({
  id: recoveryID, name: z.string().min(1).max(240), updatedAt: z.iso.datetime({ offset: true }), hasDraft: z.boolean(), damaged: z.boolean().optional(),
});
export type RecoveryEntry = z.infer<typeof recoveryEntrySchema>;

export async function listDOCXRecovery(): Promise<RecoveryEntry[]> {
  const response = await fetch(recoveryRoute, { credentials: 'same-origin', cache: 'no-store', signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error('DOCX recovery list failed');
  return z.array(recoveryEntrySchema).max(3).parse(await response.json());
}

export function removeDOCXRecovery(id: string): Promise<Response> {
  return fetch(`${recoveryRoute}/${recoveryID.parse(id)}`, { method: 'DELETE', credentials: 'same-origin', signal: AbortSignal.timeout(5000) });
}
