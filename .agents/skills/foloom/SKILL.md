---
name: foloom
description: Inspect, create DOCX, or edit local XLSX and DOCX files through the Foloom MCP server when the user gives a file path or names an open workspace document.
---

# Foloom

Use the Foloom MCP tools for the requested file. Route by its
extension or the active document format. Ask for an absolute path if no
matching document is open and the user has not supplied a path. For creation,
use the user-specified destination. Do not search unrelated directories.

## XLSX

1. Open an existing file with `open_workbook`. Call `get_workbook` to check the
   active workbook and revision, then use `read_range` for the affected cells.
2. Use one coherent `apply_operations` batch with the current `baseRevision`.
   Use `restore_history` with the current revision when the user requests
   undo or redo. A batch is one history step.
3. Re-read the affected cells and report the saved revision and any relevant
   compatibility warnings.

## DOCX

1. Use `open_docx` for an existing file or `create_docx` with ordered paragraph
   and simple table blocks for a new file. Use `get_docx` to inspect the active
   document, its revision, and the editable text targets.
2. For existing text, use `apply_docx_operations` with the current
   `sessionId`, `baseRevision`, target ID, and `expectedText`. Choose
   `replace_run_text` for body text and `replace_cell_text` for table text.
   Preserve unsupported content. Server undo and redo are unavailable for DOCX.
3. Re-read with `get_docx` and report the saved revision and any relevant
   compatibility warnings.

## Shared workflow

Use the returned `browserUrl` in the host browser when the user wants to see
or edit the file. Keep the short-lived URL out of chat and logs. If a write
conflicts, refresh the document and check the intended targets before
resubmitting. If the outcome is unknown, read the current revision and content
before attempting another write. Report only verified changes and do not claim
native Excel or Word fidelity without direct verification.
