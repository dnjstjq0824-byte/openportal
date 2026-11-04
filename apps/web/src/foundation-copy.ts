import type { Locale } from './i18n';
export const foundationMessages: Record<Locale,{developmentIndex:string;providerUnavailable:string}>={
  ko:{developmentIndex:'개발용 검색 색인입니다. 테스트 문서가 포함될 수 있으며, 실제 웹 전체의 검색 범위와 최신성은 아직 검증하지 않았습니다.',providerUnavailable:'검색 공급원이 아직 설정되지 않았습니다. 잠시 후 다시 시도하세요.'},
  th:{developmentIndex:'นี่คือดัชนีสำหรับการพัฒนา อาจมีเอกสารทดสอบ ยังไม่ได้ตรวจสอบขอบเขตทั้งเว็บและความเป็นปัจจุบันของข้อมูล',providerUnavailable:'ยังไม่ได้ตั้งค่าแหล่งค้นหา โปรดลองอีกครั้งภายหลัง'},
  en:{developmentIndex:'This is a development search index and may include test documents. Web coverage and freshness have not been verified.',providerUnavailable:'The search provider is not configured yet. Please try again later.'},
};
