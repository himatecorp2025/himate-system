import 'package:flutter/material.dart';

class HimateColors {
  static const navy = Color(0xFF071B33);
  static const navySoft = Color(0xFF102A4A);
  static const gold = Color(0xFFD5A23F);
  static const canvas = Color(0xFFF5F7FA);
  static const border = Color(0xFFE2E8F0);
  static const text = Color(0xFF162033);
  static const muted = Color(0xFF667085);
  static const success = Color(0xFF18794E);
  static const warning = Color(0xFFB54708);
}

ThemeData buildHimateTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: HimateColors.navy,
    brightness: Brightness.light,
    primary: HimateColors.navy,
    secondary: HimateColors.gold,
    surface: Colors.white,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: HimateColors.canvas,
    fontFamily: 'Arial',
    cardTheme: const CardTheme(
      color: Colors.white,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(18)),
        side: BorderSide(color: HimateColors.border),
      ),
    ),
    inputDecorationTheme: const InputDecorationTheme(
      filled: true,
      fillColor: Colors.white,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.all(Radius.circular(12)),
      ),
    ),
  );
}
