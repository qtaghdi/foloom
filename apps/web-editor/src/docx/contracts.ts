import { z } from 'zod';

export const runSchema = z.object({
  id: z.string(), text: z.string(), editable: z.boolean(), bold: z.boolean(), italic: z.boolean(),
});
const paragraphSchema = z.object({ id: z.string(), runs: z.array(runSchema) });
export const snapshotSchema = z.object({
  id: z.string(), name: z.string(), revision: z.number().int().nonnegative(),
  sequence: z.number().int().nonnegative(), warnings: z.array(z.string()),
  blocks: z.array(z.discriminatedUnion('type', [
    z.object({ id: z.string(), type: z.literal('paragraph'), paragraph: paragraphSchema }),
    z.object({ id: z.string(), type: z.literal('table'), rows: z.array(z.object({
      cells: z.array(z.object({ id: z.string(), paragraphs: z.array(paragraphSchema) })),
    })) }),
  ])),
});
export const eventSchema = z.object({
  sequence: z.number().int().nonnegative(), revision: z.number().int().nonnegative(),
  type: z.enum(['docx.committed', 'docx.reload']), actor: z.string().optional(),
});
export type Snapshot = z.infer<typeof snapshotSchema>;
export type Run = z.infer<typeof runSchema>;
export type Operation = {
  type: 'replace_run_text' | 'replace_cell_text'; targetId: string; expectedText: string; text: string;
};
