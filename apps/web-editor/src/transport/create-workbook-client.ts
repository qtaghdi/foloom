import { HTTPWorkbookClient } from './http-workbook-client';
import { MCPWorkbookClient } from './mcp-workbook-client';
import type { WorkbookClient } from './workbook-client';

export function createWorkbookClient(): WorkbookClient {
  return (window.parent === window || new URLSearchParams(window.location.search).has('document')) ? new HTTPWorkbookClient() : new MCPWorkbookClient();
}
