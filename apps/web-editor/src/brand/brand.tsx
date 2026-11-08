import symbol from '../../../../docs/brand/foloom-symbol.svg?raw';
import { Show } from 'solid-js';

export function FoloomBrand(props: { symbolOnly?: boolean; large?: boolean }) {
  return <span class="inline-flex shrink-0 items-center gap-2.5 text-teal-900" aria-label="폴룸 Foloom">
    <span class={props.large ? 'block size-20 [&>svg]:size-full' : 'block size-8 [&>svg]:size-full'} aria-hidden="true" innerHTML={symbol}/>
    <Show when={!props.symbolOnly}><span class="text-[22px] font-semibold tracking-[-0.055em] leading-none">foloom</span></Show>
  </span>;
}
