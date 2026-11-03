import { Show } from 'solid-js';
import { AppView } from './app-view';
import { StatusPanel } from './status-panel';
import { EditorHeader } from './editor-chrome';

import { ActivityPanel } from './activity-panel';
import { WorkbookToolbar } from './workbook-toolbar';

export function WorkbookShell(props: { view: AppView }) {
  const state = props.view.state;
  return <>
    <a class="skip-link" href="#spreadsheet">본문으로 이동</a>
    <div id="app-shell" class="flex h-full min-h-0 flex-col overflow-hidden">
      <EditorHeader name={state().name} kind="스프레드시트" nameId="workbook-name" statusId="revision-status"
        message={state().message} revision={state().revision} status={state().status} onActivity={props.view.activity.toggle} activityOpen={props.view.activity.open()}
        actions={<Show when={state().presence}><span id="ai-presence" class="max-w-56 truncate text-xs text-indigo-700" role="status" aria-live="polite">{state().presence}</span></Show>}/>
      <WorkbookToolbar view={props.view}/>
      <Show when={state().warning} keyed>{(panel) => <StatusPanel id="feature-warnings" panel={panel} onDismiss={() => props.view.dismiss('warning')}/>}</Show>
      <Show when={state().error} keyed>{(panel) => <StatusPanel id="fatal-error" panel={panel} danger onDismiss={() => props.view.dismiss('error')}/>}</Show>
      <div class="editor-workspace flex min-h-0 min-w-0 flex-1 overflow-hidden" data-activity-open={props.view.activity.open()}>
        {/* The container stays mounted when activity or other view state changes. */}
        <main id="spreadsheet" ref={(element) => { props.view.spreadsheet = element; }} class="editor-primary min-h-0 min-w-0 flex-1" aria-label="스프레드시트 편집" tabindex={-1} aria-busy={state().status === 'pending'}/>
        <Show when={props.view.activity.open()}><ActivityPanel entries={props.view.activity.entries()} onClose={props.view.activity.toggle} presence={state().presence}/></Show>
      </div>

    </div>
  </>;
}
