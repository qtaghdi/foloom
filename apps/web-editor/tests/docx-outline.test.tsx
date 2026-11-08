import { render } from 'solid-js/web';
import { createSignal } from 'solid-js';
import { afterEach, expect, it, vi } from 'vitest';
import type { Snapshot } from '../src/docx/contracts';
import { DocumentOutline } from '../src/docx/document-outline';

let dispose = () => {};
afterEach(() => { dispose(); document.body.replaceChildren(); });

it('locates saved bookmarks, clears old locations and restores focus on Escape', () => {
  const original: Snapshot = {
    id: 'first', name: 'first.docx', revision: 1, sequence: 0, warnings: [], blocks: [],
    bookmarks: [{name: 'Summary', paragraphId: 'p1'}],
  };
  const [snapshot, setSnapshot] = createSignal(original);
  const locate = vi.fn();
  dispose = render(() => <DocumentOutline snapshot={snapshot()} styles={[]} onLocate={locate}/>, document.body);
  const toggle = document.querySelector<HTMLButtonElement>('button')!;
  toggle.click();
  const bookmark = () => document.querySelector<HTMLButtonElement>('[aria-label=북마크] button')!;

  bookmark().click();
  expect(locate).toHaveBeenCalledWith('p1');
  expect(bookmark().getAttribute('aria-current')).toBe('location');

  setSnapshot({...original, revision: 2});
  expect(bookmark().hasAttribute('aria-current')).toBe(false);
  bookmark().dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true}));
  expect(document.querySelector('nav')).toBeNull();
  expect(document.activeElement).toBe(toggle);
});
