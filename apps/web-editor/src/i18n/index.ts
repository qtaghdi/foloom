import { createSignal } from 'solid-js';
import { z } from 'zod';
import { catalog, type Locale, type MessageKey } from './catalog';
export { catalog, type Locale, type MessageKey } from './catalog';

export const localeSchema = z.enum(['ko', 'en', 'ja', 'zh-CN']);
const storageKey = 'document-workspace:locale:v1';
export function resolveLocale(languages: readonly string[]): Locale {
  for (const language of languages) {
    const code = language.toLowerCase();
    if (/^ko(?:-|$)/.test(code)) return 'ko';
    if (/^ja(?:-|$)/.test(code)) return 'ja';
    if (/^en(?:-|$)/.test(code)) return 'en';
    if (/^zh(?:$|-cn$|-sg$|-hans(?:-|$))/.test(code)) return 'zh-CN';
  }
  return 'en';
}
function initialLocale(): Locale {
  try { const stored = localeSchema.safeParse(localStorage.getItem(storageKey)); if (stored.success) return stored.data; } catch { /* Preferences are optional. */ }
  return typeof navigator === 'undefined' ? 'ko' : resolveLocale(navigator.languages ?? [navigator.language]);
}
export const [locale, updateLocale] = createSignal<Locale>(initialLocale());
type Parameters = Record<string, string | number>;
export function source(key: MessageKey, parameters: Parameters = {}): string { return interpolate(catalog[key].ko, parameters); }
export function t(key: MessageKey, parameters: Parameters = {}): string { return interpolate(catalog[key][locale()], parameters); }
function interpolate(value: string, parameters: Parameters): string {
  return value.replace(/\{(\w+)\}/g, (token, key: string) => String(parameters[key] ?? token));
}
export function setLocale(value: Locale, persist = true): void {
  const selected = localeSchema.parse(value);
  updateLocale(selected);
  if (typeof document !== 'undefined') document.documentElement.lang = selected;
  if (persist) { try { localStorage.setItem(storageKey, selected); } catch { /* Keep the current session usable without local storage. */ } }
  if (typeof document !== 'undefined' && typeof window !== 'undefined') {
    for (const frame of document.querySelectorAll<HTMLIFrameElement>('iframe[data-document-editor]')) frame.contentWindow?.postMessage({ type: 'workspace:locale', locale: selected }, window.location.origin);
  }
}
export function initializeI18n(): () => void {
  setLocale(locale(), false);
  const message = (event: MessageEvent) => {
    if (event.origin !== window.location.origin || event.source !== window.parent || window.parent === window) return;
    const parsed = z.object({ type: z.literal('workspace:locale'), locale: localeSchema }).safeParse(event.data);
    if (parsed.success) setLocale(parsed.data.locale, false);
  };
  const storage = (event: StorageEvent) => {
    if (event.key !== storageKey) return;
    const selected = localeSchema.safeParse(event.newValue);
    if (selected.success) setLocale(selected.data, false);
  };
  window.addEventListener('message', message);
  window.addEventListener('storage', storage);
  const dispose = () => { window.removeEventListener('message', message); window.removeEventListener('storage', storage); window.removeEventListener('pagehide', dispose); };
  window.addEventListener('pagehide', dispose, { once: true });
  return dispose;
}

const exact = new Map<string, MessageKey>();
const templates: Array<{ key: MessageKey; pattern: RegExp; names: string[] }> = [];
for (const key of Object.keys(catalog) as MessageKey[]) {
  for (const text of Object.values(catalog[key])) {
    if (!text.includes('{')) { if (!exact.has(text)) exact.set(text, key); continue; }
    const names: string[] = [];
    const escaped = text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace(/\\\{(\w+)\\\}/g, (_token, name: string) => { names.push(name); return '(.+?)'; });
    templates.push({ key, names, pattern: new RegExp(`^${escaped}$`, 'u') });
  }
}

// Only application-owned messages enter this function. Parameters such as
// file names, cell addresses and user text are preserved verbatim.
export function localizeMessage(value: string): string {
  if (value.includes('\n\n')) return value.split('\n\n').map(localizeMessage).join('\n\n');
  const presence = /^(AI 편집 중|AI 선택|AI 변경 저장): (.*)$/u.exec(value);
  if (presence) return `${localizeMessage(presence[1]!)}: ${presence[2]}`;
  const key = exact.get(value);
  if (key) return t(key);
  for (const template of templates) {
    const match = template.pattern.exec(value);
    if (!match) continue;
    const parameters = Object.fromEntries(template.names.map((name, index) => [name, name === 'problem' ? localizeMessage(match[index + 1]!) : match[index + 1]!]));
    return t(template.key, parameters);
  }
  return value;
}
export function knownMessage(value: string): boolean { return exact.has(value); }
