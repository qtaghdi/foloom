import { z } from 'zod';

export interface EditorState { message: string; dirty: boolean; busy: boolean }
const commandSchema = z.object({ type: z.literal('workspace:check'), request: z.string(), save: z.boolean().optional() });
const documentID = () => new URLSearchParams(window.location.search).get('document');
export function publishEditorState(state: EditorState): void {
  if (!documentID() || window.parent === window) return;
  window.parent.postMessage({ type: 'workspace:state', id: documentID(), ...state }, window.location.origin);
}
export function connectWorkspaceBridge(read: () => EditorState, save?: () => Promise<void>): () => void {
  const listener = async (event: MessageEvent) => {
    if (event.origin !== window.location.origin || event.source !== window.parent) return;
    const parsed = commandSchema.safeParse(event.data);
    if (!parsed.success) return;
    if (parsed.data.save && !read().busy) await save?.();
    const state = read();
    publishEditorState(state);
    window.parent.postMessage({ type: 'workspace:checked', id: documentID(), request: parsed.data.request, ...state }, window.location.origin);
  };
  window.addEventListener('message', listener);
  return () => window.removeEventListener('message', listener);
}
