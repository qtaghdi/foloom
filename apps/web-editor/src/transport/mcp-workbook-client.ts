import { savedReceiptSchema, type DocumentCheckpoint } from './document-files';
import { App, PostMessageTransport } from '@modelcontextprotocol/ext-apps';
import type { z } from 'zod';

import type { HistoryDirection, SelectionChange, WorkbookEvent, WorkbookOperation, WorkbookSnapshot } from '../contracts';
import {
  searchResultSchema,
  applyResponseSchema,
  eventPollResponseSchema,
  presenceResponseSchema,
  workbookRangeSchema,
  workbookSnapshotSchema,
  sheetObjectsSchema,
} from './schemas';
import type { EventSubscription, WorkbookClient } from './workbook-client';

export class MCPWorkbookClient implements WorkbookClient {
  private documentId?: string;
  private app: App | undefined;

  async connect(): Promise<void> {
    this.app = new App(
      { name: 'document-workspace', version: '0.1.0' },
      { availableDisplayModes: ['inline', 'fullscreen'] },
      { autoResize: true },
    );
    await this.app.connect(new PostMessageTransport(window.parent, window.parent));
  }

  async search(query: string) {
    return this.callTool('search_workbook', { query, documentId: this.documentId }, searchResultSchema);
  }

  async confirmSave(checkpoint: DocumentCheckpoint) {
    const receipt = await this.callTool('confirm_workbook_save', { sessionId: checkpoint.id, baseRevision: checkpoint.revision }, savedReceiptSchema);
    if (receipt.id !== checkpoint.id || receipt.revision !== checkpoint.revision || receipt.name !== checkpoint.name) throw new Error('document revision conflict during save confirmation');
  }

  async getWorkbook() {
    const snapshot = await this.callTool('get_workbook', { documentId: this.documentId }, workbookSnapshotSchema);
    this.documentId = snapshot.id;
    return snapshot;
  }

  async readRange(sheet: string, range: string) {
    return this.callTool('read_range', { sheet, range, documentId: this.documentId }, workbookRangeSchema);
  }

  async readSheetObjects(sheet: string) {
    return this.callTool('get_sheet_objects', { sheet, documentId: this.documentId }, sheetObjectsSchema);
  }

  async applyOperations(baseRevision: number, operations: WorkbookOperation[]) {
    return this.callTool('apply_user_operations', { baseRevision, operations, documentId: this.documentId }, applyResponseSchema);
  }

  async restoreHistory(baseRevision: number, direction: HistoryDirection) {
    return this.callTool('restore_user_history', { baseRevision, direction, documentId: this.documentId }, applyResponseSchema);
  }

  async updatePresence(selection: SelectionChange): Promise<void> {
    await this.callTool('update_user_presence', { ...selection }, presenceResponseSchema);
  }

  subscribe(onEvent: (event: WorkbookEvent) => void, checkpoint: WorkbookSnapshot): EventSubscription {
    let closed = false;
    let timer: number | undefined;
    let afterSequence = checkpoint.sequence;
    const subscription: EventSubscription = {
      onerror: null,
      close: () => {
        closed = true;
        window.clearTimeout(timer);
      },
    };
    const poll = async (): Promise<void> => {
      try {
        const response = await this.callTool('get_events', { afterSequence, documentId: this.documentId }, eventPollResponseSchema);
        if (closed) return;
        if (response.workbook.id !== checkpoint.id) {
          onEvent({ type: 'workbook.reload', state: 'resync', actor: 'system',
            sequence: response.workbook.sequence, revision: response.workbook.revision });
          return;
        }
        for (const event of response.events) {
          afterSequence = Math.max(afterSequence, event.sequence);
          onEvent(event);
        }
      } catch (error) {
        subscription.onerror?.(error);
      } finally {
        if (!closed) {
          timer = window.setTimeout(() => void poll(), 500);
        }
      }
    };
    void poll();
    return subscription;
  }

  private async callTool<T>(name: string, args: Record<string, unknown>, schema: z.ZodType<T>): Promise<T> {
    if (!this.app) {
      throw new Error('MCP App bridge is not connected');
    }
    const result = await this.app.callServerTool({ name, arguments: args });
    if (result.isError) {
      const message = result.content
        .filter((item): item is { type: 'text'; text: string } => item.type === 'text')
        .map((item) => item.text)
        .join('\n');
      throw new Error(message || `Tool ${name} failed`);
    }
    if (!result.structuredContent) {
      throw new Error(`Tool ${name} returned no structured content`);
    }
    return schema.parse(result.structuredContent);
  }
}
