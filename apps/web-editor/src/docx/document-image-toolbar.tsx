import { createSignal, Show } from 'solid-js';
import { buttonClass } from '../ui/editor-chrome';
import type { ImageOperation, Paragraph } from './contracts';

const maxBytes = 512*1024;

export function DocumentImageToolbar(props: {
  target?:Paragraph;
  disabled:boolean;
  pending:boolean;
  onApply:(operation:ImageOperation)=>boolean;
}) {
  const [file,setFile]=createSignal<File>(),[alt,setAlt]=createSignal('');
  const [reading,setReading]=createSignal(false),[error,setError]=createSignal('');
  const disabled=()=>props.disabled||props.pending||reading();
  let picker!:HTMLInputElement;
  const apply=async()=>{
    const target=props.target,fileToRead=file();
    if(!target||!fileToRead||disabled())return;
    if(fileToRead.size===0||fileToRead.size>maxBytes){setError('512 KiB 이하 PNG 또는 JPEG를 선택하세요.');return;}
    setReading(true);
    try{
      const bytes=new Uint8Array(await fileToRead.arrayBuffer());
      const png=bytes[0]===137&&bytes[1]===80&&bytes[2]===78&&bytes[3]===71;
      const jpeg=bytes[0]===255&&bytes[1]===216&&bytes[2]===255;
      if(!png&&!jpeg){setError('PNG 또는 JPEG 그림만 삽입할 수 있습니다.');return;}
      if(props.target?.id!==target.id){setError('선택한 문단이 바뀌었습니다. 문단을 다시 선택하세요.');return;}
      let binary='';
      for(let start=0;start<bytes.length;start+=8192)binary+=String.fromCharCode(...bytes.subarray(start,start+8192));
      const accepted=props.onApply({type:'update_image',targetId:target.id,image:{action:'insert',base64:btoa(binary),alt:alt()}});
      setError(accepted?'':'다른 초안을 먼저 저장하거나 실행 취소하세요.');
      if(accepted){setFile(undefined);setAlt('');picker.value='';}
    }catch{setError('그림 파일을 읽지 못했습니다.');}
    finally{setReading(false);}
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">그림 삽입</summary>
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
      <button type="button" class={buttonClass} disabled={disabled()||!file()||!props.target} onClick={()=>void apply()}>그림 적용</button>
      <p class="basis-full text-slate-600">{props.pending?'그림을 저장하거나 실행 취소하세요.':'512 KiB 이하 그림을 선택한 문단의 끝에 삽입합니다.'}</p>
      <Show when={error()}><p role="alert" class="basis-full text-red-700">{error()}</p></Show>
    </div>
  </details>;
}
