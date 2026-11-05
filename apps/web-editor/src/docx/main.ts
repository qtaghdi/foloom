import { WorkspaceApp } from '../workspace/workspace-app';
import '../styles.css';
import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { DocumentApp } from './document-app';

const root = document.getElementById('root');
if (!root) throw new Error('Missing application root');
const embedded = new URLSearchParams(window.location.search).get('embedded') === '1';
render(() => embedded ? createComponent(DocumentApp, {}) : createComponent(WorkspaceApp, {}), root);
