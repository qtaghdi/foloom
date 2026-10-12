import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import test from 'node:test';
import { mkdtempSync } from 'node:fs';

const script = resolve(dirname(fileURLToPath(import.meta.url)), 'check-embedded-assets.mjs');
const run = (directory) => spawnSync(process.execPath, [script], {
  encoding: 'utf8',
  env: { ...process.env, EMBEDDED_STATIC_DIR: directory },
});
const marker = '<title>Workbook editor</title>';

test('missing generated MCP App fails with an actionable message', () => {
  const directory = mkdtempSync(resolve(tmpdir(), 'embedded-assets-missing-'));
  writeFileSync(resolve(directory, 'index.html'), marker);
  const result = run(directory);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /MCP App entry is missing/);
  assert.match(result.stderr, /Run make web-build/);
});

test('missing referenced browser asset fails verification', () => {
  const directory = mkdtempSync(resolve(tmpdir(), 'embedded-assets-stale-'));
  writeFileSync(resolve(directory, 'index.html'), `${marker}<link href="/assets/style.css"><script src="/assets/app.js"></script>`);
  writeFileSync(resolve(directory, 'app.html'), `${marker}<style>body{}</style><script>0</script>`);
  mkdirSync(resolve(directory, 'assets'));
  writeFileSync(resolve(directory, 'assets/style.css'), 'body{}');
  const result = run(directory);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /browser asset \/assets\/app\.js is missing or empty/);
});

test('complete browser and self-contained MCP App assets pass verification', () => {
  const directory = mkdtempSync(resolve(tmpdir(), 'embedded-assets-valid-'));
  writeFileSync(resolve(directory, 'index.html'), `${marker}<link href="/assets/style.css"><script src="/assets/app.js"></script>`);
  writeFileSync(resolve(directory, 'app.html'), `${marker}<style>body{}</style><script>0</script>`);
  mkdirSync(resolve(directory, 'assets'));
  writeFileSync(resolve(directory, 'assets/style.css'), 'body{}');
  writeFileSync(resolve(directory, 'assets/app.js'), '0');
  const result = run(directory);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /verification passed/);
});
