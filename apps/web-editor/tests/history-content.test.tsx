import { expect, it } from 'vitest';
import { historyContentChanges } from '../src/spreadsheet/history-content';
import type { WorkbookCell, WorkbookSnapshot } from '../src/contracts';

const snapshot: WorkbookSnapshot = {
  id: 'workbook', name: 'book.xlsx', sheets: ['Sheet1'], sheetDimensions: [{ name: 'Sheet1', rows: 1, columns: 3 }],
  revision: 1, sequence: 1, canUndo: true, canRedo: false,
};
const preview = (cells: WorkbookCell[]) => ({
  snapshot, ranges: [{ sheet: 'Sheet1', ref: 'A1:C1', rows: [cells] }], objects: [{ sheet: 'Sheet1' }],
});
const before = preview([{ address: 'A1', value: 'keep' }, { address: 'B1', value: '' }, { address: 'C1', value: '0', formula: '=LEN(B1)' }]);

it('only updates edited content and leaves dependent formula rendering to the engine', () => {
  const after = preview([{ address: 'A1', value: 'keep' }, { address: 'B1', value: 'Amount' }, { address: 'C1', value: '6', formula: '=LEN(B1)' }]);
  expect(historyContentChanges(before, after)).toEqual([{ sheet: 'Sheet1', cells: [{ cell: 'B1', formula: undefined, value: 'Amount' }] }]);
  expect(historyContentChanges(after, before)).toEqual([{ sheet: 'Sheet1', cells: [{ cell: 'B1', formula: undefined, value: null }] }]);
});

it('keeps the replacement path when style, structure, or preview coverage differs', () => {
  const styled = preview([{ address: 'A1', value: 'keep', style: { bold: true } }, ...before.ranges[0]!.rows[0]!.slice(1)]);
  expect(historyContentChanges(before, styled)).toBeUndefined();
  expect(historyContentChanges(before, { ...before, ranges: [{ ...before.ranges[0]!, merges: ['A1:B1'] }] })).toBeUndefined();
  expect(historyContentChanges(before, { ...before, objects: [{ sheet: 'Sheet1', truncated: true }] })).toBeUndefined();
});
