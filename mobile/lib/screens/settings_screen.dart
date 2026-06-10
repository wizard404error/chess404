import 'package:flutter/material.dart';
import '../theme/chess_theme.dart';

class SettingsScreen extends StatelessWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Settings'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _buildSection('Account', [
            ListTile(
              title: const Text('Guest Mode'),
              subtitle: const Text('Playing as guest'),
              trailing: const Icon(Icons.chevron_right),
            ),
          ]),
          _buildSection('Preferences', [
            SwitchListTile(
              title: const Text('Sound Effects'),
              subtitle: const Text('Play sounds during moves'),
              value: true,
              onChanged: (_) {},
            ),
            SwitchListTile(
              title: const Text('Haptic Feedback'),
              subtitle: const Text('Vibrate on piece selection'),
              value: true,
              onChanged: (_) {},
            ),
          ]),
          _buildSection('About', [
            ListTile(
              title: const Text('Version'),
              subtitle: const Text('1.0.0'),
            ),
          ]),
        ],
      ),
    );
  }

  Widget _buildSection(String title, List<Widget> children) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text(
            title.toUpperCase(),
            style: const TextStyle(
              color: ChessTheme.textSecondary,
              fontWeight: FontWeight.bold,
              letterSpacing: 1.2,
            ),
          ),
        ),
        Card(child: Column(children: children)),
        const SizedBox(height: 16),
      ],
    );
  }
}
