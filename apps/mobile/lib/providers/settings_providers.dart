import 'package:flutter_riverpod/flutter_riverpod.dart';

// Korean-first default preserves the shipped v0 behavior; the in-app
// switcher (app bar + settings) changes it per session.
final localeProvider = StateProvider<String>((_) => 'ko');
final safeSearchProvider = StateProvider<String>((_) => 'moderate');
final recentOptInProvider = StateProvider<bool>((_) => false);
final recentSearchesProvider = StateProvider<List<String>>((_) => const []);
