import './styles.css';

import { WorkbookController } from './app/workbook-controller';
import { createWorkbookClient } from './transport/create-workbook-client';
import { AppView } from './ui/app-view';
import { UniverSpreadsheetEngine } from './univer-engine';

const view = new AppView(document);
const controller = new WorkbookController(
  createWorkbookClient(),
  view,
  (container) => new UniverSpreadsheetEngine(container),
);

void controller.start();
window.addEventListener('beforeunload', (event) => {
  if (controller.shouldWarnBeforeUnload()) {
    event.preventDefault();
    event.returnValue = '';
  }
});
window.addEventListener('pagehide', () => controller.dispose(), { once: true });
