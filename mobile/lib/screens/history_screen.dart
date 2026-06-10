import 'package:flutter/material.dart';
import '../theme/chess_theme.dart';

class HistoryScreen extends StatelessWidget {
  const HistoryScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Match History'),
      ),
      body: const Center(
        child: Text('Match history will appear here'),
      ),
    );
  }
}
