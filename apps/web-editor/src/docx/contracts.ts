import { z } from 'zod';

const baseStyleSchema = z.object({styleId:z.string().max(120).optional(),bold:z.boolean().optional(),italic:z.boolean().optional(),underline:z.boolean().optional(),strike:z.boolean().optional(),color:z.string().regex(/^#[0-9a-f]{6}$/i).optional(),font:z.string().max(120).optional(),sizeHalfPoints:z.number().int().min(8).max(288).optional(),highlight:z.string().max(40).optional(),vertical:z.string().max(20).optional()});
const bookmarkName=z.string().regex(/^[A-Za-z][A-Za-z0-9_]{0,39}$/);
const fractionSchema = z.object({numerator:z.string().max(1027),denominator:z.string().max(1027)});

export const cropInset=z.number().int().min(0).max(99000);
const cropSchema=z.object({left:cropInset,top:cropInset,right:cropInset,bottom:cropInset})
  .refine(crop=>crop.left+crop.right<=99000&&crop.top+crop.bottom<=99000);
const runSchema = z.object({
  object:z.object({kind:z.enum(['math','control','embedded','textbox','shape','chart','smartart','drawing','footnote','endnote','tracked_insert','tracked_delete']),text:z.string().max(1027).optional(),author:z.string().max(163).optional(),date:z.string().max(30).optional(),label:z.string().max(163).optional(),editable:z.boolean(),deleteEditable:z.boolean().optional(),fraction:fractionSchema.optional()}).optional(),
  link:z.union([z.object({url:z.string().max(2048).regex(/^https?:\/\/\S+$/),bookmark:z.undefined().optional()}),
    z.object({bookmark:bookmarkName,url:z.undefined().optional()})]).optional(),
  picture:z.object({widthEmu:z.number().int().min(1).max(18288000),heightEmu:z.number().int().min(1).max(18288000),alt:z.string().max(1024),preview:z.boolean(),rotation:z.literal(180).optional(),rotationEditable:z.boolean().optional(),cropEditable:z.boolean().optional(),crop:cropSchema.optional()}).optional(),
  characterStyleId:z.string().max(120).optional(),baseStyle:baseStyleSchema.optional(),underline:z.boolean().optional(),strike:z.boolean().optional(),highlight:z.string().max(40).optional(),vertical:z.string().max(20).optional(),
  id: z.string(), text: z.string(), editable: z.boolean(), trackable:z.boolean().optional(), bold: z.boolean(), italic: z.boolean(),
  sizeHalfPoints: z.number().int().min(8).max(288).optional(),
  color: z.string().regex(/^#[0-9a-f]{6}$/i).optional(),
  font: z.string().max(120).regex(/^[^"\\\u0000-\u001f\u007f]+$/u).optional(),
});
const spacing = z.number().int().min(0).max(2880).optional();
const paragraphListSchema=z.object({id:z.number().int().min(0).max(2147483647).optional(),key:z.string().regex(/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/).optional(),level:z.number().int().min(0).max(8).optional()});
const listInfoSchema=z.object({id:z.number().int().positive().max(2147483647),levels:z.array(z.object({level:z.number().int().min(0).max(8),kind:z.string(),text:z.string(),start:z.number().int().nonnegative(),legal:z.boolean().optional(),restart:z.number().int().min(0).max(7).optional()})).max(9)});
const tabStopSchema=z.object({position:z.number().int().min(0).max(28800),align:z.enum(['left','center','right','decimal','bar','clear']),leader:z.enum(['none','dot','hyphen','underscore','heavy','middleDot']).optional()});
export type TabStop=z.infer<typeof tabStopSchema>;
const paragraphFormatSchema = z.object({tabs:z.array(tabStopSchema).max(32).optional(),list:paragraphListSchema.optional(),styleId:z.string().max(120).optional(),
 align:z.enum(['start','center','end','justify']).optional(),before:spacing,after:spacing,
 left:z.number().int().min(-14400).max(14400).optional(),right:z.number().int().min(-14400).max(14400).optional(),firstLine:z.number().int().min(0).max(14400).optional(),hanging:z.number().int().min(0).max(14400).optional(),
 line:z.number().int().min(20).max(2880).optional(),lineRule:z.enum(['auto','exact','atLeast']).optional(),bidi:z.boolean().optional(),textDirection:z.string().max(20).optional(),
});
const outlineLevel=z.number().int().min(0).max(9).optional();
const paragraphSchema = z.object({
  list:paragraphListSchema.optional(),listLabel:z.string().max(240).optional(),
  anchor:z.string().regex(/^[0-7][0-9A-F]{7}$/).optional(),
  outlineLevel,outlineOverride:outlineLevel,
  sectionId:z.string().optional(),styleId:z.string().max(120).optional(),id: z.string(), runs: z.array(runSchema), structureEditable: z.boolean().optional(),
  pageBreakBefore: z.boolean().optional(), pageBreakAfter: z.boolean().optional(),
  moveBookmarkUp:z.boolean().optional(),moveBookmarkDown:z.boolean().optional(),
  layout: paragraphFormatSchema.optional(),
});
const paperSize = z.number().int().min(2000).max(50000);
const paperMargin = z.number().int().min(0).max(50000);
const pageSchema = z.object({
  width: paperSize, height: paperSize,
  top: paperMargin, right: paperMargin, bottom: paperMargin, left: paperMargin,
}).refine(page => page.left + page.right <= page.width - 1200 && page.top + page.bottom <= page.height - 1200);
const headerFooterUpdateSchema=z.object({kind:z.enum(['header','footer']),variant:z.enum(['default','first','even']),linkPrevious:z.boolean().optional(),pageNumber:z.boolean().optional(),totalPages:z.boolean().optional(),text:z.string().max(65536).optional()});
const headerFooterInfoSchema=z.object({kind:z.enum(['header','footer']),variant:z.enum(['default','first','even']),linked:z.boolean(),sourceSectionId:z.string().optional(),pageNumber:z.boolean().optional(),totalPages:z.boolean().optional(),text:z.string().max(65536),editable:z.boolean(),truncated:z.boolean().optional()});
export type HeaderFooterInfo=z.infer<typeof headerFooterInfoSchema>;
const pageNumberFormatSchema=z.enum(['decimal','upperRoman','lowerRoman','upperLetter','lowerLetter']);
export const sectionFormatSchema=z.object({pageNumberStart:z.number().int().min(-1).max(2147483647).optional(),pageNumberFormat:pageNumberFormatSchema.optional(),headerFooters:z.array(headerFooterUpdateSchema).max(6).optional(),firstPage:z.boolean().optional(),width:paperSize.optional(),height:paperSize.optional(),top:paperMargin.optional(),right:paperMargin.optional(),bottom:paperMargin.optional(),left:paperMargin.optional(),orientation:z.enum(['portrait','landscape']).optional(),columns:z.number().int().min(1).max(12).optional(),columnSpace:z.number().int().min(0).max(14400).optional(),breakType:z.enum(['nextPage','continuous','evenPage','oddPage','nextColumn']).optional()});
const sectionSchema=z.object({pageNumberStart:z.number().int().min(0).max(2147483647).optional(),pageNumberFormat:z.string().optional(),headerFooters:z.array(headerFooterInfoSchema).max(6).optional(),firstPage:z.boolean().optional(),id:z.string(),startBlock:z.number().int().nonnegative(),endBlock:z.number().int().min(-1),page:pageSchema,orientation:z.enum(['portrait','landscape']),columns:z.number().int().min(1).max(12),columnSpace:z.number().int().min(0).max(14400),breakType:z.string().max(40),editable:z.boolean()});
export type SectionFormat=z.infer<typeof sectionFormatSchema>;
export type SectionInfo=z.infer<typeof sectionSchema>;
export type SectionOperation={type:'update_section';sectionId:string;section:SectionFormat};
const styleInfoSchema=z.object({outlineLevel,id:z.string().max(120),name:z.string().max(256),kind:z.enum(['paragraph','character']),basedOn:z.string().optional(),editable:z.boolean(),run:baseStyleSchema.extend({reset:z.boolean().optional()}).optional(),paragraph:paragraphFormatSchema.optional()});
export const snapshotSchema = z.object({
  lists:z.array(listInfoSchema).max(500).optional(),
  trackedChanges:z.boolean().optional(),
  comments:z.array(z.object({id:z.string(),author:z.string(),date:z.string().optional(),text:z.string(),paragraphId:z.string().optional()})).max(100).optional(),
  bookmarks:z.array(z.object({name:bookmarkName,paragraphId:z.string(),renameEditable:z.boolean().optional()})).max(500).optional(),
  evenAndOddHeaders: z.boolean().optional(),
 sections:z.array(sectionSchema).max(128).optional(),
  styles:z.array(styleInfoSchema).max(500).optional(),id: z.string(), name: z.string(), revision: z.number().int().nonnegative(),
  sequence: z.number().int().nonnegative(), page: pageSchema.optional(), warnings: z.array(z.string()),
  history: z.object({ undo: z.number().int().min(0).max(10), redo: z.number().int().min(0).max(10), undoActor: z.enum(['human', 'ai']).optional(), redoActor: z.enum(['human', 'ai']).optional() }).optional(),
  blocks: z.array(z.discriminatedUnion('type', [
    z.object({ id: z.string(), type: z.literal('paragraph'), paragraph: paragraphSchema }),
    z.object({ id: z.string(), type: z.literal('table'), tableEditable: z.boolean().optional(), rows: z.array(z.object({
      height: z.number().int().min(1).max(31680).optional(),
      cells: z.array(z.object({ id: z.string(), paragraphs: z.array(paragraphSchema) })),
    })) }),
  ])),
});
export const eventSchema = z.object({
  textOnly:z.boolean().optional(),
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
export type ListDefinition={key:string;kind:'bullet'|'decimal'|'upperRoman'|'lowerRoman'|'upperLetter'|'lowerLetter';start?:number};
export type ListOperation={type:'define_list';listDefinition:ListDefinition};
export type ParagraphList=z.infer<typeof paragraphListSchema>;
export type TableAction = 'insert' | 'delete' | 'add_row' | 'delete_row' | 'move_row' | 'add_column' | 'delete_column' | 'move_column' | 'set_row_height';
export type TableEdit = {action:TableAction;rows?:number;columns?:number;index?:number;destination?:number;height?:number};
export type TableOperation = {type:'update_table';targetId:string;table:TableEdit};
export type BoundaryMode = 'page_before' | 'no_page_before' | 'no_page_after' | 'section_next_page' | 'section_continuous' | 'remove_section' | 'move_section_earlier' | 'move_section_later';
export type BoundaryOperation = {type:'update_boundary'; targetId:string; boundary:BoundaryMode};
export type ImageOperation = {type:'update_image';targetId:string;image:{action:'insert';base64:string;alt?:string;widthEmu?:number}|{action:'set_alt';alt:string}|{action:'delete'}|{action:'resize';widthEmu:number}|{action:'replace';base64:string}|{action:'crop';crop:z.infer<typeof cropSchema>}|{action:'rotate';rotation:0|180}};
export type ReferenceEdit = {action:'insert_bookmark';bookmark:string} | {action:'rename_bookmark';bookmark:string;newBookmark:string} | {action:'delete_bookmark';bookmark:string} | {action:'insert_footnote'|'insert_endnote'|'set_footnote_text'|'set_endnote_text';text:string} | {action:'delete_footnote'|'delete_endnote'} |
  {action:'insert_link';text:string;url:string} | {action:'insert_link';text:string;bookmark:string} | {action:'set_link_url';url:string} | {action:'unlink'};
export type ReferenceOperation = {type:'update_reference';targetId:string;reference:ReferenceEdit};
export type CommentRange = {startRunId:string;endRunId:string;startOffset:number;endOffset:number};
export type CommentOperation = {type:'add_comment';targetId:string;comment:{text:string} & Partial<CommentRange>};
export type MathOperation = {type:'update_math';targetId:string;math:{action:'insert'|'replace'|'delete';expression?:string}|{action:'insert_fraction'|'replace_fraction';fraction:z.infer<typeof fractionSchema>}};
export type BookmarkMoveOperation = {type:'move_bookmark_up';targetId:string}|{type:'move_bookmark_down';targetId:string};
export type RevisionReviewOperation = {type:'accept_revision';targetId:string}|{type:'reject_revision';targetId:string};
export type TrackTextOperation = {type:'track_run_text';targetId:string;expectedText:string;text:string};
export type ObjectTextOperation = {type:'update_control_text'|'update_textbox_text';targetId:string;expectedText:string;text:string};
export type Operation = TrackTextOperation | ObjectTextOperation | RevisionReviewOperation | BookmarkMoveOperation | TextOperation | ParagraphOperation | StyleOperation | SectionOperation | { type: 'update_header_parity'; evenAndOddHeaders: boolean } | BoundaryOperation | ImageOperation | ReferenceOperation | CommentOperation | MathOperation | ListOperation | TableOperation;
export type DraftFocus = { runId: string; start: number; end: number; selection?: import('./selection').TextSelection };
export type StructureAction = 'split' | 'mergeBackward' | 'mergeForward' | 'moveUp' | 'moveDown';
