import {fail} from './mail.mjs';
// Optional licensed API integration, disabled unless an operator explicitly
// approves the provider terms and configures a key. No scraping or caching.
export async function braveSearch(config,q,page,fetcher=fetch){
  if(!config.braveApproved||!config.braveKey)throw fail('SEARCH_NOT_CONFIGURED',503);
  if(page>10)throw fail('INVALID_QUERY');
  const url=new URL('https://api.search.brave.com/res/v1/web/search');url.search=new URLSearchParams({q,count:'10',offset:String(page-1),safesearch:'moderate',text_decorations:'false',result_filter:'web'}).toString();
  let response;try{response=await fetcher(url,{headers:{Accept:'application/json','X-Subscription-Token':config.braveKey},signal:AbortSignal.timeout(8000),redirect:'error'});}catch{throw fail('SEARCH_UNAVAILABLE',503);}
  if(!response.ok)throw fail(response.status===429?'RATE_LIMITED':'SEARCH_UNAVAILABLE',response.status===429?429:503);
  let body;try{body=await response.json();}catch{throw fail('SEARCH_UNAVAILABLE',502);}
  if(typeof body.query?.original!=='string'||(body.web&& !Array.isArray(body.web.results)))throw fail('SEARCH_UNAVAILABLE',502);
  const seen=new Set();const hits=(body.web?.results||[]).filter(x=>{try{if(typeof x.title!=='string'||typeof x.description!=='string'||!['https:','http:'].includes(new URL(x.url).protocol)||seen.has(x.url))return false;seen.add(x.url);return true;}catch{return false;}}).map(x=>({title:x.title,url:x.url,description:x.description}));
  return {query:q,total:null,hits,hasMore:page<10&&body.query.more_results_available===true,source:'brave-api'};
}
