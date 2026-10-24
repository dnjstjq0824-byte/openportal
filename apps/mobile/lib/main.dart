import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

// Emulator default only. In production use HTTPS and configured release API endpoints.
const apiBase = String.fromEnvironment('API_BASE_URL', defaultValue: 'http://10.0.2.2:8080');

void main() => runApp(const OpenPortalMobileApp());

class OpenPortalMobileApp extends StatelessWidget {
  final Future<Map<String, dynamic>> Function(String query, String lang, String safeSearch)? transport;
  const OpenPortalMobileApp({super.key, this.transport});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'OpenPortal Search',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF2967CC)),
        useMaterial3: true,
      ),
      darkTheme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF2967CC), brightness: Brightness.dark),
        useMaterial3: true,
      ),
      home: SearchHome(transport: transport),
    );
  }
}

class SearchHome extends StatefulWidget {
  final Future<Map<String, dynamic>> Function(String query, String lang, String safeSearch)? transport;
  const SearchHome({super.key, this.transport});
  @override
  State<SearchHome> createState() => _SearchHomeState();
}

class _SearchHomeState extends State<SearchHome> {
  final controller = TextEditingController();
  final focusNode = FocusNode();
  List<dynamic> results = [];
  List<String> queryHistoryStack = [];
  List<String> recentSearches = [];
  bool recentOptIn = false;
  String selectedLang = 'ko';
  String safeSearch = 'moderate';
  String? correctedQuery;
  String status = '개발용 샘플 데이터셋 검색';
  bool loading = false;
  bool hasSearched = false;

  Future<Map<String, dynamic>> _defaultTransport(String q, String lang, String safe) async {
    final client = HttpClient()..connectionTimeout = const Duration(seconds: 5);
    try {
      final params = <String, String>{'q': q, 'safe_search': safe};
      if (lang.isNotEmpty) params['lang'] = lang;
      final u = Uri.parse('$apiBase/api/v1/search').replace(queryParameters: params);
      final request = await client.getUrl(u);
      final response = await request.close().timeout(const Duration(seconds: 5));
      if (response.statusCode != 200) throw Exception('HTTP ${response.statusCode}');
      final body = await utf8.decoder.bind(response).join();
      return jsonDecode(body) as Map<String, dynamic>;
    } finally {
      client.close(force: true);
    }
  }

  Future<void> search({String? overrideQuery, bool pushHistory = true}) async {
    final q = (overrideQuery ?? controller.text).trim();
    if (q.isEmpty || loading) return;
    if (overrideQuery != null) {
      controller.text = q;
    }
    focusNode.unfocus();
    setState(() {
      loading = true;
      hasSearched = true;
      status = '검색 중…';
      correctedQuery = null;
    });
    try {
      final runner = widget.transport ?? _defaultTransport;
      final json = await runner(q, selectedLang, safeSearch);
      final hits = (json['hits'] as List<dynamic>?) ?? [];
      final total = json['total'] ?? hits.length;
      final corr = json['corrected_query'] as String?;
      setState(() {
        results = hits;
        correctedQuery = (corr != null && corr.isNotEmpty && corr != q) ? corr : null;
        status = total == 0 ? '일치하는 샘플 문서가 없습니다.' : '$total개 검색 결과 · 샘플 데이터';
        if (pushHistory && (queryHistoryStack.isEmpty || queryHistoryStack.last != q)) {
          queryHistoryStack.add(q);
        }
        if (recentOptIn) {
          recentSearches.remove(q);
          recentSearches.insert(0, q);
          if (recentSearches.length > 10) {
            recentSearches = recentSearches.sublist(0, 10);
          }
        }
      });
    } catch (_) {
      setState(() {
        results = [];
        status = '검색 서버에 연결할 수 없습니다.';
      });
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> shareHit(Map<String, dynamic> hit) async {
    final title = '${hit['title'] ?? ''}';
    final url = '${hit['url'] ?? ''}';
    await Clipboard.setData(ClipboardData(text: '$title - $url'));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('공유 링크 복사됨: $url'),
        duration: const Duration(seconds: 2),
      ),
    );
  }

  void handleBackNavigation() {
    if (queryHistoryStack.length > 1) {
      queryHistoryStack.removeLast();
      final prev = queryHistoryStack.last;
      search(overrideQuery: prev, pushHistory: false);
    } else if (hasSearched) {
      setState(() {
        queryHistoryStack.clear();
        hasSearched = false;
        results = [];
        correctedQuery = null;
        controller.clear();
        status = '개발용 샘플 데이터셋 검색';
      });
    }
  }

  @override
  void dispose() {
    controller.dispose();
    focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !hasSearched,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          handleBackNavigation();
        }
      },
      child: Scaffold(
        appBar: AppBar(
          leading: hasSearched
              ? Semantics(
                  button: true,
                  label: '이전 검색 또는 홈으로 돌아가기',
                  child: IconButton(
                    icon: const Icon(Icons.arrow_back),
                    tooltip: '뒤로가기',
                    onPressed: handleBackNavigation,
                  ),
                )
              : null,
          title: Semantics(
            header: true,
            child: const Text('OpenPortal Search'),
          ),
          actions: [
            DropdownButtonHideUnderline(
              child: DropdownButton<String>(
                value: selectedLang,
                tooltip: '검색 언어 선택',
                items: const [
                  DropdownMenuItem(value: 'ko', child: Text('한국어')),
                  DropdownMenuItem(value: 'en', child: Text('English')),
                ],
                onChanged: (val) {
                  if (val != null) setState(() => selectedLang = val);
                },
              ),
            ),
            const SizedBox(width: 8),
          ],
        ),
        body: SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Semantics(
                  label: '검색어 입력창',
                  textField: true,
                  child: TextField(
                    controller: controller,
                    focusNode: focusNode,
                    textInputAction: TextInputAction.search,
                    maxLength: 200,
                    onSubmitted: (_) => search(),
                    decoration: InputDecoration(
                      labelText: '검색어',
                      hintText: '검색어를 입력하세요',
                      counterText: '',
                      border: const OutlineInputBorder(),
                      suffixIcon: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (controller.text.isNotEmpty)
                            IconButton(
                              tooltip: '검색어 지우기',
                              icon: const Icon(Icons.clear),
                              onPressed: () => setState(() => controller.clear()),
                            ),
                          IconButton(
                            tooltip: '검색 실행',
                            onPressed: loading ? null : () => search(),
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
                      label: const Text('최근검색 저장 (Opt-in)'),
                      selected: recentOptIn,
                      onSelected: (v) {
                        setState(() {
                          recentOptIn = v;
                          if (!v) recentSearches.clear();
                        });
                      },
                    ),
                    DropdownButton<String>(
                      value: safeSearch,
                      items: const [
                        DropdownMenuItem(value: 'strict', child: Text('세이프서치: 엄격')),
                        DropdownMenuItem(value: 'moderate', child: Text('세이프서치: 보통')),
                        DropdownMenuItem(value: 'off', child: Text('세이프서치: 끔')),
                      ],
                      onChanged: (v) {
                        if (v != null) setState(() => safeSearch = v);
                      },
                    ),
                  ],
                ),
                if (correctedQuery != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Semantics(
                      button: true,
                      label: '추천 검색어 $correctedQuery 로 다시 검색',
                      child: InkWell(
                        onTap: () => search(overrideQuery: correctedQuery),
                        child: Text(
                          '다음 검색어로 검색하시겠습니까? $correctedQuery',
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
                const SizedBox(height: 12),
                Expanded(
                  child: ListView.builder(
                    itemCount: results.length,
                    itemBuilder: (c, i) {
                      final h = results[i] as Map<String, dynamic>;
                      final title = '${h['title'] ?? ''}';
                      final desc = '${h['description'] ?? ''}';
                      final url = '${h['url'] ?? ''}';
                      return Semantics(
                        container: true,
                        label: '검색 결과 ${i + 1}: $title, 출처 $url',
                        child: Card(
                          margin: const EdgeInsets.only(bottom: 12),
                          child: ListTile(
                            title: Text(title, style: const TextStyle(fontWeight: FontWeight.bold)),
                            subtitle: Text('$desc\n$url'),
                            isThreeLine: true,
                            trailing: IconButton(
                              tooltip: '결과 링크 공유 및 복사',
                              icon: const Icon(Icons.share),
                              onPressed: () => shareHit(h),
                            ),
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
