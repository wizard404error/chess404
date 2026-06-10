import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'providers/auth_provider.dart';
import 'providers/match_provider.dart';
import 'screens/home_screen.dart';
import 'screens/login_screen.dart';
import 'screens/game_screen.dart';
import 'screens/computer_game_screen.dart';
import 'screens/history_screen.dart';
import 'screens/settings_screen.dart';
import 'theme/chess_theme.dart';

void main() {
  runApp(const Chess404App());
}

class Chess404App extends StatelessWidget {
  const Chess404App({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => AuthProvider()),
        ChangeNotifierProvider(create: (_) => MatchProvider()),
      ],
      child: MaterialApp(
        title: 'Chess404',
        theme: ChessTheme.darkTheme,
        initialRoute: '/',
        routes: {
          '/': (context) => const HomeScreen(),
          '/login': (context) => const LoginScreen(),
          '/game': (context) => const GameScreen(),
          '/computer': (context) => const ComputerGameScreen(),
          '/history': (context) => const HistoryScreen(),
          '/settings': (context) => const SettingsScreen(),
        },
      ),
    );
  }
}
