class MatchState {
  final String matchId;
  final String modeId;
  final String turn;
  final String status;
  final String? winner;
  final List<String> moveHistory;

  MatchState({
    required this.matchId,
    required this.modeId,
    required this.turn,
    required this.status,
    this.winner,
    this.moveHistory = const [],
  });

  factory MatchState.fromJson(Map<String, dynamic> json) {
    return MatchState(
      matchId: json['matchId'] ?? '',
      modeId: json['modeId'] ?? 'open_cards',
      turn: json['turn'] ?? 'white',
      status: json['status'] ?? 'waiting',
      winner: json['winner'],
      moveHistory: List<String>.from(json['moveHistory'] ?? []),
    );
  }
}

class MoveIntent {
  final String type;
  final String matchId;
  final Map<String, dynamic>? from;
  final Map<String, dynamic>? to;
  final String? message;

  MoveIntent({
    required this.type,
    required this.matchId,
    this.from,
    this.to,
    this.message,
  });

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'matchId': matchId,
      if (from != null) 'from': from,
      if (to != null) 'to': to,
      if (message != null) 'message': message,
    };
  }
}
