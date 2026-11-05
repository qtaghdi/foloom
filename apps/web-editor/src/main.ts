import { initializeI18n } from './i18n';
import { WorkspaceApp } from './workspace/workspace-app';
import './styles.css';
import { render } from 'solid-js/web';
import { createComponent } from 'solid-js';
import { WorkbookApp } from './workbook-app';
import { createWorkbookClient } from './transport/create-workbook-client';
import { UniverSpreadsheetEngine } from './univer-engine';

initializeI18n();
const root = document.getElementById('root');
if (!root) throw new Error('Missing application root');
const embedded = new URLSearchParams(window.location.search).get('embedded') === '1';
render(() => !embedded && window.parent === window ? createComponent(WorkspaceApp, {}) : createComponent(WorkbookApp, {
  client: createWorkbookClient(),
  createEngine: (container) => new UniverSpreadsheetEngine(container),
}), root);
