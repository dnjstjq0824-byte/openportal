import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../l10n/app_strings.dart';
import '../providers/search_providers.dart';
import '../providers/settings_providers.dart';
import '../theme/app_theme.dart';
import '../widgets/result_widgets.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  final controller = TextEditingController();
  final focusNode = FocusNode();

  @override
  void dispose() {
    controller.dispose();
    focusNode.dispose();
    super.dispose();
  }

  String t(String key, [Map<String, Object>? values]) =>
      AppStrings.get(ref.read(localeProvider), key, values);

  Future<void> submit() async {
    final q = controller.text.trim();
    if (q.isEmpty) return;
    focusNode.unfocus();
    await ref.read(searchProvider.notifier).run(q);
  }

  Future<void> submitQuery(String q) async {
    controller.text = q;
    focusNode.unfocus();
    await ref.read(searchProvider.notifier).run(q);
  }

  Future<void> goBack() async {
    final prev = await ref.read(searchProvider.notifier).goBack();
    controller.text = prev;
  }

  Future<void> shareHit(Map<String, dynamic> hit) async {
    final title = '${hit['title'] ?? ''}';
    final url = '${hit['url'] ?? ''}';
    await Clipboard.setData(ClipboardData(text: '$title - $url'));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(t('shareCopied', {'url': url})),
        duration: const Duration(seconds: 2),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final locale = ref.watch(localeProvider);
    final search = ref.watch(searchProvider);
    final safeSearch = ref.watch(safeSearchProvider);
    final recentOptIn = ref.watch(recentOptInProvider);
    final recents = ref.watch(recentSearchesProvider);
    String s(String key, [Map<String, Object>? values]) => AppStrings.get(locale, key, values);

    final status = switch (search.phase) {
      SearchPhase.loading => s('statusSearching'),
      SearchPhase.zero => s('statusZero'),
      SearchPhase.results => s('resultsStatus', {'total': search.total}),
      SearchPhase.error => s('statusError'),
      SearchPhase.idle => s('statusHome'),
    };

    return PopScope(
      canPop: !search.hasSearched,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) goBack();
      },
      child: Scaffold(
        appBar: AppBar(
          leading: search.hasSearched
              ? Semantics(
                  button: true,
                  label: s('backSemantics'),
                  child: IconButton(
                    icon: const Icon(Icons.arrow_back),
                    tooltip: s('backTooltip'),
                    onPressed: goBack,
                  ),
                )
              : null,
          title: Semantics(header: true, child: Text(s('appTitle'))),
          actions: [
            DropdownButtonHideUnderline(
              child: Tooltip(
                message: s('langTooltip'),
                child: DropdownButton<String>(
                  value: locale,
                  items: [
                    DropdownMenuItem(value: 'ko', child: Text(s('langKo'))),
                    DropdownMenuItem(value: 'en', child: Text(s('langEn'))),
                    DropdownMenuItem(value: 'th', child: Text(s('langTh'))),
                  ],
                  onChanged: (val) {
                    if (val != null) ref.read(localeProvider.notifier).state = val;
                  },
                ),
              ),
            ),
            IconButton(
              tooltip: s('settingsTooltip'),
              icon: const Icon(Icons.settings),
              onPressed: () => context.push('/settings'),
            ),
            const SizedBox(width: 8),
          ],
        ),
        body: SafeArea(
          child: LayoutBuilder(
            builder: (context, constraints) {
              final tier = widthTierOf(constraints.maxWidth);
              final horizontal = tier == WidthTier.compact ? 16.0 : 24.0;
              return Padding(
                padding: EdgeInsets.symmetric(horizontal: horizontal, vertical: 16),
                child: Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 960),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Semantics(
                          label: s('searchSemantics'),
                          textField: true,
                          child: TextField(
                            controller: controller,
                            focusNode: focusNode,
                            textInputAction: TextInputAction.search,
                            maxLength: 200,
                            onSubmitted: (_) => submit(),
                            decoration: InputDecoration(
                              labelText: s('searchLabel'),
                              hintText: s('searchHint'),
                              counterText: '',
                              border: const OutlineInputBorder(),
                              suffixIcon: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  if (controller.text.isNotEmpty)
                                    IconButton(
                                      tooltip: s('clearTooltip'),
                                      icon: const Icon(Icons.clear),
                                      onPressed: () => setState(() => controller.clear()),
                                    ),
                                  IconButton(
                                    tooltip: s('searchTooltip'),
                                    onPressed: search.loading ? null : submit,
                                    icon: const Icon(Icons.search),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 12,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            FilterChip(
                              label: Text(s('historyChip')),
                              selected: recentOptIn,
                              onSelected: (v) {
                                ref.read(recentOptInProvider.notifier).state = v;
                                if (!v) ref.read(recentSearchesProvider.notifier).state = [];
                              },
                            ),
                            DropdownButton<String>(
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
                          ],
                        ),
                        if (search.correctedQuery != null)
                          Padding(
                            padding: const EdgeInsets.only(top: 8),
                            child: Semantics(
                              button: true,
                              label: s('correctionSemantics', {'q': search.correctedQuery!}),
                              child: InkWell(
                                onTap: () => submitQuery(search.correctedQuery!),
                                child: Text(
                                  s('correctionPrompt', {'q': search.correctedQuery!}),
                                  style: TextStyle(
                                    color: Theme.of(context).colorScheme.primary,
                                    fontWeight: FontWeight.w600,
                                  ),
                                ),
                              ),
                            ),
                          ),
                        const SizedBox(height: 12),
                        Semantics(
                          liveRegion: true,
                          child: Text(status, style: Theme.of(context).textTheme.bodyMedium),
                        ),
                        if (search.phase == SearchPhase.error) ...[
                          const SizedBox(height: 8),
                          Align(
                            alignment: Alignment.centerLeft,
                            child: FilledButton.tonal(
                              onPressed: () => submitQuery(controller.text),
                              child: Text(s('retryButton')),
                            ),
                          ),
                        ],
                        if (!search.hasSearched && recentOptIn && recents.isNotEmpty) ...[
                          const SizedBox(height: 12),
                          Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              for (final r in recents)
                                ActionChip(label: Text(r), onPressed: () => submitQuery(r)),
                            ],
                          ),
                        ],
                        const SizedBox(height: 12),
                        Expanded(
                          child: RefreshIndicator(
                            onRefresh: () async {
                              if (controller.text.trim().isNotEmpty) {
                                await ref.read(searchProvider.notifier).run(controller.text, pushHistory: false);
                              }
                            },
                            child: search.loading
                                ? const LoadingSkeleton()
                                : tier == WidthTier.expanded
                                    ? GridView.builder(
                                        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                                          crossAxisCount: 2,
                                          mainAxisExtent: 148,
                                          crossAxisSpacing: 12,
                                        ),
                                        itemCount: search.results.length,
                                        itemBuilder: (c, i) => ResultCard(
                                          index: i,
                                          hit: search.results[i],
                                          locale: locale,
                                          onShare: shareHit,
                                        ),
                                      )
                                    : ListView.builder(
                                        itemCount: search.results.length,
                                        itemBuilder: (c, i) => ResultCard(
                                          index: i,
                                          hit: search.results[i],
                                          locale: locale,
                                          onShare: shareHit,
                                        ),
                                      ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}
