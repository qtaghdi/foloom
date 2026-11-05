// Pin every request to the editor's document. Never derive a write target from
// the workspace's selected tab or a shared cookie.
export function scopedDocumentURL(path: string): string {
  if (typeof window === 'undefined') return path;
  const id = new URLSearchParams(window.location.search).get('document');
  if (!id) return path;
  const url = new URL(path, window.location.origin);
  url.searchParams.set('document', id);
  return `${url.pathname}${url.search}`;
}
