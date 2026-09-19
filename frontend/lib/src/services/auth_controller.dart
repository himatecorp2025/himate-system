import 'package:flutter/foundation.dart';

import '../api/api_client.dart';
import '../models/user.dart';

class AuthController extends ChangeNotifier {
  AuthController(this.api);

  final ApiClient api;

  HimateUser? _user;
  bool _initializing = true;

  HimateUser? get user => _user;
  bool get initializing => _initializing;
  bool get authenticated => _user != null;

  Future<void> initialize() async {
    try {
      final json = await api.getJson('/api/v1/auth/me');
      _user = HimateUser.fromJson(json);
    } on ApiException catch (error) {
      if (error.statusCode != 401) {
        rethrow;
      }
      _user = null;
    } finally {
      _initializing = false;
      notifyListeners();
    }
  }

  Future<void> login(String email, String password) async {
    final json = await api.postJson('/api/v1/auth/login', {
      'email': email.trim(),
      'password': password,
    });
    _user = HimateUser.fromJson(json);
    notifyListeners();
  }

  Future<void> logout() async {
    try {
      await api.postJson('/api/v1/auth/logout');
    } finally {
      _user = null;
      notifyListeners();
    }
  }
}
