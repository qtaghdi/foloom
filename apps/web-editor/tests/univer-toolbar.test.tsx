import { expect, it, vi } from 'vitest';
import { bindToolbarAccessibility } from '../src/univer-toolbar';

it('labels overflow tools and returns keyboard focus to the font input', async () => {
  const container = document.createElement('div');
  container.innerHTML = `<div data-u-comp="ribbon-toolbar">
    <button data-slot="popover-trigger" aria-expanded="false" aria-controls="test-overflow"></button>
  </div>`;
  document.body.append(container);
  const dispose = bindToolbarAccessibility(container);
  const more = container.querySelector('button')!;
  const panel = document.createElement('div');
  panel.id = 'test-overflow';
  panel.innerHTML = `<div data-u-command="sheet.command.set-range-font-family" data-slot="dropdown-menu-trigger"><input></div><button>서식</button>`;
  try {
    for (const control of panel.querySelectorAll('input, button')) {
      vi.spyOn(control, 'getClientRects').mockReturnValue([{ width: 24 }] as unknown as DOMRectList);
    }
    expect(more.getAttribute('aria-label')).toBe('서식 도구 더보기');
    more.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    more.setAttribute('aria-expanded', 'true');
    document.body.append(panel);
    await Promise.resolve();
    const input = panel.querySelector('input')!;
    const trigger = input.parentElement!;
    expect(document.activeElement).toBe(input);
    expect(input.getAttribute('aria-label')).toBe('글꼴');
    const keys: string[] = [];
    trigger.addEventListener('keydown', event => keys.push(event.key));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', altKey: true, bubbles: true, cancelable: true }));
    expect(keys).toEqual(['ArrowDown']);
    trigger.focus();
    expect(document.activeElement).toBe(input);
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }));
    expect(document.activeElement).toBe(panel.querySelector('button'));
  } finally {
    dispose();
    panel.remove();
    container.remove();
  }
});

it('styles history commands as labeled icons without replacing command handlers or disabled state', () => {
  const container = document.createElement('div');
  container.innerHTML = '<button data-u-command="univer.command.undo" disabled><svg/></button><button data-u-command="univer.command.redo"><svg/></button>';
  document.body.append(container);
  const redo = container.querySelectorAll('button')[1]!;
  const action = vi.fn(); redo.addEventListener('click', action);
  const dispose = bindToolbarAccessibility(container);
  const undo = container.querySelector('button')!;
  expect(undo.dataset.workspaceHistory).toBe('undo'); expect(undo.disabled).toBe(true);
  expect(undo.getAttribute('aria-label')).toBe('실행 취소'); expect(undo.title).toContain('Ctrl/⌘ Z');
  expect(redo.dataset.workspaceHistory).toBe('redo'); expect(redo.getAttribute('aria-keyshortcuts')).toContain('Meta+Shift+Z');
  redo.click(); expect(action).toHaveBeenCalledOnce();
  dispose(); container.remove();
});
