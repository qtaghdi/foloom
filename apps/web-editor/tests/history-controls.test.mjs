import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import test from 'node:test';
import ts from 'typescript';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

class FakeClassList {
  values = new Set();

  add(...names) {
    names.forEach((name) => this.values.add(name));
  }

  remove(...names) {
    names.forEach((name) => this.values.delete(name));
  }
}

class FakeElement {
  constructor(ownerDocument, id = '') {
    this.ownerDocument = ownerDocument;
    this.id = id;
    this.classList = new FakeClassList();
    this.children = [];
    this.textContent = '';
    this.dataset = {};
    this.attributes = new Map();
    this.focused = false;
  }

  append(...children) {
    this.children.push(...children);
  }

  replaceChildren(...children) {
    this.children = children;
    this.textContent = '';
  }

  setAttribute(name, value) {
    this.attributes.set(name, value);
  }

  getAttribute(name) {
    return this.attributes.get(name) ?? null;
  }

  focus() {
    this.focused = true;
  }
}

class FakeButton extends FakeElement {
  constructor(ownerDocument, id = '') {
    super(ownerDocument, id);
    this.disabled = true;
    this.listeners = new Map();
  }

  addEventListener(type, listener) {
    this.listeners.set(type, listener);
  }

  removeEventListener(type, listener) {
    if (this.listeners.get(type) === listener) {
      this.listeners.delete(type);
    }
  }

  click() {
    if (!this.disabled) {
      this.listeners.get('click')?.();
      this.onclick?.();
    }
  }
}

const createDocument = () => {
  const document = {
    elements: new Map(),
    getElementById(id) {
      return this.elements.get(id) ?? null;
    },
    createElement(tag) {
      if (tag === 'button') {
        const button = new FakeButton(this);
        button.disabled = false;
        return button;
      }
      return new FakeElement(this);
    },
  };
  for (const id of ['workbook-name', 'revision-status', 'ai-presence', 'ai-presence-text', 'fatal-error', 'feature-warnings', 'spreadsheet']) {
    document.elements.set(id, new FakeElement(document, id));
  }
  document.elements.set('history-undo', new FakeButton(document, 'history-undo'));
  document.elements.set('history-redo', new FakeButton(document, 'history-redo'));
  return document;
};

const loadAppView = (window) => {
  const module = { exports: {} };
  const path = resolve(root, 'src/ui/app-view.ts');
  const code = ts.transpileModule(readFileSync(path, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  vm.runInNewContext(code, {
    module,
    exports: module.exports,
    HTMLButtonElement: FakeButton,
    window,
  }, { filename: path });
  return module.exports.AppView;
};

test('history controls reflect availability and recovery disables both actions', () => {
  const document = createDocument();
  const window = { clearTimeout() {}, setTimeout: () => 1, location: { reload() {} } };
  const AppView = loadAppView(window);
  const view = new AppView(document);
  const undo = document.getElementById('history-undo');
  const redo = document.getElementById('history-redo');
  const actions = [];
  view.onHistoryAction((direction) => actions.push(direction));

  view.setHistoryState(false, false);
  assert.equal(undo.disabled, true);
  assert.equal(redo.disabled, true);
  undo.click();
  redo.click();
  assert.deepEqual(actions, []);

  view.setHistoryState(true, false);
  assert.equal(undo.disabled, false);
  assert.equal(redo.disabled, true);
  undo.click();
  redo.click();
  assert.deepEqual(actions, ['undo']);

  view.setHistoryState(true, true);
  view.showRecovery('Connection lost.', 7);
  assert.equal(undo.disabled, true);
  assert.equal(redo.disabled, true);

  view.dispose();
});

test('revision, failure, and conflict states are visible and accessible', () => {
  const document = createDocument();
  const window = { clearTimeout() {}, setTimeout: () => 1, location: { reload() {} } };
  const AppView = loadAppView(window);
  const view = new AppView(document);
  const revision = document.getElementById('revision-status');
  const spreadsheet = document.getElementById('spreadsheet');
  const error = document.getElementById('fatal-error');

  view.showRevision('Saving Sheet1!A1…', 2, 'pending');
  assert.equal(revision.dataset.state, 'pending');
  assert.equal(spreadsheet.getAttribute('aria-busy'), 'true');

  view.showRevision('Saved', 3, 'success');
  assert.equal(revision.dataset.state, 'success');
  assert.equal(spreadsheet.getAttribute('aria-busy'), 'false');

  view.showError(new Error('Network unavailable'), 3);
  assert.equal(revision.dataset.state, 'error');
  assert.equal(error.children[0].children[0].textContent, 'Operation failed');
  assert.equal(error.focused, true);
  const errorControls = error.children[0].children[1];
  const errorToggle = errorControls.children[0];
  const errorClose = errorControls.children[1];
  assert.equal(errorToggle.getAttribute('aria-expanded'), 'true');
  errorToggle.click();
  assert.equal(errorToggle.getAttribute('aria-expanded'), 'false');
  assert.equal(error.children[1].classList.values.has('hidden'), true);
  errorToggle.click();
  assert.equal(error.children[1].classList.values.has('hidden'), false);
  errorClose.click();
  assert.equal(error.classList.values.has('hidden'), true);
  assert.equal(spreadsheet.focused, true);

  error.focused = false;
  view.showRecovery('Revision conflict with a newer workbook.', 4);
  assert.equal(revision.dataset.state, 'conflict');
  assert.equal(error.children[0].children[0].textContent, 'Conflict detected');
  assert.equal(error.focused, true);
  const recoveryControls = error.children[0].children[1];
  assert.equal(recoveryControls.children.length, 1);
  recoveryControls.children[0].click();
  assert.equal(error.classList.values.has('hidden'), false);
  assert.equal(error.children[1].classList.values.has('hidden'), true);

  view.dispose();
});

test('compatibility notice can collapse, expand, and close', () => {
  const document = createDocument();
  const window = { clearTimeout() {}, setTimeout: () => 1, location: { reload() {} } };
  const AppView = loadAppView(window);
  const view = new AppView(document);
  const warning = document.getElementById('feature-warnings');

  view.showWarnings([{ feature: 'validation', message: 'Some rules are read-only.' }]);
  assert.equal(warning.classList.values.has('hidden'), false);
  assert.equal(warning.children[0].children[0].textContent, 'Compatibility notice');
  assert.match(warning.children[1].children[0].textContent, /Some rules are read-only/);
  const controls = warning.children[0].children[1];
  const toggle = controls.children[0];
  const close = controls.children[1];
  toggle.click();
  assert.equal(toggle.textContent, 'Expand');
  assert.equal(warning.children[1].classList.values.has('hidden'), true);
  toggle.click();
  assert.equal(toggle.textContent, 'Collapse');
  assert.equal(warning.children[1].classList.values.has('hidden'), false);
  close.click();
  assert.equal(warning.classList.values.has('hidden'), true);

  view.dispose();
});
