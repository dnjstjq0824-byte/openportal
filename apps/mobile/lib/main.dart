import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';

// Emulator default only. In production use HTTPS and configured release API endpoints.
const apiBase = String.fromEnvironment('API_BASE_URL', defaultValue: 'http://10.0.2.2:8080');
void main() => runApp(const MaterialApp(home: SearchHome()));
class SearchHome extends StatefulWidget {
  const SearchHome({super.key});
  @override
  State<SearchHome> createState() => _SearchHomeState();
}
class _SearchHomeState extends State<SearchHome> {
  final controller = TextEditingController();
  List<dynamic> results = [];
  String status = '개발용 샘플 데이터셋 검색';
  bool loading = false;
  Future<void> search() async {
    if (controller.text.trim().isEmpty) return;
    setState(() { loading=true; status='검색 중…'; });
    final client=HttpClient()..connectionTimeout=const Duration(seconds: 5);
    try {
      final u=Uri.parse('$apiBase/api/v1/search').replace(queryParameters:{'q':controller.text.trim()});
      final request=await client.getUrl(u);
      final response=await request.close().timeout(const Duration(seconds: 5));
      if(response.statusCode!=200) throw Exception('HTTP ${response.statusCode}');
      final body=await utf8.decoder.bind(response).join();
      final json=jsonDecode(body) as Map<String,dynamic>;
      setState(() { results=json['hits'] as List<dynamic>;status='${json['total']}개 검색 결과 · 샘플 데이터'; });
    }catch(_){setState(() { results=[];status='검색 서버에 연결할 수 없습니다.'; });}
    finally { client.close(force:true); if(mounted)setState(()=>loading=false); }
  }
  @override
  void dispose(){controller.dispose();super.dispose();}
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('OpenPortal Search')),
    body: Padding(padding:const EdgeInsets.all(18),child:Column(children:[
      TextField(controller:controller,onSubmitted:(_)=>search(),decoration:InputDecoration(labelText:'검색어',suffixIcon:IconButton(onPressed:loading?null:search,icon:const Icon(Icons.search)))),
      const SizedBox(height:18),Text(status),const SizedBox(height:16),
      Expanded(child:ListView.builder(itemCount:results.length,itemBuilder:(c,i){final h=results[i] as Map<String,dynamic>;return ListTile(title:Text('${h['title']}'),subtitle:Text('${h['description']}\n${h['url']}'),isThreeLine:true);}))
    ])),
  );
}
