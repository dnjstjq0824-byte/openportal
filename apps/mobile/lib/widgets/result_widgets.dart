import 'package:flutter/material.dart';

import '../l10n/app_strings.dart';

class ResultCard extends StatelessWidget {
  final int index;
  final Map<String, dynamic> hit;
  final String locale;
  final void Function(Map<String, dynamic> hit) onShare;

  const ResultCard({super.key, required this.index, required this.hit, required this.locale, required this.onShare});

  @override
  Widget build(BuildContext context) {
    final title = '${hit['title'] ?? ''}';
    final desc = '${hit['description'] ?? ''}';
    final url = '${hit['url'] ?? ''}';
    return Semantics(
      container: true,
      label: AppStrings.get(locale, 'resultSemantics', {'i': index + 1, 'title': title, 'url': url}),
      child: Card(
        margin: const EdgeInsets.only(bottom: 12),
        child: ListTile(
          title: Text(title, style: const TextStyle(fontWeight: FontWeight.bold)),
          subtitle: Text('$desc\n$url'),
          isThreeLine: true,
          trailing: IconButton(
            tooltip: AppStrings.get(locale, 'shareTooltip'),
            icon: const Icon(Icons.share),
            onPressed: () => onShare(hit),
          ),
        ),
      ),
    );
  }
}

class LoadingSkeleton extends StatelessWidget {
  const LoadingSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    final base = Theme.of(context).colorScheme.surfaceContainerHighest.withValues(alpha: 0.6);
    return ListView.builder(
      physics: const NeverScrollableScrollPhysics(),
      itemCount: 4,
      itemBuilder: (_, __) => Container(
        height: 96,
        margin: const EdgeInsets.only(bottom: 12),
        decoration: BoxDecoration(color: base, borderRadius: BorderRadius.circular(10)),
      ),
    );
  }
}
