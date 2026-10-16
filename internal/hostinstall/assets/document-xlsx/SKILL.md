---
name: document-xlsx
description: Inspect and safely edit a local XLSX through the document-workspace MCP server when the user provides a workbook path and an editing task.
---

# Document XLSX

Use the document-workspace MCP tools. Do not parse the XLSX with shell commands or load the whole workbook into context.

1. Call `open_workbook` with the user-provided absolute path.
2. Open the returned `browserUrl` in the host browser surface so the user can inspect and directly edit the workbook. Do not use an inline image or MCP App preview as the editing surface, and do not repeat the short-lived URL in chat.
3. Call `get_workbook`, then read only the ranges needed to understand the request.
4. Summarize the intended changes before a destructive tool call when the host has not already presented an approval.
5. Use the latest revision and apply one coherent `apply_operations` batch when practical.
6. After a conflict, refresh and present the revised change instead of retrying blindly.
7. Re-read changed ranges and report the saved revision. Mention compatibility warnings relevant to the edit.

Never invent a path, search unrelated directories, expose workbook contents in logs, or claim native Excel fidelity without verification.
