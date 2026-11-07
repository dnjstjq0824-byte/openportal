import 'package:flutter_test/flutter_test.dart';
import 'package:openscanner_mobile/l10n/app_strings.dart';

void main() {
  test('ko/en/th dictionaries share exactly the same non-empty keys', () {
    final tables = {'ko': AppStrings.ko, 'en': AppStrings.en, 'th': AppStrings.th};
    final base = AppStrings.ko.keys.toSet();
    expect(base, isNotEmpty);
    for (final entry in tables.entries) {
      expect(entry.value.keys.toSet(), base, reason: '${entry.key} key parity');
      for (final kv in entry.value.entries) {
        expect(kv.value.trim(), isNotEmpty, reason: '${entry.key}.${kv.key}');
      }
    }
  });

  test('parameterized templates fill every placeholder', () {
    final out = AppStrings.get('ko', 'resultsStatus', {'total': 3});
    expect(out, '3개 검색 결과 · 샘플 데이터');
    expect(out.contains('{'), isFalse);
    expect(AppStrings.get('en', 'correctionPrompt', {'q': 'x'}), 'Did you mean: x?');
    expect(AppStrings.get('th', 'shareCopied', {'url': 'u'}), 'คัดลอกลิงก์แล้ว: u');
  });
}
