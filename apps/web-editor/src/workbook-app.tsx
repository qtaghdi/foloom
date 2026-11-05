import { onCleanup, onMount } from 'solid-js';
import { WorkbookController, type SpreadsheetEngineFactory } from './app/workbook-controller';
import type { WorkbookClient } from './transport/workbook-client';
import { AppView } from './ui/app-view';
import { WorkbookShell } from './ui/workbook-shell';

export function WorkbookApp(props: { client: WorkbookClient; createEngine: SpreadsheetEngineFactory }) {
  const view = new AppView();
  const controller = new WorkbookController(props.client, view, props.createEngine);
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (controller.shouldWarnBeforeUnload()) { event.preventDefault(); event.returnValue = ''; }
  };
  let disposed = false;
  const dispose = () => {
    if (disposed) return;
    disposed = true;
    window.removeEventListener('beforeunload', beforeUnload);
    window.removeEventListener('pagehide', dispose);
    controller.dispose();
  };
  onMount(() => {
    window.addEventListener('beforeunload', beforeUnload);
    window.addEventListener('pagehide', dispose, { once: true });
    void controller.start();
  });
  onCleanup(dispose);
  return <WorkbookShell view={view} search={(query) => controller.search(query)}/>;
}
