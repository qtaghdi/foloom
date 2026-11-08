import { z } from 'zod';

const baseStyleSchema = z.object({styleId:z.string().max(120).optional(),bold:z.boolean().optional(),italic:z.boolean().optional(),underline:z.boolean().optional(),strike:z.boolean().optional(),color:z.string().regex(/^#[0-9a-f]{6}$/i).optional(),font:z.string().max(120).optional(),sizeHalfPoints:z.number().int().min(8).max(288).optional(),highlight:z.string().max(40).optional(),vertical:z.string().max(20).optional()});
export const runSchema = z.object({
  characterStyleId:z.string().max(120).optional(),baseStyle:baseStyleSchema.optional(),underline:z.boolean().optional(),strike:z.boolean().optional(),highlight:z.string().max(40).optional(),vertical:z.string().max(20).optional(),
  id: z.string(), text: z.string(), editable: z.boolean(), bold: z.boolean(), italic: z.boolean(),
  sizeHalfPoints: z.number().int().min(8).max(288).optional(),
  color: z.string().regex(/^#[0-9a-f]{6}$/i).optional(),
  font: z.string().max(120).regex(/^[^"\\\u0000-\u001f\u007f]+$/u).optional(),
});
const spacing = z.number().int().min(0).max(2880).optional();
const paragraphFormatSchema = z.object({styleId:z.string().max(120).optional(),
 align:z.enum(['start','center','end','justify']).optional(),before:spacing,after:spacing,
 left:z.number().int().min(-14400).max(14400).optional(),right:z.number().int().min(-14400).max(14400).optional(),firstLine:z.number().int().min(0).max(14400).optional(),hanging:z.number().int().min(0).max(14400).optional(),
 line:z.number().int().min(20).max(2880).optional(),lineRule:z.enum(['auto','exact','atLeast']).optional(),bidi:z.boolean().optional(),textDirection:z.string().max(20).optional(),
});
const paragraphSchema = z.object({
  sectionId:z.string().optional(),styleId:z.string().max(120).optional(),id: z.string(), runs: z.array(runSchema), structureEditable: z.boolean().optional(),
  pageBreakBefore: z.boolean().optional(), pageBreakAfter: z.boolean().optional(),
  layout: paragraphFormatSchema.optional(),
});
const paperSize = z.number().int().min(2000).max(50000);
const paperMargin = z.number().int().min(0).max(50000);
const pageSchema = z.object({
  width: paperSize, height: paperSize,
  top: paperMargin, right: paperMargin, bottom: paperMargin, left: paperMargin,
}).refine(page => page.left + page.right <= page.width - 1200 && page.top + page.bottom <= page.height - 1200);
const headerFooterUpdateSchema=z.object({kind:z.enum(['header','footer']),variant:z.enum(['default','first','even']),linkPrevious:z.boolean().optional(),text:z.string().max(65536).optional()});
const headerFooterInfoSchema=z.object({kind:z.enum(['header','footer']),variant:z.enum(['default','first','even']),linked:z.boolean(),sourceSectionId:z.string().optional(),text:z.string().max(65536),editable:z.boolean(),truncated:z.boolean().optional()});
export type HeaderFooterInfo=z.infer<typeof headerFooterInfoSchema>;
const pageNumberFormatSchema=z.enum(['decimal','upperRoman','lowerRoman','upperLetter','lowerLetter']);
export const sectionFormatSchema=z.object({pageNumberStart:z.number().int().min(-1).max(2147483647).optional(),pageNumberFormat:pageNumberFormatSchema.optional(),headerFooters:z.array(headerFooterUpdateSchema).max(6).optional(),firstPage:z.boolean().optional(),width:paperSize.optional(),height:paperSize.optional(),top:paperMargin.optional(),right:paperMargin.optional(),bottom:paperMargin.optional(),left:paperMargin.optional(),orientation:z.enum(['portrait','landscape']).optional(),columns:z.number().int().min(1).max(12).optional(),columnSpace:z.number().int().min(0).max(14400).optional(),breakType:z.enum(['nextPage','continuous','evenPage','oddPage','nextColumn']).optional()});
const sectionSchema=z.object({pageNumberStart:z.number().int().min(0).max(2147483647).optional(),pageNumberFormat:z.string().optional(),headerFooters:z.array(headerFooterInfoSchema).max(6).optional(),firstPage:z.boolean().optional(),id:z.string(),startBlock:z.number().int().nonnegative(),endBlock:z.number().int().min(-1),page:pageSchema,orientation:z.enum(['portrait','landscape']),columns:z.number().int().min(1).max(12),columnSpace:z.number().int().min(0).max(14400),breakType:z.string().max(40),editable:z.boolean()});
export type SectionFormat=z.infer<typeof sectionFormatSchema>;
export type SectionInfo=z.infer<typeof sectionSchema>;
export type SectionOperation={type:'update_section';sectionId:string;section:SectionFormat};
const styleInfoSchema=z.object({id:z.string().max(120),name:z.string().max(256),kind:z.enum(['paragraph','character']),basedOn:z.string().optional(),editable:z.boolean(),run:baseStyleSchema.extend({reset:z.boolean().optional()}).optional(),paragraph:paragraphFormatSchema.optional()});
export const snapshotSchema = z.object({
  evenAndOddHeaders: z.boolean().optional(),
 sections:z.array(sectionSchema).max(128).optional(),
  styles:z.array(styleInfoSchema).max(500).optional(),id: z.string(), name: z.string(), revision: z.number().int().nonnegative(),
  sequence: z.number().int().nonnegative(), page: pageSchema.optional(), warnings: z.array(z.string()),
  history: z.object({ undo: z.number().int().min(0).max(10), redo: z.number().int().min(0).max(10), undoActor: z.enum(['human', 'ai']).optional(), redoActor: z.enum(['human', 'ai']).optional() }).optional(),
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
export type Paragraph = z.infer<typeof paragraphSchema>;
export type Run = z.infer<typeof runSchema>;
export type TextOperation = {
  type: 'replace_run_text' | 'replace_cell_text'; targetId: string; expectedText: string; text: string;
};

export type StyleInfo=z.infer<typeof styleInfoSchema>;
export type StyleDefinition={id:string;name?:string;kind:'paragraph'|'character';basedOn?:string;run?:InlineStyle;paragraph?:ParagraphFormat};
export type InlineStyle = {styleId?:string; reset?: boolean; underline?: boolean; strike?: boolean; highlight?: string; vertical?: string; bold?: boolean; italic?: boolean; color?: string; font?: string; sizeHalfPoints?: number };
export type ParagraphFormat = z.infer<typeof paragraphFormatSchema>;
export type ParagraphOperation = {
 type: 'replace_paragraphs'; targetId: string; endTargetId: string; expectedText: string; text: '';
 paragraphs: Array<{ sourceId: string; format?: ParagraphFormat; runs: Array<{ sourceId: string; text: string; style?: InlineStyle }> }>;
};
export type StyleOperation={type:'define_style';styleDefinition:StyleDefinition};
export type BoundaryMode = 'page_before' | 'no_page_before' | 'section_next_page' | 'section_continuous';
export type BoundaryOperation = {type:'update_boundary'; targetId:string; boundary:BoundaryMode};
export type Operation = TextOperation | ParagraphOperation | StyleOperation | SectionOperation | { type: 'update_header_parity'; evenAndOddHeaders: boolean } | BoundaryOperation;
export type DraftFocus = { runId: string; start: number; end: number; selection?: import('./selection').TextSelection };
export type StructureAction = 'split' | 'mergeBackward' | 'mergeForward' | 'moveUp' | 'moveDown';
