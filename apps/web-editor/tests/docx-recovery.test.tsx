import { afterEach, expect, it, vi } from 'vitest';
import { DraftRecovery } from '../src/docx/recovery';

afterEach(() => vi.unstubAllGlobals());

it('only checkpoints with consent, retains the last receipt on failure and stops after deletion', async () => {
  const id = '1234567890abcdef1234567890abcdef';
  const entry = { id, name: 'test.docx', updatedAt: '2026-11-07T12:00:00Z', hasDraft: true };
  const fetcher = vi.fn<typeof fetch>()
    .mockResolvedValueOnce(Response.json([]))
    .mockResolvedValueOnce(Response.json(entry))
    .mockRejectedValueOnce(new Error('disconnected'))
    .mockResolvedValueOnce(Response.json({ ...entry, updatedAt: '2026-11-07T12:01:00Z' }))
    .mockResolvedValueOnce(new Response(null, { status: 204 }));
  vi.stubGlobal('fetch', fetcher);
  let body = 'first draft';
  const view = vi.fn();
  const recovery = new DraftRecovery(id, () => body, view);
  await recovery.load();
  await recovery.flush();
  expect(fetcher).toHaveBeenCalledTimes(1);
  await recovery.enable();
  expect(fetcher.mock.calls[1]?.[1]).toMatchObject({ method: 'POST', body: 'first draft' });
  body = 'second draft';
  await recovery.flush();
  expect(view).toHaveBeenLastCalledWith({ entry, error: true });
  await recovery.flush();
  expect(fetcher.mock.calls[3]?.[1]).toMatchObject({ method: 'PUT', body: 'second draft' });
  await recovery.remove();
  body = 'third draft';
  await recovery.flush();
  recovery.dispose();
  expect(fetcher.mock.calls[4]?.[1]).toMatchObject({ method: 'DELETE' });
  expect(fetcher).toHaveBeenCalledTimes(5);
});
