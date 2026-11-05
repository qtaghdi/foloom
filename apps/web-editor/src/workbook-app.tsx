import { connectWorkspaceBridge, publishEditorState } from './ui/workspace-bridge';
import { createEffect, onCleanup, onMount } from 'solid-js';
import { WorkbookController, type SpreadsheetEngineFactory } from './app/workbook-controller';
import type { WorkbookClient } from './transport/workbook-client';
import { AppView } from './ui/app-view';
import { WorkbookShell } from './ui/workbook-shell';

export function WorkbookApp(props: { client: WorkbookClient; createEngine: SpreadsheetEngineFactory }) {
  const view = new AppView();
  const controller = new WorkbookController(props.client, view, props.createEngine);
  const keydown = (event: KeyboardEvent) => {
    if (!event.isComposing && (event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 's') {
      event.preventDefault(); event.stopPropagation(); void controller.save();
    }
  };
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (controller.shouldWarnBeforeUnload()) { event.preventDefault(); event.returnValue = ''; }
  };
  const editorState = () => ({ message: view.state().message, busy: view.state().status === 'pending' || Boolean(view.state().fileBusy), dirty: controller.shouldWarnBeforeUnload() });
  createEffect(() => { view.state(); queueMicrotask(() => publishEditorState(editorState())); });
  let disconnectWorkspace = () => {};
  let disposed = false;
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    window.removeEventListener('beforeunload', beforeUnload);
    window.removeEventListener('keydown', keydown, true);
    window.removeEventListener('pagehide', dispose);
    disconnectWorkspace();
    controller.dispose();
  };
  onMount(() => {
    disconnectWorkspace = connectWorkspaceBridge(editorState);
    window.addEventListener('beforeunload', beforeUnload);
    window.addEventListener('keydown', keydown, true);
    window.addEventListener('pagehide', dispose, { once: true });
    void controller.start();
  });
  onCleanup(dispose);
  return <WorkbookShell view={view} save={() => controller.save()} download={props.client.download ? () => controller.download() : undefined} search={(query) => controller.search(query)}/>;
}
