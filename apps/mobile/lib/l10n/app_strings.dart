// App strings for ko/en/th. Korean entries preserve the shipped v0 wording
// so existing widget tests and UX stay byte-identical.
abstract final class AppStrings {
  static const supported = ['ko', 'en', 'th'];

  static const Map<String, String> ko = {
    'appTitle': 'OpenScanner',
    'statusHome': '개발용 샘플 데이터셋 검색',
    'statusSearching': '검색 중…',
    'statusZero': '일치하는 샘플 문서가 없습니다.',
    'resultsStatus': '{total}개 검색 결과 · 샘플 데이터',
    'statusError': '검색 서버에 연결할 수 없습니다.',
    'backSemantics': '이전 검색 또는 홈으로 돌아가기',
    'backTooltip': '뒤로가기',
    'langTooltip': '검색 언어 선택',
    'langKo': '한국어',
    'langEn': 'English',
    'langTh': 'ไทย',
    'searchSemantics': '검색어 입력창',
    'searchLabel': '검색어',
    'searchHint': '검색어를 입력하세요',
    'clearTooltip': '검색어 지우기',
    'searchTooltip': '검색 실행',
    'historyChip': '최근검색 저장 (Opt-in)',
    'safeStrict': '세이프서치: 엄격',
    'safeModerate': '세이프서치: 보통',
    'safeOff': '세이프서치: 끔',
    'correctionSemantics': '추천 검색어 {q} 로 다시 검색',
    'correctionPrompt': '다음 검색어로 검색하시겠습니까? {q}',
    'shareCopied': '공유 링크 복사됨: {url}',
    'resultSemantics': '검색 결과 {i}: {title}, 출처 {url}',
    'shareTooltip': '결과 링크 공유 및 복사',
    'settingsTooltip': '설정 열기',
    'settingsTitle': '설정',
    'languageLabel': '언어',
    'safeSearchLabel': '세이프서치',
    'historyLabel': '최근검색 저장',
    'historyHint': '켜면 이 기기에만 최근 검색어를 저장합니다.',
    'clearHistory': '기록 지우기',
    'retryButton': '다시 시도',
    'notFoundTitle': '페이지를 찾을 수 없습니다',
    'notFoundHome': '홈으로',
  };

  static const Map<String, String> en = {
    'appTitle': 'OpenScanner',
    'statusHome': 'Development sample dataset search',
    'statusSearching': 'Searching…',
    'statusZero': 'No sample documents matched.',
    'resultsStatus': '{total} results · sample dataset',
    'statusError': 'Could not connect to the search server.',
    'backSemantics': 'Back to previous search or home',
    'backTooltip': 'Back',
    'langTooltip': 'Select search language',
    'langKo': '한국어',
    'langEn': 'English',
    'langTh': 'ไทย',
    'searchSemantics': 'Search query input',
    'searchLabel': 'Query',
    'searchHint': 'Enter a search query',
    'clearTooltip': 'Clear query',
    'searchTooltip': 'Run search',
    'historyChip': 'Save recent searches (Opt-in)',
    'safeStrict': 'SafeSearch: Strict',
    'safeModerate': 'SafeSearch: Moderate',
    'safeOff': 'SafeSearch: Off',
    'correctionSemantics': 'Search again with suggested query {q}',
    'correctionPrompt': 'Did you mean: {q}?',
    'shareCopied': 'Share link copied: {url}',
    'resultSemantics': 'Result {i}: {title}, source {url}',
    'shareTooltip': 'Share and copy result link',
    'settingsTooltip': 'Open settings',
    'settingsTitle': 'Settings',
    'languageLabel': 'Language',
    'safeSearchLabel': 'SafeSearch',
    'historyLabel': 'Save recent searches',
    'historyHint': 'When on, recent queries stay on this device only.',
    'clearHistory': 'Clear history',
    'retryButton': 'Retry',
    'notFoundTitle': 'Page not found',
    'notFoundHome': 'Home',
  };

  static const Map<String, String> th = {
    'appTitle': 'OpenScanner',
    'statusHome': 'ค้นหาชุดข้อมูลตัวอย่างสำหรับพัฒนา',
    'statusSearching': 'กำลังค้นหา…',
    'statusZero': 'ไม่พบเอกสารตัวอย่างที่ตรงกัน',
    'resultsStatus': 'ผลการค้นหา {total} รายการ · ชุดข้อมูลตัวอย่าง',
    'statusError': 'ไม่สามารถเชื่อมต่อเซิร์ฟเวอร์ค้นหาได้',
    'backSemantics': 'กลับไปผลค้นหาก่อนหน้าหรือหน้าหลัก',
    'backTooltip': 'ย้อนกลับ',
    'langTooltip': 'เลือกภาษาการค้นหา',
    'langKo': '한국어',
    'langEn': 'English',
    'langTh': 'ไทย',
    'searchSemantics': 'ช่องป้อนคำค้นหา',
    'searchLabel': 'คำค้นหา',
    'searchHint': 'ป้อนคำค้นหา',
    'clearTooltip': 'ล้างคำค้นหา',
    'searchTooltip': 'ค้นหา',
    'historyChip': 'บันทึกการค้นหาล่าสุด (Opt-in)',
    'safeStrict': 'ค้นหาปลอดภัย: เข้มงวด',
    'safeModerate': 'ค้นหาปลอดภัย: ปานกลาง',
    'safeOff': 'ค้นหาปลอดภัย: ปิด',
    'correctionSemantics': 'ค้นหาอีกครั้งด้วยคำแนะนำ {q}',
    'correctionPrompt': 'คุณหมายถึง: {q} หรือไม่',
    'shareCopied': 'คัดลอกลิงก์แล้ว: {url}',
    'resultSemantics': 'ผลลัพธ์ที่ {i}: {title} จาก {url}',
    'shareTooltip': 'แชร์และคัดลอกลิงก์ผลลัพธ์',
    'settingsTooltip': 'เปิดการตั้งค่า',
    'settingsTitle': 'การตั้งค่า',
    'languageLabel': 'ภาษา',
    'safeSearchLabel': 'ค้นหาปลอดภัย',
    'historyLabel': 'บันทึกการค้นหาล่าสุด',
    'historyHint': 'เมื่อเปิด คำค้นหาล่าสุดจะอยู่บนอุปกรณ์นี้เท่านั้น',
    'clearHistory': 'ล้างประวัติ',
    'retryButton': 'ลองอีกครั้ง',
    'notFoundTitle': 'ไม่พบหน้า',
    'notFoundHome': 'หน้าหลัก',
  };

  static Map<String, String> table(String locale) {
    switch (locale) {
      case 'en':
        return en;
      case 'th':
        return th;
      case 'ko':
      default:
        return ko;
    }
  }

  static String get(String locale, String key, [Map<String, Object>? values]) {
    final template = table(locale)[key] ?? ko[key] ?? key;
    if (values == null || values.isEmpty) return template;
    var out = template;
    values.forEach((name, value) {
      out = out.replaceAll('{$name}', value.toString());
    });
    return out;
  }
}
