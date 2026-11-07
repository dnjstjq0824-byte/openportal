import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../l10n/app_strings.dart';
import '../providers/settings_providers.dart';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final safeSearch = ref.watch(safeSearchProvider);
    final recentOptIn = ref.watch(recentOptInProvider);
    String s(String key) => AppStrings.get(locale, key);

    return Scaffold(
      appBar: AppBar(title: Text(s('settingsTitle'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            ListTile(
              title: Text(s('languageLabel')),
              trailing: DropdownButton<String>(
                value: locale,
                items: [
                  DropdownMenuItem(value: 'ko', child: Text(s('langKo'))),
                  DropdownMenuItem(value: 'en', child: Text(s('langEn'))),
                  DropdownMenuItem(value: 'th', child: Text(s('langTh'))),
                ],
                onChanged: (v) {
                  if (v != null) ref.read(localeProvider.notifier).state = v;
                },
              ),
            ),
            const Divider(),
            ListTile(
              title: Text(s('safeSearchLabel')),
              trailing: DropdownButton<String>(
                value: safeSearch,
                items: [
                  DropdownMenuItem(value: 'strict', child: Text(s('safeStrict'))),
                  DropdownMenuItem(value: 'moderate', child: Text(s('safeModerate'))),
                  DropdownMenuItem(value: 'off', child: Text(s('safeOff'))),
                ],
                onChanged: (v) {
                  if (v != null) ref.read(safeSearchProvider.notifier).state = v;
                },
              ),
            ),
            const Divider(),
            SwitchListTile(
              title: Text(s('historyLabel')),
              subtitle: Text(s('historyHint')),
              value: recentOptIn,
              onChanged: (v) {
                ref.read(recentOptInProvider.notifier).state = v;
                if (!v) ref.read(recentSearchesProvider.notifier).state = [];
              },
            ),
            const Divider(),
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: OutlinedButton(
                onPressed: () => ref.read(recentSearchesProvider.notifier).state = [],
                child: Text(s('clearHistory')),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class NotFoundScreen extends ConsumerWidget {
  const NotFoundScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    return Scaffold(
      appBar: AppBar(title: Text(AppStrings.get(locale, 'appTitle'))),
      body: SafeArea(
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(AppStrings.get(locale, 'notFoundTitle'), style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(AppStrings.get(locale, 'notFoundHome')),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
