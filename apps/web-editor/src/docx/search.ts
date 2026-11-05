import type { SearchResults } from '../ui/document-search';

// Read current fields so local drafts participate without saving or rerendering.
// Concatenating runs also finds phrases split by formatting within a paragraph.
export function searchDocument(root: HTMLElement | null, query: string): SearchResults {
  const items: SearchResults['items'] = [];
  const needle = query.toLowerCase();
  if (!needle.trim() || !root) return { items };
  for (const paragraph of root.querySelectorAll<HTMLElement>('.docx-paragraph')) {
    const fields = [...paragraph.querySelectorAll<HTMLElement>('textarea, [data-docx-text]')];
    const values = fields.map((field) => field instanceof HTMLTextAreaElement ? field.value : field.textContent ?? '');
    const text = values.join('');
    const index = text.toLowerCase().indexOf(needle);
    if (index < 0) continue;
    if (items.length === 200) return { items, truncated: true };
    items.push({ label: `본문·표 위치 ${items.length + 1}`, text: text.slice(Math.max(0, index - 40), index + query.length + 120), locate: () => {
      paragraph.scrollIntoView({ block: 'center' });
      let offset = 0;
      for (let i = 0; i < fields.length; i++) {
        if (index < offset + values[i]!.length) {
          const field = fields[i]!;
          if (field instanceof HTMLTextAreaElement) { field.focus({ preventScroll: true }); field.setSelectionRange(index - offset, Math.min(values[i]!.length, index - offset + query.length)); }
          else paragraph.focus({ preventScroll: true });
          break;
        }
        offset += values[i]!.length;
      }
    } });
  }
  return { items };
}
