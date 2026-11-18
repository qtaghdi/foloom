import { createSignal, For, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { ImageOperation, Paragraph, Run } from './contracts';
import { utf8Length } from './validation';

const maxBytes = 512*1024;

const cropSides={left:'왼쪽',top:'위쪽',right:'오른쪽',bottom:'아래쪽'} as const;
const sides=Object.keys(cropSides) as (keyof typeof cropSides)[];

export function DocumentImageToolbar(props: {
  target?:Paragraph;
  pictures: Run[];
  disabled:boolean;
  pending:boolean;
  onApply:(operation:ImageOperation)=>boolean;
}) {
  const [file,setFile]=createSignal<File>(),[alt,setAlt]=createSignal('');
  const [reading,setReading]=createSignal(false),[error,setError]=createSignal('');
  const [pictureId,setPictureId] = createSignal('');
  const selectedPicture = () => props.pictures.find(picture => picture.id === pictureId());
  const disabled=()=>props.disabled||props.pending||reading();
  let picker!:HTMLInputElement;
  const apply=async(replace:boolean)=>{
    const target=props.target,fileToRead=file();
    const targetId=replace ? selectedPicture()?.id : target?.id;
    if(!targetId||!fileToRead||disabled())return;
    if(fileToRead.size===0||fileToRead.size>maxBytes){setError('512 KiB 이하 PNG 또는 JPEG를 선택하세요.');return;}
    setReading(true);
    try{
      const bytes=new Uint8Array(await fileToRead.arrayBuffer());
      const png=bytes[0]===137&&bytes[1]===80&&bytes[2]===78&&bytes[3]===71;
      const jpeg=bytes[0]===255&&bytes[1]===216&&bytes[2]===255;
      if(!png&&!jpeg){setError('PNG 또는 JPEG 그림을 선택하세요.');return;}
      if((replace ? selectedPicture()?.id : props.target?.id)!==targetId){
        setError('선택이 바뀌었습니다. 편집할 대상을 다시 선택하세요.');return;
      }
      let binary='';
      for(let start=0;start<bytes.length;start+=8192)binary+=String.fromCharCode(...bytes.subarray(start,start+8192));
      const base64=btoa(binary);
      const accepted=props.onApply({type:'update_image',targetId,image:replace
        ? {action:'replace',base64}
        : {action:'insert',base64,alt:alt()}});
      setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
      if(accepted){setFile(undefined);setAlt('');picker.value='';}
    }catch{setError('그림 파일을 읽지 못했습니다.');}
    finally{setReading(false);}
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">그림 편집</summary>
    <div class="flex max-h-[35dvh] min-w-0 flex-wrap items-center gap-2 overflow-y-auto pb-2">
      <label class="flex min-w-0 flex-col gap-1">PNG 또는 JPEG
        <input ref={picker} type="file" accept="image/png,image/jpeg,.png,.jpg,.jpeg" disabled={disabled()}
          aria-label="그림 파일" class="min-h-11 max-w-full rounded border border-slate-300 p-2"
          onChange={event=>{setFile(event.currentTarget.files?.[0]);setError('');}}/>
      </label>
      <label class="flex min-w-0 flex-col gap-1">대체 텍스트
        <input type="text" maxlength="256" disabled={disabled()} value={alt()} class="min-h-11 rounded border border-slate-300 px-2"
          onInput={event=>setAlt(event.currentTarget.value)}/>
      </label>
      <button type="button" class={buttonClass} disabled={disabled()||!file()||!props.target} onClick={()=>void apply(false)}>그림 삽입</button>
      <button type="button" class={buttonClass} disabled={disabled()||!file()||!selectedPicture()} onClick={()=>void apply(true)}>선택한 그림 교체</button>
      <p class="basis-full text-slate-600">{props.pending?'그림을 저장하거나 실행 취소하세요.':'512 KiB 이하 파일을 문단 끝에 삽입하거나 기존 그림과 교체합니다.'}</p>
      <Show when={props.pictures.length}>
        <div class="flex basis-full flex-wrap items-end gap-2 border-t border-slate-200 pt-2">
          <label class="flex min-w-0 flex-col gap-1">기존 그림
            <select class="editor-select min-h-11 max-w-64" disabled={disabled()} value={selectedPicture()?.id ?? ''}
              onChange={event=>{setPictureId(event.currentTarget.value);setError('');}}>
              <option value="">그림 선택</option>
              <For each={props.pictures}>{(picture,index)=>
                <option value={picture.id}>그림 {index()+1}{picture.picture?.alt ? ` · ${picture.picture.alt.slice(0,60)}` : ''}</option>
              }</For>
            </select>
          </label>
          <Show when={selectedPicture()} keyed>{picture=>
            <>
            <form class="flex min-w-0 flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if (disabled()) return;
              const input = event.currentTarget.elements.namedItem('description') as HTMLInputElement;
              const changed = props.onApply({type:'update_image',targetId:picture.id,image:{action:'set_alt',alt:input.value}});
              setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex min-w-0 flex-col gap-1">기존 그림 대체 텍스트
                <input name="description" type="text" maxlength="1024" value={picture.picture?.alt ?? ''}
                  class="min-h-11 max-w-full rounded border border-slate-300 px-2" disabled={disabled()}/>
              </label>
              <button type="submit" class={buttonClass} disabled={disabled()}>대체 텍스트 적용</button>
              <button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
                const changed=props.onApply({type:'update_image',targetId:picture.id,image:{action:'delete'}});
                setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>그림 삭제</button>
            </form>
            <form class="flex min-w-0 flex-wrap items-end gap-2" onSubmit={event=>{
              event.preventDefault();
              if (disabled()) return;
              const input=event.currentTarget.elements.namedItem('width') as HTMLInputElement;
              const widthEmu=Math.round(input.valueAsNumber*12700);
              if (!Number.isFinite(widthEmu)||widthEmu<9525||widthEmu>18288000) return;
              const changed=props.onApply({type:'update_image',targetId:picture.id,image:{action:'resize',widthEmu}});
              setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
            }}>
              <label class="flex flex-col gap-1">그림 너비 (pt)
                <input name="width" type="number" min=".75" max="1440" step="any" required
                  value={((picture.picture?.widthEmu??9525)/12700).toFixed(2)} disabled={disabled()}
                  class="min-h-11 w-24 rounded border border-slate-300 px-2"/>
              </label>
              <button type="submit" class={buttonClass} disabled={disabled()}>비율 유지 크기 변경</button>
            </form>
            <Show when={picture.picture?.captionEditable}>
              <form class="flex min-w-0 flex-wrap items-end gap-2" onSubmit={event=>{
                event.preventDefault();
                if(disabled())return;
                const input=event.currentTarget.elements.namedItem('caption') as HTMLInputElement;
                const text=input.value.trim();
                if(!text){setError('그림 캡션을 입력하세요.');return;}
                if(utf8Length(text)>1024){setError('캡션을 UTF-8 1 KiB 이하로 줄이세요.');return;}
                const accepted=props.onApply({type:'insert_picture_caption',targetId:picture.id,expectedText:'',text});
                setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>
                <label class="flex min-w-0 flex-col gap-1">그림 캡션
                  <input name="caption" type="text" maxlength="1024" class="min-h-11 max-w-full rounded border border-slate-300 px-2" disabled={disabled()}/>
                </label>
                <button type="submit" class={buttonClass} disabled={disabled()}>그림 번호와 캡션 삽입</button>
              </form>
            </Show>
            <Show when={picture.picture?.rotationEditable}>
              <form class="flex min-w-0 flex-wrap items-end gap-2" onSubmit={event=>{
                event.preventDefault();
                if (disabled()) return;
                const select=event.currentTarget.elements.namedItem('rotation') as HTMLSelectElement;
                const rotation=Number(select.value);
                if ((rotation!==0&&rotation!==90&&rotation!==180&&rotation!==270) ||
                    rotation===(picture.picture?.rotation??0)) return;
                const changed=props.onApply({type:'update_image',targetId:picture.id,image:{action:'rotate',rotation}});
                setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>
                <label class="flex flex-col gap-1">그림 방향
                  <select name="rotation" aria-label="그림 방향" class="editor-select min-h-11" value={picture.picture?.rotation??0} disabled={disabled()}>
                    <option value={0}>기본 방향</option>
                    <option value={90}>90도</option>
                    <option value={180}>180도</option>
                    <option value={270}>270도</option>
                  </select>
                </label>
                <button type="submit" class={buttonClass} disabled={disabled()}>그림 방향 적용</button>
              </form>
            </Show>
            <Show when={picture.picture?.cropEditable}>
              <form class="flex basis-full flex-wrap items-end gap-2" onSubmit={event=>{
                event.preventDefault();
                if (disabled()) return;
                const fields=event.currentTarget.elements;
                const value=(side:keyof typeof cropSides)=>Math.round((fields.namedItem(side) as HTMLInputElement).valueAsNumber*1000);
                const crop={left:value('left'),top:value('top'),right:value('right'),bottom:value('bottom')};
                if (!Object.values(crop).every(n=>Number.isInteger(n)&&n>=0&&n<=99000)||
                    crop.left+crop.right>99000||crop.top+crop.bottom>99000){
                  setError('서로 마주 보는 두 변의 합은 99% 이하여야 합니다.');return;
                }
                if (!picture.picture?.crop&&Object.values(crop).every(n=>n===0)) return;
                const changed=props.onApply({type:'update_image',targetId:picture.id,image:{action:'crop',crop}});
                setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
              }}>
                <For each={sides}>{side=>
                  <label class="flex flex-col gap-1">{picture.picture?.rotation?'원본 ':''}{cropSides[side]} 자르기 (%)
                    <input name={side} type="number" min="0" max="99" step=".001" required disabled={disabled()}
                      value={(picture.picture?.crop?.[side]??0)/1000} class="min-h-11 w-20 rounded border border-slate-300 px-2"/>
                  </label>
                }</For>
                <button type="submit" class={buttonClass} disabled={disabled()}>그림 자르기 적용</button>
                <Show when={picture.picture?.crop}><button type="button" class={buttonClass} disabled={disabled()} onClick={()=>{
                  const changed=props.onApply({type:'update_image',targetId:picture.id,image:{action:'crop',crop:{left:0,top:0,right:0,bottom:0}}});
                  setError(changed?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
                }}>자르기 해제</button></Show>
              </form>
            </Show>
            </>
          }</Show>
        </div>
      </Show>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
