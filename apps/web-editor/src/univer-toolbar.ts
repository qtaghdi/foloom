import { UndoCommand, RedoCommand } from '@univerjs/core';
const toolbar = '[data-u-comp="ribbon-toolbar"]';
const menuTrigger = '[data-u-command][data-slot="dropdown-menu-trigger"]';
const overflowTrigger = `${toolbar} button[data-slot="popover-trigger"]`;
const fields = new Map([
  ['sheet.command.set-range-font-family', '글꼴'],
  ['sheet.command.set-range-fontsize', '글꼴 크기'],
]);

export function bindToolbarAccessibility(container: HTMLElement): () => void {
  const doc = container.ownerDocument;
  let focusOverflow = false;
  const overflowPanel = () => {
    const trigger = container.querySelector(`${overflowTrigger}[aria-expanded="true"]`);
    return doc.getElementById(trigger?.getAttribute('aria-controls') ?? '');
  };
  const overflowControls = (panel: HTMLElement) => [...panel.querySelectorAll<HTMLElement>(
    'button:not(:disabled), input:not(:disabled), [tabindex="0"]',
  )].filter(control => control.getAttribute('aria-disabled') !== 'true' && control.getClientRects().length > 0);
  const update = () => {
    for (const [command, action, label, shortcut] of [
      [UndoCommand.id, 'undo', '실행 취소', 'Control+Z Meta+Z'],
      [RedoCommand.id, 'redo', '다시 실행', 'Control+Y Meta+Shift+Z'],
    ]) {
      for (const button of container.querySelectorAll<HTMLButtonElement>(`button[data-u-command="${command}"]`)) {
        button.dataset.workspaceHistory = action;
        button.setAttribute('aria-label', label!);
        button.setAttribute('title', `${label} · ${action === 'undo' ? 'Ctrl/⌘ Z' : 'Ctrl Y / ⌘ ⇧ Z'}`);
        button.setAttribute('aria-keyshortcuts', shortcut!);
      }
    }
    const more = container.querySelector(overflowTrigger);
    more?.setAttribute('aria-label', '서식 도구 더보기');
    const panel = overflowPanel();
    panel?.setAttribute('data-workspace-toolbar-overflow', '');
    for (const root of [container.querySelector(toolbar), panel]) {
      for (const trigger of root?.querySelectorAll<HTMLElement>(menuTrigger) ?? []) {
        const label = fields.get(trigger.dataset.uCommand ?? '');
        const input = trigger.querySelector('input');
        if (!label || !input) continue;
        // Radix returns focus to its trigger after a menu closes.
        trigger.tabIndex = -1;
        input.setAttribute('aria-label', label);
        input.setAttribute('aria-haspopup', 'menu');
        input.setAttribute('aria-keyshortcuts', 'Alt+ArrowDown');
        input.setAttribute('title', `${label} · Alt + ↓ 메뉴 열기`);
      }
    }
    if (panel && focusOverflow) {
      focusOverflow = false;
      overflowControls(panel)[0]?.focus();
    }
  };
  const fieldTrigger = (element: HTMLElement) => {
    const trigger = element.closest<HTMLElement>(menuTrigger);
    if (!trigger || !fields.has(trigger.dataset.uCommand ?? '')) return;
    if ((container.contains(trigger) && trigger.closest(toolbar)) || overflowPanel()?.contains(trigger)) return trigger;
  };
  const onKeyDown = (event: KeyboardEvent) => {
    const target = event.target;
    if (target instanceof HTMLElement && container.contains(target) && target.matches(overflowTrigger) &&
        (event.key === 'Enter' || event.key === ' ')) {
      focusOverflow = target.getAttribute('aria-expanded') === 'false';
    }
    const panel = overflowPanel();
    if (event.key === 'Tab' && panel && target instanceof HTMLElement && panel.contains(target)) {
      // Univer's sheet-level Tab handler also sees events from this portal.
      event.preventDefault();
      event.stopPropagation();
      const controls = overflowControls(panel);
      const next = controls[controls.indexOf(target) + (event.shiftKey ? -1 : 1)];
      if (next) {
        next.focus();
      } else {
        const more = container.querySelector<HTMLButtonElement>(overflowTrigger);
        more?.click();
        more?.focus();
      }
      return;
    }
    if (event.key !== 'ArrowDown' || !event.altKey || event.ctrlKey || event.metaKey || event.shiftKey ||
        event.isComposing || !(target instanceof HTMLInputElement) || target.disabled) return;
    const trigger = fieldTrigger(target);
    if (!trigger) return;
    event.preventDefault();
    event.stopPropagation();
    // Let the engine open and focus its own menu as it does for other toolbar triggers.
    trigger.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }));
  };
  const onFocus = (event: FocusEvent) => {
    const target = event.target;
    if (target instanceof HTMLElement && target === fieldTrigger(target)) target.querySelector('input')?.focus();
  };
  const observer = new MutationObserver(update);
  observer.observe(container, { childList: true, subtree: true, attributes: true, attributeFilter: ['aria-expanded'] });
  // Univer portals overflow outside the engine container.
  observer.observe(doc.body, { childList: true });
  doc.addEventListener('keydown', onKeyDown, true);
  doc.addEventListener('focusin', onFocus);
  update();
  return () => {
    observer.disconnect();
    doc.removeEventListener('keydown', onKeyDown, true);
    doc.removeEventListener('focusin', onFocus);
  };
}
