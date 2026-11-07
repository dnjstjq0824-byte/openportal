import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

// Emulator default only. Clients must use the gateway (/api/v2); Go /api/v1 is
// an internal engine surface. In production use HTTPS and configured endpoints.
const apiBase = String.fromEnvironment('API_BASE_URL', defaultValue: 'http://10.0.2.2:8090');

typedef SearchTransport = Future<Map<String, dynamic>> Function(String query, String lang, String safeSearch);

class SearchApiException implements Exception {
  final String code;
  const SearchApiException(this.code);
  @override
  String toString() => 'SearchApiException($code)';
}

class SearchApi {
  final String baseUrl;
  final http.Client _client;
  final Duration timeout;

  SearchApi({String? baseUrl, http.Client? client, this.timeout = const Duration(seconds: 5)})
      : baseUrl = baseUrl ?? apiBase,
        _client = client ?? http.Client();

  Future<Map<String, dynamic>> search(String query, String lang, String safeSearch) async {
    final uri = Uri.parse('$baseUrl/api/v2/search').replace(queryParameters: {
      'q': query,
      'lang': lang,
      'safe_search': safeSearch,
    });
    try {
      final response = await _client.get(uri).timeout(timeout);
      if (response.statusCode != 200) {
        throw SearchApiException('HTTP_${response.statusCode}');
      }
      final decoded = jsonDecode(utf8.decode(response.bodyBytes));
      if (decoded is! Map<String, dynamic>) {
        throw const SearchApiException('BAD_RESPONSE');
      }
      return decoded;
    } on TimeoutException {
      throw const SearchApiException('TIMEOUT');
    } on SearchApiException {
      rethrow;
    } catch (_) {
      throw const SearchApiException('CONNECTION_FAILED');
    }
  }

  void close() => _client.close();
}
