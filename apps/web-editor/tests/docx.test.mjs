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
function loadModule(path, globals) {
  const module = { exports: {} };
  const code = ts.transpileModule(readFileSync(path, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  vm.runInNewContext(code, { ...globals, module, exports: module.exports,
    require: (name) => name.startsWith('.') ? loadModule(resolve(dirname(path), `${name}.ts`), globals) : nativeRequire(name),
  });
  return module.exports;
}
const snapshot = { id: 'docx-session', name: 'test.docx', revision: 1, sequence: 0,
  blocks: [{ id: 'p1', type: 'paragraph', paragraph: { id: 'p1', runs: [
    { id: 'r1', text: 'old', editable: true, bold: false, italic: false },
  ] } }], warnings: [] };
const edit = { type: 'replace_run_text', targetId: 'r1', expectedText: 'old', text: 'draft' };
function fixture() {
  const state = { writes: [], statuses: [], renders: [], errors: [], enabled: [], events: [] };
  let handler = async (_url, init) => {
    if (_url.startsWith('/api/save')) return { ok: true, json: async () => ({ id: snapshot.id, revision: snapshot.revision, name: snapshot.name }) };
    if (init?.method === 'POST') state.writes.push(JSON.parse(init.body));
    return { ok: true, json: async () => ({ ...snapshot, revision: init?.method === 'POST' ? 2 : 1 }) };
  };
  class EventSource {
    constructor(url) { this.url = url; state.events.push(this); }
    addEventListener(_name, listener) { this.listener = listener; }
    close() { this.closed = true; }
  }
  const { DocumentController } = loadModule(resolve(root, 'src/docx/controller.ts'), {
    fetch: (...args) => handler(...args), EventSource, URLSearchParams,
  });
  const controller = new DocumentController({
    render: (s) => state.renders.push(s), status: (...args) => state.statuses.push(args),
    error: (e) => state.errors.push(e), canSave: (v) => state.enabled.push(v),
  });
  return { state, controller, fetch: (next) => { handler = next; } };
}

test('DOCX batches save against the inspected session and revision', async () => {
  const f = fixture(); await f.controller.load(); f.controller.edit(edit); await f.controller.save();
  assert.equal(f.state.writes.length, 1);
  assert.equal(f.state.writes[0].sessionId, snapshot.id);
  assert.equal(f.state.writes[0].baseRevision, 1);
  assert.deepEqual(f.state.writes[0].operations, [edit]);
  assert.equal(f.controller.hasDrafts(), false);
  assert.equal(f.state.renders.at(-1).revision, 2);
});

test('DOCX lost write responses retain drafts and block retries', async () => {
  const f = fixture(); await f.controller.load(); f.controller.edit(edit);
  let calls = 0;
  f.fetch(async () => { calls++; throw new Error('response lost'); });
  await f.controller.save(); await f.controller.save();
  assert.equal(calls, 1);
  assert.equal(f.controller.hasDrafts(), true);
  assert.equal(f.state.enabled.at(-1), false);
  assert.equal(f.state.renders.length, 1);
  assert.equal(f.state.statuses.at(-1)[1], 'failed');
});

test('DOCX remote commits preserve drafts and require explicit recovery', async () => {
  const f = fixture(); await f.controller.load(); f.controller.edit(edit);
  f.state.events.at(-1).listener({ data: JSON.stringify({ type: 'docx.committed', sequence: 1, revision: 2, actor: 'ai' }) });
  await f.controller.save();
  assert.equal(f.controller.hasDrafts(), true);
  assert.equal(f.state.writes.length, 0);
  assert.equal(f.state.renders.length, 1);
  assert.equal(f.state.events.at(-1).closed, true);
  await f.controller.load();
  assert.equal(f.controller.hasDrafts(), false);
});

test('DOCX connection failure cannot silently resume writes', async () => {
  const f = fixture(); await f.controller.load(); f.state.events.at(-1).onerror();
  f.controller.edit(edit); await f.controller.save();
  assert.equal(f.state.writes.length, 0);
  assert.equal(f.state.enabled.at(-1), false);
});

test('DOCX invalid snapshots do not report a successful save', async () => {
  const f = fixture(); await f.controller.load(); f.controller.edit(edit);
  f.fetch(async () => ({ ok: true, json: async () => ({ ...snapshot, blocks: [{ type: 'unknown' }] }) }));
  await f.controller.save();
  assert.equal(f.controller.hasDrafts(), true);
  assert.equal(f.state.statuses.at(-1)[1], 'failed');
});

test('DOCX reverted drafts do not create a write', async () => {
  const f = fixture(); await f.controller.load(); f.controller.edit(edit);
  f.controller.edit({ ...edit, text: edit.expectedText }); await f.controller.save();
  assert.equal(f.state.writes.length, 0);
});

test('DOCX closed event sources cannot interrupt a newer save', async () => {
  const f = fixture(); await f.controller.load(); f.controller.edit(edit);
  const oldSource = f.state.events.at(-1);
  let finish;
  f.fetch(() => new Promise((resolve) => { finish = resolve; }));
  const pending = f.controller.save();
  oldSource.onerror();
  oldSource.listener({ data: JSON.stringify({ type: 'docx.reload', sequence: 1, revision: 2 }) });
  finish({ ok: true, json: async () => ({ ...snapshot, revision: 2 }) });
  await pending;
  f.controller.edit({ ...edit, targetId: 'r2', text: 'next' });
  assert.equal(f.state.enabled.at(-1), true);
  assert.equal(f.state.statuses.some(([, state]) => state === 'failed'), false);
});

test('DOCX rejects unsupported draft text before sending and accepts corrected emoji text', async () => {
  const f = fixture(); await f.controller.load();
  f.controller.edit({ ...edit, text: 'two\tcolumns' }); await f.controller.save();
  assert.equal(f.state.writes.length, 0); assert.equal(f.state.enabled.at(-1), false);
  f.controller.edit({ ...edit, text: '문서 😀' });
  assert.equal(f.state.enabled.at(-1), true); await f.controller.save();
  assert.equal(f.state.writes.length, 1);
});

test('DOCX coalesces save requests until IME commits its final draft', async () => {
  const f = fixture(); await f.controller.load(); f.controller.composition(true);
  const first = f.controller.save(), second = f.controller.save();
  assert.equal(f.state.writes.length, 0);
  await f.controller.load();
  assert.equal(f.state.renders.length, 1);
  f.controller.edit({ ...edit, text: '한글 日本語 中文' });
  f.controller.composition(false);
  assert.deepEqual(await Promise.all([first, second]), [true, true]);
  assert.equal(f.state.writes.length, 1);
  assert.equal(f.state.writes[0].operations[0].text, '한글 日本語 中文');
});

test('DOCX cancels queued IME saves on remote conflict and preserves final input', async () => {
  const f = fixture(); await f.controller.load(); f.controller.composition(true);
  const pending = f.controller.save();
  f.state.events.at(-1).listener({ data: JSON.stringify({ type: 'docx.committed', sequence: 1, revision: 2, actor: 'ai' }) });
  assert.equal(await pending, false);
  assert.equal(f.state.statuses.at(-1)[1], 'conflicted');
  f.controller.edit({ ...edit, text: '조합 보존' }); f.controller.composition(false);
  assert.equal(await f.controller.save(), false);
  assert.equal(f.state.writes.length, 0);
  assert.equal(f.controller.hasDrafts(), true);
  assert.equal(f.state.renders.length, 1);
});

test('DOCX disposal resolves an outstanding composition save without writing', async () => {
  const f = fixture(); await f.controller.load(); f.controller.composition(true);
  const pending = f.controller.save(); f.controller.dispose();
  assert.equal(await pending, false);
  f.controller.composition(false);
  assert.equal(f.state.writes.length, 0);
});

test('DOCX draft undo coalesces typing and never also restores server history', async () => {
  const f = fixture(); await f.controller.load();
  f.controller.edit({...edit,text:'a'}); f.controller.edit({...edit,text:'ab'});
  assert.equal(await f.controller.restore('undo'),true);
  assert.equal(f.controller.hasDrafts(),false);
  assert.equal(f.state.renders.at(-1).blocks[0].paragraph.runs[0].text,'old');
  assert.equal(f.state.writes.length,0);
  assert.equal(await f.controller.restore('redo'),true);
  assert.equal(f.state.renders.at(-1).blocks[0].paragraph.runs[0].text,'ab');
  f.controller.edit({...edit,text:'new branch'});
  assert.equal(await f.controller.restore('redo'),false);
  assert.equal(f.state.writes.length,0);
});

test('DOCX server undo uses current revision once and a lost response blocks retries', async () => {
  const f=fixture();
  f.fetch(async()=>({ok:true,json:async()=>({...snapshot,history:{undo:1,redo:0,undoActor:'ai'}})}));
  await f.controller.load();const writes=[];
  f.fetch(async(url,init)=>{writes.push([url,JSON.parse(init.body)]);throw new Error('lost response');});
  assert.equal(await f.controller.restore('undo'),false);
  assert.equal(await f.controller.restore('undo'),false);
  assert.equal(writes.length,1);
  assert.equal(writes[0][0],'/api/docx/history');
  assert.deepEqual(writes[0][1],{sessionId:snapshot.id,baseRevision:1,direction:'undo'});
  assert.equal(f.state.renders.length,1);
  assert.equal(f.state.statuses.at(-1)[1],'failed');
});

test('DOCX style drafts participate in undo, redo and one revisioned save batch',async()=>{
 const f=fixture();await f.controller.load();
 assert.equal(f.controller.editStyle({id:'Custom',name:'제목',kind:'paragraph',run:{bold:true}}),true);
 assert.equal(f.controller.hasDrafts(),true);assert.equal(f.controller.styles().length,1);
 await f.controller.restore('undo');assert.equal(f.controller.hasDrafts(),false);assert.equal(f.controller.styles().length,0);
 await f.controller.restore('redo');await f.controller.save();
 assert.equal(f.state.writes.length,1);
 assert.equal(f.state.writes[0].operations[0].type,'define_style');
 assert.equal(f.state.writes[0].operations[0].styleDefinition.id,'Custom');
 assert.equal(f.controller.hasDrafts(),false);
});

test('DOCX style inheritance can be corrected before an invalid draft reaches the server',async()=>{
 const f=fixture();await f.controller.load();
 f.controller.editStyle({id:'A',name:'A',kind:'paragraph',basedOn:'B'});
 f.controller.editStyle({id:'B',name:'B',kind:'paragraph',basedOn:'A'});
 assert.equal(await f.controller.save(),false);assert.equal(f.state.writes.length,0);
 f.controller.editStyle({id:'B',kind:'paragraph',basedOn:''});
 assert.equal(await f.controller.save(),true);assert.equal(f.state.writes.length,1);
});

test('DOCX section drafts share one revisioned save and undo history with text',async()=>{
 const f=fixture();
 const section={id:'section-current',startBlock:0,endBlock:0,page:{width:11906,height:16838,top:1440,right:1440,bottom:1440,left:1440},orientation:'portrait',columns:1,columnSpace:720,breakType:'nextPage',editable:true};
 const operations=[];
 f.fetch(async(_url,init)=>{
  if(init?.method==='POST')operations.push(JSON.parse(init.body));
  return {ok:true,json:async()=>({...snapshot,sections:[section],revision:init?.method==='POST'?2:1})};
 });
 await f.controller.load();
 assert.equal(f.controller.editSection(section.id,{width:16838,height:11906,orientation:'landscape'}),true);
 assert.equal(f.controller.sections()[0].page.width,16838);
 await f.controller.restore('undo');assert.equal(f.controller.sections()[0].page.width,11906);
 await f.controller.restore('redo');assert.equal(f.controller.sections()[0].page.width,16838);
 assert.equal(f.controller.editSection(section.id,{left:50000}),false);
 assert.equal(f.controller.editSection('stale',{columns:2}),false);
 f.controller.edit(edit);await f.controller.save();
 assert.equal(operations.length,1);
 assert.equal(operations[0].baseRevision,1);
 assert.deepEqual(operations[0].operations.map(op=>op.type),['update_section','replace_run_text']);
 assert.equal(operations[0].operations[0].section.orientation,'landscape');
});
