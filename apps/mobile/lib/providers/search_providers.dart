import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/search_api.dart';
import 'settings_providers.dart';

final searchApiProvider = Provider<SearchApi>((_) => SearchApi());

final transportProvider = Provider<SearchTransport>((ref) {
  final api = ref.watch(searchApiProvider);
  return (query, lang, safe) => api.search(query, lang, safe);
});

enum SearchPhase { idle, loading, zero, results, error }

class SearchState {
  final SearchPhase phase;
  final List<Map<String, dynamic>> results;
  final List<String> history;
  final String? correctedQuery;
  final int total;
  final bool hasSearched;

  const SearchState({
    this.phase = SearchPhase.idle,
    this.results = const [],
    this.history = const [],
    this.correctedQuery,
    this.total = 0,
    this.hasSearched = false,
  });

  bool get loading => phase == SearchPhase.loading;

  SearchState copyWith({
    SearchPhase? phase,
    List<Map<String, dynamic>>? results,
    List<String>? history,
    String? Function()? correctedQuery,
    int? total,
    bool? hasSearched,
  }) {
    return SearchState(
      phase: phase ?? this.phase,
      results: results ?? this.results,
      history: history ?? this.history,
      correctedQuery: correctedQuery != null ? correctedQuery() : this.correctedQuery,
      total: total ?? this.total,
      hasSearched: hasSearched ?? this.hasSearched,
    );
  }
}

class SearchController extends Notifier<SearchState> {
  @override
  SearchState build() => const SearchState();

  Future<void> run(String rawQuery, {bool pushHistory = true}) async {
    final q = rawQuery.trim();
    if (q.isEmpty || state.loading) return;
    final transport = ref.read(transportProvider);
    final lang = ref.read(localeProvider);
    final safe = ref.read(safeSearchProvider);
    state = state.copyWith(phase: SearchPhase.loading, hasSearched: true, correctedQuery: () => null);
    try {
      final json = await transport(q, lang, safe);
      final hits = ((json['hits'] as List?) ?? [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
      final total = json['total'] is int ? json['total'] as int : hits.length;
      final corr = (json['did_you_mean'] ?? json['corrected_query']) as String?;
      var history = state.history;
      if (pushHistory && (history.isEmpty || history.last != q)) {
        history = [...history, q];
      }
      if (ref.read(recentOptInProvider)) {
        final recents = ref.read(recentSearchesProvider).where((x) => x != q).toList();
        recents.insert(0, q);
        ref.read(recentSearchesProvider.notifier).state = recents.take(10).toList();
      }
      state = state.copyWith(
        phase: total == 0 ? SearchPhase.zero : SearchPhase.results,
        results: hits,
        history: history,
        correctedQuery: () => (corr != null && corr.isNotEmpty && corr != q) ? corr : null,
        total: total,
      );
    } catch (_) {
      state = state.copyWith(phase: SearchPhase.error, results: []);
    }
  }

  /// Returns the query the UI should show: previous query, or '' after reset.
  Future<String> goBack() async {
    if (state.history.length > 1) {
      final history = state.history.sublist(0, state.history.length - 1);
      state = state.copyWith(history: history);
      final prev = history.last;
      await run(prev, pushHistory: false);
      return prev;
    }
    state = const SearchState();
    return '';
  }
}

final searchProvider = NotifierProvider<SearchController, SearchState>(SearchController.new);
