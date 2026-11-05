import { createSignal } from 'solid-js';
import { createActivityModel } from '../ui/activity';
import type { DocumentView } from './controller';
import type { Snapshot } from './contracts';

export interface DocumentViewState {
  snapshot?: Snapshot; message: string; status: 'pending' | 'committed' | 'failed' | 'conflicted';
  error: string; errorDetails?: string; validation: string; canSave: boolean; saving: boolean; loading: boolean;
}
export function createDocumentView() {
  const activity = createActivityModel();
  const [state, setState] = createSignal<DocumentViewState>({ message: '연결 중', status: 'pending', error: '', validation: '', canSave: false, saving: false, loading: true });
  const patch = (update: Partial<DocumentViewState>) => setState((previous) => ({ ...previous, ...update }));
  const view: DocumentView = {
    activity: activity.record,
    validation: (validation) => patch({ validation }),
    render: (snapshot) => patch({ snapshot }),
    status: (message, status) => patch({ message, status, saving: status === 'pending' && message === '저장 중', loading: status === 'pending' && message === '문서를 불러오는 중' }),
    error: (error, errorDetails) => patch({ error, errorDetails }),
    canSave: (canSave) => patch({ canSave }),
  };
  return { state, view, activity };
}
