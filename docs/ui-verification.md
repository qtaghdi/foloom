# XLSX UI Verification

Date: 2026-10-12
Scope: X-10 browser and MCP App shell, interaction boundaries, and status UI

## Automated Evidence

The frontend tests verify that keyboard-confirmed cell edits become persisted
cell operations, clipboard paste becomes one atomic rectangular range
operation, and spreadsheet undo delegates to server-owned history. View tests
cover enabled and disabled history controls plus pending, saved, failed,
conflict, and recovery states.

Both HTML entry points are checked for a skip link, keyboard-focusable workbook
surface, live status regions, an assertive error region, toolbar semantics,
visible focus styles, 44 px history controls, and zoom-safe viewport metadata.
The stylesheet contract covers safe areas, small screens, and reduced motion.

## Viewport Review

The production browser bundle was served by the compiled Go application with
the LibreOffice compatibility fixture. Local headless Chrome 154.0.8037.98 was
reviewed at these viewport sizes:

| Viewport | Mode | Observed result |
| --- | --- | --- |
| 1440 x 900 | Desktop | Workbook identity, revision status, and history controls remained visible; status panels stayed within the viewport. |
| 1024 x 768 | Small laptop | The toolbar stayed on one row with no hidden critical controls or horizontal page overflow. |
| 390 x 844 | Mobile fallback | The toolbar wrapped into two rows; workbook identity and both history controls remained visible and touch-sized. |
| 390 x 844 | Reduced motion | The same mobile layout loaded with forced reduced-motion preference and no required information depended on animation. |

The macOS headless renderer did not paint the complete Univer canvas, so this
record certifies the application shell and responsive fallback rather than
pixel fidelity of grid cells. Grid editing, paste, and history integration are
covered at the application event boundary. A release candidate should still
repeat native interactive checks in a visible browser before public release.

## State Behavior

- Pending saves use an ellipsis, a live status region, a visible activity dot,
  and `aria-busy` on the workbook surface.
- Successful saves clear `aria-busy` and expose the new revision.
- Ordinary failures focus an assertive alert with an actionable heading.
- Conflict-like failures are labeled `Conflict detected`, pause editing, disable
  history controls, and offer an explicit reload of saved state.
- Recovery text preserves unsaved-draft guidance and never promises that an
  uncertain write succeeded.
- Navigation warns while writes are pending or recovery is required; canceled
  navigation keeps the controller and live subscription active.
