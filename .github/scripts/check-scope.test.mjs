import assert from 'node:assert/strict';
import { test } from 'node:test';
import { classifyChanges, changedPaths } from './check-scope.mjs';

test('known documentation edits skip expensive checks', () => {
  for (const path of ['README.md', 'AGENTS.md', 'CHANGELOG.md', 'docs/xlsx-performance.md', '.agents/skills/document-xlsx/SKILL.md', '.github/ISSUE_TEMPLATE/task.yml']) {
    assert.ok(Object.values(classifyChanges([path])).every(value => !value), path);
  }
});
test('DOCX code and fixtures select DOCX and packaging without XLSX gates', () => {
  for (const path of ['internal/formats/docx/create.go', 'internal/transport/httpapi/docx_test.go', 'apps/web-editor/src/docx/controller.ts', 'apps/web-editor/src/docx/main.ts', 'testdata/docx/simple.docx']) {
    assert.deepEqual(classifyChanges([path]), { regression: false, docx: true, safety: false, performance: false, package: true }, path);
  }
});
test('packaging scripts verify packages', () => {
  assert.deepEqual(classifyChanges(['scripts/package-smoke.sh']), { regression: false, docx: false, safety: false, performance: false, package: true });
});
test('XLSX, shared, dependencies, workflows, and unknown paths run everything', () => {
  for (const path of ['internal/formats/xlsx/session.go', 'testdata/xlsx/basic.xlsx', 'internal/transport/httpapi/server.go', 'apps/web-editor/src/ui/app-view.ts', 'pnpm-lock.yaml', 'go.mod', 'Makefile', 'internal/hostinstall/install.go', '.github/scripts/check-scope.mjs', '.github/workflows/package.yml', 'new/README.md', 'docs/executable.sh', 'scripts/package-smoke.sh.extra']) {
    assert.ok(Object.values(classifyChanges([path])).every(Boolean), path);
  }
});
test('mixed paths, deleted source, empty diff and manual dispatch fail closed', () => {
  for (const paths of [[], ['docs/guide.md', 'internal/formats/docx/session.go', 'internal/formats/xlsx/session.go'], ['internal/formats/xlsx/removed.go', 'docs/moved.md']]) {
    assert.ok(Object.values(classifyChanges(paths)).every(Boolean));
  }
  assert.ok(Object.values(classifyChanges(['README.md'], true)).every(Boolean));
});
test('diff rejects missing or option-shaped commit inputs', () => {
  assert.throws(() => changedPaths('--help', 'a'.repeat(40)), /Invalid/);
  assert.throws(() => changedPaths(undefined, 'a'.repeat(40)), /Invalid/);
});

test('real PR diff covers all commits and code renamed into docs', async () => {
  const { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, renameSync } = await import('node:fs');
  const { tmpdir } = await import('node:os');
  const { resolve, join } = await import('node:path');
  const { execFileSync } = await import('node:child_process');
  const root = mkdtempSync(join(tmpdir(), 'ci-scope-'));
  const script = resolve('.github/scripts/check-scope.mjs');
  const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8' }).trim();
  const commit = message => { git('add', '.'); git('-c', 'user.name=CI Test', '-c', 'user.email=ci@example.invalid', 'commit', '-m', message); return git('rev-parse', 'HEAD'); };
  const run = (base, head, eventName = 'pull_request') => {
    const event = join(root, '.git', 'event.json');
    const output = join(root, '.git', 'output');
    const summary = join(root, '.git', 'summary');
    writeFileSync(event, JSON.stringify({ pull_request: { base: { sha: base }, head: { sha: head } } }));
    writeFileSync(output, ''); writeFileSync(summary, '');
    execFileSync(process.execPath, [script], { cwd: root, env: { ...process.env, GITHUB_EVENT_NAME: eventName, GITHUB_EVENT_PATH: event, GITHUB_OUTPUT: output, GITHUB_STEP_SUMMARY: summary } });
    return Object.fromEntries(readFileSync(output, 'utf8').trim().split('\n').map(line => line.split('=').map((value, index) => index ? value === 'true' : value)));
  };
  try {
    git('init', '-q');
    mkdirSync(join(root, 'internal/formats/xlsx'), { recursive: true });
    mkdirSync(join(root, 'docs'));
    writeFileSync(join(root, 'internal/formats/xlsx/session.go'), 'source\n');
    const base = commit('base');
    writeFileSync(join(root, 'docs/guide.md'), 'guide\n');
    const docs = commit('docs');
    assert.ok(Object.values(run(base, docs)).every(value => !value));
    renameSync(join(root, 'internal/formats/xlsx/session.go'), join(root, 'docs/moved.md'));
    const renamed = commit('move source');
    assert.ok(Object.values(run(base, renamed)).every(Boolean));
    writeFileSync(join(root, 'docs/guide.md'), 'another documentation commit\n');
    const lastDocs = commit('follow-up docs');
    assert.ok(Object.values(run(base, lastDocs)).every(Boolean));
    assert.ok(Object.values(run(base, docs, 'workflow_dispatch')).every(Boolean));
  } finally { rmSync(root, { recursive: true, force: true }); }
});
