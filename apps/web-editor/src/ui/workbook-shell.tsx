import { t, localizeMessage, source } from '../i18n';
import { DocumentSearch, type SearchResults } from './document-search';
import { Show, createSignal } from 'solid-js';
import { AppView } from './app-view';
import { StatusPanel } from './status-panel';
import { EditorHeader, quietButtonClass } from './editor-chrome';

import { ActivityPanel } from './activity-panel';

export function WorkbookShell(props: { view: AppView; search?: (query: string) => Promise<SearchResults> }) {
  const state = props.view.state;
  const [warningsOpen, setWarningsOpen] = createSignal(false);
  return <>
    <a class="skip-link" href="#spreadsheet">{t('skipToContent')}</a>
    <div id="app-shell" class="flex h-full min-h-0 flex-col overflow-hidden">
      <EditorHeader name={state().name === source('openingDocument') ? t('openingDocument') : state().name} kind={t('spreadsheet')} nameId="workbook-name" statusId="revision-status"
        message={state().message} status={state().status} onActivity={props.view.activity.toggle} activityOpen={props.view.activity.open()}
        actions={<>
          <Show when={props.search}>{(search) => <DocumentSearch version={`${state().name}:${state().revision}:${state().status}`} search={search()} hint={t('searchSavedValuesAcrossAllSheetsIgnoringCase')}/>}</Show>
          <Show when={state().presence}><span id="ai-presence" class="max-w-40 truncate text-xs text-teal-800 max-sm:sr-only" role="status" aria-live="polite">{state().presence}</span></Show>
          <Show when={state().warning}><button type="button" class={quietButtonClass} aria-expanded={warningsOpen()} aria-controls="workbook-notices" onClick={() => setWarningsOpen(!warningsOpen())}>{t('compatibility')}</button></Show>
        </>}/>
      <Show when={state().warning} keyed>{(panel) => <div id="workbook-notices" hidden={!warningsOpen()}>
        <StatusPanel id="feature-warnings" panel={panel} initiallyExpanded onDismiss={() => { setWarningsOpen(false); props.view.dismiss('warning'); }}/>
      </div>}</Show>
      <Show when={state().error} keyed>{(panel) => <StatusPanel id="fatal-error" panel={panel} danger onDismiss={() => props.view.dismiss('error')}/>}</Show>
      <div class="editor-workspace flex min-h-0 min-w-0 flex-1 overflow-hidden" data-activity-open={props.view.activity.open()}>
        {/* The container stays mounted when activity or other view state changes. */}
        <main id="spreadsheet" ref={(element) => { props.view.spreadsheet = element; }} class="editor-primary min-h-0 min-w-0 flex-1" aria-label={t('spreadsheetEditor')} tabindex={-1} aria-busy={state().status === 'pending'}/>
        <Show when={props.view.activity.open()}><ActivityPanel entries={props.view.activity.entries()} onClose={props.view.activity.toggle} presence={state().presence}
          canLocate={(entry) => Boolean(entry.sheetTarget) && !['pending', 'conflict', 'recovery'].includes(state().status)}
          onLocate={(entry) => { if (entry.sheetTarget) props.view.locate(entry.sheetTarget); }}/></Show>
      </div>
      <footer class="shrink-0 border-t border-slate-200 bg-white px-4 py-2 text-xs text-slate-500">{t('formulaResultsArePreviews')}</footer>
    </div>
  </>;
}
