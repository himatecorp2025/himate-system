import 'package:flutter/material.dart';

import 'api/api_client.dart';
import 'screens/login_screen.dart';
import 'screens/shell_screen.dart';
import 'services/auth_controller.dart';
import 'theme/himate_theme.dart';

class HimateApp extends StatefulWidget {
  const HimateApp({super.key});

  @override
  State<HimateApp> createState() => _HimateAppState();
}

class _HimateAppState extends State<HimateApp> {
  late final AuthController _auth;

  @override
  void initState() {
    super.initState();
    _auth = AuthController(ApiClient());
    _auth.addListener(_refresh);
    _auth.initialize();
  }

  @override
  void dispose() {
    _auth.removeListener(_refresh);
    _auth.dispose();
    super.dispose();
  }

  void _refresh() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'HIMATE System',
      debugShowCheckedModeBanner: false,
      theme: buildHimateTheme(),
      home: _auth.initializing
          ? const _StartupScreen()
          : _auth.authenticated
              ? ShellScreen(auth: _auth)
              : LoginScreen(auth: _auth),
    );
  }
}

class _StartupScreen extends StatelessWidget {
  const _StartupScreen();

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      body: Center(child: CircularProgressIndicator()),
    );
  }
}
