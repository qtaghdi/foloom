import '../styles.css';
import { createComponent } from 'solid-js';
import { render } from 'solid-js/web';
import { DocumentApp } from './document-app';

const root = document.getElementById('root');
if (!root) throw new Error('Missing application root');
render(() => createComponent(DocumentApp, {}), root);
