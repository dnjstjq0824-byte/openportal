export async function readJSON(response: Response): Promise<any> {
  if (!response.ok) throw new Error(`검색 서버 오류 (${response.status})`);
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw new Error('검색 서비스에 연결할 수 없습니다. 잠시 후 다시 시도해 주세요.');
  }
  try { return await response.json(); }
  catch { throw new Error('검색 응답을 처리하지 못했습니다. 다시 시도해 주세요.'); }
}

export function validateSearch(data: any) {
  if (!data || typeof data.query !== 'string' || !Number.isInteger(data.total) || data.total < 0 || !Array.isArray(data.hits) ||
      !data.hits.every((hit: any) => hit && typeof hit.title === 'string' && typeof hit.description === 'string' && safeURL(hit.url))) {
    throw new Error('검색 응답 형식이 올바르지 않습니다. 다시 시도해 주세요.');
  }
  return data;
}
function safeURL(value: unknown): boolean {
  if (typeof value !== 'string') return false;
  try { return ['https:', 'http:'].includes(new URL(value).protocol); } catch { return false; }
}
