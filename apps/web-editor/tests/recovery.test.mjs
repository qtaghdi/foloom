import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import test from 'node:test';
import ts from 'typescript';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const nativeRequire = createRequire(import.meta.url);
// Compile the real controller in an isolated browser environment without adding
// a browser framework or changing production module resolution for tests.
const loadModule = (path, globals) => {
  const module = { exports: {} };
  const code = ts.transpileModule(readFileSync(path, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  vm.runInNewContext(code, {
    ...globals, module, exports: module.exports,
    require: (name) => name.startsWith('.')
      ? loadModule(resolve(dirname(path), `${name}.ts`), globals)
      : nativeRequire(name),
  }, { filename: path });
  return module.exports;
};

const fixture = async (overrides = {}, snapshotOverrides = {}) => {
  const state = { writes: 0, reloads: 0, recoveries: [], statuses: [], revisionStates: [], operations: [], appliedRevisions: [], remoteEvents: [], historyDirections: [], historyStates: [], callbacks: {}, subscriptionClosed: false };
  const snapshot = { id: 'session', sequence: 5, name: 'book.xlsx', sheets: ['Sheet1'],
    sheetDimensions: [{ name: 'Sheet1', rows: 1, columns: 1 }], canUndo: true, canRedo: false, revision: 2,
    ...snapshotOverrides };
  const window = { clearTimeout() {}, setTimeout: () => 1, location: { reload: () => state.reloads++ } };
  const { WorkbookController } = loadModule(resolve(root, 'src/app/workbook-controller.ts'), { window });
  let current = snapshot;
  const client = {
    connect: async () => {}, getWorkbook: async () => current,
    readRange: async () => ({ sheet: 'Sheet1', ref: 'A1', rows: [] }),
    readSheetObjects: async () => ({ sheet: 'Sheet1' }),
    applyOperations: async (revision, operations) => { state.writes++; state.appliedRevisions.push(revision); state.operations.push(...operations); return { workbook: { ...snapshot, revision: 3 }, applied: 1 }; },
    restoreHistory: async (_revision, direction) => { state.writes++; state.historyDirections.push(direction); return { workbook: { ...snapshot, revision: 3 }, applied: 1 }; },
    subscribe: (handler, checkpoint) => {
      state.onEvent = handler; state.checkpoint = checkpoint;
      state.subscription = { onerror: null, close: () => { state.subscriptionClosed = true; } };
      return state.subscription;
    },
    ...overrides,
  };
  let restoreHistory = client.restoreHistory;
  Object.defineProperty(client, 'restoreHistory', {
    get: () => async (...args) => {
      const response = await restoreHistory(...args);
      current = response.workbook;
      return response;
    },
    set: (fn) => { restoreHistory = fn; },
  });
  const view = { spreadsheet: {}, setWorkbookName() {}, showWarnings() {},
    recordActivity: (entry) => { state.activities ??= []; state.activities.push(entry); },
    showRevision: (message, revision, status) => { state.statuses.push(message); state.revisionStates.push([message, revision, status]); },
    showRecovery: (message) => state.recoveries.push(message), showError: (error) => { throw error; },
    showAIPresence() {}, dispose() {},
  };
  const engine = {
    initialize() {}, dispose() {}, replaceWorkbook: (workbook) => { state.replacements = (state.replacements ?? 0) + 1; state.replaced = workbook; },
    setHistoryState: (canUndo, canRedo) => state.historyStates.push([canUndo, canRedo]),
    applyRemoteEvent: (event) => state.remoteEvents.push(event),
  };
  for (const method of ['onCellEdit','onRangeEdit','onSelectionChange','onOperation','onViewportChange','onHistoryAction']) {
    engine[method] = (callback) => { state.callbacks[method] = callback; };
  }
  const controller = new WorkbookController(client, view, () => engine);
  await controller.start();
  return { state, controller, client, snapshot };
};
const edit = { type: 'set_cell', sheet: 'Sheet1', cell: 'A1', value: 'draft' };
const flush = () => new Promise((resolve) => setImmediate(resolve));

test('lost response blocks queued edits and reconciles without retry or reload', async () => {
  const { state, client, controller } = await fixture();
  client.applyOperations = async () => { state.writes++; throw new Error('response lost'); };
  state.callbacks.onOperation(edit);
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.ok(state.recoveries.length > 0);
  assert.equal(state.subscriptionClosed, true);
  assert.equal(controller.shouldWarnBeforeUnload(), true);
  state.callbacks.onOperation(edit);
  state.callbacks.onHistoryAction('undo');
  await flush();
  assert.equal(state.writes, 1);
  assert.ok(!state.statuses.includes('Saved'));
});

test('event loss pauses editing even when the status read is unavailable', async () => {
  const { state, client } = await fixture();
  client.getWorkbook = async () => { throw new Error('offline'); };
  state.subscription.onerror(new Error('disconnected'));
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 0);
  assert.match(state.recoveries.at(-1), /could not be checked/);
  assert.equal(state.reloads, 0);
});

test('subscription starts at the snapshot checkpoint and resync is not ignored', async () => {
  const { state } = await fixture();
  assert.equal(state.checkpoint.sequence, 5);
  state.onEvent({ type: 'workbook.reload', state: 'resync', actor: 'system', sequence: 600, revision: 9 });
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 0);
  assert.match(state.recoveries[0], /history is incomplete/);
});

test('a successful queued write still commits normally', async () => {
  const { state, client, snapshot, controller } = await fixture({}, { canUndo: false, canRedo: false });
  client.applyOperations = async () => {
    state.writes++;
    return { workbook: { ...snapshot, revision: 3, canUndo: true, canRedo: false }, applied: 1 };
  };
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.ok(state.statuses.includes('Saved'));
  assert.equal(state.recoveries.length, 0);
  assert.equal(controller.shouldWarnBeforeUnload(), false);
  assert.deepEqual(state.historyStates.at(-1), [true, false]);
  assert.ok(state.revisionStates.some(([message, , status]) => message === 'Saving Sheet1!A1…' && status === 'pending'));
  assert.ok(state.revisionStates.some(([message, , status]) => message === 'Saved' && status === 'success'));
});

test('keyboard-confirmed cell editing becomes a persisted cell operation', async () => {
  const { state } = await fixture();
  state.callbacks.onCellEdit({ sheet: 'Sheet1', cell: 'B2', value: 'keyboard value' });
  await flush();
  assert.deepEqual(JSON.parse(JSON.stringify(state.operations.at(-1))), {
    type: 'set_cell', sheet: 'Sheet1', cell: 'B2', value: 'keyboard value',
  });
});

test('clipboard paste becomes one atomic rectangular range operation', async () => {
  const { state } = await fixture();
  state.callbacks.onRangeEdit({
    sheet: 'Sheet1', range: 'B2:C3', cells: [[{ value: 'A' }, { value: 'B' }], [{ value: 1 }, { value: 2 }]],
  });
  await flush();
  assert.deepEqual(JSON.parse(JSON.stringify(state.operations.at(-1))), {
    type: 'paste_range', sheet: 'Sheet1', range: 'B2:C3',
    cells: [[{ value: 'A' }, { value: 'B' }], [{ value: 1 }, { value: 2 }]],
  });
});

test('separate cleared cells are saved in one revision', async () => {
  const { state, client, snapshot } = await fixture();
  const clears = ['B1', 'D1'].map(cell => ({ type: 'set_cell', sheet: 'Sheet1', cell, value: null }));
  client.applyOperations = async (revision, operations) => {
    state.writes++;
    assert.equal(revision, snapshot.revision);
    assert.deepEqual([...operations], clears);
    return { workbook: { ...snapshot, revision: 3 }, applied: 2 };
  };
  state.callbacks.onOperation(clears[0], clears.slice(1));
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.revisionStates.at(-1)[1], 3);
});

test('spreadsheet keyboard undo delegates to server-owned history', async () => {
  const { state } = await fixture();
  state.callbacks.onHistoryAction('undo');
  await flush();
  assert.deepEqual(state.historyDirections, ['undo']);
  assert.equal(state.reloads, 0);
  assert.equal(state.replaced.revision, 3);
});

test('unavailable history remains disabled and never calls the server', async () => {
  const { state } = await fixture({}, { canUndo: false, canRedo: false });
  assert.deepEqual(state.historyStates.at(-1), [false, false]);
  state.callbacks.onHistoryAction('undo');
  state.callbacks.onHistoryAction('redo');
  await flush();
  assert.equal(state.writes, 0);
  assert.ok(state.statuses.includes('Nothing to undo'));
  assert.ok(state.statuses.includes('Nothing to redo'));
});

test('remote AI commit updates history availability', async () => {
  const { state } = await fixture({}, { canUndo: false, canRedo: false });
  state.onEvent({
    type: 'cell.commit', actor: 'ai', revision: 3, sequence: 6, sheet: 'Sheet1', cell: 'A1',
    canUndo: true, canRedo: false,
  });
  assert.deepEqual(state.historyStates.at(-1), [true, false]);
});

test('remote human commit advances an idle browser without conflict', async () => {
  const { state } = await fixture({}, { canUndo: false, canRedo: false });
  const event = {
    type: 'cell.commit', actor: 'human', revision: 3, sequence: 6, sheet: 'Sheet1', cell: 'A1',
    canUndo: true, canRedo: false,
  };
  state.onEvent(event);
  state.callbacks.onOperation({ ...edit, cell: 'A2' });
  await flush();
  assert.deepEqual(state.appliedRevisions, [3]);
  assert.equal(state.recoveries.length, 0);
  assert.equal(state.remoteEvents.at(-1), event);
  assert.ok(state.historyStates.some(([canUndo, canRedo]) => canUndo && !canRedo));
  assert.ok(state.revisionStates.some(([message, revision]) => message === 'Remote change saved' && revision === 3));
});

test('remote commit during an unresolved local save cannot rebase queued drafts', async () => {
  const { state, client, snapshot } = await fixture();
  let finish;
  client.applyOperations = () => { state.writes++; return new Promise((resolve) => { finish = resolve; }); };
  state.callbacks.onOperation(edit);
  state.callbacks.onOperation(edit);
  await flush();
  state.onEvent({ type: 'cell.commit', actor: 'ai', revision: 3, sequence: 6, sheet: 'Sheet1', cell: 'A1' });
  finish({ workbook: { ...snapshot, revision: 4 }, applied: 1 });
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.ok(!state.statuses.includes('Saved'));
});

test('failed undo does not allow a queued edit to continue', async () => {
  const { state, client } = await fixture();
  client.restoreHistory = async () => { state.writes++; throw new Error('unknown undo outcome'); };
  state.callbacks.onHistoryAction('undo');
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
});

test('successful undo cancels queued edits based on the previous grid', async () => {
  const { state, client, snapshot } = await fixture();
  client.restoreHistory = async () => {
    state.writes++;
    return { workbook: { ...snapshot, revision: 3, canUndo: false, canRedo: true }, applied: 1 };
  };
  state.callbacks.onHistoryAction('undo');
  state.callbacks.onOperation(edit);
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.deepEqual(state.historyStates.at(-1), [false, true]);
});

test('successful redo replaces the workbook in place', async () => {
  const { state, client, snapshot } = await fixture({}, { canUndo: false, canRedo: true });
  client.restoreHistory = async () => {
    state.writes++;
    return { workbook: { ...snapshot, revision: 3, canUndo: true, canRedo: false }, applied: 1 };
  };
  state.callbacks.onHistoryAction('redo');
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.deepEqual(state.historyStates.at(-1), [true, false]);
});

test('undo replaces the preview once when the live event precedes the response', async () => {
  const { state, client, snapshot } = await fixture();
  client.restoreHistory = async () => {
    state.onEvent({ type: 'workbook.reload', state: 'undo', actor: 'human', sequence: 6, revision: 3 });
    assert.equal(state.reloads, 0);
    return { workbook: { ...snapshot, sequence: 6, revision: 3 }, applied: 1 };
  };
  state.callbacks.onHistoryAction('undo');
  state.callbacks.onHistoryAction('undo');
  await flush();
  assert.equal(state.reloads, 0);
  assert.equal(state.replacements, 1);
  assert.equal(state.replaced.revision, 3);
  assert.equal(state.checkpoint.sequence, 6);
});

test('history recovery keeps the current preview when the confirmed revision changed', async () => {
  const { state, client, snapshot } = await fixture();
  client.getWorkbook = async () => ({ ...snapshot, revision: 4 });
  state.callbacks.onHistoryAction('undo');
  await flush();
  assert.equal(state.writes, 1);
  assert.equal(state.reloads, 0);
  assert.equal(state.replacements, undefined);
  assert.match(state.recoveries[0], /changed while loading/);
});

test('unmount while connecting never mounts an engine or opens a subscription', async () => {
  let finish;
  const { state, controller } = await fixture();
  const connecting = new Promise((resolve) => { finish = resolve; });
  // A fresh real controller exercises the lifecycle before the first snapshot.
  const { WorkbookController } = loadModule(resolve(root, 'src/app/workbook-controller.ts'), { window: { clearTimeout() {} } });
  const getWorkbook = () => { throw new Error('must not read after unmount'); };
  let mounted = false;
  const late = new WorkbookController({ connect: () => connecting, getWorkbook }, { dispose() {} }, () => { mounted = true; });
  const start = late.start(); late.dispose(); finish(); await start;
  assert.equal(mounted, false);
  controller.dispose();
});

test('confirmed history restore completes without warning about nonexistent unsaved edits', async () => {
  const { state, controller } = await fixture();
  state.callbacks.onHistoryAction('undo'); await flush();
  assert.equal(state.reloads, 0);
  assert.equal(controller.shouldWarnBeforeUnload(), false);
  controller.dispose();
});


test('toolbar formatting saves the selected range and blocks history while saving', async () => {
  const { state, client, snapshot } = await fixture();
  let finish;
  client.applyOperations = async (revision, operations) => {
    state.appliedRevisions.push(revision);
    state.operations.push(...operations);
    return new Promise((resolve) => { finish = resolve; });
  };
  const format = { type: 'set_format', sheet: 'Sheet1', range: 'A1:B2', format: { bold: true } };
  state.callbacks.onOperation(format);
  state.callbacks.onHistoryAction('undo');
  await flush();
  assert.deepEqual(state.operations, [format]);
  assert.deepEqual(state.historyDirections, []);
  assert.deepEqual(state.historyStates.at(-1), [false, false]);
  finish({ workbook: { ...snapshot, revision: 3 }, applied: 1 });
  await flush();
  assert.deepEqual(state.historyStates.at(-1), [true, false]);
});

test('an own in-flight SSE receipt does not duplicate the confirmed activity entry', async () => {
  const { state, client, snapshot } = await fixture();
  let finish;
  client.applyOperations = async () => new Promise((resolve) => { finish = resolve; });
  state.callbacks.onOperation(edit); await flush();
  state.onEvent({ type: 'cell.commit', actor: 'human', sheet: 'Sheet1', cell: 'A1', revision: 3, sequence: 6 });
  assert.equal(state.activities.filter((entry) => entry.state === 'committed').length, 0);
  finish({ workbook: { ...snapshot, revision: 3 }, applied: 1 }); await flush();
  assert.equal(state.activities.filter((entry) => entry.state === 'committed').length, 1);
});
