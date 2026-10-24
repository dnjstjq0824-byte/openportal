import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openportal_mobile/main.dart';

void main() {
  testWidgets('OpenPortal mobile search, keyboard submit, correction, back navigation, and share accessibility', (WidgetTester tester) async {
    Future<Map<String, dynamic>> fakeTransport(String q, String lang, String safe) async {
      if (q == 'rjatordpswls') {
        return {
          'query': q,
          'total': 0,
          'hits': <dynamic>[],
          'elapsed_ms': 3,
          'corrected_query': '검색엔진',
        };
      }
      return {
        'query': q,
        'total': 1,
        'hits': [
          {
            'document_id': 'fixture-001',
            'title': '오픈포털 검색엔진 프로젝트',
            'description': '독립 검색엔진 아키텍처 개발을 위한 예시 문서',
            'url': 'https://example.org/intro',
            'language': 'ko',
            'score': 2.5,
          }
        ],
        'elapsed_ms': 5,
      };
    }

    await tester.pumpWidget(OpenPortalMobileApp(transport: fakeTransport));
    expect(find.text('OpenPortal Search'), findsOneWidget);

    // Enter typo query and submit via keyboard action
    await tester.enterText(find.byType(TextField), 'rjatordpswls');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();

    expect(find.textContaining('다음 검색어로 검색하시겠습니까? 검색엔진'), findsOneWidget);

    // Tap suggested correction
    await tester.tap(find.textContaining('다음 검색어로 검색하시겠습니까? 검색엔진'));
    await tester.pumpAndSettle();

    expect(find.text('오픈포털 검색엔진 프로젝트'), findsOneWidget);
    expect(find.byTooltip('결과 링크 공유 및 복사'), findsOneWidget);

    // Test back navigation button
    await tester.tap(find.byTooltip('뒤로가기'));
    await tester.pumpAndSettle();
    expect(find.text('일치하는 샘플 문서가 없습니다.'), findsOneWidget);
  });
}

