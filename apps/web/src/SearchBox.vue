<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue';
import { request } from './client';
import { messages, type Locale } from './i18n';
import { designMessages } from './design-copy';
const props=defineProps<{modelValue:string;locale:Locale;id:string;recent:string[];busy?:boolean}>();
const emit=defineEmits<{ 'update:modelValue':[value:string]; submit:[]; removeRecent:[value:string] }>();
const t=computed(()=>messages[props.locale]);const copy=computed(()=>designMessages[props.locale]);
const suggestions=ref<string[]>([]);const open=ref(false);const active=ref(-1);
let timer:ReturnType<typeof setTimeout>|undefined;let closeTimer:ReturnType<typeof setTimeout>|undefined;
let controller:AbortController|undefined;let sequence=0;
const items=computed(()=>{
  const unique=new Set<string>();const q=props.modelValue.trim().toLowerCase();
  return [...props.recent.filter(x=>!q||x.toLowerCase().includes(q)).map(text=>({text,recent:true})),...suggestions.value.map(text=>({text,recent:false}))].filter(x=>{if(unique.has(x.text))return false;unique.add(x.text);return true;}).slice(0,8);
});
function changed(event:Event){
  const value=(event.target as HTMLInputElement).value;emit('update:modelValue',value);
  clearTimeout(timer);controller?.abort();++sequence;active.value=-1;open.value=true;suggestions.value=[];
  if(!value.trim())return;
  const own=sequence;
  timer=setTimeout(async()=>{const current=new AbortController();controller=current;try{const data=await request(`/suggest?${new URLSearchParams({q:value.trim()})}`,{signal:AbortSignal.any([current.signal,AbortSignal.timeout(3000)])});if(own===sequence&&!current.signal.aborted&&Array.isArray(data.suggestions))suggestions.value=data.suggestions.filter((x:unknown)=>typeof x==='string');}catch{}},200);
}
function step(direction:number){open.value=true;if(!items.value.length)return;active.value=(active.value+direction+items.value.length)%items.value.length;}
function choose(value:string){emit('update:modelValue',value);open.value=false;active.value=-1;emit('submit');}
function submit(){if(open.value&&active.value>=0&&items.value[active.value])choose(items.value[active.value].text);else{open.value=false;emit('submit');}}
function blur(){closeTimer=setTimeout(()=>{open.value=false;},120);}
function focus(){clearTimeout(closeTimer);open.value=true;}
onUnmounted(()=>{clearTimeout(timer);clearTimeout(closeTimer);controller?.abort();});
</script>
<template>
  <div class="search-control" :class="{scanning:busy}">
    <form class="search-form" role="search" @submit.prevent="submit">
      <label class="sr-only" :for="id">{{t.search}}</label>
      <svg class="search-symbol" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 4 4"/></svg>
      <input :id="id" :value="modelValue" type="search" :placeholder="t.searchPlaceholder" maxlength="512" enterkeyhint="search" autocomplete="off" required :aria-expanded="open&&items.length>0" aria-autocomplete="list" :aria-controls="`${id}-suggestions`" :aria-activedescendant="open&&active>=0?`${id}-option-${active}`:undefined" @input="changed" @focus="focus" @blur="blur" @keydown.down.prevent="step(1)" @keydown.up.prevent="step(-1)" @keydown.esc="open=false;active=-1"/>
      <button v-if="modelValue" type="button" class="icon-button clear-query" :aria-label="t.clear" @click="emit('update:modelValue','');suggestions=[];open=false">×</button>
      <button class="button primary search-submit" type="submit">{{copy.searchButton}}</button>
      <span class="scanning-laser-beam" aria-hidden="true"></span>
    </form>
    <ul v-if="open&&items.length" :id="`${id}-suggestions`" class="suggestions" role="listbox" :aria-label="copy.recent">
      <li v-for="(item,index) in items" :id="`${id}-option-${index}`" :key="item.text" role="option" :aria-selected="active===index" :class="{active:active===index}">
        <button type="button" class="suggestion-text" @mousedown.prevent @click="choose(item.text)"><span aria-hidden="true">{{item.recent?'↶':'↗'}}</span>{{item.text}}</button>
        <button v-if="item.recent" type="button" class="icon-button" :aria-label="`${t.clear}: ${item.text}`" @mousedown.prevent @click="emit('removeRecent',item.text)">×</button>
      </li>
    </ul>
  </div>
</template>
