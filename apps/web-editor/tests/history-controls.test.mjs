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
  }

  append(child) {
    this.children.push(child);
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
      return tag === 'button' ? new FakeButton(this) : new FakeElement(this);
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
