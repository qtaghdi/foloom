import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const script = resolve(dirname(fileURLToPath(import.meta.url)), 'check-private-alpha-policy.mjs');
const validWorkflow = `name: Private alpha package
on:
  workflow_dispatch:
permissions:
  contents: read
jobs: {}
`;

const run = (releaseSource, extraWorkflow) => {
  const directory = mkdtempSync(resolve(tmpdir(), 'private-alpha-policy-'));
  writeFileSync(resolve(directory, 'release.yml'), releaseSource);
  if (extraWorkflow) {
    writeFileSync(resolve(directory, 'other.yml'), extraWorkflow);
  }
  return spawnSync(process.execPath, [script], {
    encoding: 'utf8',
    env: { ...process.env, WORKFLOW_DIR: directory },
  });
};

test('manual read-only artifact workflow passes', () => {
  const result = run(validWorkflow);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Private alpha policy passed/);
});

test('push trigger fails private alpha policy', () => {
  const result = run(validWorkflow.replace('  workflow_dispatch:', '  workflow_dispatch:\n  push:'));
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /must not run from a branch or tag push/);
});

test('write permission fails private alpha policy', () => {
  const result = run(validWorkflow.replace('contents: read', 'contents: write'));
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /must not request write access/);
});

test('release publisher in any workflow fails private alpha policy', () => {
  const result = run(validWorkflow, 'jobs:\n  publish:\n    steps:\n      - run: gh release create v1 artifact.zip\n');
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /contains a public release publisher/);
});
