import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'data/search_api.dart';
import 'l10n/app_strings.dart';
import 'providers/search_providers.dart';
import 'providers/settings_providers.dart';
import 'router.dart';
import 'theme/app_theme.dart';

void main() => runApp(const OpenScannerMobileApp());

class OpenScannerMobileApp extends StatelessWidget {
  final SearchTransport? transport;
  const OpenScannerMobileApp({super.key, this.transport});

  @override
  Widget build(BuildContext context) {
    return ProviderScope(
      overrides: transport == null ? [] : [transportProvider.overrideWithValue(transport!)],
      child: const _App(),
    );
  }
}

class _App extends ConsumerWidget {
  const _App();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final router = ref.watch(routerProvider);
    return MaterialApp.router(
      title: AppStrings.get(locale, 'appTitle'),
      theme: AppTheme.light(),
      darkTheme: AppTheme.dark(),
      locale: Locale(locale),
      supportedLocales: const [Locale('ko'), Locale('en'), Locale('th')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      routerConfig: router,
    );
  }
}
