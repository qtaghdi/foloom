import { sectionPaper } from './sections';
import type { SectionInfo, Snapshot } from './contracts';

type Paper = NonNullable<Snapshot['page']>;
const defaultPaper: Paper = { width: 11906, height: 16838, top: 1440, right: 1440, bottom: 1440, left: 1440 };

export function printPaperName(paper: Paper): string {
  return 'dw-' + [paper.width,paper.height,paper.top,paper.right,paper.bottom,paper.left].join('-');
}

export function printPaperRules(snapshot: Snapshot): string {
  const defaultLayout = snapshot.page ?? defaultPaper;
  const layouts = new Map([[printPaperName(defaultLayout),defaultLayout]]);
  for (const section of snapshot.sections??[]) {
    const paper=sectionPaper(section);
    layouts.set(printPaperName(paper),paper);
  }
  const points = (twips: number) => `${twips/20}pt`;
  return [...layouts].map(([name,paper]) =>
    `@page ${name} { size: ${points(paper.width)} ${points(paper.height)}; margin: ${[paper.top,paper.right,paper.bottom,paper.left].map(points).join(' ')}; }`
  ).join('\n');
}
export function printPaperFor(snapshot: Snapshot, section?: SectionInfo): string {
  return printPaperName(section?sectionPaper(section):snapshot.page??defaultPaper);
}

export async function printDocument(article: HTMLElement, current: () => boolean): Promise<boolean> {
  const doc = article.ownerDocument;
  const view = doc.defaultView;
  if (!view) throw new Error('Print window unavailable');
  const pictures = [...article.querySelectorAll('img')];
  for (const picture of pictures) picture.loading = 'eager';
  await Promise.all([doc.fonts?.ready,...pictures.map(picture=>picture.decode())]);
  if (!current()) return false;
  view.print();
  return true;
}
