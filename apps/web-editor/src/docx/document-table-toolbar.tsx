import { For, Show, createMemo, createSignal } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { Paragraph, Snapshot, TableAction, TableOperation } from './contracts';

const actions = {
  insert:'표 삽입',
  delete:'표 삭제',
  add_row:'행 추가',
  delete_row:'행 삭제',
  move_row:'행 이동',
  add_column:'열 추가',
  delete_column:'열 삭제',
  move_column:'열 이동',
  set_row_height:'행 높이 설정',
} as const;

export function DocumentTableToolbar(props: {
  blocks: Snapshot['blocks'];
  insertAfter?: Paragraph;
  disabled: boolean;
  pending: boolean;
  onApply: (operation:TableOperation)=>boolean;
}) {
  const [action,setAction] = createSignal<TableAction>('insert');
  const [tableId,setTableId] = createSignal('');
  const [rows,setRows] = createSignal(2), [columns,setColumns] = createSignal(2);
  const [position,setPosition] = createSignal(1), [destination,setDestination] = createSignal(2);
  const [height,setHeight] = createSignal(24);
  const [error,setError] = createSignal('');
  const tables = createMemo(()=>props.blocks.filter(block=>block.type==='table'));
  const leading=()=>tables().find(table=>table.prependParagraph);
  const selected = createMemo(()=>tables().find(table=>table.id===tableId())??tables()[0]);
  const dimension = () => (action().endsWith('_row') || action()==='set_row_height') ? selected()?.rows.length??0 : selected()?.rows[0]?.cells.length??0;
  const disabled = () => props.disabled||props.pending;
  const valid = (number:number, max:number) => Number.isInteger(number)&&number>=1&&number<=max;
  const apply = () => {
    const kind = action();
    let operation:TableOperation;
    if (kind==='insert') {
      if (!props.insertAfter || !valid(rows(),100) || !valid(columns(),50) || rows()*columns()>3000) {
        setError('행은 최대 100개, 열은 최대 50개, 전체 셀은 최대 3,000개입니다.');
        return;
      }
      operation = {type:'update_table',targetId:props.insertAfter.id,table:{action:kind,rows:rows(),columns:columns()}};
    } else {
      const table = selected();
      if (!table?.tableEditable) return;
      const edit:TableOperation['table'] = {action:kind};
      if (kind!=='delete') {
        if (!valid(position(),dimension()+Number(kind.startsWith('add_')))) {
          setError('표 범위 안의 위치를 선택하세요.');
          return;
        }
        edit.index = position()-1;
      }
      if (kind==='set_row_height') {
        if (!valid(height(),1584)) {
          setError('행 높이는 1~1,584 pt로 입력하세요.');
          return;
        }
        edit.height = height()*20;
      }
      if (kind.startsWith('move_')) {
        if (!valid(destination(),dimension()) || destination()===position()) {
          setError('표 범위 안의 다른 이동 위치를 선택하세요.');
          return;
        }
        edit.destination = destination()-1;
      }
      operation = {type:'update_table',targetId:table.id,table:edit};
    }
    setError(props.onApply(operation)?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  const prepend = (tableId:string) => {
    const operation:TableOperation={type:'update_table',targetId:tableId,table:{action:'insert_body_paragraph_before'}};
    setError(props.onApply(operation)?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  const field = (label:string,value:()=>number,setValue:(value:number)=>void,max:()=>number) =>
    <label class="flex min-h-11 items-center gap-2">{label}
      <input type="number" aria-label={label} min="1" max={max()} step="1"
        class="min-h-11 w-20 rounded border border-slate-300 px-2" disabled={disabled()}
        value={value()} onChange={event=>setValue(event.currentTarget.valueAsNumber)}/>
    </label>;
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">표 편집</summary>
    <div class="flex max-h-[35dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto pb-2">
      <Show when={leading()}>{table=><button type="button" class={buttonClass} disabled={disabled()}
        onClick={()=>prepend(table().id)}>첫 표 앞에 본문 문단 추가</button>}</Show>
      <select aria-label="표 작업" class="editor-select min-h-11" value={action()} disabled={disabled()}
        onChange={event=>{setAction(event.currentTarget.value as TableAction);setError('');}}>
        <For each={Object.entries(actions)}>{([value,label])=><option value={value}>{label}</option>}</For>
      </select>
      <Show when={action()==='insert'} fallback={<>
        <select aria-label="편집할 표" class="editor-select min-h-11" disabled={disabled()} value={selected()?.id??''}
          onChange={event=>{setTableId(event.currentTarget.value);setError('');}}>
          <Show when={!tables().length}><option value="">편집할 표 없음</option></Show>
          <For each={tables()}>{(table,index)=><option value={table.id}>표 {index()+1}{table.tableEditable?'':' (보호됨)'}</option>}</For>
        </select>
        <Show when={action()!=='delete'}>{field('행 또는 열 위치',position,setPosition,()=>dimension()+Number(action().startsWith('add_')))}</Show>
        <Show when={action()==='set_row_height'}>{field('최소 행 높이 (pt)',height,setHeight,()=>1584)}</Show>
        <Show when={action().startsWith('move_')}>{field('이동 후 위치',destination,setDestination,dimension)}</Show>
      </>}>
        {field('새 표의 행 수',rows,setRows,()=>100)}
        {field('새 표의 열 수',columns,setColumns,()=>50)}
      </Show>
      <button type="button" class={buttonClass} disabled={disabled()||!(action()==='insert'?props.insertAfter:selected()?.tableEditable)}
        onClick={apply}>표 작업 적용</button>
      <p class="basis-full text-slate-600">{props.pending?'표 작업을 저장하거나 실행 취소하세요.':action()==='insert'
        ? `삽입 위치: ${props.insertAfter?.runs.map(run=>run.text).join('').trim().slice(0,80)||'편집 가능한 본문 문단'} 뒤`
        : action()==='set_row_height' ? '내용이 많으면 행은 지정한 최소 높이보다 커질 수 있습니다. 적용 후 저장으로 확정합니다.' : '삭제는 선택한 표, 행 또는 열의 내용을 포함합니다. 적용 후 저장으로 확정합니다.'}</p>
      <Show when={error()}><p class="basis-full text-red-700" role="alert">{error()}</p></Show>
    </div>
  </details>;
}
