import { appendFileSync, readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

const full = () => ({ regression: true, docx: true, web: true, safety: true, performance: true, package: true });
const empty = () => Object.fromEntries(Object.keys(full()).map(key => [key, false]));

export function classifyChanges(paths, force = false) {
  if (force || paths.length === 0) return full();
  const plan = empty();
  for (const path of paths) {
    // Limit the documentation exemption to known locations. New executable paths fail closed.
    if (/^(docs\/.*\.(md|png|jpg|svg)|(?:README|CHANGELOG|AGENTS|SECURITY|CONTRIBUTING|LICENSE)(?:\.md)?|\.agents\/skills\/.*\.md|\.github\/(?:ISSUE_TEMPLATE\/.*|pull_request_template\.md))$/.test(path)) continue;
    if (/^(internal\/formats\/docx\/|internal\/transport\/httpapi\/docx(?:_test)?\.go$|internal\/transport\/httpapi\/static\/docx\.html$|apps\/web-editor\/(?:src\/docx\/|docx\.html$|tests\/docx(?:-[a-z0-9-]+)?\.test\.(?:mjs|tsx?)$)|testdata\/docx\/)/.test(path)) {
      plan.docx = true;
      plan.package = true;
      continue;
    }
    if (/^(?:internal\/formats\/xlsx\/)/.test(path)) {
      Object.assign(plan, { regression: true, safety: true, performance: true, package: true });
      continue;
    }
    if (/^scripts\/verify-xlsx-(?:safety|pre-excel)\.sh$/.test(path)) {
      plan.safety = true;
      continue;
    }
    if (/^cmd\/xlsx-performance\//.test(path)) {
      plan.performance = true;
      continue;
    }
    if (/^(?:testdata\/xlsx\/|legacy\/xlsx-python\/)/.test(path)) {
      plan.regression = true;
      continue;
    }
    if (/^apps\/web-editor\/(?:src\/|tests\/|scripts\/|vite(?:\.[a-z]+)?\.config\.ts$|(?:tsconfig|vitest\.config)\.(?:json|ts)$|index\.html$)/.test(path)) {
      plan.web = true;
      plan.package = true;
      continue;
    }
    // The generated entry is verified alongside both bundled editors.
    if (/^(?:internal\/transport\/httpapi\/static\/index\.html|scripts\/(?:build-release|check-release-artifacts|release-smoke|package-smoke)\.sh|scripts\/check-(?:embedded-assets|private-alpha-policy)(?:\.test)?\.mjs)$/.test(path)) {
      plan.package = true;
      continue;
    }
    return full();
  }
  return plan;
}

export function changedPaths(base, head) {
  if (![base, head].every(sha => /^[a-f0-9]{40}$/.test(sha ?? ''))) throw new Error('Invalid pull request commit SHA');
  // Disable rename detection so moving code into a documentation path still checks the deleted source.
  return execFileSync('git', ['diff', '--no-renames', '--name-only', '-z', `${base}...${head}`, '--'], { maxBuffer: 16 * 1024 * 1024 }).toString().split('\0').filter(Boolean);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const manual = process.env.GITHUB_EVENT_NAME === 'workflow_dispatch';
  if (!manual && process.env.GITHUB_EVENT_NAME !== 'pull_request') throw new Error('Unsupported CI event');
  const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
  const paths = manual ? [] : changedPaths(event.pull_request?.base?.sha, event.pull_request?.head?.sha);
  const plan = classifyChanges(paths, manual);
  const outputs = Object.entries(plan).map(([key, value]) => `${key}=${value}`).join('\n') + '\n';
  appendFileSync(process.env.GITHUB_OUTPUT, outputs);
  appendFileSync(process.env.GITHUB_STEP_SUMMARY, '## 변경 범위별 검사\n\n' + Object.entries(plan).map(([key, value]) => `- ${key}: ${value && plan.regression && ['docx', 'web'].includes(key) ? '통합 회귀에서 검사' : value ? '실행' : '생략'}`).join('\n') + '\n');
  console.log(outputs.trim());
}
