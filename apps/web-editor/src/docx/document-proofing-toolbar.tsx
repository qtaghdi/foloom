import { For } from 'solid-js';
import type { InlineStyle } from './contracts';

const languages = [
  {key:'proofingLanguage', label:'라틴 문자 언어', example:'en-GB'},
  {key:'eastAsiaProofingLanguage', label:'동아시아 문자 언어', example:'ko-KR'},
] as const;

export function DocumentProofingToolbar(props:{styles:InlineStyle[]; disabled:boolean; onFormat:(style:InlineStyle)=>void}) {
  const common = (key:typeof languages[number]['key']) => {
    const first=props.styles[0]?.[key];
    return props.styles.some(style=>style[key]!==first) ? 'mixed' : first;
  };
  return <details class="min-w-0 shrink-0 border-b border-slate-200 bg-stone-50 px-3 text-xs sm:px-6">
    <summary class="min-h-11 cursor-pointer content-center focus-visible:outline-2 focus-visible:outline-teal-700">맞춤법 검사 언어</summary>
    <div class="flex min-w-0 flex-wrap gap-2 pb-2">
      <For each={languages}>{item=><label class="flex items-center gap-2">
        {item.label}
        <input class="editor-select min-h-11 w-28 rounded px-2 text-sm" maxLength={35} disabled={props.disabled}
          value={common(item.key)==='mixed'?'':common(item.key)??''}
          placeholder={common(item.key)==='mixed'?'혼합 언어':item.example}
          onInput={event=>event.currentTarget.setCustomValidity('')}
          onChange={event=>{
            const control=event.currentTarget;
            const tag=control.value.trim();
            if (!tag) {
              control.value=common(item.key)==='mixed'?'':common(item.key)??'';
              return;
            }
            let locale: string|undefined;
            try {
              [locale]=Intl.getCanonicalLocales(tag);
            } catch {
              control.setCustomValidity('en-GB 또는 ko-KR와 같은 언어 태그를 입력하세요.');
              control.reportValidity();
              return;
            }
            if (locale) props.onFormat({[item.key]:locale});
          }}/>
      </label>}</For>
    </div>
  </details>;
}
