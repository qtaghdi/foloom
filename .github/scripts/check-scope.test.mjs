import assert from 'node:assert/strict';
import { test } from 'node:test';
import { classifyChanges, changedPaths } from './check-scope.mjs';
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

const workflow = readFileSync(new URL('../workflows/pr-ci.yml', import.meta.url), 'utf8');
const targetsScript = workflow.match(/          targets=\(web-install\)[\s\S]*?          make "\$\{targets\[@\]\}"/)?.[0];
const gateScript = workflow.match(/          node <<'JS'\n([\s\S]*?)\n          JS/)?.[1];

test('editor and package targets share one Make invocation', () => {
  assert.ok(targetsScript);
  const run = (paths) => {
    const plan = classifyChanges(paths);
    return execFileSync('bash', ['-c', `make() { printf '%s\\n' "$@"; }\n${targetsScript}`], {
      encoding: 'utf8', env: { ...process.env, DOCX: String(plan.docx), WEB: String(plan.web), PACKAGE: String(plan.package), REGRESSION: String(plan.regression) },
    }).trim().split('\n');
  };
  assert.deepEqual(run(['internal/formats/docx/create.go']), ['web-install', 'docx-verify', 'package-verify', 'release-package-test']);
  assert.deepEqual(run(['apps/web-editor/src/ui/app-view.ts']), ['web-install', 'web-test', 'package-verify', 'release-package-test']);
  assert.deepEqual(run(['go.mod']), ['web-install', 'package-verify', 'release-package-test']);
});

test('combined required job rejects failures, omissions and invalid scope results', () => {
  assert.ok(gateScript);
  const plan = classifyChanges(['internal/formats/docx/create.go']);
  const jobs = {
    scope: { result: 'success', outputs: Object.fromEntries(Object.entries(plan).map(([key, value]) => [key, String(value)])) },
    regression: { result: 'skipped' }, editor_package: { result: 'success' }, safety: { result: 'skipped' }, performance: { result: 'skipped' },
  };
  const run = (results) => execFileSync(process.execPath, ['-e', gateScript], { env: { ...process.env, RESULTS: JSON.stringify(results) }, stdio: 'pipe' });
  assert.doesNotThrow(() => run(jobs));
  for (const result of ['failure', 'cancelled', 'skipped']) assert.throws(() => run({ ...jobs, editor_package: { result } }));
  assert.throws(() => run({ ...jobs, scope: { ...jobs.scope, result: 'failure' } }));
  assert.throws(() => run({ ...jobs, scope: { ...jobs.scope, outputs: {} } }));
  const full = classifyChanges(['go.mod']);
  assert.doesNotThrow(() => run({ scope: { result: 'success', outputs: Object.fromEntries(Object.entries(full).map(([key, value]) => [key, String(value)])) }, regression: { result: 'success' }, editor_package: { result: 'success' }, safety: { result: 'success' }, performance: { result: 'success' } }));
});

test('known documentation edits skip expensive checks', () => {
  for (const path of ['README.md', 'AGENTS.md', 'CHANGELOG.md', 'docs/xlsx-performance.md', '.agents/skills/document-xlsx/SKILL.md', '.github/ISSUE_TEMPLATE/task.yml']) {
    assert.ok(Object.values(classifyChanges([path])).every(value => !value), path);
  }
});
test('DOCX code and fixtures select DOCX and packaging without XLSX gates', () => {
  for (const path of ['internal/formats/docx/create.go', 'internal/transport/httpapi/docx_test.go', 'apps/web-editor/src/docx/controller.ts', 'apps/web-editor/src/docx/main.ts', 'apps/web-editor/tests/docx-header-footer.test.tsx', 'testdata/docx/simple.docx']) {
    assert.deepEqual(classifyChanges([path]), { regression: false, docx: true, web: false, safety: false, performance: false, package: true }, path);
  }
});
test('packaging scripts verify packages', () => {
  assert.deepEqual(classifyChanges(['scripts/package-smoke.sh']), { regression: false, docx: false, web: false, safety: false, performance: false, package: true });
});
function selected(paths) {
  const plan = classifyChanges(paths);
  return Object.keys(plan).filter(key => plan[key]);
}
test('independent changes select the union of the relevant jobs', () => {
  assert.deepEqual(selected(['internal/formats/xlsx/session.go']), ['regression', 'safety', 'performance', 'package']);
  assert.deepEqual(selected(['testdata/xlsx/basic.xlsx']), ['regression']);
  assert.deepEqual(selected(['cmd/xlsx-performance/main.go']), ['performance']);
  assert.deepEqual(selected(['scripts/verify-xlsx-safety.sh']), ['safety']);
  assert.deepEqual(selected(['apps/web-editor/src/ui/app-view.ts']), ['web', 'package']);
  assert.deepEqual(selected(['apps/web-editor/tests/docx-sections.test.tsx', 'internal/transport/httpapi/static/index.html']), ['docx', 'package']);
  assert.deepEqual(selected(['apps/web-editor/src/docx/controller.ts', 'cmd/xlsx-performance/main.go']), ['docx', 'performance', 'package']);
});
test('shared backend, dependencies, CI and unknown paths fail closed', () => {
  for (const path of ['internal/transport/httpapi/server.go', 'pnpm-lock.yaml', 'go.mod', 'Makefile', 'internal/hostinstall/install.go', '.github/scripts/check-scope.mjs', '.github/workflows/package.yml', 'new/README.md', 'docs/executable.sh', 'scripts/package-smoke.sh.extra']) {
    assert.ok(Object.values(classifyChanges([path])).every(Boolean), path);
  }
  assert.ok(Object.values(classifyChanges([])).every(Boolean));
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
    assert.deepEqual(run(base, renamed), classifyChanges(['internal/formats/xlsx/session.go']));
    writeFileSync(join(root, 'docs/guide.md'), 'another documentation commit\n');
    const lastDocs = commit('follow-up docs');
    assert.deepEqual(run(base, lastDocs), classifyChanges(['internal/formats/xlsx/session.go']));
    assert.ok(Object.values(run(base, docs, 'workflow_dispatch')).every(Boolean));
  } finally { rmSync(root, { recursive: true, force: true }); }
});
