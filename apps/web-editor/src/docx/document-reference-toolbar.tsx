import { createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { Paragraph, Run, ReferenceEdit, ReferenceOperation, Snapshot } from './contracts';

function safeWebAddress(address:string):boolean {
  try {
    const parsed=new URL(address);
    return /^https?:\/\/\S+$/.test(address) && !parsed.username && !parsed.password && Boolean(parsed.hostname);
  } catch {
    return false;
  }
}

export function DocumentReferenceToolbar(props:{
  target?:Paragraph;links:Run[];notes:Run[];referenceLists:Run[];dateFields:Run[];bookmarks:NonNullable<Snapshot['bookmarks']>;pending:boolean;disabled:boolean;
  onApply:(operation:ReferenceOperation)=>boolean;
}) {
  const [mode,setMode]=createSignal<'bookmark'|'url'|'anchor'|'reference'|'pageReference'|'numberReference'|'contents'|'figureList'|'tableList'|'dateField'|'footnote'|'endnote'>('bookmark');
  const [bookmark,setBookmark]=createSignal(''),[text,setText]=createSignal(''),[url,setURL]=createSignal(''),[anchor,setAnchor]=createSignal('');
  const crossReference=()=>mode()==='reference'||mode()==='pageReference'||mode()==='numberReference';
  const bookmarkChoices=()=>crossReference()?props.bookmarks.filter(item=>mode()==='numberReference'?item.numberReference:item.textReference):props.bookmarks;
  const targetBookmark=()=>bookmarkChoices().find(item=>item.name===anchor())?.name??bookmarkChoices()[0]?.name;
  const [linkId,setLinkId]=createSignal('');
  const selectedLink = () => props.links.find(link=>link.id===linkId());
  const [noteId,setNoteId]=createSignal('');
  const selectedNote=()=>props.notes.find(note=>note.id===noteId());
  const existingNoteLabel=()=>selectedNote()?.object?.kind==='endnote'?'미주':'각주';
  const [dateFieldID,setDateFieldID]=createSignal('');
  const selectedDateField=()=>props.dateFields.find(field=>field.id===dateFieldID());
  const [editBookmark,setEditBookmark]=createSignal('');
  const renameableBookmarks=()=>props.bookmarks.filter(item=>item.renameEditable);
  const bookmarkToRename=()=>renameableBookmarks().find(item=>item.name===editBookmark());
  const [error,setError]=createSignal('');
  const disabled=()=>props.disabled||props.pending;
  const addingNote=()=>mode()==='footnote'||mode()==='endnote';
  const noteLabel=()=>mode()==='endnote'?'미주':'각주';
  const apply=()=>{
    if(!props.target?.structureEditable)return;
    let reference:ReferenceEdit;
    if(mode()==='bookmark'){
      if(!/^[A-Za-z][A-Za-z0-9_]{0,39}$/.test(bookmark())){setError('북마크 이름은 영문자로 시작하는 영문자·숫자·밑줄 1~40자를 사용하세요.');return;}
      if(props.bookmarks.some(item=>item.name.toLowerCase()===bookmark().toLowerCase())){setError('같은 이름의 북마크가 있습니다.');return;}
      reference={action:'insert_bookmark',bookmark:bookmark()};
    }else if(mode()==='contents'){
      reference={action:'insert_toc'};
    }else if(mode()==='figureList'||mode()==='tableList'){
      reference={action:mode()==='figureList'?'insert_figure_list':'insert_table_list'};
    }else if(mode()==='dateField'){
      reference={action:'insert_date_field'};
    }else if(crossReference()){
      const name=targetBookmark();
      if(!name){setError('평문 북마크를 선택하세요.');return;}
      const action=mode()==='pageReference'?'insert_bookmark_page_ref':
        mode()==='numberReference'?'insert_bookmark_number_ref':'insert_bookmark_ref';
      reference={action,bookmark:name};
    }else if(addingNote()){
      if(!text().trim()){setError(`${noteLabel()} 내용을 입력하세요.`);return;}
      reference={action:mode()==='endnote'?'insert_endnote':'insert_footnote',text:text()};
    }else{
      if(!text().trim()){setError('표시할 링크 텍스트를 입력하세요.');return;}
      if(mode()==='url'){
        const value=url().trim();
        if(!safeWebAddress(value)){setError('인증 정보가 없는 HTTP 또는 HTTPS 주소를 입력하세요.');return;}
        reference={action:'insert_link',text:text(),url:value};
      }else{
        const name=targetBookmark();
        if(!name||!props.bookmarks.some(item=>item.name===name)){setError('이동할 북마크를 먼저 선택하세요.');return;}
        reference={action:'insert_link',text:text(),bookmark:name};
      }
    }
    setError(props.onApply({type:'update_reference',targetId:props.target.id,reference})?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">링크·북마크·각주·미주·목차·필드</summary>
    <div class="flex max-h-[35dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto pb-2">
      <select aria-label="참조 작업" class="editor-select min-h-11" disabled={disabled()} value={mode()}
        onChange={event=>{setMode(event.currentTarget.value as ReturnType<typeof mode>);setError('');}}>
        <option value="bookmark">북마크 추가</option><option value="url">웹 링크 삽입</option><option value="anchor">북마크 링크 삽입</option><option value="reference">텍스트 상호 참조 삽입</option><option value="pageReference">쪽 번호 상호 참조 삽입</option><option value="numberReference">목록 번호 상호 참조 삽입</option><option value="contents">자동 목차 삽입</option><option value="figureList">그림 목록 삽입</option><option value="tableList">표 목록 삽입</option><option value="dateField">날짜 필드 삽입</option><option value="footnote">각주 삽입</option><option value="endnote">미주 삽입</option>
      </select>
      <Show when={mode()!=='contents'&&mode()!=='figureList'&&mode()!=='tableList'&&mode()!=='dateField'} fallback={<span class="text-slate-600">
        {mode()==='contents'?'선택한 문단 앞에 제목 1~3 수준의 목차를 넣습니다.':mode()==='dateField'?'선택한 문단 끝에 UTC 기준 날짜를 넣습니다.':'선택한 문단 앞에 본문의 번호 캡션을 모읍니다.'}
      </span>}>
      <Show when={addingNote()} fallback={<Show when={mode()==='bookmark'} fallback={<>
        <Show when={!crossReference()}><input type="text" maxlength="256" aria-label="링크 표시 텍스트" placeholder="표시 텍스트" class="min-h-11 rounded border border-slate-300 px-2"
          disabled={disabled()} value={text()} onInput={event=>setText(event.currentTarget.value)}/></Show>
        <Show when={mode()==='url'} fallback={<select aria-label={crossReference()?'상호 참조할 북마크':'링크 대상 북마크'} class="editor-select min-h-11" disabled={disabled()}
          value={targetBookmark()??''} onChange={event=>setAnchor(event.currentTarget.value)}>
          <Show when={!bookmarkChoices().length}><option value="">북마크 없음</option></Show>
          <For each={bookmarkChoices()}>{item=><option value={item.name}>{item.name}</option>}</For>
        </select>}>
          <input type="url" maxlength="2048" aria-label="웹 링크 주소" placeholder="https://" class="min-h-11 max-w-full rounded border border-slate-300 px-2"
            disabled={disabled()} value={url()} onInput={event=>setURL(event.currentTarget.value)}/>
        </Show>
      </>}>
        <input type="text" maxlength="40" aria-label="북마크 이름" placeholder="북마크 이름" class="min-h-11 rounded border border-slate-300 px-2"
          disabled={disabled()} value={bookmark()} onInput={event=>setBookmark(event.currentTarget.value)}/>
      </Show>}>
        <input type="text" maxlength="1000" aria-label={`${noteLabel()} 내용`} placeholder={`${noteLabel()} 내용`}
          class="min-h-11 rounded border border-slate-300 px-2"
          disabled={disabled()} value={text()} onInput={event=>setText(event.currentTarget.value)}/>
      </Show>
      </Show>
      <button type="button" class={buttonClass} disabled={disabled()||!props.target?.structureEditable} onClick={apply}>참조 적용</button>
      <p class="basis-full text-slate-600">{props.pending?'저장하거나 실행 취소하세요.':props.target?.structureEditable
        ? `대상 문단: ${props.target.runs.map(run=>run.text).join('').trim().slice(0,80)||'빈 문단'}`
        : '편집 가능한 본문 문단에 커서를 놓으세요.'}</p>
      <For each={props.referenceLists}>{list=>
        <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
          const action=list.object?.kind==='toc'?'update_toc':'update_caption_list';
          const accepted=props.onApply({type:'update_reference',targetId:list.id,reference:{action}});
          setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
        }}>{list.object?.kind==='toc'?'목차 제목 업데이트':list.object?.kind==='figure_list'?'그림 목록 업데이트':'표 목록 업데이트'}</button>
      }</For>
      <Show when={props.dateFields.length}>
        <div class="flex basis-full flex-wrap items-end gap-2 border-t border-slate-200 pt-2">
          <label class="flex min-w-0 flex-col gap-1">기존 날짜 필드
            <select class="editor-select min-h-11 max-w-56" value={selectedDateField()?.id??''} disabled={disabled()}
              onChange={event=>{setDateFieldID(event.currentTarget.value);setError('');}}>
              <option value="">날짜 필드 선택</option>
              <For each={props.dateFields}>{(field,index)=><option value={field.id}>{index()+1}. {field.text}</option>}</For>
            </select>
          </label>
          <button type="button" class={buttonClass} disabled={disabled()||!selectedDateField()} onClick={()=>{
            const field=selectedDateField();
            if(!field)return;
            const accepted=props.onApply({type:'update_reference',targetId:field.id,reference:{action:'refresh_date_field'}});
            setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
          }}>오늘 날짜로 갱신</button>
        </div>
      </Show>
      <Show when={props.links.length}>
        <div class="flex basis-full flex-wrap items-end gap-2 border-t border-slate-200 pt-2">
          <label class="flex min-w-0 flex-col gap-1">기존 링크
            <select aria-label="기존 링크" class="editor-select min-h-11 max-w-64" value={selectedLink()?.id??''}
              disabled={disabled()} onChange={event=>{setLinkId(event.currentTarget.value);setError('');}}>
              <option value="">링크 선택</option>
              <For each={props.links}>{(link,index)=><option value={link.id}>{index()+1}. {link.text.slice(0,60)}</option>}</For>
            </select>
          </label>
          <Show when={selectedLink()} keyed>{link=><>
            <Show when={link.link?.url}>
            <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if (disabled()) return;
              const input=event.currentTarget.elements.namedItem('address') as HTMLInputElement;
              const address=input.value.trim();
              if (!safeWebAddress(address)) {setError('인증 정보가 없는 HTTP 또는 HTTPS 주소를 입력하세요.');return;}
              const changed=props.onApply({type:'update_reference',targetId:link.id,reference:{action:'set_link_url',url:address}});
              setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex min-w-0 flex-col gap-1">새 웹 주소
                <input name="address" type="url" required maxlength="2048" value={link.link?.url??''} disabled={disabled()}
                  class="min-h-11 max-w-full rounded border border-slate-300 px-2"/>
              </label>
              <button type="submit" class={buttonClass} disabled={disabled()}>웹 주소 변경 적용</button>
            </form>
            </Show>
            <Show when={link.link&&'bookmarkEditable' in link.link&&link.link.bookmarkEditable&&props.bookmarks.length>1}>
              <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
                event.preventDefault();
                if(disabled())return;
                const input=event.currentTarget.elements.namedItem('bookmark') as HTMLSelectElement;
                const bookmark=input.value;
                if(!bookmark||bookmark===link.link?.bookmark||!props.bookmarks.some(item=>item.name===bookmark)){
                  setError('새 대상 북마크를 선택하세요.');
                  return;
                }
                const accepted=props.onApply({type:'update_reference',targetId:link.id,reference:{action:'set_link_bookmark',bookmark}});
                setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>
                <label class="flex min-w-0 flex-col gap-1">새 대상 북마크
                  <select name="bookmark" aria-label="새 대상 북마크" class="editor-select min-h-11 max-w-56" required disabled={disabled()}>
                    <option value="">북마크 선택</option>
                    <For each={props.bookmarks.filter(item=>item.name!==link.link?.bookmark)}>
                      {item=><option value={item.name}>{item.name}</option>}
                    </For>
                  </select>
                </label>
                <button type="submit" class={buttonClass} disabled={disabled()}>북마크 대상 변경 적용</button>
              </form>
            </Show>
            <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
              const changed=props.onApply({type:'update_reference',targetId:link.id,reference:{action:'unlink'}});
              setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>링크 해제</button>
          </>}</Show>
        </div>
      </Show>
      <Show when={props.notes.length}>
        <div class="flex basis-full flex-wrap items-end gap-2 border-t border-slate-200 pt-2">
          <label class="flex min-w-0 flex-col gap-1">기존 각주·미주
            <select aria-label="기존 각주·미주" class="editor-select min-h-11 max-w-56" disabled={disabled()} value={selectedNote()?.id??''}
              onChange={event=>{setNoteId(event.currentTarget.value);setError('');}}>
              <option value="">각주·미주 선택</option>
              <For each={props.notes}>{note=><option value={note.id}>{note.object?.kind==='endnote'?'미주':'각주'} {note.object?.text?.slice(0,60)}</option>}</For>
            </select>
          </label>
          <Show when={selectedNote()} keyed>{note=>
            <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if (disabled()) return;
              const input=event.currentTarget.elements.namedItem('content') as HTMLInputElement;
              if(!input.value.trim()){setError(`${existingNoteLabel()} 내용을 입력하세요.`);return;}
              const accepted=props.onApply({type:'update_reference',targetId:note.id,
                reference:{action:note.object?.kind==='endnote'?'set_endnote_text':'set_footnote_text',text:input.value}});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex min-w-0 flex-col gap-1">기존 {existingNoteLabel()} 내용
                <input name="content" type="text" required maxlength="256" disabled={disabled()}
                  value={note.object?.text??''} class="min-h-11 rounded border border-slate-300 px-2"/>
              </label>
              <button type="submit" class={buttonClass} disabled={disabled()}>{existingNoteLabel()} 변경 적용</button>
              <Show when={note.object?.kind==='footnote'||note.object?.deleteEditable}><button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
                const accepted=props.onApply({type:'update_reference',targetId:note.id,reference:{action:note.object?.kind==='endnote'?'delete_endnote':'delete_footnote'}});
                setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>{existingNoteLabel()} 삭제</button></Show>
            </form>
          }</Show>
        </div>
      </Show>
      <Show when={renameableBookmarks().length}>
        <div class="flex basis-full flex-wrap items-end gap-2 border-t border-slate-200 pt-2">
          <select aria-label="편집할 북마크" class="editor-select min-h-11 max-w-56"
            value={bookmarkToRename()?.name??''} disabled={disabled()} onChange={event=>{setEditBookmark(event.currentTarget.value);setError('');}}>
            <option value="">북마크 선택</option>
            <For each={renameableBookmarks()}>{item=><option value={item.name}>{item.name}</option>}</For>
          </select>
          <Show when={bookmarkToRename()} keyed>{item=>
            <form class="flex flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if (disabled()) return;
              const input=event.currentTarget.elements.namedItem('newBookmark') as HTMLInputElement;
              const newBookmark=input.value.trim();
              if (!/^[A-Za-z][A-Za-z0-9_]{0,39}$/.test(newBookmark)){
                setError('북마크 이름은 영문자로 시작하는 영문자·숫자·밑줄 1~40자를 사용하세요.');return;
              }
              if (newBookmark===item.name) return;
              if (props.bookmarks.some(other=>other.name!==item.name&&other.name.toLowerCase()===newBookmark.toLowerCase())){
                setError('같은 이름의 북마크가 있습니다.');return;
              }
              const accepted=props.onApply({type:'update_reference',targetId:item.paragraphId,
                reference:{action:'rename_bookmark',bookmark:item.name,newBookmark}});
              setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex min-w-0 flex-col gap-1">새 북마크 이름
                <input name="newBookmark" type="text" maxlength="40" required disabled={disabled()}
                  value={item.name} class="min-h-11 rounded border border-slate-300 px-2"/>
              </label>
              <button type="submit" class={buttonClass} disabled={disabled()}>북마크 이름 변경 적용</button>
              <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
                const accepted=props.onApply({type:'update_reference',targetId:item.paragraphId,
                  reference:{action:'delete_bookmark',bookmark:item.name}});
                setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>북마크와 내부 링크 해제</button>
            </form>
          }</Show>
        </div>
      </Show>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
