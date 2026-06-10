import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/match_provider.dart';
import '../theme/chess_theme.dart';

class ComputerGameScreen extends StatefulWidget {
  const ComputerGameScreen({super.key});

  @override
  State<ComputerGameScreen> createState() => _ComputerGameScreenState();
}

class _ComputerGameScreenState extends State<ComputerGameScreen> {
  String _selectedDifficulty = 'medium';

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Play vs Computer'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              'Select Difficulty',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 16),
            ..._buildDifficultyButtons(),
            const Spacer(),
            ElevatedButton(
              onPressed: _startGame,
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.all(16),
                backgroundColor: ChessTheme.accentBlue,
              ),
              child: Text(
                'Play vs ${_selectedDifficulty.toUpperCase()}',
                style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
              ),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildDifficultyButtons() {
    final difficulties = [
      ('beginner', 'Beginner', 'Learns basics'),
      ('easy', 'Easy', 'Solid fundamentals'),
      ('medium', 'Medium', 'Good tactics'),
      ('hard', 'Hard', 'Strong player'),
      ('expert', 'Expert', 'Full engine depth'),
    ];

    return difficulties.map((d) {
      final isSelected = _selectedDifficulty == d.$1;
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Card(
          color: isSelected ? ChessTheme.accentBlue.withOpacity(0.2) : null,
          child: InkWell(
            onTap: () => setState(() => _selectedDifficulty = d.$1),
            borderRadius: BorderRadius.circular(16),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Radio<String>(
                    value: d.$1,
                    groupValue: _selectedDifficulty,
                    onChanged: (v) => setState(() => _selectedDifficulty = v!),
                  ),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(d.$2, style: const TextStyle(fontWeight: FontWeight.bold)),
                        Text(d.$3, style: Theme.of(context).textTheme.bodySmall),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }).toList();
  }

  void _startGame() async {
    await context.read<MatchProvider>().createMatch(modeId: 'computer');
    if (mounted) {
      Navigator.pushReplacementNamed(context, '/game');
    }
  }
}
