import { locale, localeSchema, setLocale, t } from './index';
export function LocaleSelect() {
  return <label class="flex min-h-11 shrink-0 items-center gap-2 text-xs text-slate-600">
    <span class="sr-only">{t('language')}</span>
    <select aria-label={t('language')} class="editor-select min-h-11 max-w-36 cursor-pointer rounded-md border border-slate-200 bg-white px-2 text-sm" value={locale()} onChange={event => setLocale(localeSchema.parse(event.currentTarget.value))}>
      <option value="ko">한국어</option><option value="en">English</option><option value="ja">日本語</option><option value="zh-CN">简体中文</option>
    </select>
  </label>;
}
