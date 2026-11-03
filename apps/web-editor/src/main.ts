import './styles.css';
import { render } from 'solid-js/web';
import { createComponent } from 'solid-js';
import { WorkbookApp } from './workbook-app';
import { createWorkbookClient } from './transport/create-workbook-client';
import { UniverSpreadsheetEngine } from './univer-engine';

const root = document.getElementById('root');
if (!root) throw new Error('Missing application root');
render(() => createComponent(WorkbookApp, {
  client: createWorkbookClient(),
  createEngine: (container) => new UniverSpreadsheetEngine(container),
}), root);
