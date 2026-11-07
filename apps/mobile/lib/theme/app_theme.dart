import 'package:flutter/material.dart';

// Design tokens shared with the web portal.css variables.
abstract final class AppTokens {
  // Light
  static const primaryLight = Color(0xFF1F6FEB);
  static const primaryHoverLight = Color(0xFF1A60CC);
  static const primarySoftLight = Color(0xFFE9F1FD);
  static const bgLight = Color(0xFFF7F8FA);
  static const surfaceLight = Color(0xFFFFFFFF);
  static const inkLight = Color(0xFF14181D);
  static const mutedLight = Color(0xFF5D6B7A);
  static const lineLight = Color(0xFFE3E8EF);
  static const lineStrongLight = Color(0xFFC9D2DC);
  // Dark
  static const primaryDark = Color(0xFF5AA2FF);
  static const primaryHoverDark = Color(0xFF7AB5FF);
  static const primarySoftDark = Color(0x1F5AA2FF);
  static const bgDark = Color(0xFF0E1116);
  static const surfaceDark = Color(0xFF151C25);
  static const inkDark = Color(0xFFE8EDF3);
  static const mutedDark = Color(0xFF9AA6B4);
  static const lineDark = Color(0xFF232D3A);
  static const lineStrongDark = Color(0xFF354052);
  // Status
  static const okLight = Color(0xFF1A7F4D);
  static const okBgLight = Color(0xFFE6F4EC);
  static const warnLight = Color(0xFF8A5A00);
  static const warnBgLight = Color(0xFFFDF3E0);
  static const errorLight = Color(0xFFC03636);
  static const errorBgLight = Color(0xFFFDEEEE);
  static const okDark = Color(0xFF4CC38A);
  static const warnDark = Color(0xFFE0A63C);
  static const errorDark = Color(0xFFF08080);
  // Shape & spacing (4px base)
  static const radius = 10.0;
  static const radiusSmall = 8.0;
}

abstract final class AppTheme {
  static ThemeData light() {
    final scheme = ColorScheme.fromSeed(
      seedColor: AppTokens.primaryLight,
      brightness: Brightness.light,
    ).copyWith(
      primary: AppTokens.primaryLight,
      surface: AppTokens.surfaceLight,
    );
    return _build(scheme, true);
  }

  static ThemeData dark() {
    final scheme = ColorScheme.fromSeed(
      seedColor: AppTokens.primaryLight,
      brightness: Brightness.dark,
    ).copyWith(
      primary: AppTokens.primaryDark,
      surface: AppTokens.surfaceDark,
    );
    return _build(scheme, false);
  }

  static ThemeData _build(ColorScheme scheme, bool isLight) {
    return ThemeData(
      colorScheme: scheme,
      useMaterial3: true,
      scaffoldBackgroundColor: isLight ? AppTokens.bgLight : AppTokens.bgDark,
      cardTheme: CardThemeData(
        color: isLight ? AppTokens.surfaceLight : AppTokens.surfaceDark,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radius),
          side: BorderSide(color: isLight ? AppTokens.lineLight : AppTokens.lineDark),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        border: const OutlineInputBorder(
          borderRadius: BorderRadius.all(Radius.circular(AppTokens.radiusSmall)),
        ),
        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 14),
      ),
      chipTheme: ChipThemeData(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
      ),
    );
  }
}

// Width tiers: <600 compact phone, 600-839 medium, >=840 expanded (tablet/desktop).
enum WidthTier { compact, medium, expanded }

WidthTier widthTierOf(double width) {
  if (width >= 840) return WidthTier.expanded;
  if (width >= 600) return WidthTier.medium;
  return WidthTier.compact;
}
