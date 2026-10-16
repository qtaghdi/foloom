import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const entries = ['index.html', 'app.html'];

for (const entry of entries) {
  test(`${entry} exposes keyboard, status, and responsive accessibility contracts`, () => {
    const html = readFileSync(resolve(root, entry), 'utf8');
    assert.match(html, /href="#spreadsheet">Skip to workbook/);
    assert.match(html, /id="revision-status"[^>]+role="status"[^>]+aria-live="polite"/);
    assert.match(html, /id="history-undo"[^>]+min-h-11[^>]+focus-visible:/);
    assert.match(html, /id="history-redo"[^>]+min-h-11[^>]+focus-visible:/);
    assert.match(html, /role="toolbar" aria-label="Workbook history"/);
    assert.match(html, /id="spreadsheet"[^>]+tabindex="-1"[^>]+aria-busy="true"/);
    assert.match(html, /id="fatal-error"[^>]+role="alert"[^>]+aria-live="assertive"/);
    assert.match(html, /id="feature-warnings"[^>]+role="status"[^>]+aria-live="polite"/);
    assert.match(html, /name="theme-color" content="#065f46"/);
    assert.doesNotMatch(html, /user-scalable=no|maximum-scale=1/);
  });
}

test('styles provide small-screen, safe-area, skip-link, and reduced-motion behavior', () => {
  const css = readFileSync(resolve(root, 'src/styles.css'), 'utf8');
  assert.match(css, /env\(safe-area-inset-top\)/);
  assert.match(css, /@media \(max-width: 640px\)/);
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)/);
  assert.match(css, /\.skip-link:focus-visible/);
  const view = readFileSync(resolve(root, 'src/ui/app-view.ts'), 'utf8');
  assert.match(view, /aria-expanded/);
  assert.match(view, /min-h-11 min-w-11/);
  assert.match(css, /animation-duration: 0\.01ms !important/);
  assert.doesNotMatch(css, /transition:\s*all/);
});

test('navigation warns for uncertain work without disposing a canceled page', () => {
  const source = readFileSync(resolve(root, 'src/main.ts'), 'utf8');
  assert.match(source, /beforeunload/);
  assert.match(source, /shouldWarnBeforeUnload\(\)/);
  assert.match(source, /event\.preventDefault\(\)/);
  assert.match(source, /pagehide/);
  const beforeUnload = source.slice(source.indexOf("'beforeunload'"), source.indexOf("'pagehide'"));
  assert.doesNotMatch(beforeUnload, /controller\.dispose\(\)/);
});
