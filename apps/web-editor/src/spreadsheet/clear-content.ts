import type { WorkbookOperation } from '../contracts';
import { toA1Range } from './a1';

type ClearRange = { top: number; bottom: number; left: number; right: number };
const maxRangeCells = 10_000;

export function clearContentOperations(sheet: string, cellValue: Record<number, Record<number, unknown>>): WorkbookOperation[] {
  const ranges: ClearRange[] = [];
  const lastByColumns = new Map<string, ClearRange>();
  const append = (row: number, left: number, right: number) => {
    const key = `${left}:${right}`;
    const previous = lastByColumns.get(key);
    if (previous && previous.bottom === row - 1 && (row - previous.top + 1) * (right - left + 1) <= maxRangeCells) {
      previous.bottom = row;
      return;
    }
    const range = { top: row, bottom: row, left, right };
    ranges.push(range);
    lastByColumns.set(key, range);
  };
  // The mutation contains only affected cells, including holes for filtered rows.
  for (const [key, cells] of Object.entries(cellValue)) {
    const row = Number(key);
    const columns = Object.keys(cells).map(Number);
    let left = columns[0];
    let right = left;
    if (left === undefined || right === undefined) continue;
    for (const column of columns.slice(1)) {
      if (column !== right + 1 || column - left >= maxRangeCells) {
        append(row, left, right);
        left = column;
      }
      right = column;
    }
    append(row, left, right);
  }
  return ranges.map(({ top, bottom, left, right }) => {
    const range = toA1Range(top, left, bottom, right);
    return top === bottom && left === right
      ? { type: 'set_cell', sheet, cell: range, value: null }
      : { type: 'paste_range', sheet, range, cells: Array.from({ length: bottom - top + 1 },
          () => Array.from({ length: right - left + 1 }, () => ({ value: null }))) };
  });
}
