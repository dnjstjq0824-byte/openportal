import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:openscanner_mobile/data/search_api.dart';

void main() {
  late HttpServer server;
  late Uri received;

  setUp(() async {
    server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  });

  tearDown(() async {
    await server.close(force: true);
  });

  test('search sends q, lang and safe_search and decodes the gateway payload', () async {
    server.listen((request) async {
      received = request.uri;
      request.response.statusCode = 200;
      request.response.headers.contentType = ContentType.json;
      request.response.write(jsonEncode({'total': 1, 'hits': []}));
      await request.response.close();
    });
    final api = SearchApi(baseUrl: 'http://127.0.0.1:${server.port}');
    final json = await api.search('검색엔진', 'ko', 'strict');
    expect(json['total'], 1);
    expect(received.path, '/api/v2/search');
    expect(received.queryParameters['q'], '검색엔진');
    expect(received.queryParameters['lang'], 'ko');
    expect(received.queryParameters['safe_search'], 'strict');
    api.close();
  });

  test('non-200 responses map to HTTP_<status>', () async {
    server.listen((request) async {
      request.response.statusCode = 503;
      await request.response.close();
    });
    final api = SearchApi(baseUrl: 'http://127.0.0.1:${server.port}');
    expect(() => api.search('q', 'en', 'moderate'), throwsA(isA<SearchApiException>().having((e) => e.code, 'code', 'HTTP_503')));
    api.close();
  });

  test('non-object payloads map to BAD_RESPONSE', () async {
    server.listen((request) async {
      request.response.statusCode = 200;
      request.response.headers.contentType = ContentType.json;
      request.response.write('[1,2]');
      await request.response.close();
    });
    final api = SearchApi(baseUrl: 'http://127.0.0.1:${server.port}');
    expect(() => api.search('q', 'en', 'moderate'), throwsA(isA<SearchApiException>().having((e) => e.code, 'code', 'BAD_RESPONSE')));
    api.close();
  });
}
