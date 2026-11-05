import { expect, it } from 'vitest';
import { clearContentOperations } from '../src/spreadsheet/clear-content';

it('clears only the mutated cells and groups contiguous cells for one batch', () => {
  expect(clearContentOperations('Sheet1', {
    0: { 1: {}, 2: {}, 4: {} },
    1: { 1: {}, 2: {} },
    3: { 1: {} },
  })).toEqual([
    { type: 'paste_range', sheet: 'Sheet1', range: 'B1:C2', cells: [[{ value: null }, { value: null }], [{ value: null }, { value: null }]] },
    { type: 'set_cell', sheet: 'Sheet1', cell: 'E1', value: null },
    { type: 'set_cell', sheet: 'Sheet1', cell: 'B4', value: null },
  ]);
});
