import 'package:flutter/material.dart';

class ChessTheme {
  static const Color primaryGold = Color(0xFFFFCF72);
  static const Color backgroundDark = Color(0xFF0A0E1A);
  static const Color surfaceDark = Color(0xFF121E30);
  static const Color accentBlue = Color(0xFF7AA6FF);
  static const Color textPrimary = Color(0xFFF4E8C8);
  static const Color textSecondary = Color(0xBBBBBBA0);

  static ThemeData get darkTheme {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      colorScheme: const ColorScheme.dark(
        primary: primaryGold,
        secondary: accentBlue,
        surface: surfaceDark,
        background: backgroundDark,
      ),
      scaffoldBackgroundColor: backgroundDark,
      textTheme: const TextTheme(
        headlineLarge: TextStyle(color: textPrimary, fontWeight: FontWeight.w900),
        headlineMedium: TextStyle(color: textPrimary, fontWeight: FontWeight.w800),
        titleLarge: TextStyle(color: textPrimary, fontWeight: FontWeight.w800),
        titleMedium: TextStyle(color: textPrimary, fontWeight: FontWeight.w700),
        bodyLarge: TextStyle(color: textPrimary),
        bodyMedium: TextStyle(color: textSecondary),
      ),
    );
  }
}
