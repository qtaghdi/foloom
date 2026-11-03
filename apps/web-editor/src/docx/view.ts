import { createSignal } from 'solid-js';
import { createActivityModel } from '../ui/activity';
import type { DocumentView } from './controller';
import type { Snapshot } from './contracts';

export interface DocumentViewState {
  snapshot?: Snapshot; message: string; status: 'pending' | 'committed' | 'failed' | 'conflicted';
  error: string; canSave: boolean; saving: boolean;
}
export function createDocumentView() {
  const activity = createActivityModel();
  const [state, setState] = createSignal<DocumentViewState>({ message: '연결 중', status: 'pending', error: '', canSave: false, saving: false });
  const patch = (update: Partial<DocumentViewState>) => setState((previous) => ({ ...previous, ...update }));
  const view: DocumentView = {
    activity: activity.record,
    render: (snapshot) => patch({ snapshot }),
    status: (message, status) => patch({ message, status, saving: status === 'pending' && message === '저장 중' }),
    error: (error) => patch({ error }),
    canSave: (canSave) => patch({ canSave }),
  };
  return { state, view, activity };
}
