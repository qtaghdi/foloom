---
name: document-editor-ui
description: Refine and verify document-workspace editor interaction, visual fidelity, keyboard focus, pointer states, motion, and activity navigation in the XLSX and DOCX browser surfaces.
---

# Document editor UI

Use this skill for editor UI implementation and visual review. Start with the
affected surface, the current branch, and a disposable sample document. Read
`AGENTS.md` and the relevant sections of `docs/ui-renewal.md`.

## Interaction decisions

- Keep one clear editing surface per format. XLSX uses the engine ribbon,
  formula bar, grid, and sheet tabs. Use shared application controls for
  file status, activity, and recovery. DOCX keeps writing and its AI markers
  in the paper surface.
- Reuse `editor-action` for application buttons. Check rest, hover, pressed,
  keyboard focus, and disabled states. Keep transitions short and local.
  Leave layout, selection, and the caret stable. Allow equivalent feedback
  on touch and with reduced motion.
- Use semantic pointer cues. Preserve the sheet engine's canvas cursors
  for selection, resize, drag, and text editing. Editable DOCX text uses an
  I-beam. Disabled controls communicate why through the surrounding UI.
- Keep activity navigation usable by keyboard. Verify Escape and focus
  return, including the narrow layout where activity replaces the canvas.
  Confirm the live editor is still mounted after closing activity.
- Treat canvas motion separately from CSS. Verify that reduced motion
  keeps an intelligible AI range highlight and that idle clears it.
- Inspect the source workbook style before changing a rendered color.
  Preserve real document colors. Test any synthetic fixture correction
  without touching a user's document.

## Verification and output

Build the browser assets and open the real local editor on a copy. Inspect
desktop, small laptop, and mobile layouts. Exercise the changed interaction
with mouse and keyboard, and check reduced motion using browser emulation or
an operating-system preference if available. Check actual save and reload
when controls can write. For XLSX toolbar work, verify server undo and redo
after reload instead of relying on the engine's local history.

Run `make web-test` and the relevant build or package smoke check. Use the
architecture skill for changed persistence or transport boundaries, and
the compatibility skill for changes that can affect XLSX round trips.
Record the observed interactions, viewport sizes, relevant checks, and any
untested environment. Keep test files and screenshots out of commits.
