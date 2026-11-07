import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openscanner_mobile/main.dart';

void main() {
  testWidgets('settings route opens from home and pops back', (WidgetTester tester) async {
    await tester.pumpWidget(OpenScannerMobileApp(
      transport: (q, lang, safe) async => {'total': 0, 'hits': <dynamic>[]},
    ));
    await tester.pumpAndSettle();
    expect(find.text('OpenScanner'), findsOneWidget);

    await tester.tap(find.byTooltip('설정 열기'));
    await tester.pumpAndSettle();
    expect(find.text('설정'), findsWidgets);
    expect(find.text('언어'), findsOneWidget);
    expect(find.text('세이프서치'), findsOneWidget);
    expect(find.text('최근검색 저장'), findsOneWidget);

    // Language switch re-renders the settings screen in English.
    await tester.tap(find.widgetWithText(DropdownButton<String>, '한국어'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('English').last);
    await tester.pumpAndSettle();
    expect(find.text('Settings'), findsOneWidget);
    expect(find.text('Language'), findsOneWidget);

    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.text('OpenScanner'), findsOneWidget);
  });
}
