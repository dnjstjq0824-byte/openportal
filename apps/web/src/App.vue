<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import MailView from './MailView.vue';
import { APIError, mutate, request, session } from './client';
import { errors, locales, messages, type Locale } from './i18n';

const locale=ref<Locale>('en'),route=ref(''),search=ref(''),page=ref(1),safeSearch=ref('moderate'),verticalNotice=ref('');
const result=ref<{total:number|null;has_more:boolean;elapsed_ms?:number;corrected_query?:string;did_you_mean?:string;hits:{title:string;url:string;description:string;site_name?:string;source?:string}[]}|null>(null);
const busy=ref(false),error=ref(''),notice=ref(''),user=ref<any>(null),ready=ref(false);
const email=ref(''),password=ref(''),confirm=ref(''),displayName=ref(''),agree=ref(false),token=ref(''),theme=ref('system');
const suggestions=ref<string[]>([]);
const showSuggestions=ref(false);
const activeSuggestionIndex=ref(-1);
const recentSearches=ref<string[]>([]);
try{recentSearches.value=JSON.parse(localStorage.getItem('openportal.recent_searches')||'[]');}catch{recentSearches.value=[];}

const suggestionList=computed(()=>{
  const current=search.value.trim().toLowerCase();
  const list:{text:string;isRecent:boolean}[]=[];
  const seen=new Set<string>();
  if(current){
    for(const r of recentSearches.value){
      if(r.toLowerCase().includes(current)&&!seen.has(r)){list.push({text:r,isRecent:true});seen.add(r);if(list.length>=3)break;}
    }
  }else{
    for(const r of recentSearches.value.slice(0,5)){list.push({text:r,isRecent:true});seen.add(r);}
  }
  for(const s of suggestions.value){
    if(!seen.has(s)){list.push({text:s,isRecent:false});seen.add(s);if(list.length>=10)break;}
  }
  return list;
});

let suggestTimer:ReturnType<typeof setTimeout>|undefined;
function onSearchInput(){
  activeSuggestionIndex.value=-1;
  const q=search.value.trim();
  if(!q){suggestions.value=[];showSuggestions.value=recentSearches.value.length>0;return;}
  showSuggestions.value=true;
  clearTimeout(suggestTimer);
  suggestTimer=setTimeout(async()=>{
    try{const data=await request(`/suggest?${new URLSearchParams({q})}`);if(Array.isArray(data.suggestions))suggestions.value=data.suggestions;}catch{}
  },150);
}
function onSearchFocus(){if(search.value.trim()||recentSearches.value.length>0)showSuggestions.value=true;}
function onSearchBlur(){setTimeout(()=>{showSuggestions.value=false;},200);}
function closeSuggestions(){showSuggestions.value=false;activeSuggestionIndex.value=-1;}
function onSuggestDown(){
  if(!showSuggestions.value){showSuggestions.value=true;return;}
  const max=suggestionList.value.length;if(max===0)return;
  activeSuggestionIndex.value=(activeSuggestionIndex.value+1)%max;
  if(activeSuggestionIndex.value>=0&&activeSuggestionIndex.value<max)search.value=suggestionList.value[activeSuggestionIndex.value].text;
}
function onSuggestUp(){
  if(!showSuggestions.value)return;
  const max=suggestionList.value.length;if(max===0)return;
  activeSuggestionIndex.value=(activeSuggestionIndex.value-1+max)%max;
  if(activeSuggestionIndex.value>=0&&activeSuggestionIndex.value<max)search.value=suggestionList.value[activeSuggestionIndex.value].text;
}
function selectSuggestion(text:string){search.value=text;showSuggestions.value=false;submitSearch(1);}
function clearSearch(){search.value='';suggestions.value=[];showSuggestions.value=false;activeSuggestionIndex.value=-1;}
function removeRecent(text:string){
  recentSearches.value=recentSearches.value.filter(x=>x!==text);
  try{localStorage.setItem('openportal.recent_searches',JSON.stringify(recentSearches.value));}catch{}
}
function saveRecentSearch(q:string){
  const trimmed=q.trim();if(!trimmed)return;
  const filtered=recentSearches.value.filter(x=>x!==trimmed);
  recentSearches.value=[trimmed,...filtered].slice(0,10);
  try{localStorage.setItem('openportal.recent_searches',JSON.stringify(recentSearches.value));}catch{}
}
function applyCorrection(corrected:string){search.value=corrected;submitSearch(1);}
function getHostname(urlStr:string):string{try{return new URL(urlStr).hostname;}catch{return urlStr;}}
function showVerticalNotice(name:string){verticalNotice.value=name;setTimeout(()=>{verticalNotice.value='';},6000);}

const t=computed(()=>messages[locale.value]);const errorText=computed(()=>errors[locale.value][error.value]||errors[locale.value].fallback);
let controller:AbortController|undefined;let sequence=0;let refreshTimer:ReturnType<typeof setInterval>|undefined;
const titles=computed(()=>({login:t.value.login,signup:t.value.signup,'forgot-password':t.value.reset,'reset-password':t.value.reset,'verify-email':t.value.verify,account:t.value.account,mail:t.value.mail,privacy:t.value.privacy,terms:t.value.terms} as Record<string,string>));
const isSearch=computed(()=>route.value===''||route.value==='search');
function safeReturn(value:string|null){
  if(!value||/[\\\r\n]/.test(value))return `/${locale.value}/mail`;
  try{const url=new URL(value,location.origin);return url.origin===location.origin&&/^\/(ko|th|en)\/(mail|account)(\/|$)/.test(url.pathname)?url.pathname+url.search:`/${locale.value}/mail`;}catch{return `/${locale.value}/mail`;}
}
function navigate(url:string,replace=false){history[replace?'replaceState':'pushState'](null,'',url);restore();}
function link(event:MouseEvent,url:string){if(event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;event.preventDefault();navigate(url);}
function changeLanguage(event:Event){const value=(event.target as HTMLSelectElement).value as Locale;localStorage.setItem('openportal.locale',value);navigate(`/${value}${route.value?'/'+route.value:''}${location.search}${location.hash}`);}
function submitSearch(nextPage=1){
  if(!search.value.trim())return;
  saveRecentSearch(search.value.trim());
  showSuggestions.value=false;
  const params=new URLSearchParams({q:search.value.trim()});
  if(nextPage>1)params.set('page',String(nextPage));
  if(safeSearch.value&&safeSearch.value!=='moderate')params.set('safe_search',safeSearch.value);
  navigate(`/${locale.value}/search?${params}`);
}
async function runSearch(){
  const own=++sequence;controller?.abort();controller=new AbortController();result.value=null;error.value='';
  if(!search.value.trim()){busy.value=false;return;}busy.value=true;
  try{
    const params=new URLSearchParams({q:search.value,page:String(page.value)});
    if(safeSearch.value&&safeSearch.value!=='moderate')params.set('safe_search',safeSearch.value);
    const data=await request(`/search?${params}`,{signal:AbortSignal.any([controller.signal,AbortSignal.timeout(10000)])});if(own!==sequence)return;
    if(!Array.isArray(data.hits)||(data.total!==null&&!Number.isInteger(data.total)))throw new APIError('SEARCH_UNAVAILABLE',502);
    result.value={
      total:data.total,
      has_more:Boolean(data.has_more??data.hasMore??(data.total!==null&&page.value*10<data.total)),
      elapsed_ms:typeof data.elapsed_ms==='number'?data.elapsed_ms:undefined,
      corrected_query:typeof data.corrected_query==='string'?data.corrected_query:(typeof data.did_you_mean==='string'?data.did_you_mean:undefined),
      did_you_mean:typeof data.did_you_mean==='string'?data.did_you_mean:(typeof data.corrected_query==='string'?data.corrected_query:undefined),
      hits:data.hits.filter((x:any)=>{try{return typeof x.title==='string'&&typeof x.description==='string'&&['https:','http:'].includes(new URL(x.url).protocol);}catch{return false;}}).map((x:any)=>({
        title:x.title,
        url:x.url,
        description:x.description,
        site_name:x.site_name,
        source:x.source
      }))
    };
  }catch(e){if(own===sequence&&!controller?.signal.aborted)error.value=e instanceof APIError?e.code:'SEARCH_UNAVAILABLE';}finally{if(own===sequence)busy.value=false;}
}
function metadata(){
  document.documentElement.lang=locale.value;
  document.title=`${titles.value[route.value]||t.value.tagline} · Open Scanner`;
  document.querySelector('meta[name="description"]')?.setAttribute('content',t.value.intro);
  document.querySelector('meta[name="robots"]')?.setAttribute('content',route.value===''?'index,follow':'noindex,nofollow');
  document.querySelectorAll('link[rel="canonical"],link[hreflang]').forEach(x=>x.remove());
  if(route.value===''){
    const canonical=document.createElement('link');canonical.rel='canonical';canonical.href=`${location.origin}/${locale.value}`;document.head.append(canonical);
    for(const language of locales){const alt=document.createElement('link');alt.rel='alternate';alt.hreflang=language;alt.href=`${location.origin}/${language}`;document.head.append(alt);}
  }
}
function restore(){
  const segments=location.pathname.split('/').filter(Boolean);
  if(!locales.includes(segments[0] as Locale)){
    const saved=localStorage.getItem('openportal.locale') as Locale|null;
    const browser=navigator.languages.map(x=>x.split('-')[0]).find(x=>locales.includes(x as Locale));
    const country=(document.querySelector('meta[name="country"]')?.getAttribute('content')||'').toUpperCase();
    const accountLocale=locales.includes(user.value?.locale)?user.value.locale:null;
    const value=(saved&&locales.includes(saved)?saved:accountLocale||browser||(country==='TH'?'th':country==='KR'?'ko':'en')) as Locale;
    const legacy=location.pathname==='/register'?'/signup':location.pathname;
    return navigate(`/${value}${legacy==='/'?'':legacy}${location.search}${location.hash}`,true);
  }
  locale.value=segments[0] as Locale;route.value=segments.slice(1).join('/');if(route.value==='register')return navigate(`/${locale.value}/signup${location.search}`,true);
  error.value='';notice.value='';password.value='';confirm.value='';agree.value=false;result.value=null;controller?.abort();++sequence;busy.value=false;
  token.value=new URLSearchParams(location.hash.slice(1)).get('token')||'';if(token.value)history.replaceState(null,'',location.pathname+location.search);metadata();
  if(['mail','account'].includes(route.value)&&ready.value&&!user.value)return navigate(`/${locale.value}/login?${new URLSearchParams({returnTo:location.pathname+location.search})}`,true);
  if(route.value==='search'){
    const params=new URLSearchParams(location.search);
    search.value=params.get('q')||'';
    const ss=params.get('safe_search');
    if(ss&&['strict','moderate','off'].includes(ss))safeSearch.value=ss;
    const requested=Number(params.get('page')||1);
    page.value=Number.isInteger(requested)&&requested>=1&&requested<=100?requested:1;
    runSearch();
  }else search.value='';
  if(route.value==='account'&&user.value)displayName.value=user.value.display_name;
}
async function authSubmit(){busy.value=true;error.value='';notice.value='';try{
  if(['signup','reset-password'].includes(route.value)&&password.value!==confirm.value){notice.value=t.value.mismatch;return;}
  if(route.value==='signup'){await mutate('/auth/signup',{email:email.value,password:password.value,displayName:displayName.value,locale:locale.value,agree:agree.value});notice.value=t.value.signupNotice;}
  else if(route.value==='login'){await mutate('/auth/login',{email:email.value,password:password.value});user.value=await session();navigate(safeReturn(new URLSearchParams(location.search).get('returnTo')));}
  else if(route.value==='forgot-password'){await mutate('/auth/forgot-password',{email:email.value});notice.value=t.value.resetNotice;}
  else if(route.value==='verify-email'){await mutate('/auth/verify',{token:token.value});token.value='';notice.value=t.value.verified;}
  else if(route.value==='reset-password'){await mutate('/auth/reset-password',{token:token.value,password:password.value});token.value='';user.value=null;notice.value=t.value.resetDone;}
}catch(e){error.value=e instanceof APIError?e.code:'fallback';}finally{busy.value=false;password.value='';confirm.value='';}}
async function resend(){busy.value=true;try{await mutate('/auth/resend',{email:email.value});notice.value=t.value.signupNotice;}catch(e){error.value=e instanceof APIError?e.code:'fallback';}finally{busy.value=false;}}
async function logout(){try{await mutate('/auth/logout',{});user.value=null;navigate(`/${locale.value}`);}catch(e){error.value=e instanceof APIError?e.code:'fallback';}}
async function saveAccount(){busy.value=true;try{await mutate('/account',{displayName:displayName.value,locale:locale.value},'PATCH');user.value=await session();notice.value=t.value.saved;}catch(e){error.value=e instanceof APIError?e.code:'fallback';}finally{busy.value=false;}}
async function deleteAccount(){busy.value=true;try{await mutate('/account',{password:password.value},'DELETE');user.value=null;navigate(`/${locale.value}`);}catch(e){error.value=e instanceof APIError?e.code:'fallback';}finally{busy.value=false;password.value='';}}
function expired(){user.value=null;navigate(`/${locale.value}/login?${new URLSearchParams({returnTo:location.pathname+location.search})}`,true);}
watch(theme,value=>{document.documentElement.dataset.theme=value;localStorage.setItem('openportal.theme',value);});
onMounted(async()=>{theme.value=localStorage.getItem('openportal.theme')||'system';try{user.value=await session();}catch{error.value='fallback';}ready.value=true;restore();window.addEventListener('popstate',restore);refreshTimer=setInterval(async()=>{if(user.value)try{await mutate('/auth/refresh',{});}catch(e){if(e instanceof APIError&&e.status===401)expired();}},15*60*1000);});
onUnmounted(()=>{controller?.abort();if(refreshTimer)clearInterval(refreshTimer);window.removeEventListener('popstate',restore);});
</script>

<template>
  <a class="skip-link" href="#main-content">{{t.skip}}</a><div class="portal">
    <header class="site-header">
      <a class="brand" :href="`/${locale}`" @click="link($event,`/${locale}`)">
        <svg class="brand-symbol" aria-hidden="true" viewBox="0 0 32 32" fill="none">
          <path d="M4 10V6a2 2 0 0 1 2-2h4M22 4h4a2 2 0 0 1 2 2v4M28 22v4a2 2 0 0 1-2 2h-4M10 28H6a2 2 0 0 1-2-2v-4" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
          <ellipse cx="16" cy="16" rx="9" ry="4" transform="rotate(-30 16 16)" stroke="#22c7e8" stroke-width="1.8" stroke-dasharray="3 2"/>
          <circle cx="16" cy="16" r="3.5" fill="#7c3aed"/>
        </svg>
        <span class="brand-name">Open Scanner</span>
      </a>
      <nav :aria-label="t.services" class="header-nav">
        <label><span class="sr-only">{{t.language}}</span><select :aria-label="t.language" :value="locale" @change="changeLanguage"><option value="ko">한국어</option><option value="th">ไทย</option><option value="en">English</option></select></label>
        <label><span class="sr-only">{{t.theme}}</span><select v-model="theme" :aria-label="t.theme"><option value="system">{{t.system}}</option><option value="light">{{t.light}}</option><option value="dark">{{t.dark}}</option></select></label>
        <details class="services-dropdown">
          <summary class="services-trigger" :title="t.services" :aria-label="t.services">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <circle cx="5" cy="5" r="2"/><circle cx="12" cy="5" r="2"/><circle cx="19" cy="5" r="2"/>
              <circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/>
              <circle cx="5" cy="19" r="2"/><circle cx="12" cy="19" r="2"/><circle cx="19" cy="19" r="2"/>
            </svg>
          </summary>
          <div class="dropdown services-panel">
            <div class="ecosystem-header">
              <strong>Open Scanner Ecosystem</strong>
              <span>Search Anything · Discover Everything · Do Anything</span>
            </div>
            <div class="ecosystem-grid">
              <a :href="`/${locale}/mail`" @click="link($event,`/${locale}/mail`)" class="ecosystem-item active">
                <span class="eco-icon mail-icon">✉</span>
                <div class="eco-info">
                  <strong>Open Mail</strong>
                  <small>E2EE JMAP Webmail</small>
                </div>
              </a>
              <div class="ecosystem-item roadmap" title="Search-to-Action Ecosystem Roadmap">
                <span class="eco-icon trip-icon">✈</span>
                <div class="eco-info">
                  <strong>TripScanner</strong>
                  <small>Metasearch & Stays</small>
                </div>
              </div>
              <div class="ecosystem-item roadmap" title="Search-to-Action Ecosystem Roadmap">
                <span class="eco-icon price-icon">🏷</span>
                <div class="eco-info">
                  <strong>PriceScanner</strong>
                  <small>Price Intelligence</small>
                </div>
              </div>
              <div class="ecosystem-item roadmap" title="Search-to-Action Ecosystem Roadmap">
                <span class="eco-icon ai-icon">✦</span>
                <div class="eco-info">
                  <strong>Open AI</strong>
                  <small>RAG Deep Inference</small>
                </div>
              </div>
              <div class="ecosystem-item roadmap" title="Search-to-Action Ecosystem Roadmap">
                <span class="eco-icon dev-icon">⚙</span>
                <div class="eco-info">
                  <strong>Open Developer</strong>
                  <small>OpenAPI 3.1 & SDKs</small>
                </div>
              </div>
              <div class="ecosystem-item roadmap" title="Search-to-Action Ecosystem Roadmap">
                <span class="eco-icon space-icon">◈</span>
                <div class="eco-info">
                  <strong>Open Workspace</strong>
                  <small>Markdown Sync</small>
                </div>
              </div>
            </div>
          </div>
        </details>
        <details v-if="user"><summary class="account-button">{{user.display_name}}</summary><div class="dropdown"><a :href="`/${locale}/account`" @click="link($event,`/${locale}/account`)">{{t.account}}</a><button @click="logout">{{t.logout}}</button></div></details>
        <a v-else class="button primary" :href="`/${locale}/login`" @click="link($event,`/${locale}/login`)">{{t.login}}</a>
      </nav>
    </header>
    <main id="main-content" :class="{'home-main':route==='','mail-main':route==='mail'}">
      <template v-if="isSearch">
        <section v-if="route===''" class="hero">
          <p class="eyebrow">OPEN SCANNER</p>
          <h1>{{t.tagline}}</h1>
          <p class="hero-slogan">{{t.slogan}}</p>
          <p class="hero-intro">{{t.intro}}</p>
        </section>
        <form class="search-form" role="search" @submit.prevent="submitSearch()">
          <label class="sr-only" for="query">{{t.search}}</label>
          <div class="search-input-wrapper">
            <svg class="search-prefix-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <input id="query" v-model="search" type="search" :placeholder="t.searchPlaceholder" maxlength="512" enterkeyhint="search" autocomplete="off" @input="onSearchInput" @focus="onSearchFocus" @blur="onSearchBlur" @keydown.down.prevent="onSuggestDown" @keydown.up.prevent="onSuggestUp" @keydown.esc="closeSuggestions" required/>
            <button v-if="search" type="button" class="clear-query-btn" @click="clearSearch" :title="t.clearQuery" aria-label="Clear query">✕</button>
            <div v-if="busy" class="scanning-laser-beam" aria-hidden="true"></div>
            <ul v-if="showSuggestions && suggestionList.length" class="suggest-dropdown" role="listbox">
              <li v-for="(item, idx) in suggestionList" :key="item.text" :class="{active: idx === activeSuggestionIndex}" @mousedown.prevent="selectSuggestion(item.text)">
                <span class="suggest-icon" :class="{'recent-color': item.isRecent}">
                  <svg v-if="item.isRecent" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
                </span>
                <span class="suggest-text">{{item.text}}</span>
                <button v-if="item.isRecent" class="remove-recent-item-btn" @mousedown.stop.prevent="removeRecent(item.text)" :title="t.clear">✕</button>
              </li>
            </ul>
          </div>
          <button class="button primary" type="submit">{{t.search}} <span aria-hidden="true">↗</span></button>
        </form>
        <p class="source-note">{{t.searchSource}}</p>

        <section v-if="route==='search'" class="results" aria-live="polite" :aria-busy="busy">
          <div class="vertical-tabs" role="tablist">
            <button type="button" class="category-tab active" role="tab" aria-selected="true">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              {{t.verticalAll}}
            </button>
            <button type="button" class="category-tab gated" role="tab" aria-selected="false" @click="showVerticalNotice(t.verticalImages)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/></svg>
              {{t.verticalImages}}
            </button>
            <button type="button" class="category-tab gated" role="tab" aria-selected="false" @click="showVerticalNotice(t.verticalNews)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 20H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v1m2 13a2 2 0 0 1-2-2V7m2 13a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z"/></svg>
              {{t.verticalNews}}
            </button>
            <button type="button" class="category-tab gated" role="tab" aria-selected="false" @click="showVerticalNotice(t.verticalVideos)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="23 7 16 12 23 17 23 7"/><rect x="1" y="5" width="15" height="14" rx="2" ry="2"/></svg>
              {{t.verticalVideos}}
            </button>
            <button type="button" class="category-tab gated" role="tab" aria-selected="false" @click="showVerticalNotice(t.verticalMaps)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="1 6 1 22 8 18 16 22 23 18 23 2 16 6 8 2 1 6"/><line x1="8" y1="2" x2="8" y2="18"/><line x1="16" y1="6" x2="16" y2="22"/></svg>
              {{t.verticalMaps}}
            </button>
            <button type="button" class="category-tab gated" role="tab" aria-selected="false" @click="showVerticalNotice(t.verticalShopping)">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/></svg>
              {{t.verticalShopping}}
            </button>
          </div>
          <div v-if="verticalNotice" class="vertical-banner" role="status">
            <span>🔒 {{verticalNotice}}: {{t.verticalGated}}</span>
            <button type="button" class="dismiss-btn" @click="verticalNotice=''">✕</button>
          </div>

          <div class="serp-toolbar">
            <p class="result-count">
              {{result&&result.total!==null?`${t.resultCount} · 약 ${new Intl.NumberFormat(locale).format(result.total)}건`:t.resultCount}}
              <span v-if="result&&typeof result.elapsed_ms==='number'" class="elapsed-badge">({{result.elapsed_ms}}ms)</span>
            </p>
            <div class="safesearch-control">
              <label class="safesearch-label">
                <span class="sr-only">{{t.safeSearch}}</span>
                <select v-model="safeSearch" :aria-label="t.safeSearch" @change="submitSearch(1)">
                  <option value="moderate">{{t.safeSearch}}: {{t.safeSearchModerate}}</option>
                  <option value="strict">{{t.safeSearch}}: {{t.safeSearchStrict}}</option>
                  <option value="off">{{t.safeSearch}}: {{t.safeSearchOff}}</option>
                </select>
              </label>
            </div>
          </div>

          <p v-if="busy" role="status">{{t.loading}}</p>
          <div v-else-if="error" class="alert" role="alert">
            <p>{{errorText}}</p>
            <button class="button" @click="runSearch">{{t.retry}}</button>
          </div>
          <template v-else-if="result">
            <div v-if="result.did_you_mean || result.corrected_query" class="did-you-mean">
              <span>{{t.didYouMean}}</span>
              <a href="#" @click.prevent="applyCorrection(result.did_you_mean || result.corrected_query || '')">{{result.did_you_mean || result.corrected_query}}</a>
            </div>
            <div v-if="!result.hits.length" class="empty-state">
              <p class="empty-title">{{t.empty}}</p>
              <ul class="empty-tips">
                <li>{{t.emptyTipSpelling}}</li>
                <li>{{t.emptyTipGeneral}}</li>
                <li>{{t.emptyTipFewer}}</li>
              </ul>
            </div>
            <article v-for="hit in result.hits" :key="hit.url" class="hit">
              <div class="hit-meta">
                <svg class="hit-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/>
                  <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
                </svg>
                <span class="hit-domain">{{getHostname(hit.url)}}</span>
              </div>
              <a :href="hit.url" rel="noopener noreferrer"><h2>{{hit.title}}</h2></a>
              <p>{{hit.description}}</p>
              <p v-if="hit.source" class="source-tag">{{hit.source}}</p>
            </article>
            <nav v-if="result.has_more||(result.total!==null&&result.total>0)" class="pagination" :aria-label="t.resultCount">
              <button v-if="page>1" class="button" @click="submitSearch(page-1)">{{t.previous}}</button>
              <span>{{new Intl.NumberFormat(locale).format(page)}}</span>
              <button v-if="result.has_more||(result.total!==null&&page*10<result.total&&page<100)" class="button" @click="submitSearch(page+1)">{{t.next}}</button>
            </nav>
          </template>
        </section>
      </template>
      <MailView v-else-if="route==='mail'&&user" :locale="locale" :user="user" @expired="expired"/>
      <section v-else-if="['login','signup','forgot-password','reset-password','verify-email'].includes(route)" class="auth-card"><p class="eyebrow">OPEN SCANNER</p><h1>{{titles[route]}}</h1><p v-if="!ready" role="status">{{t.loading}}</p><form v-else @submit.prevent="authSubmit" class="form-stack"><label v-if="['login','signup','forgot-password'].includes(route)">{{t.email}}<input v-model="email" type="email" autocomplete="email" required maxlength="254"/></label><label v-if="route==='signup'">{{t.displayName}}<input v-model="displayName" autocomplete="nickname" required maxlength="80"/></label><label v-if="['login','signup','reset-password'].includes(route)">{{t.password}}<input v-model="password" :aria-label="t.password" type="password" :autocomplete="route==='login'?'current-password':'new-password'" :minlength="route==='login'?1:12" maxlength="1024" required/><small v-if="route!=='login'">{{t.passwordHint}}</small></label><label v-if="['signup','reset-password'].includes(route)">{{t.confirmPassword}}<input v-model="confirm" type="password" autocomplete="new-password" minlength="12" maxlength="1024" required/></label><label v-if="route==='signup'" class="checkbox"><input v-model="agree" type="checkbox" required/>{{t.agree}}</label><p v-if="['verify-email','reset-password'].includes(route)&&!token&&!notice">{{t.invalidLink}}</p><button class="button primary" :disabled="busy||(['verify-email','reset-password'].includes(route)&&!token)" type="submit">{{busy?t.loading:titles[route]}}</button></form>
        <div v-if="route==='login'" class="auth-links"><a :href="`/${locale}/signup`" @click="link($event,`/${locale}/signup`)">{{t.signup}}</a><a :href="`/${locale}/forgot-password`" @click="link($event,`/${locale}/forgot-password`)">{{t.forgot}}</a></div><a v-else :href="`/${locale}/login`" @click="link($event,`/${locale}/login`)">{{t.login}}</a><details v-if="route==='login'" class="resend"><summary>{{t.resend}}</summary><form @submit.prevent="resend"><label>{{t.email}}<input v-model="email" type="email" required autocomplete="email"/></label><button class="button" :disabled="busy">{{t.resend}}</button></form></details><p v-if="notice" role="status" class="notice">{{notice}}</p><p v-if="error" role="alert" class="alert">{{errorText}}</p>
      </section>
      <section v-else-if="route==='account'&&user" class="auth-card"><h1>{{t.account}}</h1><p>{{user.email}}</p><form class="form-stack" @submit.prevent="saveAccount"><label>{{t.displayName}}<input v-model="displayName" required maxlength="80"/></label><button class="button primary" :disabled="busy">{{t.save}}</button></form><hr/><details><summary>{{t.deleteAccount}}</summary><p>{{t.deleteWarning}}</p><form class="form-stack" @submit.prevent="deleteAccount"><label>{{t.password}}<input v-model="password" :aria-label="t.password" type="password" autocomplete="current-password" required/></label><button class="button danger" :disabled="busy">{{t.deleteAccount}}</button></form></details><p v-if="notice" role="status">{{notice}}</p><p v-if="error" role="alert">{{errorText}}</p></section>
      <section v-else-if="['privacy','terms'].includes(route)" class="policy"><h1>{{titles[route]}}</h1><p class="notice">{{t.policyDraft}}</p><p>{{route==='privacy'?t.privacyBody:t.termsBody}}</p></section><section v-else-if="!ready" class="auth-card" role="status">{{t.loading}}</section><section v-else class="auth-card"><h1>{{t.notFound}}</h1><a :href="`/${locale}`" @click="link($event,`/${locale}`)">{{t.backHome}}</a></section>
    </main>
    <footer>
      <div class="footer-brand">
        <strong>Open Scanner</strong> · <span>{{t.slogan}}</span>
      </div>
      <div class="footer-links">
        <a :href="`/${locale}/privacy`" @click="link($event,`/${locale}/privacy`)">{{t.privacy}}</a>
        <a :href="`/${locale}/terms`" @click="link($event,`/${locale}/terms`)">{{t.terms}}</a>
      </div>
    </footer>
  </div>
</template>
