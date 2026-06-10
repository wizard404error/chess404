import 'package:flutter/foundation.dart';
import '../models/match_state.dart';

class MatchProvider extends ChangeNotifier {
  MatchState? _currentMatch;
  bool _isConnecting = false;
  String? _error;

  MatchState? get currentMatch => _currentMatch;
  bool get isConnecting => _isConnecting;
  String? get error => _error;

  Future<void> createMatch({required String modeId, int clockSeconds = 600}) async {
    _isConnecting = true;
    _error = null;
    notifyListeners();

    await Future.delayed(const Duration(seconds: 1));

    _currentMatch = MatchState(
      matchId: 'match_${DateTime.now().millisecondsSinceEpoch}',
      modeId: modeId,
      turn: 'white',
      status: 'active',
    );

    _isConnecting = false;
    notifyListeners();
  }

  Future<void> joinMatch(String matchId) async {
    _isConnecting = true;
    _error = null;
    notifyListeners();

    await Future.delayed(const Duration(seconds: 1));

    _isConnecting = false;
    notifyListeners();
  }

  void leaveMatch() {
    _currentMatch = null;
    notifyListeners();
  }

  void updateMatch(MatchState match) {
    _currentMatch = match;
    notifyListeners();
  }
}
