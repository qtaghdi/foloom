import type { CellChange, SheetObjects, WorkbookRange, WorkbookSnapshot } from '../contracts';

type Preview = { snapshot: WorkbookSnapshot; ranges: WorkbookRange[]; objects: SheetObjects[] };
export type HistoryContentChange = { sheet: string; cells: CellChange[] };

const same = (left: unknown, right: unknown) => JSON.stringify(left) === JSON.stringify(right);

export function historyContentChanges(before: Preview, after: Preview): HistoryContentChange[] | undefined {
  if (before.snapshot.id !== after.snapshot.id ||
      !same(before.snapshot.sheets, after.snapshot.sheets) ||
      !same(before.snapshot.sheetDimensions, after.snapshot.sheetDimensions) ||
      before.objects.some(object => object.truncated) || after.objects.some(object => object.truncated) ||
      !same(before.objects, after.objects) || before.ranges.length !== after.ranges.length) return;

  const changes: HistoryContentChange[] = [];
  for (const [index, oldRange] of before.ranges.entries()) {
    const nextRange = after.ranges[index]!;
    if (oldRange.sheet !== nextRange.sheet || oldRange.ref !== nextRange.ref ||
        !same(oldRange.merges, nextRange.merges) || !same(oldRange.validations, nextRange.validations) ||
        !same(oldRange.conditionalFormatting, nextRange.conditionalFormatting) ||
        oldRange.rows.length !== nextRange.rows.length) return;
    const cells: CellChange[] = [];
    for (const [rowIndex, oldRow] of oldRange.rows.entries()) {
      const nextRow = nextRange.rows[rowIndex]!;
      if (oldRow.length !== nextRow.length) return;
      for (const [column, oldCell] of oldRow.entries()) {
        const nextCell = nextRow[column]!;
        if (oldCell.address !== nextCell.address || !same(oldCell.style, nextCell.style)) return;
        if (oldCell.formula === nextCell.formula && (nextCell.formula || oldCell.value === nextCell.value)) continue;
        cells.push({ cell: nextCell.address, formula: nextCell.formula, value: nextCell.value === '' ? null : nextCell.value });
      }
    }
    if (cells.length) changes.push({ sheet: nextRange.sheet, cells });
  }
  return changes;
}
