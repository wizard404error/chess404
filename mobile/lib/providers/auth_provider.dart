import 'package:flutter/foundation.dart';

class AuthProvider extends ChangeNotifier {
  String? _guestId;
  String? _sessionSecret;
  bool _isLoading = false;

  String? get guestId => _guestId;
  String? get sessionSecret => _sessionSecret;
  bool get isLoading => _isLoading;
  bool get isAuthenticated => _guestId != null;

  Future<void> loadSession() async {
    _isLoading = true;
    notifyListeners();

    await Future.delayed(const Duration(seconds: 1));

    _isLoading = false;
    notifyListeners();
  }

  Future<void> loginAsGuest() async {
    _isLoading = true;
    notifyListeners();

    _guestId = 'guest_${DateTime.now().millisecondsSinceEpoch}';
    _sessionSecret = 'secret_${DateTime.now().millisecondsSinceEpoch}';

    _isLoading = false;
    notifyListeners();
  }

  void logout() {
    _guestId = null;
    _sessionSecret = null;
    notifyListeners();
  }
}
