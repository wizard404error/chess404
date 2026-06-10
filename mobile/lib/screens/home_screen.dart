import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';
import '../theme/chess_theme.dart';

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 40),
              Text(
                'Chess404',
                style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                  color: ChessTheme.primaryGold,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                'Chess with card effects',
                style: Theme.of(context).textTheme.bodyMedium,
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 60),
              _buildMenuButton(
                context,
                icon: Icons.play_arrow,
                label: 'Play vs Computer',
                onTap: () => Navigator.pushNamed(context, '/computer'),
              ),
              const SizedBox(height: 16),
              _buildMenuButton(
                context,
                icon: Icons.people,
                label: 'Quick Pair',
                onTap: () => Navigator.pushNamed(context, '/game'),
              ),
              const SizedBox(height: 16),
              _buildMenuButton(
                context,
                icon: Icons.history,
                label: 'Match History',
                onTap: () => Navigator.pushNamed(context, '/history'),
              ),
              const SizedBox(height: 16),
              _buildMenuButton(
                context,
                icon: Icons.settings,
                label: 'Settings',
                onTap: () => Navigator.pushNamed(context, '/settings'),
              ),
              const Spacer(),
              if (!context.watch<AuthProvider>().isAuthenticated)
                TextButton(
                  onPressed: () => Navigator.pushNamed(context, '/login'),
                  child: const Text('Sign In'),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildMenuButton(
    BuildContext context, {
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(16),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Row(
            children: [
              Icon(icon, color: ChessTheme.accentBlue, size: 28),
              const SizedBox(width: 16),
              Text(
                label,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const Spacer(),
              const Icon(Icons.chevron_right, color: ChessTheme.textSecondary),
            ],
          ),
        ),
      ),
    );
  }
}
