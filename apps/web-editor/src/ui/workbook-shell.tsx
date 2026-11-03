import { Show } from 'solid-js';
import { AppView } from './app-view';
import { StatusPanel } from './status-panel';
import { EditorHeader } from './editor-chrome';

import { WorkbookToolbar } from './workbook-toolbar';

export function WorkbookShell(props: { view: AppView }) {
  const state = props.view.state;
  return <>
    <a class="skip-link" href="#spreadsheet">본문으로 이동</a>
    <div id="app-shell" class="grid h-full grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden">
      <EditorHeader name={state().name} kind="스프레드시트" nameId="workbook-name" statusId="revision-status"
        message={state().message} revision={state().revision} status={state().status}
        actions={<Show when={state().presence}><span id="ai-presence" class="max-w-56 truncate text-xs text-indigo-700" role="status" aria-live="polite">{state().presence}</span></Show>}/>
      <WorkbookToolbar view={props.view}/>
      {/* Solid owns this container's lifecycle; Univer exclusively owns its children. */}
      <main id="spreadsheet" ref={(element) => { props.view.spreadsheet = element; }} class="min-h-0 min-w-0" aria-label="Workbook editor" tabindex={-1} aria-busy={state().status === 'pending'}/>
      <Show when={state().warning} keyed>{(panel) => <StatusPanel id="feature-warnings" panel={panel} onDismiss={() => props.view.dismiss('warning')}/>}</Show>
      <Show when={state().error} keyed>{(panel) => <StatusPanel id="fatal-error" panel={panel} danger onDismiss={() => props.view.dismiss('error')}/>}</Show>
    </div>
  </>;
}
