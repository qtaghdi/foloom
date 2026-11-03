import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
for (const entry of ['index.html', 'app.html', 'docx.html']) {
  test(`${entry} mounts a Korean Solid app without restricting zoom`, () => {
    const html = readFileSync(resolve(root, entry), 'utf8');
    assert.match(html, /lang="ko"/);
    assert.match(html, /id="root"/);
    assert.match(html, /type="module"/);
    assert.doesNotMatch(html, /user-scalable=no|maximum-scale=1/);
  });
}
test('shared styles preserve safe-area, keyboard focus and reduced motion', () => {
  const css = readFileSync(resolve(root, 'src/styles.css'), 'utf8');
  assert.match(css, /env\(safe-area-inset-top\)/);
  assert.match(css, /@media \(max-width: 640px\)/);
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)/);
  assert.match(css, /\.skip-link:focus-visible/);
  assert.doesNotMatch(css, /transition:\s*all/);
});
