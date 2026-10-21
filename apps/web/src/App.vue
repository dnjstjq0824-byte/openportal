<script setup lang="ts">
import { ref, computed } from 'vue';
type Hit = {title:string;url:string;description:string;language:string;score:number};
type Response = {query:string;total:number;hits:Hit[];elapsed_ms:number};
const q=ref('');const submitted=ref('');const page=ref(1);
const result=ref<Response|null>(null);const busy=ref(false);const error=ref('');
const hasSearch=computed(()=>submitted.value.length>0);
async function runSearch(nextPage=1){
  const text=q.value.trim();if(!text || busy.value)return;
  busy.value=true;error.value='';page.value=nextPage;submitted.value=text;
  try{
    const res=await fetch(`/api/v1/search?q=${encodeURIComponent(text)}&page=${page.value}`);
    if(!res.ok)throw new Error(`검색 서버 오류 (${res.status})`);
    result.value=await res.json() as Response;
  }catch(e){result.value=null;error.value=e instanceof Error?e.message:'알 수 없는 오류';}
  finally{busy.value=false;}
}
</script>
<template>
  <div class="app" :class="{'has-search':hasSearch}">
    <header><span class="brand">Open<span>Portal</span></span><span class="tag">독립 검색엔진 개발 데모 · 웹 검색만 구현</span></header>
    <main>
      <section class="search-panel">
        <h1 v-if="!hasSearch">무엇을 찾고 있나요?</h1>
        <form role="search" @submit.prevent="runSearch(1)">
          <label class="sr-only" for="query">검색어</label>
          <input id="query" v-model="q" autocomplete="off" placeholder="검색어를 입력하세요" maxlength="200" />
          <button type="submit" :disabled="busy">{{busy?'검색 중…':'검색'}}</button>
        </form>
        <nav aria-label="검색 종류"><span class="active">웹</span><span title="후속 Phase">이미지 (예정)</span><span title="후속 Phase">뉴스 (예정)</span><span title="후속 Phase">동영상 (예정)</span><span title="후속 Phase">지도 (예정)</span></nav>
      </section>
      <section v-if="hasSearch" class="results" aria-live="polite">
        <p v-if="error" class="error">{{error}}</p>
        <template v-else-if="result">
          <p class="count">검색 결과 {{result.total}}개 · 서버 {{result.elapsed_ms}}ms · 샘플 데이터셋</p>
          <article v-for="h in result.hits" :key="h.url" class="hit">
            <div class="url">{{h.url}}</div><h2><a :href="h.url" target="_blank" rel="noopener noreferrer">{{h.title}}</a></h2>
            <p>{{h.description}}</p>
          </article>
          <p v-if="result.total===0">일치하는 샘플 문서가 없습니다.</p>
          <div v-if="result.total>10" class="pager"><button :disabled="page<=1 || busy" @click="runSearch(page-1)">이전</button><span>{{page}} 페이지</span><button :disabled="page*10>=result.total || busy" @click="runSearch(page+1)">다음</button></div>
        </template>
      </section>
    </main>
    <footer>© OpenPortal Search · 본 프로젝트는 Google과 관계없는 독립 개발 샘플입니다.</footer>
  </div>
</template>
