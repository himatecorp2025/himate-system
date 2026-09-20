import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_web_plugins/url_strategy.dart';
import 'package:http/browser_client.dart';
import 'package:http/http.dart' as http;
import 'package:google_fonts/google_fonts.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  usePathUrlStrategy();
  runApp(const HimateApp());
}

const brandNavy = Color(0xFF0B1F3B);
const brandSteel = Color(0xFF2E5B87);
const brandGold = Color(0xFFD4AF6B);
const brandIvory = Color(0xFFF8F9FB);
const brandMist = Color(0xFFE4E7EC);
const brandCharcoal = Color(0xFF1F2937);
const brandWhite = Color(0xFFFFFFFF);
const brandSuccess = Color(0xFF20866A);
const brandWarning = Color(0xFFB7791F);
const brandDanger = Color(0xFFB54444);
const brandNavyDeep = Color(0xFF071426);
const brandNavySoft = Color(0xFF143555);
const brandTextSoft = Color(0xFF667085);

const navy = brandNavy;
const gold = brandGold;
const canvas = brandIvory;
const muted = brandTextSoft;
const success = brandSuccess;

ThemeData buildBrandTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: brandNavy,
    brightness: Brightness.light,
    primary: brandNavy,
    secondary: brandGold,
    surface: brandWhite,
    error: brandDanger,
  );
  final base = GoogleFonts.interTextTheme();
  final display = GoogleFonts.cormorantGaramondTextTheme();
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: brandIvory,
    visualDensity: VisualDensity.standard,
    splashFactory: InkSparkle.splashFactory,
    textTheme: base.copyWith(
      displaySmall: display.displaySmall?.copyWith(color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.7, height: 1.02),
      headlineLarge: display.headlineLarge?.copyWith(color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.45, height: 1.03),
      headlineMedium: display.headlineMedium?.copyWith(color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.3, height: 1.05),
      headlineSmall: display.headlineSmall?.copyWith(color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.15, height: 1.08),
      titleLarge: display.titleLarge?.copyWith(color: brandNavy, fontWeight: FontWeight.w700),
      titleMedium: base.titleMedium?.copyWith(color: brandNavy, fontWeight: FontWeight.w700),
      bodyLarge: base.bodyLarge?.copyWith(color: brandCharcoal, height: 1.5),
      bodyMedium: base.bodyMedium?.copyWith(color: brandCharcoal, height: 1.45),
      bodySmall: base.bodySmall?.copyWith(color: brandTextSoft, height: 1.4),
      labelLarge: base.labelLarge?.copyWith(color: brandNavy, fontWeight: FontWeight.w700, letterSpacing: .05),
    ),
    cardTheme: CardThemeData(
      color: brandWhite,
      elevation: 0,
      margin: EdgeInsets.zero,
      shadowColor: brandNavy.withOpacity(.08),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(10),
        side: const BorderSide(color: brandMist),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: brandWhite,
      labelStyle: base.bodyMedium?.copyWith(color: brandTextSoft),
      hintStyle: base.bodyMedium?.copyWith(color: const Color(0xFF98A2B3)),
      prefixIconColor: brandSteel,
      suffixIconColor: brandSteel,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
      enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: const BorderSide(color: Color(0xFFD7DEE7))),
      focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: const BorderSide(color: brandSteel, width: 1.4)),
      errorBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: const BorderSide(color: brandDanger)),
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(8), borderSide: const BorderSide(color: brandMist)),
    ),
    checkboxTheme: CheckboxThemeData(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(2)),
      fillColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.selected) ? brandNavy : brandWhite),
      checkColor: WidgetStateProperty.all(brandWhite),
      side: const BorderSide(color: brandNavy, width: 1.4),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: ButtonStyle(
        backgroundColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.disabled)) return brandNavy.withOpacity(.42);
          if (states.contains(WidgetState.hovered)) return const Color(0xFF102B50);
          if (states.contains(WidgetState.pressed)) return const Color(0xFF06162B);
          return brandNavy;
        }),
        foregroundColor: WidgetStateProperty.all(brandWhite),
        overlayColor: WidgetStateProperty.all(brandGold.withOpacity(.08)),
        padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 21, vertical: 16)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(7))),
        textStyle: WidgetStateProperty.all(base.labelLarge?.copyWith(fontWeight: FontWeight.w700)),
        elevation: WidgetStateProperty.all(0),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.hovered) ? brandSteel : brandNavy),
        side: WidgetStateProperty.resolveWith((states) => BorderSide(color: states.contains(WidgetState.hovered) ? brandSteel : const Color(0xFFB9C5D3))),
        overlayColor: WidgetStateProperty.all(brandSteel.withOpacity(.05)),
        padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 20, vertical: 15)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(7))),
        textStyle: WidgetStateProperty.all(base.labelLarge?.copyWith(fontWeight: FontWeight.w700)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.hovered) ? brandSteel : brandNavy),
        overlayColor: WidgetStateProperty.all(brandSteel.withOpacity(.05)),
        textStyle: WidgetStateProperty.all(base.labelLarge?.copyWith(fontWeight: FontWeight.w600)),
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.all(brandNavy),
        overlayColor: WidgetStateProperty.all(brandSteel.withOpacity(.07)),
      ),
    ),
    dividerColor: brandMist,
    scrollbarTheme: ScrollbarThemeData(
      thumbColor: WidgetStateProperty.all(brandSteel.withOpacity(.35)),
      radius: const Radius.circular(12),
      thickness: WidgetStateProperty.all(6),
    ),
    appBarTheme: AppBarTheme(
      backgroundColor: brandWhite,
      foregroundColor: brandNavy,
      elevation: 0,
      surfaceTintColor: Colors.transparent,
      titleTextStyle: display.titleLarge?.copyWith(color: brandNavy, fontWeight: FontWeight.w600),
    ),
  );
}

class ApiError implements Exception {
  ApiError(this.status, this.message);
  final int status;
  final String message;
  @override
  String toString() => message;
}

class _ApiCacheEntry {
  const _ApiCacheEntry(this.data, this.expiresAt);
  final Map<String, dynamic> data;
  final DateTime expiresAt;
}

class Api {
  Api() : client = BrowserClient()..withCredentials = true;
  final BrowserClient client;
  final Map<String, _ApiCacheEntry> _cache = <String, _ApiCacheEntry>{};
  final Map<String, Future<Map<String, dynamic>>> _inflight = <String, Future<Map<String, dynamic>>>{};

  Future<Map<String, dynamic>> get(
    String path, {
    Duration maxAge = const Duration(seconds: 8),
    bool force = false,
  }) {
    if (!force) {
      final cached = _cache[path];
      if (cached != null && DateTime.now().isBefore(cached.expiresAt)) {
        return Future<Map<String, dynamic>>.value(cached.data);
      }
      final pending = _inflight[path];
      if (pending != null) return pending;
    }

    final future = request('GET', path).then((data) {
      _cache[path] = _ApiCacheEntry(data, DateTime.now().add(maxAge));
      return data;
    }).whenComplete(() => _inflight.remove(path));

    _inflight[path] = future;
    return future;
  }

  Future<Map<String, dynamic>> post(String path, [Map<String, dynamic>? body]) async {
    final result = await request('POST', path, body);
    clearCache();
    return result;
  }

  Future<Map<String, dynamic>> put(String path, Map<String, dynamic> body) async {
    final result = await request('PUT', path, body);
    clearCache();
    return result;
  }

  Future<Map<String, dynamic>> patch(String path, Map<String, dynamic> body) async {
    final result = await request('PATCH', path, body);
    clearCache();
    return result;
  }

  void clearCache([String? prefix]) {
    if (prefix == null) {
      _cache.clear();
      return;
    }
    _cache.removeWhere((key, _) => key.startsWith(prefix));
  }

  Future<Map<String, dynamic>> request(String method, String path, [Map<String, dynamic>? body]) async {
    final headers = <String, String>{'Accept': 'application/json'};
    if (body != null) headers['Content-Type'] = 'application/json';
    late http.Response response;
    final uri = Uri.parse(path);
    final timeout = const Duration(seconds: 8);
    if (method == 'POST') {
      response = await client.post(uri, headers: headers, body: jsonEncode(body ?? <String, dynamic>{})).timeout(timeout);
    } else if (method == 'PUT') {
      response = await client.put(uri, headers: headers, body: jsonEncode(body)).timeout(timeout);
    } else if (method == 'PATCH') {
      response = await client.patch(uri, headers: headers, body: jsonEncode(body)).timeout(timeout);
    } else {
      response = await client.get(uri, headers: headers).timeout(timeout);
    }
    if (response.statusCode == 204) return <String, dynamic>{};
    Map<String, dynamic> data = <String, dynamic>{};
    if (response.body.trim().isNotEmpty) {
      final decoded = jsonDecode(response.body);
      if (decoded is Map) data = Map<String, dynamic>.from(decoded);
    }
    if (response.statusCode >= 200 && response.statusCode < 300) return data;
    final error = data['error'];
    if (error is Map && error['message'] != null) {
      throw ApiError(response.statusCode, '${error['message']}');
    }
    throw ApiError(response.statusCode, 'Request failed (${response.statusCode})');
  }
}

List<Map<String, dynamic>> items(Map<String, dynamic> json) {
  final raw = json['items'];
  if (raw is! List) return <Map<String, dynamic>>[];
  return raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
}

double number(dynamic value) => value is num ? value.toDouble() : double.tryParse('$value') ?? 0;
String money(dynamic value) => '\$${number(value).toStringAsFixed(2)}';

class HimateApp extends StatefulWidget {
  const HimateApp({super.key});

  @override
  State<HimateApp> createState() => _HimateAppState();
}

class _HimateAppState extends State<HimateApp> {
  final api = Api();
  final navigatorKey = GlobalKey<NavigatorState>();
  Map<String, dynamic>? user;
  bool loading = true;

  Timer? _restoreFallback;
  String? _pendingDeepLink;
  bool _deepLinkHandled = false;

  @override
  void initState() {
    super.initState();
    final path = Uri.base.path;
    if (path == '/app' || path.startsWith('/app/')) {
      if (path != '/app') _pendingDeepLink = path;
      _restoreFallback = Timer(const Duration(seconds: 3), () {
        if (mounted && loading) {
          setState(() => loading = false);
        }
      });
      restore();
    } else {
      // The public login route must paint immediately. Session restoration is
      // only required when opening a protected application route.
      loading = false;
    }
  }

  @override
  void dispose() {
    _restoreFallback?.cancel();
    super.dispose();
  }

  Future<void> restore() async {
    try {
      user = await api
          .get('/api/v1/auth/me', force: true)
          .timeout(const Duration(seconds: 3));
    } catch (_) {
      // Any auth/network failure falls back to the login screen instead of
      // trapping the user behind an endless loading indicator.
      user = null;
    } finally {
      _restoreFallback?.cancel();
      if (mounted) {
        setState(() => loading = false);
        if (user != null && _pendingDeepLink != null && !_deepLinkHandled) {
          _deepLinkHandled = true;
          final target = _pendingDeepLink!;
          WidgetsBinding.instance.addPostFrameCallback((_) {
            navigatorKey.currentState?.pushNamed(target);
          });
        }
      }
    }
  }

  Future<void> login(String email, String password) async {
    user = await api.post('/api/v1/auth/login', {'email': email, 'password': password});
    if (!mounted) return;
    setState(() {});
    final target = _pendingDeepLink;
    navigatorKey.currentState?.pushNamedAndRemoveUntil('/app', (route) => false);
    if (target != null && !_deepLinkHandled) {
      _deepLinkHandled = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        navigatorKey.currentState?.pushNamed(target);
      });
    }
  }

  Future<void> logout() async {
    await api.post('/api/v1/auth/logout');
    user = null;
    if (!mounted) return;
    setState(() {});
    navigatorKey.currentState?.pushNamedAndRemoveUntil('/login', (route) => false);
  }

  Widget loadingScreen() => const Scaffold(
        backgroundColor: brandNavyDeep,
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              BrandMark(size: 42),
              SizedBox(height: 16),
              SizedBox(
                width: 28,
                height: 28,
                child: CircularProgressIndicator(strokeWidth: 2.2, color: brandGold),
              ),
            ],
          ),
        ),
      );

  @override
  Widget build(BuildContext context) {
    final path = Uri.base.path;
    final initial = path == '/app' || path.startsWith('/app/') ? '/app' : '/login';

    return MaterialApp(
      navigatorKey: navigatorKey,
      debugShowCheckedModeBanner: false,
      title: 'HIMATE System',
      theme: buildBrandTheme(),
      initialRoute: initial,
      routes: {
        // The public login route must never be gated by protected-route
        // session restoration. This guarantees that a failed /auth/me request
        // cannot strand visitors behind a global spinner.
        '/login': (_) => user == null
            ? LoginPage(onLogin: login)
            : _SignedInRedirect(onContinue: () {
                navigatorKey.currentState?.pushNamedAndRemoveUntil('/app', (route) => false);
              }),
        '/app': (_) => loading
            ? loadingScreen()
            : user == null
                ? LoginPage(onLogin: login)
                : Shell(api: api, user: user!, onLogout: logout),
      },
      onGenerateRoute: (settings) {
        final name = settings.name ?? '';
        final match = RegExp(r'^/app/partners/([^/]+)(?:/([^/]+))?$').firstMatch(name);
        if (match == null) return null;
        final partnerId = Uri.decodeComponent(match.group(1)!);
        final section = match.group(2);
        return MaterialPageRoute(
          settings: settings,
          builder: (_) => loading
              ? loadingScreen()
              : user == null
                  ? LoginPage(onLogin: login)
                  : PartnerRouteLoader(api: api, partnerId: partnerId, initialSection: section),
        );
      },
      onUnknownRoute: (_) => MaterialPageRoute(
        settings: const RouteSettings(name: '/login'),
        builder: (_) => user == null
            ? LoginPage(onLogin: login)
            : _SignedInRedirect(onContinue: () {
                navigatorKey.currentState?.pushNamedAndRemoveUntil('/app', (route) => false);
              }),
      ),
    );
  }
}

class PartnerRouteLoader extends StatelessWidget {
  const PartnerRouteLoader({required this.api, required this.partnerId, this.initialSection, super.key});
  final Api api;
  final String partnerId;
  final String? initialSection;

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: api.get('/api/v1/partners/$partnerId', force: true),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) return const _BrandLoading();
        if (snapshot.hasError || snapshot.data == null) {
          return Scaffold(
            backgroundColor: brandIvory,
            appBar: AppBar(leading: IconButton(onPressed: () => Navigator.maybePop(context), icon: const Icon(Icons.arrow_back_rounded))),
            body: Padding(
              padding: const EdgeInsets.all(24),
              child: _MessageCard(icon: Icons.error_outline_rounded, title: 'Partner could not be opened', message: '${snapshot.error ?? 'Partner not found'}'),
            ),
          );
        }
        return PartnerWorkspace(api: api, partner: snapshot.data!, initialSection: initialSection);
      },
    );
  }
}

String workspaceRouteSlug(String title) => title
    .toLowerCase()
    .replaceAll('&', 'and')
    .replaceAll(RegExp(r'[^a-z0-9]+'), '-')
    .replaceAll(RegExp(r'^-+|-+$'), '');


class _SignedInRedirect extends StatelessWidget {
  const _SignedInRedirect({required this.onContinue});
  final VoidCallback onContinue;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: brandIvory,
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Card(
            child: Padding(
              padding: const EdgeInsets.all(28),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const HimateLogo(width: 210),
                  const SizedBox(height: 22),
                  const Text(
                    'You are already signed in.',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: brandNavy, fontSize: 22, fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 8),
                  const Text(
                    'Continue to the HIMATE administration platform.',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: brandTextSoft),
                  ),
                  const SizedBox(height: 20),
                  FilledButton.icon(
                    onPressed: onContinue,
                    icon: const Icon(Icons.arrow_forward_rounded),
                    label: const Text('Open admin platform'),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class LoginPage extends StatefulWidget {
  const LoginPage({required this.onLogin, super.key});
  final Future<void> Function(String email, String password) onLogin;

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final email = TextEditingController();
  final password = TextEditingController();
  bool busy = false;
  bool obscure = true;
  bool remember = true;
  String? error;

  @override
  void dispose() {
    email.dispose();
    password.dispose();
    super.dispose();
  }

  Future<void> submit() async {
    if (email.text.trim().isEmpty || password.text.isEmpty) {
      setState(() => error = 'Enter your administrator email and password.');
      return;
    }
    setState(() { busy = true; error = null; });
    try {
      await widget.onLogin(email.text.trim(), password.text);
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  void info(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), behavior: SnackBarBehavior.floating, backgroundColor: brandNavy),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: brandNavyDeep,
      body: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < 820;
          return Stack(
            fit: StackFit.expand,
            children: [
              const _LoginArtwork(),
              DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.centerLeft,
                    end: Alignment.centerRight,
                    colors: compact
                        ? [const Color(0xB8071426), const Color(0x70071426), const Color(0xB8071426)]
                        : [const Color(0xE6071426), const Color(0x990B1F3B), const Color(0x30071426)],
                    stops: compact ? const [0, .52, 1] : const [0, .50, 1],
                  ),
                ),
              ),
              SafeArea(
                child: compact
                    ? _CompactLoginComposition(
                        email: email,
                        password: password,
                        busy: busy,
                        obscure: obscure,
                        remember: remember,
                        error: error,
                        onTogglePassword: () => setState(() => obscure = !obscure),
                        onRemember: (v) => setState(() => remember = v ?? true),
                        onSubmit: submit,
                        onForgot: () => info('Password recovery will be connected in the security phase.'),
                        onSso: () => info('SSO is not configured for this environment yet.'),
                      )
                    : _DesktopLoginComposition(
                        email: email,
                        password: password,
                        busy: busy,
                        obscure: obscure,
                        remember: remember,
                        error: error,
                        onTogglePassword: () => setState(() => obscure = !obscure),
                        onRemember: (v) => setState(() => remember = v ?? true),
                        onSubmit: submit,
                        onForgot: () => info('Password recovery will be connected in the security phase.'),
                        onSso: () => info('SSO is not configured for this environment yet.'),
                      ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _DesktopLoginComposition extends StatelessWidget {
  const _DesktopLoginComposition({
    required this.email,
    required this.password,
    required this.busy,
    required this.obscure,
    required this.remember,
    required this.error,
    required this.onTogglePassword,
    required this.onRemember,
    required this.onSubmit,
    required this.onForgot,
    required this.onSso,
  });
  final TextEditingController email, password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot, onSso;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    final aspect = size.height == 0 ? 1.5 : size.width / size.height;
    final macBookLike = size.width >= 1280 &&
        size.width <= 1800 &&
        aspect >= 1.45 &&
        aspect <= 1.70;
    final leftInset = macBookLike ? 94.0 : 82.0;
    final rightInset = macBookLike ? 58.0 : 50.0;
    final logoWidth = macBookLike ? 238.0 : 226.0;
    final headlineSize = macBookLike ? 74.0 : 68.0;
    final cardWidth = macBookLike ? 590.0 : 560.0;

    return Padding(
      padding: EdgeInsets.fromLTRB(leftInset, 38, rightInset, 42),
      child: Row(
        children: [
          Expanded(
            flex: 50,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                HimateLogo(onDark: true, width: logoWidth),
                const Spacer(),
                Text(
                  'Culture\nConnects\nPeople',
                  style: GoogleFonts.cormorantGaramond(
                    color: brandWhite,
                    fontSize: headlineSize,
                    height: .88,
                    fontWeight: FontWeight.w500,
                    letterSpacing: -.8,
                  ),
                ),
                const SizedBox(height: 24),
                _LetterspacedLabel('BUILDING A BRIGHTER\nCULTURAL TOMORROW', color: const Color(0xFFE8EDF3), fontSize: macBookLike ? 13.2 : 12.4),
                const SizedBox(height: 38),
                const _HeroValue(icon: Icons.groups_2_outlined, label: 'STRONGER COMMUNITIES'),
                const SizedBox(height: 15),
                const _HeroValue(icon: Icons.bar_chart_rounded, label: 'MORE OPPORTUNITIES'),
                const SizedBox(height: 15),
                const _HeroValue(icon: Icons.shield_outlined, label: 'GREATER IMPACT'),
                const Spacer(),
                Row(children: [
                  const SizedBox(width: 38, child: Divider(color: brandGold, thickness: 1.5)),
                  const SizedBox(width: 12),
                  _LetterspacedLabel(
                    'HERITAGE MEETS INNOVATION',
                    color: const Color(0xFFE8EDF3),
                    fontSize: macBookLike ? 12.6 : 11.4,
                  ),
                ]),
              ],
            ),
          ),
          Expanded(
            flex: 50,
            child: Align(
              alignment: Alignment.center,
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: cardWidth),
                child: _LoginCard(
                  email: email,
                  password: password,
                  busy: busy,
                  obscure: obscure,
                  remember: remember,
                  error: error,
                  onTogglePassword: onTogglePassword,
                  onRemember: onRemember,
                  onSubmit: onSubmit,
                  onForgot: onForgot,
                  onSso: onSso,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _CompactLoginComposition extends StatelessWidget {
  const _CompactLoginComposition({
    required this.email,
    required this.password,
    required this.busy,
    required this.obscure,
    required this.remember,
    required this.error,
    required this.onTogglePassword,
    required this.onRemember,
    required this.onSubmit,
    required this.onForgot,
    required this.onSso,
  });
  final TextEditingController email, password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot, onSso;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    final narrow = MediaQuery.of(context).size.width < 520;
    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(narrow ? 18 : 30, 24, narrow ? 18 : 30, 34),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const HimateLogo(onDark: true, width: 188),
          SizedBox(height: narrow ? 58 : 90),
          Text(
            'Culture Connects People',
            style: GoogleFonts.cormorantGaramond(
              color: brandWhite,
              fontSize: narrow ? 38 : 47,
              height: .95,
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 10),
          const _LetterspacedLabel('BUILDING A BRIGHTER CULTURAL TOMORROW', color: Color(0xFFE8EDF3), fontSize: 8.5),
          SizedBox(height: narrow ? 36 : 48),
          Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 440),
              child: _LoginCard(
                email: email,
                password: password,
                busy: busy,
                obscure: obscure,
                remember: remember,
                error: error,
                onTogglePassword: onTogglePassword,
                onRemember: onRemember,
                onSubmit: onSubmit,
                onForgot: onForgot,
                onSso: onSso,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _LoginArtwork extends StatelessWidget {
  const _LoginArtwork();

  @override
  Widget build(BuildContext context) {
    final compact = MediaQuery.of(context).size.width < 820;
    final source = Uri.base.resolve(
      compact ? '/art/login_mobile_r5.png' : '/art/login_desktop_r5.png',
    ).toString();

    return Image.network(
      source,
      fit: BoxFit.cover,
      alignment: compact ? Alignment.center : const Alignment(.08, 0),
      filterQuality: FilterQuality.high,
      gaplessPlayback: true,
      errorBuilder: (_, __, ___) => const ColoredBox(color: brandNavyDeep),
    );
  }
}

class _LoginCard extends StatelessWidget {
  const _LoginCard({
    required this.email,
    required this.password,
    required this.busy,
    required this.obscure,
    required this.remember,
    required this.error,
    required this.onTogglePassword,
    required this.onRemember,
    required this.onSubmit,
    required this.onForgot,
    required this.onSso,
  });

  final TextEditingController email, password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot, onSso;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(42, 44, 42, 36),
      decoration: BoxDecoration(
        color: brandWhite.withOpacity(.975),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFFD8DFE7)),
        boxShadow: [BoxShadow(color: brandNavy.withOpacity(.22), blurRadius: 54, offset: const Offset(0, 22))],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Welcome back',
            textAlign: TextAlign.center,
            style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: 39, fontWeight: FontWeight.w700, height: 1),
          ),
          const SizedBox(height: 8),
          Text('Sign in to your HIMATE System account', textAlign: TextAlign.center, style: GoogleFonts.inter(color: brandSteel, fontSize: 15.5)),
          const SizedBox(height: 28),
          TextField(
            controller: email,
            keyboardType: TextInputType.emailAddress,
            autofillHints: const [AutofillHints.email],
            style: GoogleFonts.inter(color: brandCharcoal, fontSize: 16),
            decoration: const InputDecoration(hintText: 'Email address', prefixIcon: Icon(Icons.mail_outline_rounded, size: 22)),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: password,
            obscureText: obscure,
            autofillHints: const [AutofillHints.password],
            onSubmitted: (_) => onSubmit(),
            style: GoogleFonts.inter(color: brandCharcoal, fontSize: 16),
            decoration: InputDecoration(
              hintText: 'Password',
              prefixIcon: const Icon(Icons.lock_outline_rounded, size: 22),
              suffixIcon: IconButton(
                onPressed: onTogglePassword,
                tooltip: obscure ? 'Show password' : 'Hide password',
                icon: Icon(obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined, size: 22),
              ),
            ),
          ),
          const SizedBox(height: 7),
          Row(
            children: [
              SizedBox(
                height: 34,
                child: Row(children: [
                  Checkbox(value: remember, onChanged: onRemember, visualDensity: VisualDensity.compact),
                  Text('Remember me', style: GoogleFonts.inter(color: brandNavy, fontSize: 14.2)),
                ]),
              ),
              const Spacer(),
              TextButton(
                onPressed: onForgot,
                style: TextButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 5)),
                child: Text('Forgot password?', style: GoogleFonts.inter(fontSize: 14.2, fontWeight: FontWeight.w600)),
              ),
            ],
          ),
          if (error != null) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(11),
              decoration: BoxDecoration(color: brandDanger.withOpacity(.06), borderRadius: BorderRadius.circular(7), border: Border.all(color: brandDanger.withOpacity(.18))),
              child: Row(children: [
                const Icon(Icons.error_outline_rounded, color: brandDanger, size: 18),
                const SizedBox(width: 8),
                Expanded(child: Text(error!, style: GoogleFonts.inter(color: brandDanger, fontSize: 11.5))),
              ]),
            ),
          ],
          const SizedBox(height: 15),
          FilledButton(
            onPressed: busy ? null : onSubmit,
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(60)),
            child: AnimatedSwitcher(
              duration: const Duration(milliseconds: 180),
              child: busy
                  ? const SizedBox(key: ValueKey('busy'), width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2.1, color: brandWhite))
                  : Row(key: const ValueKey('ready'), mainAxisAlignment: MainAxisAlignment.center, children: [
                      Text('Sign in', style: GoogleFonts.inter(fontWeight: FontWeight.w700)),
                      const SizedBox(width: 16),
                      const Icon(Icons.arrow_forward_rounded, size: 21),
                    ]),
            ),
          ),
          const SizedBox(height: 18),
          Row(children: [
            const Expanded(child: Divider(color: brandMist)),
            Padding(padding: const EdgeInsets.symmetric(horizontal: 12), child: Text('or continue with', style: GoogleFonts.inter(color: brandTextSoft, fontSize: 13.0))),
            const Expanded(child: Divider(color: brandMist)),
          ]),
          const SizedBox(height: 18),
          OutlinedButton.icon(
            onPressed: onSso,
            style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(56)),
            icon: const Icon(Icons.account_balance_outlined, size: 21),
            label: Text('Sign in with SSO', style: GoogleFonts.inter(fontWeight: FontWeight.w700)),
          ),
          const SizedBox(height: 24),
          Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            const Icon(Icons.verified_user_outlined, color: brandGold, size: 17),
            const SizedBox(width: 7),
            Text('Secure  •  Trusted  •  Built for a brighter tomorrow', style: GoogleFonts.inter(color: brandTextSoft, fontSize: 11.8)),
          ]),
        ],
      ),
    );
  }
}

class HimateLogo extends StatelessWidget {
  const HimateLogo({this.onDark = false, this.compact = false, this.width = 205, super.key});
  final bool onDark;
  final bool compact;
  final double width;

  @override
  Widget build(BuildContext context) {
    final asset = compact ? 'assets/himate_mark.webp' : 'assets/himate_logo_master.webp';
    final ratio = compact ? (120 / 129) : (180 / 118);
    return SizedBox(
      width: width,
      height: width / ratio,
      child: Image.asset(
        asset,
        fit: BoxFit.contain,
        filterQuality: FilterQuality.high,
        color: compact ? const Color(0xFFE0B568) : null,
        colorBlendMode: compact ? BlendMode.modulate : null,
        semanticLabel: compact ? 'HIMATE' : 'HIMATE System',
      ),
    );
  }
}

class BrandMark extends StatelessWidget {
  const BrandMark({this.size = 34, super.key});
  final double size;

  @override
  Widget build(BuildContext context) => SizedBox(
    width: size,
    height: size,
    child: Image.asset(
      'assets/himate_mark.webp',
      fit: BoxFit.contain,
      filterQuality: FilterQuality.high,
      color: const Color(0xFFE0B568),
      colorBlendMode: BlendMode.modulate,
      semanticLabel: 'HIMATE',
    ),
  );
}

class _HeroValue extends StatelessWidget {
  const _HeroValue({required this.icon, required this.label});
  final IconData icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Row(children: [
      Container(
        width: 38,
        height: 38,
        decoration: BoxDecoration(border: Border.all(color: brandGold.withOpacity(.95)), shape: BoxShape.circle, color: brandNavy.withOpacity(.42)),
        child: Icon(icon, color: brandGold, size: 18),
      ),
      const SizedBox(width: 13),
      _LetterspacedLabel(label, color: const Color(0xFFE8EDF3), fontSize: 11.5),
    ]);
  }
}

class _LetterspacedLabel extends StatelessWidget {
  const _LetterspacedLabel(this.text, {required this.color, this.fontSize = 10});
  final String text;
  final Color color;
  final double fontSize;

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: GoogleFonts.inter(color: color, fontSize: fontSize, fontWeight: FontWeight.w700, letterSpacing: 2.4, height: 1.55),
    );
  }
}

class NavSpec {
  const NavSpec(this.label, this.icon, this.subtitle);
  final String label;
  final IconData icon;
  final String subtitle;
}

enum ShellLayoutMode { mobile, tablet, desktop }

ShellLayoutMode shellLayoutForWidth(double width) {
  if (width < 720) return ShellLayoutMode.mobile;
  if (width < 980) return ShellLayoutMode.tablet;
  return ShellLayoutMode.desktop;
}

class Shell extends StatefulWidget {
  const Shell({required this.api, required this.user, required this.onLogout, super.key});
  final Api api;
  final Map<String, dynamic> user;
  final Future<void> Function() onLogout;

  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  int selected = 0;
  bool collapsed = false;

  static const nav = <NavSpec>[
    NavSpec('Dashboard', Icons.dashboard_outlined, 'Platform overview'),
    NavSpec('Partners', Icons.groups_2_outlined, 'Partner control'),
    NavSpec('Licensing & Finance', Icons.account_balance_wallet_outlined, 'Commercial management'),
    NavSpec('Impact & Reports', Icons.show_chart_rounded, 'Metrics and reporting'),
    NavSpec('Website & Marketing', Icons.campaign_outlined, 'Brand and growth'),
    NavSpec('System & Operations', Icons.settings_suggest_outlined, 'Infrastructure health'),
    NavSpec('Administration', Icons.admin_panel_settings_outlined, 'Roles and control'),
  ];

  Widget page() {
    switch (selected) {
      case 0: return DashboardPage(api: widget.api);
      case 1: return PartnersPage(api: widget.api);
      case 2: return FinancePage(api: widget.api);
      case 3: return ImpactPage(api: widget.api);
      case 4: return const PlannedPage(title: 'Website & Marketing', subtitle: 'HIMATE CMS and public marketing tools are planned for START-16–17.', icon: Icons.campaign_outlined);
      case 5: return SystemPage(api: widget.api);
      default: return const PlannedPage(title: 'Administration', subtitle: 'Roles, permissions and advanced audit controls are planned for START-18–19.', icon: Icons.admin_panel_settings_outlined);
    }
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final layoutMode = shellLayoutForWidth(constraints.maxWidth);
        final mobile = layoutMode == ShellLayoutMode.mobile;
        final tablet = layoutMode == ShellLayoutMode.tablet;
        if (mobile) {
          return Scaffold(
            appBar: AppBar(
              toolbarHeight: 64,
              titleSpacing: 12,
              title: const HimateLogo(width: 170),
              actions: [
                _TopIconButton(icon: Icons.notifications_none_rounded, onTap: () {}),
                PopupMenuButton<String>(
                  tooltip: 'Account',
                  onSelected: (value) { if (value == 'logout') widget.onLogout(); },
                  itemBuilder: (_) => const [
                    PopupMenuItem(value: 'logout', child: Row(children: [Icon(Icons.logout_rounded, size: 18), SizedBox(width: 10), Text('Sign out')])),
                  ],
                  child: Padding(padding: const EdgeInsets.symmetric(horizontal: 14), child: _Avatar(name: '${widget.user['name'] ?? 'Admin User'}')),
                ),
              ],
            ),
            drawer: Drawer(
              backgroundColor: brandNavyDeep,
              child: SafeArea(
                child: _SidebarContent(
                  nav: nav,
                  selected: selected,
                  collapsed: false,
                  user: widget.user,
                  onSelect: (i) { setState(() => selected = i); Navigator.pop(context); },
                  onToggle: null,
                  onLogout: widget.onLogout,
                ),
              ),
            ),
            body: page(),
          );
        }

        return Scaffold(
          body: Row(
            children: [
              AnimatedContainer(
                duration: const Duration(milliseconds: 230),
                curve: Curves.easeOutCubic,
                width: tablet || collapsed ? 82 : 258,
                decoration: const BoxDecoration(
                  gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color(0xFF061426), brandNavy, Color(0xFF0A2C4C)]),
                ),
                child: SafeArea(
                  child: _SidebarContent(
                    nav: nav,
                    selected: selected,
                    collapsed: tablet || collapsed,
                    user: widget.user,
                    onSelect: (i) => setState(() => selected = i),
                    onToggle: tablet ? null : () => setState(() => collapsed = !collapsed),
                    onLogout: widget.onLogout,
                  ),
                ),
              ),
              Expanded(
                child: Column(
                  children: [
                    Container(
                      height: 74,
                      padding: const EdgeInsets.symmetric(horizontal: 28),
                      decoration: const BoxDecoration(color: brandWhite, border: Border(bottom: BorderSide(color: brandMist))),
                      child: Row(
                        children: [
                          if (!tablet)
                            Expanded(
                              child: Align(
                                alignment: Alignment.centerLeft,
                                child: ConstrainedBox(
                                  constraints: const BoxConstraints(maxWidth: 420),
                                  child: TextField(
                                    readOnly: true,
                                    onTap: () => ScaffoldMessenger.of(context).showSnackBar(
                                      const SnackBar(content: Text('Global search will be activated in a later functional cycle.'), behavior: SnackBarBehavior.floating),
                                    ),
                                    decoration: const InputDecoration(isDense: true, hintText: 'Search anywhere...', prefixIcon: Icon(Icons.search_rounded, size: 19)),
                                  ),
                                ),
                              ),
                            )
                          else
                            const Spacer(),
                          const SizedBox(width: 18),
                          _TopIconButton(icon: Icons.notifications_none_rounded, hasDot: true, onTap: () {}),
                          const SizedBox(width: 8),
                          PopupMenuButton<String>(
                            tooltip: 'Admin account',
                            onSelected: (value) { if (value == 'logout') widget.onLogout(); },
                            itemBuilder: (_) => const [
                              PopupMenuItem(value: 'logout', child: Row(children: [Icon(Icons.logout_rounded, size: 18), SizedBox(width: 10), Text('Sign out')])),
                            ],
                            child: tablet
                                ? Padding(
                                    padding: const EdgeInsets.symmetric(horizontal: 4),
                                    child: _Avatar(name: '${widget.user['name'] ?? 'Admin User'}'),
                                  )
                                : Row(
                                    children: [
                                      _Avatar(name: '${widget.user['name'] ?? 'Admin User'}'),
                                      const SizedBox(width: 9),
                                      ConstrainedBox(
                                        constraints: const BoxConstraints(maxWidth: 130),
                                        child: Text(
                                          '${widget.user['name'] ?? 'Admin User'}',
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                          style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 12),
                                        ),
                                      ),
                                      const SizedBox(width: 4),
                                      const Icon(Icons.keyboard_arrow_down_rounded, color: brandTextSoft, size: 19),
                                    ],
                                  ),
                          ),
                        ],
                      ),
                    ),
                    Expanded(child: ColoredBox(color: brandIvory, child: page())),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _SidebarContent extends StatelessWidget {
  const _SidebarContent({
    required this.nav,
    required this.selected,
    required this.collapsed,
    required this.user,
    required this.onSelect,
    required this.onToggle,
    required this.onLogout,
  });

  final List<NavSpec> nav;
  final int selected;
  final bool collapsed;
  final Map<String, dynamic> user;
  final ValueChanged<int> onSelect;
  final VoidCallback? onToggle;
  final Future<void> Function() onLogout;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SizedBox(
          height: 126,
          child: Stack(
            alignment: Alignment.center,
            children: [
              Padding(
                padding: EdgeInsets.symmetric(horizontal: collapsed ? 16 : 18),
                child: HimateLogo(onDark: true, compact: collapsed, width: collapsed ? 38 : 190),
              ),
              if (onToggle != null)
                Positioned(
                  right: collapsed ? 18 : 12,
                  bottom: 4,
                  child: Tooltip(
                    message: collapsed ? 'Expand navigation' : 'Collapse navigation',
                    child: Material(
                      color: Colors.transparent,
                      child: InkWell(
                        onTap: onToggle,
                        borderRadius: BorderRadius.circular(99),
                        child: Container(
                          width: 34,
                          height: 34,
                          decoration: BoxDecoration(color: const Color(0xFF17334F), shape: BoxShape.circle, border: Border.all(color: Colors.white.withOpacity(.08))),
                          child: Icon(collapsed ? Icons.keyboard_double_arrow_right_rounded : Icons.keyboard_double_arrow_left_rounded, color: brandWhite, size: 19),
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
        Padding(padding: EdgeInsets.symmetric(horizontal: collapsed ? 13 : 16), child: Divider(color: Colors.white.withOpacity(.09), height: 1)),
        const SizedBox(height: 14),
        Expanded(
          child: ListView.separated(
            padding: EdgeInsets.symmetric(horizontal: collapsed ? 10 : 12, vertical: 6),
            itemCount: nav.length,
            separatorBuilder: (_, __) => const SizedBox(height: 6),
            itemBuilder: (context, i) => _NavItem(spec: nav[i], selected: i == selected, collapsed: collapsed, onTap: () => onSelect(i)),
          ),
        ),
        Padding(
          padding: EdgeInsets.all(collapsed ? 10 : 12),
          child: collapsed
              ? Tooltip(message: 'Sign out', child: _SidebarIconButton(icon: Icons.logout_rounded, onTap: onLogout))
              : Container(
                  padding: const EdgeInsets.all(11),
                  decoration: BoxDecoration(color: Colors.white.withOpacity(.055), borderRadius: BorderRadius.circular(12), border: Border.all(color: Colors.white.withOpacity(.08))),
                  child: Row(
                    children: [
                      _Avatar(name: '${user['name'] ?? 'Admin User'}', dark: true),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('${user['name'] ?? 'Admin User'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: GoogleFonts.inter(color: brandWhite, fontWeight: FontWeight.w700, fontSize: 11.5)),
                            const SizedBox(height: 2),
                            Text('${user['email'] ?? 'System Administrator'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: GoogleFonts.inter(color: const Color(0xFF91A4B8), fontSize: 9.5)),
                          ],
                        ),
                      ),
                      _SidebarIconButton(icon: Icons.logout_rounded, onTap: onLogout, size: 34),
                    ],
                  ),
                ),
        ),
      ],
    );
  }
}

class _NavItem extends StatefulWidget {
  const _NavItem({required this.spec, required this.selected, required this.collapsed, required this.onTap});
  final NavSpec spec;
  final bool selected, collapsed;
  final VoidCallback onTap;

  @override
  State<_NavItem> createState() => _NavItemState();
}

class _NavItemState extends State<_NavItem> {
  bool hover = false;

  @override
  Widget build(BuildContext context) {
    final active = widget.selected;
    return Tooltip(
      message: widget.collapsed ? widget.spec.label : '',
      waitDuration: const Duration(milliseconds: 450),
      child: MouseRegion(
        onEnter: (_) => setState(() => hover = true),
        onExit: (_) => setState(() => hover = false),
        child: AnimatedScale(
          scale: hover && !active ? 1.012 : 1,
          duration: const Duration(milliseconds: 150),
          curve: Curves.easeOut,
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: widget.onTap,
              borderRadius: BorderRadius.circular(10),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 170),
                curve: Curves.easeOut,
                constraints: const BoxConstraints(minHeight: 52),
                padding: EdgeInsets.symmetric(horizontal: widget.collapsed ? 0 : 12, vertical: 9),
                decoration: BoxDecoration(
                  color: active ? brandGold.withOpacity(.09) : hover ? Colors.white.withOpacity(.055) : Colors.transparent,
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(color: active ? brandGold.withOpacity(.70) : Colors.transparent),
                ),
                child: widget.collapsed
                    ? Center(child: Icon(widget.spec.icon, color: active ? brandGold : const Color(0xFFA8B7C7), size: 21))
                    : Row(
                        children: [
                          Icon(widget.spec.icon, color: active ? brandGold : const Color(0xFFA8B7C7), size: 20),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              widget.spec.label,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(color: active ? const Color(0xFFF2D79F) : const Color(0xFFD9E2EC), fontWeight: active ? FontWeight.w700 : FontWeight.w500, fontSize: 12.5),
                            ),
                          ),
                          if (active) Container(width: 3, height: 18, decoration: BoxDecoration(color: brandGold, borderRadius: BorderRadius.circular(99))),
                        ],
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _SidebarIconButton extends StatelessWidget {
  const _SidebarIconButton({required this.icon, required this.onTap, this.size = 40});
  final IconData icon;
  final VoidCallback onTap;
  final double size;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(onTap: onTap, borderRadius: BorderRadius.circular(10), child: SizedBox(width: size, height: size, child: Icon(icon, color: const Color(0xFFA8B7C7), size: 18))),
    );
  }
}

class _TopIconButton extends StatelessWidget {
  const _TopIconButton({required this.icon, required this.onTap, this.hasDot = false});
  final IconData icon;
  final VoidCallback onTap;
  final bool hasDot;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        IconButton(onPressed: onTap, icon: Icon(icon, size: 21)),
        if (hasDot)
          const Positioned(
            right: 8,
            top: 7,
            child: DecoratedBox(decoration: BoxDecoration(color: brandGold, shape: BoxShape.circle), child: SizedBox(width: 6, height: 6)),
          ),
      ],
    );
  }
}

class _Avatar extends StatelessWidget {
  const _Avatar({required this.name, this.dark = false});
  final String name;
  final bool dark;

  @override
  Widget build(BuildContext context) {
    final parts = name.trim().split(RegExp(r'\s+')).where((e) => e.isNotEmpty).toList();
    final initials = parts.isEmpty ? 'AU' : parts.take(2).map((e) => e[0].toUpperCase()).join();
    return Container(
      width: 34,
      height: 34,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: dark ? brandSteel : const Color(0xFF165A9B),
        shape: BoxShape.circle,
        border: Border.all(color: dark ? Colors.white.withOpacity(.14) : Colors.transparent),
      ),
      child: Text(initials, style: const TextStyle(color: brandWhite, fontWeight: FontWeight.w700, fontSize: 10)),
    );
  }
}

class PlannedPage extends StatelessWidget {
  const PlannedPage({required this.title, required this.subtitle, required this.icon, super.key});
  final String title, subtitle;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Content(
      title: title,
      subtitle: subtitle,
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(28),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(width: 50, height: 50, decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(12)), child: Icon(icon, color: brandGold, size: 24)),
              const SizedBox(width: 16),
              const Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Workspace prepared', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 17)),
                    SizedBox(height: 7),
                    Text('The brand system and responsive shell are ready. Functional implementation remains in its scheduled START cycle.', style: TextStyle(color: brandTextSoft, height: 1.5)),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class DashboardPage extends StatelessWidget {
  const DashboardPage({required this.api, super.key});
  final Api api;

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: api.get('/api/v1/dashboard/summary'),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) return const _BrandLoading();
        if (snapshot.hasError) {
          return Content(title: 'Welcome to HIMATE System', subtitle: 'Manage partners, programs and cultural impact — all in one place.', child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Dashboard data is temporarily unavailable', message: '${snapshot.error}'));
        }
        final d=snapshot.data??<String,dynamic>{};
        final p=Map<String,dynamic>.from(d['partners']??<String,dynamic>{});
        final m=Map<String,dynamic>.from(d['modules']??<String,dynamic>{});
        final hour=DateTime.now().hour;
        final greeting=hour<12?'Good morning,':hour<18?'Good afternoon,':'Good evening,';
        return Content(
          eyebrow:greeting,
          title:'Welcome to HIMATE System',
          subtitle:'Manage partners, programs, and cultural impact — all in one place.',
          child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
            LayoutBuilder(builder:(context,c){
              final gap=14.0;
              final cols=c.maxWidth<620?2:4;
              final w=(c.maxWidth-gap*(cols-1))/cols;
              return Wrap(spacing:gap,runSpacing:gap,children:[
                SizedBox(width:w,child:Kpi(label:'Active Partners',value:'${p['live']??0}',note:'${p['total']??0} partner records',icon:Icons.groups_2_outlined,accent:const Color(0xFF0B5DA8))),
                SizedBox(width:w,child:Kpi(label:'Active Programs',value:'${m['catalog_total']??0}',note:'Available program modules',icon:Icons.description_outlined,accent:brandNavy)),
                SizedBox(width:w,child:Kpi(label:'Revenue (YTD)',value:'—',note:'Billing analytics upcoming',icon:Icons.bar_chart_rounded,accent:brandGold)),
                SizedBox(width:w,child:Kpi(label:'People Reached',value:'—',note:'Impact data in START-13',icon:Icons.groups_rounded,accent:brandNavy)),
              ]);
            }),
            const SizedBox(height:18),
            LayoutBuilder(builder:(context,c){
              if(c.maxWidth<900)return const Column(children:[_ImpactPanel(),SizedBox(height:16),_ActivityPanel()]);
              return const Row(crossAxisAlignment:CrossAxisAlignment.start,children:[
                Expanded(flex:7,child:_ImpactPanel()),SizedBox(width:16),Expanded(flex:4,child:_ActivityPanel())
              ]);
            }),
          ]),
        );
      },
    );
  }
}

class _ImpactPanel extends StatelessWidget {
  const _ImpactPanel();
  @override
  Widget build(BuildContext context)=>SizedBox(
    height:330,
    child:Card(child:Padding(
      padding:const EdgeInsets.fromLTRB(22,20,22,16),
      child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
        Row(children:[
          Expanded(child:Text('Program Impact',style:GoogleFonts.cormorantGaramond(color:brandNavy,fontWeight:FontWeight.w700,fontSize:20))),
          Container(padding:const EdgeInsets.symmetric(horizontal:12,vertical:8),decoration:BoxDecoration(border:Border.all(color:brandMist),borderRadius:BorderRadius.circular(7)),child:Row(children:[Text('This Year',style:GoogleFonts.inter(color:brandNavy,fontSize:10.5,fontWeight:FontWeight.w600)),const SizedBox(width:5),const Icon(Icons.keyboard_arrow_down_rounded,size:16,color:brandNavy)]))
        ]),
        const SizedBox(height:12),
        const Expanded(child:_ImpactChart()),
      ]),
    )),
  );
}

class _ImpactChart extends StatelessWidget {
  const _ImpactChart();
  @override
  Widget build(BuildContext context)=>CustomPaint(painter:_ImpactChartPainter(),child:const SizedBox.expand());
}

class _ImpactChartPainter extends CustomPainter {
  @override
  void paint(Canvas canvas,Size size){
    const left=42.0,bottom=27.0,top=8.0;
    final chart=Rect.fromLTWH(left,top,size.width-left-5,size.height-top-bottom);
    final grid=Paint()..color=brandMist.withOpacity(.82)..strokeWidth=.8;
    for(var i=0;i<=4;i++){final y=chart.top+chart.height*i/4;canvas.drawLine(Offset(chart.left,y),Offset(chart.right,y),grid);}
    for(var i=0;i<12;i++){final x=chart.left+chart.width*i/11;canvas.drawLine(Offset(x,chart.top),Offset(x,chart.bottom),grid);}
    final vals=<double>[.12,.26,.20,.37,.49,.39,.53,.48,.61,.70,.68,.84];
    final line=Path(); final area=Path();
    for(var i=0;i<vals.length;i++){
      final x=chart.left+chart.width*i/(vals.length-1),y=chart.bottom-chart.height*vals[i];
      if(i==0){line.moveTo(x,y);area.moveTo(x,chart.bottom);area.lineTo(x,y);}else{line.lineTo(x,y);area.lineTo(x,y);}
    }
    area.lineTo(chart.right,chart.bottom);area.close();
    canvas.drawPath(area,Paint()..color=const Color(0xFF2E5B87).withOpacity(.11));
    canvas.drawPath(line,Paint()..color=brandNavy..strokeWidth=2.2..style=PaintingStyle.stroke..strokeCap=StrokeCap.round..strokeJoin=StrokeJoin.round);
    final dot=Paint()..color=brandNavy;
    for(var i=0;i<vals.length;i++)canvas.drawCircle(Offset(chart.left+chart.width*i/(vals.length-1),chart.bottom-chart.height*vals[i]),2.7,dot);
    final months=['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
    for(var i=0;i<12;i++){
      final tp=TextPainter(text:TextSpan(text:months[i],style:GoogleFonts.inter(fontSize:8.5,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      tp.paint(canvas,Offset(chart.left+chart.width*i/11-tp.width/2,chart.bottom+7));
    }
    for(var i=0;i<=4;i++){
      final label='${(100-25*i)}K';
      final tp=TextPainter(text:TextSpan(text:label,style:GoogleFonts.inter(fontSize:8,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      tp.paint(canvas,Offset(chart.left-tp.width-8,chart.top+chart.height*i/4-tp.height/2));
    }
  }
  @override bool shouldRepaint(covariant CustomPainter oldDelegate)=>false;
}

class _ActivityPanel extends StatelessWidget {
  const _ActivityPanel();
  @override
  Widget build(BuildContext context)=>SizedBox(
    height:330,
    child:Card(child:Padding(padding:const EdgeInsets.fromLTRB(20,20,20,16),child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
      Row(children:[Expanded(child:Text('Recent Activity',style:GoogleFonts.cormorantGaramond(color:brandNavy,fontWeight:FontWeight.w700,fontSize:20))),Text('View all',style:GoogleFonts.inter(color:brandSteel,fontSize:10.5,fontWeight:FontWeight.w600))]),
      const SizedBox(height:10),
      const _ActivityRow(icon:Icons.person_add_alt_1_outlined,title:'New partner registered',subtitle:'Partner activity',tone:Color(0xFF1D6FC2)),
      const Divider(height:12),
      const _ActivityRow(icon:Icons.description_outlined,title:'Program updated',subtitle:'Module catalog activity',tone:brandGold),
      const Divider(height:12),
      const _ActivityRow(icon:Icons.payments_outlined,title:'Payment received',subtitle:'Billing activity',tone:brandSuccess),
      const Divider(height:12),
      const _ActivityRow(icon:Icons.person_outline_rounded,title:'New user added',subtitle:'Administration activity',tone:brandNavy),
    ]))),
  );
}

class _ActivityRow extends StatelessWidget {
  const _ActivityRow({required this.icon,required this.title,required this.subtitle,required this.tone});
  final IconData icon; final String title,subtitle; final Color tone;
  @override
  Widget build(BuildContext context)=>Padding(padding:const EdgeInsets.symmetric(vertical:6),child:Row(children:[
    Container(width:38,height:38,decoration:BoxDecoration(color:tone.withOpacity(.10),shape:BoxShape.circle),child:Icon(icon,color:tone,size:18)),
    const SizedBox(width:11),
    Expanded(child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
      Text(title,style:GoogleFonts.inter(color:brandNavy,fontWeight:FontWeight.w600,fontSize:11.5)),
      const SizedBox(height:2),
      Text(subtitle,maxLines:1,overflow:TextOverflow.ellipsis,style:GoogleFonts.inter(color:brandTextSoft,fontSize:9.4)),
    ]))
  ]));
}

class PartnersPage extends StatefulWidget {
  const PartnersPage({required this.api, super.key});
  final Api api;

  @override
  State<PartnersPage> createState() => _PartnersPageState();
}

class _PartnersPageState extends State<PartnersPage> {
  List<Map<String, dynamic>> partners = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> categories = <Map<String, dynamic>>[];
  bool loading = true;
  String? error;
  String query = '';
  String categoryFilter = 'ALL';
  String lifecycleFilter = 'ALL';
  String healthFilter = 'ALL';
  static const int pageSize = 24;
  int offset = 0;
  int total = 0;
  int referenceCount = 0;
  Map<String, int> lifecycleCounts = <String, int>{};
  Timer? _searchDebounce;

  static const lifecycleOptions = [
    'PROSPECT',
    'LICENSE_PENDING',
    'READY_TO_PROVISION',
    'PROVISIONING',
    'CONFIGURATION',
    'TESTING',
    'READY_FOR_LAUNCH',
    'LIVE',
    'SUSPENDED',
    'ARCHIVED',
  ];

  @override
  void initState() {
    super.initState();
    load(loadCategories: true);
  }

  @override
  void dispose() {
    _searchDebounce?.cancel();
    super.dispose();
  }

  Uri _partnerUri() {
    final params = <String, String>{
      'limit': '$pageSize',
      'offset': '$offset',
    };
    if (query.trim().isNotEmpty) params['q'] = query.trim();
    if (categoryFilter != 'ALL') params['category'] = categoryFilter;
    if (lifecycleFilter != 'ALL') params['lifecycle'] = lifecycleFilter;
    if (healthFilter != 'ALL') params['health'] = healthFilter;
    return Uri(path: '/api/v1/partners', queryParameters: params);
  }

  Future<void> load({bool reset = false, bool loadCategories = false}) async {
    if (reset) offset = 0;
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final futures = <Future<Map<String, dynamic>>>[
        widget.api.get(_partnerUri().toString(), force: true),
        if (loadCategories || categories.isEmpty) widget.api.get('/api/v1/partner-categories', force: true),
      ];
      final r = await Future.wait(futures);
      final page = r[0];
      partners = items(page);
      total = (page['total'] as num?)?.toInt() ?? partners.length;
      referenceCount = (page['reference_count'] as num?)?.toInt() ?? 0;
      final counts = page['lifecycle_counts'];
      lifecycleCounts = counts is Map
          ? <String, int>{
              for (final entry in counts.entries) '${entry.key}': (entry.value as num?)?.toInt() ?? 0,
            }
          : <String, int>{};
      if (r.length > 1) categories = items(r[1]);
    } catch (e) {
      error = e.toString();
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void updateSearch(String value) {
    query = value;
    _searchDebounce?.cancel();
    _searchDebounce = Timer(const Duration(milliseconds: 280), () {
      if (mounted) load(reset: true);
    });
  }

  void previousPage() {
    if (offset <= 0) return;
    offset = offset >= pageSize ? offset - pageSize : 0;
    load();
  }

  void nextPage() {
    if (offset + partners.length >= total) return;
    offset += pageSize;
    load();
  }

  void success(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
    );
  }

  Future<void> addCategory() async {
    final controller = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'Add partner category',
        subtitle: 'Create a category for partner organizations that do not fit the default structure.',
        icon: Icons.category_outlined,
        child: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(labelText: 'Category name', hintText: 'e.g. Cultural Foundation'),
        ),
        primaryLabel: 'Add category',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );
    if (ok == true && controller.text.trim().isNotEmpty) {
      await widget.api.post('/api/v1/partner-categories', {'name': controller.text.trim()});
      await load();
      if (mounted) success('Partner category created.');
    }
    controller.dispose();
  }

  Future<void> addPartner() async {
    if (categories.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Create a partner category first.'), behavior: SnackBarBehavior.floating),
      );
      return;
    }

    final displayName = TextEditingController();
    final legalName = TextEditingController();
    final contactName = TextEditingController();
    final contactEmail = TextEditingController();
    final primaryDomain = TextEditingController();
    final country = TextEditingController(text: 'United States');
    String category = '${categories.first['id']}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'New Partner',
          subtitle: 'Create the partner record now. Provisioning remains a separate controlled lifecycle step.',
          icon: Icons.add_business_outlined,
          width: 680,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: TextField(controller: displayName, decoration: const InputDecoration(labelText: 'Display name *')),
                second: TextField(controller: legalName, decoration: const InputDecoration(labelText: 'Legal name')),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: category,
                decoration: const InputDecoration(labelText: 'Partner category'),
                items: [
                  for (final c in categories)
                    DropdownMenuItem(value: '${c['id']}', child: Text('${c['name']}')),
                ],
                onChanged: (v) {
                  if (v != null) setLocal(() => category = v);
                },
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: contactName, decoration: const InputDecoration(labelText: 'Primary contact')),
                second: TextField(controller: contactEmail, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Contact email')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: country, decoration: const InputDecoration(labelText: 'Country')),
                second: TextField(controller: primaryDomain, decoration: const InputDecoration(labelText: 'Primary domain', hintText: 'example.org')),
              ),
            ],
          ),
          primaryLabel: 'Create partner',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true && displayName.text.trim().isNotEmpty) {
      final created = await widget.api.post('/api/v1/partners', {
        'display_name': displayName.text.trim(),
        'legal_name': legalName.text.trim().isEmpty ? displayName.text.trim() : legalName.text.trim(),
        'category_id': category,
        'lifecycle': 'PROSPECT',
        'contact_name': contactName.text.trim(),
        'contact_email': contactEmail.text.trim(),
        'country': country.text.trim(),
        'primary_domain': primaryDomain.text.trim(),
      });
      await load();
      if (mounted) {
        success('Partner created.');
        Navigator.push(
          context,
          MaterialPageRoute(
            settings: RouteSettings(name: "/app/partners/${created['id']}"),
            builder: (_) => PartnerWorkspace(api: widget.api, partner: created),
          ),
        );
      }
    }

    displayName.dispose();
    legalName.dispose();
    contactName.dispose();
    contactEmail.dispose();
    primaryDomain.dispose();
    country.dispose();
  }

  List<Map<String, dynamic>> get filtered => partners;

  @override
  Widget build(BuildContext context) {
    final live = lifecycleCounts['LIVE'] ?? 0;
    final prospects = lifecycleCounts['PROSPECT'] ?? 0;
    final reference = referenceCount;
    final allRecords = lifecycleCounts.values.fold<int>(0, (sum, value) => sum + value);

    return Content(
      eyebrow: 'PEOPLE  |  PROGRAMS  |  IMPACT',
      title: 'Partners',
      subtitle: 'A single premium workspace for every organization connected to the HIMATE ecosystem.',
      actions: [
        OutlinedButton.icon(onPressed: addCategory, icon: const Icon(Icons.category_outlined), label: const Text('Add category')),
        FilledButton.icon(onPressed: addPartner, icon: const Icon(Icons.add_business_outlined), label: const Text('New Partner')),
      ],
      child: loading
          ? const _BrandLoading()
          : error != null
              ? _MessageCard(icon: Icons.cloud_off_outlined, title: 'Partners could not be loaded', message: error!)
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [
                        Kpi(label: 'Partner records', value: '$allRecords', note: 'All lifecycle states', icon: Icons.apartment_outlined, accent: brandNavy),
                        Kpi(label: 'Live partners', value: '$live', note: 'Operational partner environments', icon: Icons.public_outlined, accent: brandSuccess),
                        Kpi(label: 'Prospects', value: '$prospects', note: 'Pre-license pipeline', icon: Icons.handshake_outlined, accent: brandSteel),
                        Kpi(label: 'Reference partners', value: '$reference', note: 'Reference implementation', icon: Icons.workspace_premium_outlined, accent: brandGold),
                      ],
                    ),
                    const SizedBox(height: 20),
                    _FilterSurface(
                      child: LayoutBuilder(
                        builder: (context, c) {
                          final compact = c.maxWidth < 860;
                          final search = TextField(
                            onChanged: updateSearch,
                            decoration: const InputDecoration(
                              hintText: 'Search partners...',
                              prefixIcon: Icon(Icons.search_rounded),
                            ),
                          );
                          final category = DropdownButtonFormField<String>(
                            value: categoryFilter,
                            decoration: const InputDecoration(labelText: 'Category'),
                            items: [
                              const DropdownMenuItem(value: 'ALL', child: Text('All categories')),
                              for (final c in categories) DropdownMenuItem(value: '${c['id']}', child: Text('${c['name']}')),
                            ],
                            onChanged: (v) {
                              setState(() => categoryFilter = v ?? 'ALL');
                              load(reset: true);
                            },
                          );
                          final lifecycle = DropdownButtonFormField<String>(
                            value: lifecycleFilter,
                            decoration: const InputDecoration(labelText: 'Lifecycle'),
                            items: [
                              const DropdownMenuItem(value: 'ALL', child: Text('All lifecycle states')),
                              for (final state in lifecycleOptions) DropdownMenuItem(value: state, child: Text(_humanize(state))),
                            ],
                            onChanged: (v) {
                              setState(() => lifecycleFilter = v ?? 'ALL');
                              load(reset: true);
                            },
                          );
                          final health = DropdownButtonFormField<String>(
                            value: healthFilter,
                            decoration: const InputDecoration(labelText: 'Health'),
                            items: const [
                              DropdownMenuItem(value: 'ALL', child: Text('All health states')),
                              DropdownMenuItem(value: 'HEALTHY', child: Text('Healthy')),
                              DropdownMenuItem(value: 'WARNING', child: Text('Warning')),
                              DropdownMenuItem(value: 'OFFLINE', child: Text('Offline')),
                              DropdownMenuItem(value: 'UNKNOWN', child: Text('Unknown')),
                            ],
                            onChanged: (v) {
                              setState(() => healthFilter = v ?? 'ALL');
                              load(reset: true);
                            },
                          );
                          if (compact) {
                            return Column(children: [
                              search,
                              const SizedBox(height: 10),
                              category,
                              const SizedBox(height: 10),
                              lifecycle,
                              const SizedBox(height: 10),
                              health,
                            ]);
                          }
                          return Row(children: [
                            Expanded(flex: 2, child: search),
                            const SizedBox(width: 10),
                            Expanded(child: category),
                            const SizedBox(width: 10),
                            Expanded(child: lifecycle),
                            const SizedBox(width: 10),
                            Expanded(child: health),
                          ]);
                        },
                      ),
                    ),
                    const SizedBox(height: 20),
                    Row(
                      children: [
                        Text('Partner portfolio', style: Theme.of(context).textTheme.titleLarge),
                        const SizedBox(width: 10),
                        _MiniCounter(label: '${partners.length} shown · $total matched'),
                      ],
                    ),
                    const SizedBox(height: 12),
                    LayoutBuilder(
                      builder: (context, c) {
                        final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1040 ? (c.maxWidth - 14) / 2 : (c.maxWidth - 28) / 3;
                        final cards = <Widget>[
                          for (final p in filtered)
                            SizedBox(
                              width: width,
                              child: PartnerCard(
                                partner: p,
                                onTap: () => Navigator.push(
                                  context,
                                  MaterialPageRoute(
                                    settings: RouteSettings(name: "/app/partners/${p['id']}"),
                                    builder: (_) => PartnerWorkspace(api: widget.api, partner: p),
                                  ),
                                ),
                              ),
                            ),
                          SizedBox(width: width, child: NewPartnerCard(onTap: addPartner)),
                        ];
                        return Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Wrap(spacing: 14, runSpacing: 14, children: cards),
                            if (total > pageSize) ...[
                              const SizedBox(height: 18),
                              Wrap(
                                spacing: 10,
                                runSpacing: 10,
                                crossAxisAlignment: WrapCrossAlignment.center,
                                children: [
                                  OutlinedButton.icon(
                                    onPressed: offset > 0 && !loading ? previousPage : null,
                                    icon: const Icon(Icons.chevron_left_rounded),
                                    label: const Text('Previous'),
                                  ),
                                  _MiniCounter(label: 'Page ${offset ~/ pageSize + 1} of ${(total + pageSize - 1) ~/ pageSize}'),
                                  OutlinedButton.icon(
                                    onPressed: offset + partners.length < total && !loading ? nextPage : null,
                                    icon: const Icon(Icons.chevron_right_rounded),
                                    label: const Text('Next'),
                                  ),
                                ],
                              ),
                            ],
                          ],
                        );
                      },
                    ),
                  ],
                ),
    );
  }
}

class PartnerWorkspace extends StatefulWidget {
  const PartnerWorkspace({required this.api, required this.partner, this.initialSection, super.key});
  final Api api;
  final Map<String, dynamic> partner;
  final String? initialSection;

  @override
  State<PartnerWorkspace> createState() => _PartnerWorkspaceState();
}

class _PartnerWorkspaceState extends State<PartnerWorkspace> {
  late Map<String, dynamic> partner;
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> documents = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> invoices = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> subscriptions = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> environments = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> provisioningJobs = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> impactSummary = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> connectorCredentials = <Map<String, dynamic>>[];
  Map<String, dynamic>? billing;
  Map<String, dynamic>? terms;
  Map<String, dynamic>? license;
  bool loading = true;
  String? error;
  String moduleQuery = '';
  String moduleState = 'ALL';
  final GlobalKey _overviewKey = GlobalKey();
  final GlobalKey _companyKey = GlobalKey();
  final GlobalKey _environmentKey = GlobalKey();
  final GlobalKey _pricingKey = GlobalKey();
  final GlobalKey _modulesKey = GlobalKey();
  final GlobalKey _financeKey = GlobalKey();
  final GlobalKey _statisticsKey = GlobalKey();
  final GlobalKey _integrationsKey = GlobalKey();
  bool _initialSectionHandled = false;

  static const workspaceCards = <_WorkspaceSpec>[
    _WorkspaceSpec('Overview', Icons.dashboard_customize_outlined, 'Partner health and commercial snapshot', true),
    _WorkspaceSpec('Company Data', Icons.apartment_outlined, 'Legal identity, contacts and lifecycle', true),
    _WorkspaceSpec('System & Environment', Icons.dns_outlined, 'Domains, staging, deployment and provisioning', true),
    _WorkspaceSpec('Modules', Icons.grid_view_outlined, 'Entitlements, visibility and pricing', true),
    _WorkspaceSpec('Pricing & Subscription', Icons.payments_outlined, 'Activation fee and recurring terms', true),
    _WorkspaceSpec('Finance & Documents', Icons.folder_copy_outlined, 'Invoices and commercial evidence', true),
    _WorkspaceSpec('Statistics', Icons.insights_outlined, 'Partner performance metrics and provenance', true),
    _WorkspaceSpec('Evidence', Icons.verified_outlined, 'Impact evidence library', false),
    _WorkspaceSpec('Branding & Website', Icons.palette_outlined, 'Partner-facing design and CMS', false),
    _WorkspaceSpec('Users & Contacts', Icons.group_outlined, 'Partner administrators and contacts', false),
    _WorkspaceSpec('Integrations', Icons.hub_outlined, 'Secure connector identities and credentials', true),
    _WorkspaceSpec('Audit History', Icons.history_rounded, 'Immutable administrative history', false),
  ];

  @override
  void initState() {
    super.initState();
    partner = Map<String, dynamic>.from(widget.partner);
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    final id = '${partner['id']}';
    try {
      final r = await Future.wait([
        widget.api.get('/api/v1/partners/$id'),
        widget.api.get('/api/v1/partners/$id/modules'),
        widget.api.get('/api/v1/billing/partners/$id/summary'),
        widget.api.get('/api/v1/billing/partners/$id/terms'),
        widget.api.get('/api/v1/billing/partners/$id/license'),
        widget.api.get('/api/v1/billing/partners/$id/documents'),
        widget.api.get('/api/v1/billing/partners/$id/invoices'),
        widget.api.get('/api/v1/billing/partners/$id/subscriptions', force: true),
        widget.api.get('/api/v1/environments?partner_id=$id', force: true),
        widget.api.get('/api/v1/provisioning/jobs?partner_id=$id', force: true),
        widget.api.get('/api/v1/impact/summary?partner_id=$id', force: true),
        widget.api.get('/api/v1/connectors/$id/credential', force: true),
      ]);
      partner = r[0];
      modules = items(r[1]);
      billing = r[2];
      terms = r[3];
      license = r[4];
      documents = items(r[5]);
      invoices = items(r[6]);
      subscriptions = items(r[7]);
      environments = items(r[8]);
      provisioningJobs = items(r[9]);
      impactSummary = items(r[10]);
      connectorCredentials = items(r[11]);
      if (subscriptions.isEmpty && modules.any((m) => m['status'] == 'ACTIVE')) {
        subscriptions = items(await widget.api.get('/api/v1/billing/partners/$id/subscriptions', force: true));
      }
    } catch (e) {
      error = e.toString();
    } finally {
      if (mounted) {
        setState(() => loading = false);
        _scrollToInitialSection();
      }
    }
  }

  void _scrollToInitialSection() {
    if (_initialSectionHandled || widget.initialSection == null) return;
    _initialSectionHandled = true;
    final slug = widget.initialSection!;
    final key = switch (slug) {
      'overview' => _overviewKey,
      'company-data' => _companyKey,
      'system-and-environment' => _environmentKey,
      'pricing-and-subscription' => _pricingKey,
      'modules' => _modulesKey,
      'finance-and-documents' => _financeKey,
      'statistics' => _statisticsKey,
      'integrations' => _integrationsKey,
      _ => _overviewKey,
    };
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final target = key.currentContext;
      if (target != null) {
        Scrollable.ensureVisible(target, duration: const Duration(milliseconds: 280), curve: Curves.easeOutCubic, alignment: .04);
      }
    });
  }

  void success(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
    );
  }

  Map<String, dynamic>? get provisioningJob =>
      provisioningJobs.isEmpty ? null : provisioningJobs.first;

  Future<void> startProvisioning() async {
    final lifecycle = '${partner['lifecycle'] ?? ''}';
    if (!const {'READY_TO_PROVISION', 'PROVISIONING', 'CONFIGURATION'}.contains(lifecycle)) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Move the partner to READY TO PROVISION before starting provisioning.'),
          behavior: SnackBarBehavior.floating,
          backgroundColor: brandWarning,
        ),
      );
      return;
    }
    final preset = [
      for (final m in modules)
        if (m['status'] == 'ACTIVE') '${m['key']}',
    ];
    try {
      await widget.api.post('/api/v1/provisioning/jobs', {
        'partner_id': '${partner['id']}',
        'system_name': '${partner['brand_name'] ?? partner['display_name'] ?? partner['id']}',
        'admin_email': '${partner['contact_email'] ?? ''}',
        'platform_version': '${partner['platform_version'] ?? ''}',
        'desired_release': '${partner['platform_version'] ?? ''}',
        'module_preset': preset,
      });
      await load();
      if (mounted) success('Provisioning completed or resumed successfully.');
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Provisioning could not complete: $e'),
          behavior: SnackBarBehavior.floating,
          backgroundColor: brandDanger,
        ),
      );
    }
  }

  Future<void> rotateConnectorCredential() async {
    String environment = environments.any((e) => e['kind'] == 'PRODUCTION') ? 'PRODUCTION' : 'STAGING';
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Connector credential',
          subtitle: 'Generate or rotate a partner-scoped credential. The raw secret is displayed exactly once.',
          icon: Icons.key_outlined,
          width: 620,
          child: DropdownButtonFormField<String>(
            value: environment,
            decoration: const InputDecoration(labelText: 'Environment'),
            items: const [
              DropdownMenuItem(value: 'STAGING', child: Text('STAGING')),
              DropdownMenuItem(value: 'PRODUCTION', child: Text('PRODUCTION')),
            ],
            onChanged: (v) { if (v != null) setLocal(() => environment = v); },
          ),
          primaryLabel: 'Generate credential',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok != true) return;
    try {
      final result = await widget.api.post('/api/v1/connectors/${partner['id']}/credential', {'environment': environment});
      final token = '${result['token'] ?? ''}';
      await load();
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (context) => AlertDialog(
          title: const Text('Store this credential now'),
          content: SizedBox(
            width: 560,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('For security, HIMATE stores only the token hash. This raw credential will not be shown again.'),
                const SizedBox(height: 14),
                SelectableText(token, style: const TextStyle(fontFamily: 'monospace', fontSize: 12)),
              ],
            ),
          ),
          actions: [
            TextButton.icon(
              onPressed: token.isEmpty ? null : () async {
                await Clipboard.setData(ClipboardData(text: token));
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Credential copied.'), behavior: SnackBarBehavior.floating),
                  );
                }
              },
              icon: const Icon(Icons.copy_rounded),
              label: const Text('Copy'),
            ),
            FilledButton(onPressed: () => Navigator.pop(context), child: const Text('I stored it securely')),
          ],
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Credential operation failed: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
      );
    }
  }

  Future<void> editEnvironment(Map<String, dynamic> env) async {
    final hostname = TextEditingController(text: '${env['hostname'] ?? ''}');
    final version = TextEditingController(text: '${env['platform_version'] ?? ''}');
    final desiredRelease = TextEditingController(text: '${env['desired_release'] ?? ''}');
    final activeRelease = TextEditingController(text: '${env['active_release'] ?? ''}');
    String deployment = '${env['deployment_status'] ?? 'NOT_DEPLOYED'}';
    String environmentStatus = '${env['environment_status'] ?? 'CREATING'}';
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: '${env['kind']} environment',
          subtitle: 'Manage hostname, platform release and deployment/environment state.',
          icon: Icons.dns_outlined,
          width: 720,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: hostname, decoration: const InputDecoration(labelText: 'Hostname')),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: version, decoration: const InputDecoration(labelText: 'Platform version')),
                second: TextField(controller: desiredRelease, decoration: const InputDecoration(labelText: 'Desired release')),
              ),
              const SizedBox(height: 12),
              TextField(controller: activeRelease, decoration: const InputDecoration(labelText: 'Active release')),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: deployment,
                  decoration: const InputDecoration(labelText: 'Deployment status'),
                  items: const [
                    DropdownMenuItem(value: 'NOT_DEPLOYED', child: Text('NOT DEPLOYED')),
                    DropdownMenuItem(value: 'QUEUED', child: Text('QUEUED')),
                    DropdownMenuItem(value: 'DEPLOYING', child: Text('DEPLOYING')),
                    DropdownMenuItem(value: 'DEPLOYED', child: Text('DEPLOYED')),
                    DropdownMenuItem(value: 'FAILED', child: Text('FAILED')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => deployment = v); },
                ),
                second: DropdownButtonFormField<String>(
                  value: environmentStatus,
                  decoration: const InputDecoration(labelText: 'Environment status'),
                  items: const [
                    DropdownMenuItem(value: 'CREATING', child: Text('CREATING')),
                    DropdownMenuItem(value: 'CONFIGURATION_REQUIRED', child: Text('CONFIGURATION REQUIRED')),
                    DropdownMenuItem(value: 'TESTING', child: Text('TESTING')),
                    DropdownMenuItem(value: 'READY', child: Text('READY')),
                    DropdownMenuItem(value: 'LIVE', child: Text('LIVE')),
                    DropdownMenuItem(value: 'SUSPENDED', child: Text('SUSPENDED')),
                    DropdownMenuItem(value: 'FAILED', child: Text('FAILED')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => environmentStatus = v); },
                ),
              ),
            ],
          ),
          primaryLabel: 'Save environment',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok == true) {
      await widget.api.patch('/api/v1/environments/${env['id']}', {
        'hostname': hostname.text.trim(),
        'platform_version': version.text.trim(),
        'desired_release': desiredRelease.text.trim(),
        'active_release': activeRelease.text.trim(),
        'deployment_status': deployment,
        'environment_status': environmentStatus,
      });
      await load();
      if (mounted) success('Environment updated.');
    }
    for (final controller in [hostname, version, desiredRelease, activeRelease]) { controller.dispose(); }
  }

  Future<void> createProductionEnvironment() async {
    final existing = environments.where((e) => e['kind'] == 'PRODUCTION').toList();
    if (existing.isNotEmpty) {
      await editEnvironment(existing.first);
      return;
    }
    final hostname = TextEditingController(text: '${partner['primary_domain'] ?? ''}');
    final version = TextEditingController(text: '${partner['platform_version'] ?? ''}');
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'Production environment',
        subtitle: 'Register the production environment after staging validation and before launch.',
        icon: Icons.public_outlined,
        width: 620,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: hostname, decoration: const InputDecoration(labelText: 'Production hostname')),
            const SizedBox(height: 12),
            TextField(controller: version, decoration: const InputDecoration(labelText: 'Platform version')),
          ],
        ),
        primaryLabel: 'Create production environment',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );
    if (ok == true) {
      await widget.api.post('/api/v1/environments', {
        'partner_id': '${partner['id']}',
        'kind': 'PRODUCTION',
        'hostname': hostname.text.trim(),
        'platform_version': version.text.trim(),
      });
      await load();
      if (mounted) success('Production environment registered.');
    }
    hostname.dispose();
    version.dispose();
  }

  Future<void> editPartner() async {
    final display = TextEditingController(text: '${partner['display_name'] ?? ''}');
    final legal = TextEditingController(text: '${partner['legal_name'] ?? ''}');
    final brand = TextEditingController(text: '${partner['brand_name'] ?? ''}');
    final registration = TextEditingController(text: '${partner['registration_number'] ?? ''}');
    final tax = TextEditingController(text: '${partner['tax_id'] ?? ''}');
    final contact = TextEditingController(text: '${partner['contact_name'] ?? ''}');
    final email = TextEditingController(text: '${partner['contact_email'] ?? ''}');
    final financeName = TextEditingController(text: '${partner['finance_contact_name'] ?? ''}');
    final financeEmail = TextEditingController(text: '${partner['finance_contact_email'] ?? ''}');
    final technicalName = TextEditingController(text: '${partner['technical_contact_name'] ?? ''}');
    final technicalEmail = TextEditingController(text: '${partner['technical_contact_email'] ?? ''}');
    final marketingName = TextEditingController(text: '${partner['marketing_contact_name'] ?? ''}');
    final marketingEmail = TextEditingController(text: '${partner['marketing_contact_email'] ?? ''}');
    final country = TextEditingController(text: '${partner['country'] ?? ''}');
    final stateRegion = TextEditingController(text: '${partner['state_region'] ?? ''}');
    final city = TextEditingController(text: '${partner['city'] ?? ''}');
    final postal = TextEditingController(text: '${partner['postal_code'] ?? ''}');
    final address1 = TextEditingController(text: '${partner['address_line1'] ?? ''}');
    final address2 = TextEditingController(text: '${partner['address_line2'] ?? ''}');
    final website = TextEditingController(text: '${partner['website'] ?? ''}');
    final phone = TextEditingController(text: '${partner['phone'] ?? ''}');
    final primary = TextEditingController(text: '${partner['primary_domain'] ?? ''}');
    final staging = TextEditingController(text: '${partner['staging_domain'] ?? ''}');
    final logo = TextEditingController(text: '${partner['logo_url'] ?? ''}');
    final notes = TextEditingController(text: '${partner['notes'] ?? ''}');
    final lifecycleReason = TextEditingController();
    String lifecycle = '${partner['lifecycle'] ?? 'PROSPECT'}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Company Data',
          subtitle: 'Edit legal identity, contacts, lifecycle and partner references.',
          icon: Icons.apartment_outlined,
          width: 780,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: TextField(controller: display, decoration: const InputDecoration(labelText: 'Display name')),
                second: TextField(controller: legal, decoration: const InputDecoration(labelText: 'Legal name')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: brand, decoration: const InputDecoration(labelText: 'Brand / DBA')),
                second: DropdownButtonFormField<String>(
                  value: lifecycle,
                  decoration: const InputDecoration(labelText: 'Lifecycle'),
                  items: [
                    for (final value in _PartnersPageState.lifecycleOptions)
                      DropdownMenuItem(value: value, child: Text(_humanize(value))),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => lifecycle = v); },
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: lifecycleReason,
                decoration: const InputDecoration(labelText: 'Lifecycle change reason', hintText: 'Required for traceability when status changes'),
              ),
              const SizedBox(height: 18),
              const _DialogSectionLabel('REGISTRATION & ADDRESS'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: registration, decoration: const InputDecoration(labelText: 'Registration number')),
                second: TextField(controller: tax, decoration: const InputDecoration(labelText: 'Tax ID')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: country, decoration: const InputDecoration(labelText: 'Country')),
                second: TextField(controller: stateRegion, decoration: const InputDecoration(labelText: 'State / region')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: city, decoration: const InputDecoration(labelText: 'City')),
                second: TextField(controller: postal, decoration: const InputDecoration(labelText: 'Postal code')),
              ),
              const SizedBox(height: 12),
              TextField(controller: address1, decoration: const InputDecoration(labelText: 'Address line 1')),
              const SizedBox(height: 12),
              TextField(controller: address2, decoration: const InputDecoration(labelText: 'Address line 2')),
              const SizedBox(height: 18),
              const _DialogSectionLabel('CONTACTS'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: contact, decoration: const InputDecoration(labelText: 'Primary contact')),
                second: TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Primary email')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: financeName, decoration: const InputDecoration(labelText: 'Finance contact')),
                second: TextField(controller: financeEmail, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Finance email')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: technicalName, decoration: const InputDecoration(labelText: 'Technical contact')),
                second: TextField(controller: technicalEmail, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Technical email')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: marketingName, decoration: const InputDecoration(labelText: 'Marketing contact')),
                second: TextField(controller: marketingEmail, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Marketing email')),
              ),
              const SizedBox(height: 18),
              const _DialogSectionLabel('WEB & ENVIRONMENT REFERENCES'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: website, decoration: const InputDecoration(labelText: 'Website')),
                second: TextField(controller: phone, decoration: const InputDecoration(labelText: 'Phone')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: primary, decoration: const InputDecoration(labelText: 'Primary domain')),
                second: TextField(controller: staging, decoration: const InputDecoration(labelText: 'Staging domain')),
              ),
              const SizedBox(height: 12),
              TextField(controller: logo, decoration: const InputDecoration(labelText: 'Logo URL / asset reference')),
              const SizedBox(height: 12),
              TextField(controller: notes, maxLines: 3, decoration: const InputDecoration(labelText: 'Internal notes')),
            ],
          ),
          primaryLabel: 'Save changes',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true) {
      await widget.api.patch('/api/v1/partners/${partner['id']}', {
        'display_name': display.text.trim(),
        'legal_name': legal.text.trim(),
        'brand_name': brand.text.trim(),
        'registration_number': registration.text.trim(),
        'tax_id': tax.text.trim(),
        'lifecycle': lifecycle,
        'reason': lifecycleReason.text.trim(),
        'contact_name': contact.text.trim(),
        'contact_email': email.text.trim(),
        'finance_contact_name': financeName.text.trim(),
        'finance_contact_email': financeEmail.text.trim(),
        'technical_contact_name': technicalName.text.trim(),
        'technical_contact_email': technicalEmail.text.trim(),
        'marketing_contact_name': marketingName.text.trim(),
        'marketing_contact_email': marketingEmail.text.trim(),
        'country': country.text.trim(),
        'state_region': stateRegion.text.trim(),
        'city': city.text.trim(),
        'postal_code': postal.text.trim(),
        'address_line1': address1.text.trim(),
        'address_line2': address2.text.trim(),
        'website': website.text.trim(),
        'phone': phone.text.trim(),
        'primary_domain': primary.text.trim(),
        'staging_domain': staging.text.trim(),
        'logo_url': logo.text.trim(),
        'notes': notes.text.trim(),
      });
      await load();
      if (mounted) success('Partner data updated.');
    }

    for (final controller in [
      display, legal, brand, registration, tax, contact, email, financeName, financeEmail,
      technicalName, technicalEmail, marketingName, marketingEmail, country, stateRegion,
      city, postal, address1, address2, website, phone, primary, staging, logo, notes,
      lifecycleReason,
    ]) {
      controller.dispose();
    }
  }

  Future<void> editTerms() async {
    final activation = TextEditingController(text: number(terms?['activation_fee']).toStringAsFixed(2));
    final paid = TextEditingController(text: number(license?['paid_amount']).toStringAsFixed(2));
    final paymentDate = TextEditingController(text: '${license?['payment_date'] ?? ''}');
    final paymentReference = TextEditingController(text: '${license?['payment_reference'] ?? ''}');
    final verifiedBy = TextEditingController(text: '${license?['verified_by'] ?? ''}');
    final licenseNote = TextEditingController(text: '${license?['note'] ?? ''}');
    final base = TextEditingController(text: number(terms?['base_monthly_fee']).toStringAsFixed(2));
    final uplift = TextEditingController(text: number(terms?['annual_increase_percent']).toStringAsFixed(2));
    final effective = TextEditingController(text: '${terms?['price_effective_from'] ?? ''}');
    final anchor = TextEditingController(text: '${terms?['service_anchor_date'] ?? ''}');
    final waiverReason = TextEditingController(text: '${terms?['activation_fee_reason'] ?? ''}');
    final commercialReason = TextEditingController();
    bool waived = terms?['activation_fee_waived'] == true;
    String currency = '${terms?['currency'] ?? 'USD'}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Pricing & Subscription',
          subtitle: 'Partner-specific license and recurring terms with an activation-date anchored 30-day service cycle.',
          icon: Icons.payments_outlined,
          width: 760,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: currency,
                  decoration: const InputDecoration(labelText: 'Currency'),
                  items: const [
                    DropdownMenuItem(value: 'USD', child: Text('USD')),
                    DropdownMenuItem(value: 'EUR', child: Text('EUR')),
                    DropdownMenuItem(value: 'GBP', child: Text('GBP')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => currency = v); },
                ),
                second: TextField(
                  controller: activation,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(labelText: 'Initial license / activation fee'),
                ),
              ),
              const SizedBox(height: 8),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: waived,
                onChanged: (v) => setLocal(() => waived = v),
                title: const Text('Activation fee waived'),
                subtitle: const Text('Use only for an existing/reference partner where no activation transaction applies.'),
              ),
              if (waived) ...[
                const SizedBox(height: 8),
                TextField(controller: waiverReason, decoration: const InputDecoration(labelText: 'Waiver reason')),
              ],
              const SizedBox(height: 18),
              const _DialogSectionLabel('INITIAL LICENSE PAYMENT'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(
                  controller: paid,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(labelText: 'Paid amount'),
                ),
                second: TextField(
                  controller: paymentDate,
                  decoration: const InputDecoration(labelText: 'Payment date', hintText: 'YYYY-MM-DD'),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: paymentReference, decoration: const InputDecoration(labelText: 'Payment reference')),
                second: TextField(controller: verifiedBy, decoration: const InputDecoration(labelText: 'Verified by', hintText: 'Optional — current admin is used automatically')),
              ),
              const SizedBox(height: 12),
              TextField(controller: licenseNote, maxLines: 2, decoration: const InputDecoration(labelText: 'License note')),
              const SizedBox(height: 18),
              const _DialogSectionLabel('RECURRING SERVICE'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(
                  controller: base,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(labelText: 'Base 30-day service fee'),
                ),
                second: TextField(
                  controller: uplift,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(labelText: 'Annual increase %'),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: effective, decoration: const InputDecoration(labelText: 'Price effective from', hintText: 'YYYY-MM-DD')),
                second: TextField(controller: anchor, decoration: const InputDecoration(labelText: 'Service activation / anchor date', hintText: 'YYYY-MM-DD')),
              ),
              const SizedBox(height: 12),
              TextField(controller: commercialReason, decoration: const InputDecoration(labelText: 'Change reason', hintText: 'Recorded in commercial price history')),
              const SizedBox(height: 12),
              const _RuleStrip(
                items: [
                  _RuleItem(Icons.timelapse_outlined, 'Service cycle', '30 days from activation date'),
                  _RuleItem(Icons.event_repeat_outlined, 'Renewal', 'Every 30 days'),
                  _RuleItem(Icons.trending_up_rounded, 'Annual uplift', 'January 1'),
                ],
              ),
            ],
          ),
          primaryLabel: 'Save commercial terms',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true) {
      final requiredAmount = double.tryParse(activation.text) ?? 0;
      final paidAmount = double.tryParse(paid.text) ?? 0;
      final hasLicenseEvidence = documents.any((d) {
        final kind = '${d['kind'] ?? ''}'.toUpperCase();
        final storageReference = '${d['storage_url'] ?? ''}'.trim();
        final evidenceKind = kind == 'PAYMENT_EVIDENCE' || kind == 'INVOICE' || kind == 'RECEIPT' || kind == 'CONTRACT';
        return evidenceKind && storageReference.isNotEmpty;
      });
      if (!waived && requiredAmount > 0 && paidAmount >= requiredAmount && !hasLicenseEvidence) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Register the license invoice, receipt, contract, or payment evidence before marking the license paid.'),
              behavior: SnackBarBehavior.floating,
              backgroundColor: brandWarning,
            ),
          );
        }
        return;
      }

      await widget.api.put('/api/v1/billing/partners/${partner['id']}/terms', {
        'currency': currency,
        'activation_fee': double.tryParse(activation.text) ?? 0,
        'activation_fee_waived': waived,
        'activation_fee_reason': waiverReason.text.trim(),
        'base_monthly_fee': double.tryParse(base.text) ?? 0,
        'annual_increase_percent': double.tryParse(uplift.text) ?? 10,
        'price_effective_from': effective.text.trim(),
        'service_anchor_date': anchor.text.trim(),
        'reason': commercialReason.text.trim(),
      });
      await widget.api.put('/api/v1/billing/partners/${partner['id']}/license', {
        'currency': currency,
        'required_amount': double.tryParse(activation.text) ?? 0,
        'paid_amount': double.tryParse(paid.text) ?? 0,
        'payment_date': paymentDate.text.trim(),
        'payment_reference': paymentReference.text.trim(),
        'verified_by': verifiedBy.text.trim(),
        'note': licenseNote.text.trim(),
        'waived': waived,
        'waiver_reason': waiverReason.text.trim(),
      });
      await load();
      if (mounted) success('Commercial terms and initial license updated.');
    }

    for (final controller in [
      activation,
      paid,
      paymentDate,
      paymentReference,
      verifiedBy,
      licenseNote,
      base,
      uplift,
      effective,
      anchor,
      waiverReason,
      commercialReason,
    ]) {
      controller.dispose();
    }
  }

  Future<void> addDocument() async {
    final name = TextEditingController();
    final url = TextEditingController();
    final note = TextEditingController();
    String kind = 'CONTRACT';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Register document',
          subtitle: 'Register commercial document metadata with a persistent storage URL or document reference.',
          icon: Icons.note_add_outlined,
          width: 640,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: kind,
                decoration: const InputDecoration(labelText: 'Document type'),
                items: const [
                  DropdownMenuItem(value: 'CONTRACT', child: Text('Contract')),
                  DropdownMenuItem(value: 'INVOICE', child: Text('Invoice')),
                  DropdownMenuItem(value: 'RECEIPT', child: Text('Receipt')),
                  DropdownMenuItem(value: 'PAYMENT_EVIDENCE', child: Text('Payment evidence')),
                  DropdownMenuItem(value: 'OTHER', child: Text('Other')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => kind = v); },
              ),
              const SizedBox(height: 12),
              TextField(controller: name, decoration: const InputDecoration(labelText: 'Document name *')),
              const SizedBox(height: 12),
              TextField(
                controller: url,
                decoration: const InputDecoration(
                  labelText: 'Storage URL / reference',
                  hintText: 'Required for contracts, invoices, receipts and payment evidence',
                ),
              ),
              const SizedBox(height: 12),
              TextField(controller: note, maxLines: 3, decoration: const InputDecoration(labelText: 'Notes')),
            ],
          ),
          primaryLabel: 'Register document',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true && name.text.trim().isNotEmpty) {
      final evidenceKind = kind == 'CONTRACT' || kind == 'INVOICE' || kind == 'RECEIPT' || kind == 'PAYMENT_EVIDENCE';
      if (evidenceKind && url.text.trim().isEmpty) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Commercial evidence requires an attached storage URL or persistent document reference.'),
              behavior: SnackBarBehavior.floating,
              backgroundColor: brandWarning,
            ),
          );
        }
        for (final controller in [name, url, note]) {
          controller.dispose();
        }
        return;
      }
      await widget.api.post('/api/v1/billing/partners/${partner['id']}/documents', {
        'kind': kind,
        'name': name.text.trim(),
        'storage_url': url.text.trim(),
        'note': note.text.trim(),
      });
      await load();
      if (mounted) success('Document registered.');
    }

    for (final c in [name, url, note]) {
      c.dispose();
    }
  }

  Map<String, dynamic>? subscriptionFor(String moduleKey) {
    for (final item in subscriptions) {
      if ('${item['module_key']}' == moduleKey) return item;
    }
    return null;
  }

  Future<void> editModule(Map<String, dynamic> module) async {
    String state = '${module['status']}';
    bool visible = module['visible'] == true;
    bool included = module['included_in_base'] == true;
    final moduleKey = '${module['key']}';
    var subscription = subscriptionFor(moduleKey);
    bool cancelAtPeriodEnd = subscription?['cancel_at_period_end'] == true;
    final initialCancel = cancelAtPeriodEnd;
    final price = TextEditingController(text: number(module['partner_price']).toStringAsFixed(2));
    final effectiveAt = TextEditingController();
    final reason = TextEditingController();

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: '${module['label']}',
          subtitle: 'Control entitlement, partner visibility and 30-day pricing without removing the underlying module code or data.',
          icon: Icons.grid_view_outlined,
          width: 680,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: state,
                decoration: const InputDecoration(labelText: 'Module state'),
                items: const [
                  DropdownMenuItem(value: 'ACTIVE', child: Text('ACTIVE')),
                  DropdownMenuItem(value: 'NOT_LICENSED', child: Text('NOT LICENSED')),
                  DropdownMenuItem(value: 'MAINTENANCE', child: Text('MAINTENANCE')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => state = v); },
              ),
              const SizedBox(height: 8),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: visible,
                onChanged: (v) => setLocal(() => visible = v),
                title: const Text('Visible for partner'),
                subtitle: const Text('Visibility is separate from module code existence.'),
              ),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: included,
                onChanged: (v) => setLocal(() => included = v),
                title: const Text('Included in base package'),
                subtitle: const Text('Modules outside the base package contribute to recurring fees.'),
              ),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: cancelAtPeriodEnd,
                onChanged: state == 'ACTIVE' ? (v) => setLocal(() => cancelAtPeriodEnd = v) : null,
                title: const Text('Cancel at period end'),
                subtitle: Text(
                  subscription == null
                      ? 'A 30-day subscription record is created when the active module is synchronized.'
                      : 'Current period ends ${subscription['period_end_exclusive'] ?? '—'}. Cancellation keeps access through that date.',
                ),
              ),
              const SizedBox(height: 8),
              ResponsiveFieldPair(
                first: TextField(
                  controller: price,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(labelText: 'Partner 30-day price'),
                ),
                second: TextField(
                  controller: effectiveAt,
                  decoration: const InputDecoration(labelText: 'Price effective at', hintText: 'Optional RFC3339 timestamp'),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: reason,
                decoration: const InputDecoration(labelText: 'Change reason', hintText: 'Recorded in module, price and subscription history'),
              ),
            ],
          ),
          primaryLabel: 'Save module',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true) {
      await widget.api.patch(
        '/api/v1/partners/${partner['id']}/modules/$moduleKey',
        {
          'status': state,
          'visible': visible,
          'included_in_base': included,
          'partner_price': double.tryParse(price.text) ?? 0,
          'price_effective_at': effectiveAt.text.trim(),
          'reason': reason.text.trim(),
        },
      );

      if (state == 'ACTIVE' && cancelAtPeriodEnd != initialCancel) {
        if (subscription == null) {
          await widget.api.get('/api/v1/billing/partners/${partner['id']}/summary', force: true);
          final refreshed = await widget.api.get('/api/v1/billing/partners/${partner['id']}/subscriptions', force: true);
          subscriptions = items(refreshed);
          subscription = subscriptionFor(moduleKey);
        }
        if (subscription != null) {
          await widget.api.patch(
            '/api/v1/billing/partners/${partner['id']}/subscriptions/$moduleKey',
            {
              'cancel_at_period_end': cancelAtPeriodEnd,
              'reason': reason.text.trim(),
            },
          );
        }
      }

      await load();
      if (mounted) success('Module configuration updated.');
    }
    price.dispose();
    effectiveAt.dispose();
    reason.dispose();
  }

  List<Map<String, dynamic>> get filteredModules {
    final q = moduleQuery.trim().toLowerCase();
    return modules.where((m) {
      final matchText = q.isEmpty ||
          '${m['label']}'.toLowerCase().contains(q) ||
          '${m['key']}'.toLowerCase().contains(q) ||
          '${m['group_label']}'.toLowerCase().contains(q);
      final matchState = moduleState == 'ALL' || '${m['status']}' == moduleState;
      return matchText && matchState;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final active = modules.where((m) => m['status'] == 'ACTIVE').length;
    final maintenance = modules.where((m) => m['status'] == 'MAINTENANCE').length;
    final baseIncluded = modules.where((m) => m['included_in_base'] == true).length;

    return Scaffold(
      backgroundColor: brandIvory,
      appBar: AppBar(
        leading: IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.arrow_back_rounded)),
        title: MediaQuery.sizeOf(context).width < 520
            ? const HimateLogo(compact: true, width: 34)
            : const HimateLogo(width: 170),
        actions: [
          _StatusPill(label: '${partner['lifecycle'] ?? 'PROSPECT'}'),
          const SizedBox(width: 12),
          IconButton(onPressed: editPartner, tooltip: 'Edit partner', icon: const Icon(Icons.edit_outlined)),
          const SizedBox(width: 8),
        ],
      ),
      body: loading
          ? const _BrandLoading()
          : error != null
              ? Padding(
                  padding: const EdgeInsets.all(24),
                  child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Partner workspace unavailable', message: error!),
                )
              : Content(
                  eyebrow: 'PARTNER WORKSPACE  |  ${partner['id']}',
                  title: '${partner['display_name']}',
                  subtitle: [
                    '${partner['category_name']}',
                    '${partner['country']}',
                    _humanize('${partner['lifecycle']}'),
                    if ('${partner['primary_domain'] ?? ''}'.isNotEmpty) '${partner['primary_domain']}',
                    'Health: ${_humanize('${partner['system_health'] ?? 'UNKNOWN'}')}',
                    'Version: ${'${partner['platform_version'] ?? ''}'.isEmpty ? '—' : partner['platform_version']}',
                  ].join(' · '),
                  actions: [
                    OutlinedButton.icon(onPressed: editPartner, icon: const Icon(Icons.edit_outlined), label: const Text('Company data')),
                    FilledButton.icon(onPressed: editTerms, icon: const Icon(Icons.payments_outlined), label: const Text('Commercial terms')),
                  ],
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      KeyedSubtree(
                        key: _overviewKey,
                        child: Wrap(
                          spacing: 12,
                          runSpacing: 12,
                          children: [
                            Kpi(label: 'Current recurring', value: money(billing?['current_total']), note: 'Base + active extra modules', icon: Icons.account_balance_wallet_outlined, accent: brandGold),
                          Kpi(label: 'Active modules', value: '$active', note: '${modules.length} module records', icon: Icons.grid_view_outlined, accent: brandNavy),
                          Kpi(label: 'Base package', value: '$baseIncluded', note: 'Included module entitlements', icon: Icons.inventory_2_outlined, accent: brandSteel),
                            Kpi(label: 'Maintenance', value: '$maintenance', note: 'Temporarily restricted modules', icon: Icons.build_outlined, accent: brandWarning),
                          ],
                        ),
                      ),
                      const SizedBox(height: 22),
                      _SectionHeader(title: 'Workspace', subtitle: 'Current and scheduled control areas for this partner.'),
                      const SizedBox(height: 12),
                      LayoutBuilder(
                        builder: (context, c) {
                          final width = c.maxWidth < 560 ? c.maxWidth : c.maxWidth < 900 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                          return Wrap(
                            spacing: 12,
                            runSpacing: 12,
                            children: [
                              for (final spec in workspaceCards)
                                SizedBox(
                                  width: width,
                                  child: WorkspaceCard(
                                    spec: spec,
                                    onTap: spec.active
                                        ? () => Navigator.pushNamed(
                                              context,
                                              "/app/partners/${partner['id']}/${workspaceRouteSlug(spec.title)}",
                                            )
                                        : null,
                                  ),
                                ),
                            ],
                          );
                        },
                      ),
                      const SizedBox(height: 26),
                      LayoutBuilder(
                        builder: (context, c) {
                          final company = KeyedSubtree(key: _companyKey, child: _PartnerDetailsCard(partner: partner));
                          final termsCard = KeyedSubtree(key: _pricingKey, child: _CommercialSummaryCard(terms: terms ?? {}, billing: billing ?? {}, license: license ?? {}, onEdit: editTerms));
                          if (c.maxWidth < 930) {
                            return Column(children: [company, const SizedBox(height: 14), termsCard]);
                          }
                          return Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                            Expanded(child: company),
                            const SizedBox(width: 14),
                            Expanded(child: termsCard),
                          ]);
                        },
                      ),
                      const SizedBox(height: 26),
                      KeyedSubtree(
                        key: _environmentKey,
                        child: _SectionHeader(
                          title: 'System & Environment',
                          subtitle: 'Provisioning state, isolated partner infrastructure, staging/production and release metadata.',
                          trailing: Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              OutlinedButton.icon(
                                onPressed: createProductionEnvironment,
                                icon: const Icon(Icons.public_outlined),
                                label: Text(environments.any((e) => e['kind'] == 'PRODUCTION') ? 'Production settings' : 'Add production'),
                              ),
                              FilledButton.icon(
                                onPressed: startProvisioning,
                                icon: const Icon(Icons.precision_manufacturing_outlined),
                                label: Text(provisioningJob == null ? 'Start provisioning' : 'Resume provisioning'),
                              ),
                            ],
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      LayoutBuilder(
                        builder: (context, c) {
                          final cards = <Widget>[];
                          final job = provisioningJob;
                          if (job != null) {
                            cards.add(
                              _InfoCard(
                                title: 'Provisioning Engine',
                                icon: Icons.precision_manufacturing_outlined,
                                children: [
                                  _DefinitionRow(label: 'Status', value: '${job['status'] ?? 'UNKNOWN'}'),
                                  _DefinitionRow(label: 'Current step', value: '${job['current_step'] ?? '—'}'),
                                  _DefinitionRow(label: 'System name', value: '${job['system_name'] ?? '—'}'),
                                  _DefinitionRow(label: 'Release', value: '${job['desired_release'] ?? '—'}'),
                                  if ('${job['last_error'] ?? ''}'.isNotEmpty)
                                    _DefinitionRow(label: 'Last error', value: '${job['last_error']}'),
                                ],
                              ),
                            );
                          }
                          for (final env in environments) {
                            cards.add(
                              _InfoCard(
                                title: '${env['kind']}',
                                icon: env['kind'] == 'PRODUCTION' ? Icons.public_outlined : Icons.science_outlined,
                                action: IconButton(
                                  tooltip: 'Edit environment',
                                  onPressed: () => editEnvironment(env),
                                  icon: const Icon(Icons.edit_outlined, size: 18),
                                ),
                                children: [
                                  _DefinitionRow(label: 'Hostname', value: '${env['hostname'] ?? '—'}'),
                                  _DefinitionRow(label: 'Environment', value: _humanize('${env['environment_status'] ?? 'UNKNOWN'}')),
                                  _DefinitionRow(label: 'Deployment', value: _humanize('${env['deployment_status'] ?? 'UNKNOWN'}')),
                                  _DefinitionRow(label: 'Platform version', value: '${env['platform_version'] ?? '—'}'),
                                  _DefinitionRow(label: 'Active release', value: '${env['active_release'] ?? '—'}'),
                                ],
                              ),
                            );
                          }
                          if (cards.isEmpty) {
                            return const _MessageCard(
                              icon: Icons.dns_outlined,
                              title: 'No environment yet',
                              message: 'Provisioning will create the isolated partner database, base configuration and staging environment.',
                            );
                          }
                          final width = c.maxWidth < 680 ? c.maxWidth : c.maxWidth < 1080 ? (c.maxWidth - 14) / 2 : (c.maxWidth - 28) / 3;
                          return Wrap(
                            spacing: 14,
                            runSpacing: 14,
                            children: [for (final card in cards) SizedBox(width: width, child: card)],
                          );
                        },
                      ),
                      const SizedBox(height: 26),
                      KeyedSubtree(
                        key: _modulesKey,
                        child: _SectionHeader(
                          title: 'Partner Modules',
                          subtitle: 'Entitlement, visibility, base-package inclusion and partner-specific pricing.',
                          trailing: _MiniCounter(label: '${filteredModules.length} shown'),
                        ),
                      ),
                      const SizedBox(height: 12),
                      _FilterSurface(
                        child: LayoutBuilder(
                          builder: (context, c) {
                            final search = TextField(
                              onChanged: (v) => setState(() => moduleQuery = v),
                              decoration: const InputDecoration(hintText: 'Search modules...', prefixIcon: Icon(Icons.search_rounded)),
                            );
                            final state = DropdownButtonFormField<String>(
                              value: moduleState,
                              decoration: const InputDecoration(labelText: 'State'),
                              items: const [
                                DropdownMenuItem(value: 'ALL', child: Text('All states')),
                                DropdownMenuItem(value: 'ACTIVE', child: Text('Active')),
                                DropdownMenuItem(value: 'NOT_LICENSED', child: Text('Not licensed')),
                                DropdownMenuItem(value: 'MAINTENANCE', child: Text('Maintenance')),
                              ],
                              onChanged: (v) => setState(() => moduleState = v ?? 'ALL'),
                            );
                            if (c.maxWidth < 680) return Column(children: [search, const SizedBox(height: 10), state]);
                            return Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: state)]);
                          },
                        ),
                      ),
                      const SizedBox(height: 12),
                      LayoutBuilder(
                        builder: (context, c) {
                          final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1020 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                          return Wrap(
                            spacing: 12,
                            runSpacing: 12,
                            children: [
                              for (final m in filteredModules)
                                SizedBox(width: width, child: PartnerModuleCard(module: m, onTap: () => editModule(m))),
                            ],
                          );
                        },
                      ),
                      const SizedBox(height: 26),
                      KeyedSubtree(
                        key: _financeKey,
                        child: _SectionHeader(
                          title: 'Finance & Documents',
                          subtitle: 'Commercial evidence and internal invoice records for this partner.',
                          trailing: FilledButton.icon(onPressed: addDocument, icon: const Icon(Icons.note_add_outlined), label: const Text('Register document')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      LayoutBuilder(
                        builder: (context, c) {
                          final docs = _DocumentPanel(documents: documents, onAdd: addDocument);
                          final inv = _InvoicePanel(invoices: invoices);
                          if (c.maxWidth < 920) return Column(children: [docs, const SizedBox(height: 14), inv]);
                          return Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                            Expanded(child: docs),
                            const SizedBox(width: 14),
                            Expanded(child: inv),
                          ]);
                        },
                      ),
                      const SizedBox(height: 26),
                      KeyedSubtree(
                        key: _statisticsKey,
                        child: _SectionHeader(
                          title: 'Statistics',
                          subtitle: 'Partner-scoped impact metrics retain period, aggregation and provenance for auditable reporting.',
                          trailing: _MiniCounter(label: '${impactSummary.length} metrics'),
                        ),
                      ),
                      const SizedBox(height: 12),
                      impactSummary.isEmpty
                          ? const _MessageCard(
                              icon: Icons.insights_outlined,
                              title: 'No impact observations yet',
                              message: 'Create metric definitions under Impact & Reports, then record partner values manually or through the Connector Protocol.',
                            )
                          : LayoutBuilder(
                              builder: (context, c) {
                                final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1000 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                                return Wrap(
                                  spacing: 12,
                                  runSpacing: 12,
                                  children: [
                                    for (final metric in impactSummary)
                                      SizedBox(
                                        width: width,
                                        child: _InfoCard(
                                          title: '${metric['label'] ?? metric['metric_key']}',
                                          icon: Icons.insights_outlined,
                                          children: [
                                            _DefinitionRow(label: 'Value', value: '${metric['numeric_value'] ?? '—'} ${metric['unit'] ?? ''}'),
                                            _DefinitionRow(label: 'Aggregation', value: '${metric['aggregation'] ?? '—'}'),
                                            _DefinitionRow(label: 'Latest period', value: '${metric['latest_period_end'] ?? '—'}'),
                                            _DefinitionRow(label: 'Observations', value: '${metric['observations'] ?? 0}'),
                                          ],
                                        ),
                                      ),
                                  ],
                                );
                              },
                            ),
                      const SizedBox(height: 26),
                      KeyedSubtree(
                        key: _integrationsKey,
                        child: _SectionHeader(
                          title: 'Integrations',
                          subtitle: 'Partner-scoped Connector Protocol credentials. Raw secrets are never stored by HIMATE.',
                          trailing: FilledButton.icon(
                            onPressed: rotateConnectorCredential,
                            icon: const Icon(Icons.key_outlined),
                            label: const Text('Generate / rotate credential'),
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      connectorCredentials.isEmpty
                          ? const _MessageCard(
                              icon: Icons.hub_outlined,
                              title: 'No connector credential yet',
                              message: 'Provisioning creates the staging connector identity automatically. You can also generate or rotate it here.',
                            )
                          : LayoutBuilder(
                              builder: (context, c) {
                                final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 980 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                                return Wrap(
                                  spacing: 12,
                                  runSpacing: 12,
                                  children: [
                                    for (final credential in connectorCredentials)
                                      SizedBox(
                                        width: width,
                                        child: _InfoCard(
                                          title: '${credential['environment']} Connector',
                                          icon: Icons.hub_outlined,
                                          children: [
                                            _DefinitionRow(label: 'Credential ID', value: '${credential['credential_id'] ?? '—'}'),
                                            _DefinitionRow(label: 'Active', value: credential['active'] == true ? 'Yes' : 'No'),
                                            _DefinitionRow(label: 'Rotated', value: '${credential['rotated_at'] ?? '—'}'),
                                            _DefinitionRow(label: 'Last used', value: '${credential['last_used_at'] ?? 'Never'}'),
                                          ],
                                        ),
                                      ),
                                  ],
                                );
                              },
                            ),
                    ],
                  ),
                ),
    );
  }
}

class FinancePage extends StatefulWidget {
  const FinancePage({required this.api, super.key});
  final Api api;

  @override
  State<FinancePage> createState() => _FinancePageState();
}

class _FinancePageState extends State<FinancePage> {
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> groups = <Map<String, dynamic>>[];
  Map<String, dynamic>? profile;
  bool loading = true;
  String? error;
  String query = '';
  String groupFilter = 'ALL';

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final r = await Future.wait([
        widget.api.get('/api/v1/modules'),
        widget.api.get('/api/v1/module-groups'),
        widget.api.get('/api/v1/billing/profile'),
      ]);
      modules = items(r[0]);
      groups = items(r[1]);
      profile = r[2];
    } catch (e) {
      error = e.toString();
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void success(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
    );
  }

  Future<void> editProfile() async {
    final legal = TextEditingController(text: '${profile?['legal_name'] ?? ''}');
    final registration = TextEditingController(text: '${profile?['registration_number'] ?? ''}');
    final address = TextEditingController(text: '${profile?['address'] ?? ''}');
    final tax = TextEditingController(text: '${profile?['tax_id'] ?? ''}');
    final contactName = TextEditingController(text: '${profile?['contact_name'] ?? ''}');
    final email = TextEditingController(text: '${profile?['email'] ?? ''}');
    final phone = TextEditingController(text: '${profile?['phone'] ?? ''}');
    final bank = TextEditingController(text: '${profile?['bank_name'] ?? ''}');
    final bankAddress = TextEditingController(text: '${profile?['bank_address'] ?? ''}');
    final account = TextEditingController(text: '${profile?['account_number'] ?? ''}');
    final iban = TextEditingController(text: '${profile?['iban'] ?? ''}');
    final swift = TextEditingController(text: '${profile?['swift'] ?? ''}');

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'HIMATE billing profile',
        subtitle: 'Issuer and international banking data used as the foundation for future invoice-provider integration.',
        icon: Icons.account_balance_outlined,
        width: 760,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ResponsiveFieldPair(
              first: TextField(controller: legal, decoration: const InputDecoration(labelText: 'Legal name')),
              second: TextField(controller: registration, decoration: const InputDecoration(labelText: 'Registration number')),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: tax, decoration: const InputDecoration(labelText: 'Tax ID')),
              second: TextField(controller: contactName, decoration: const InputDecoration(labelText: 'Billing contact')),
            ),
            const SizedBox(height: 12),
            TextField(controller: address, decoration: const InputDecoration(labelText: 'Company address')),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Billing email')),
              second: TextField(controller: phone, keyboardType: TextInputType.phone, decoration: const InputDecoration(labelText: 'Billing phone')),
            ),
            const SizedBox(height: 18),
            const _DialogSectionLabel('BANKING DETAILS'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: TextField(controller: bank, decoration: const InputDecoration(labelText: 'Bank name')),
              second: TextField(controller: bankAddress, decoration: const InputDecoration(labelText: 'Bank address')),
            ),
            const SizedBox(height: 12),
            TextField(controller: account, decoration: const InputDecoration(labelText: 'Account number')),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: iban, decoration: const InputDecoration(labelText: 'IBAN')),
              second: TextField(controller: swift, decoration: const InputDecoration(labelText: 'SWIFT / BIC')),
            ),
          ],
        ),
        primaryLabel: 'Save billing profile',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );

    if (ok == true) {
      await widget.api.put('/api/v1/billing/profile', {
        'legal_name': legal.text.trim(),
        'registration_number': registration.text.trim(),
        'address': address.text.trim(),
        'tax_id': tax.text.trim(),
        'contact_name': contactName.text.trim(),
        'email': email.text.trim(),
        'phone': phone.text.trim(),
        'bank_name': bank.text.trim(),
        'bank_address': bankAddress.text.trim(),
        'account_number': account.text.trim(),
        'iban': iban.text.trim(),
        'swift': swift.text.trim(),
      });
      await load();
      if (mounted) success('Billing profile updated.');
    }

    for (final c in [legal, registration, address, tax, contactName, email, phone, bank, bankAddress, account, iban, swift]) {
      c.dispose();
    }
  }

  Future<void> addModule() async {
    if (groups.isEmpty) return;
    final label = TextEditingController();
    final key = TextEditingController();
    final description = TextEditingController();
    final price = TextEditingController(text: '0.00');
    String group = '${groups.first['group_key']}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Add custom module',
          subtitle: 'Create a stable module key and place the new capability inside an existing HIMATE menu group.',
          icon: Icons.add_box_outlined,
          width: 700,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: TextField(controller: label, decoration: const InputDecoration(labelText: 'Module name *')),
                second: TextField(controller: key, decoration: const InputDecoration(labelText: 'Stable key *', hintText: 'group.module_name')),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: group,
                decoration: const InputDecoration(labelText: 'Menu group'),
                items: [
                  for (final g in groups)
                    DropdownMenuItem(value: '${g['group_key']}', child: Text('${g['label']}')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => group = v); },
              ),
              const SizedBox(height: 12),
              TextField(controller: description, maxLines: 3, decoration: const InputDecoration(labelText: 'Description')),
              const SizedBox(height: 12),
              TextField(controller: price, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: 'Default monthly price (USD)')),
            ],
          ),
          primaryLabel: 'Create module',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true && label.text.trim().isNotEmpty && key.text.trim().isNotEmpty) {
      await widget.api.post('/api/v1/modules', {
        'key': key.text.trim(),
        'label': label.text.trim(),
        'group_key': group,
        'description': description.text.trim(),
        'currency': 'USD',
        'version': '1.0.0',
        'latest_version': '1.0.0',
        'default_monthly_price': double.tryParse(price.text) ?? 0,
      });
      await load();
      if (mounted) success('Custom module created.');
    }

    for (final c in [label, key, description, price]) {
      c.dispose();
    }
  }

  Future<void> editCatalogModule(Map<String, dynamic> module) async {
    final label = TextEditingController(text: '${module['label'] ?? ''}');
    final description = TextEditingController(text: '${module['description'] ?? ''}');
    final price = TextEditingController(text: number(module['default_monthly_price']).toStringAsFixed(2));
    final latestVersion = TextEditingController(text: '${module['latest_version'] ?? module['version'] ?? '1.0.0'}');
    String group = '${module['group_key'] ?? (groups.isNotEmpty ? groups.first['group_key'] : '')}';
    String availability = '${module['availability'] ?? 'ACTIVE'}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: '${module['label']}',
          subtitle: 'Manage catalog metadata and availability without changing the stable technical key.',
          icon: Icons.grid_view_outlined,
          width: 720,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: label, decoration: const InputDecoration(labelText: 'Module name')),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: group,
                  decoration: const InputDecoration(labelText: 'Menu group'),
                  items: [
                    for (final g in groups)
                      DropdownMenuItem(value: '${g['group_key']}', child: Text('${g['label']}')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => group = v); },
                ),
                second: DropdownButtonFormField<String>(
                  value: availability,
                  decoration: const InputDecoration(labelText: 'Availability'),
                  items: const [
                    DropdownMenuItem(value: 'ACTIVE', child: Text('ACTIVE')),
                    DropdownMenuItem(value: 'UNAVAILABLE', child: Text('UNAVAILABLE')),
                    DropdownMenuItem(value: 'DEPRECATED', child: Text('DEPRECATED')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => availability = v); },
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(
                  controller: price,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: const InputDecoration(labelText: 'Default 30-day price'),
                ),
                second: TextField(controller: latestVersion, decoration: const InputDecoration(labelText: 'Latest version')),
              ),
              const SizedBox(height: 12),
              TextField(controller: description, maxLines: 3, decoration: const InputDecoration(labelText: 'Description')),
              const SizedBox(height: 12),
              TextFormField(
                initialValue: '${module['key']}',
                readOnly: true,
                decoration: const InputDecoration(labelText: 'Stable technical key'),
              ),
            ],
          ),
          primaryLabel: 'Save module',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true && label.text.trim().isNotEmpty) {
      await widget.api.patch('/api/v1/modules/${module['key']}', {
        'label': label.text.trim(),
        'description': description.text.trim(),
        'group_key': group,
        'default_monthly_price': double.tryParse(price.text) ?? 0,
        'availability': availability,
        'latest_version': latestVersion.text.trim(),
      });
      await load();
      if (mounted) success('Module catalog entry updated.');
    }

    for (final controller in [label, description, price, latestVersion]) {
      controller.dispose();
    }
  }

  List<Map<String, dynamic>> get filteredModules {
    final q = query.trim().toLowerCase();
    return modules.where((m) {
      final textOk = q.isEmpty ||
          '${m['label']}'.toLowerCase().contains(q) ||
          '${m['key']}'.toLowerCase().contains(q) ||
          '${m['group_label']}'.toLowerCase().contains(q);
      final groupOk = groupFilter == 'ALL' || '${m['group_key']}' == groupFilter;
      return textOk && groupOk;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final custom = modules.where((m) => m['system'] != true).length;
    final priced = modules.where((m) => number(m['default_monthly_price']) > 0).length;

    return Content(
      eyebrow: 'COMMERCIAL CONTROL',
      title: 'Licensing & Finance',
      subtitle: 'Module catalog, pricing foundations and HIMATE issuer data — governed from one place.',
      actions: [
        OutlinedButton.icon(onPressed: editProfile, icon: const Icon(Icons.account_balance_outlined), label: const Text('Billing profile')),
        FilledButton.icon(onPressed: addModule, icon: const Icon(Icons.add_box_outlined), label: const Text('Add module')),
      ],
      child: loading
          ? const _BrandLoading()
          : error != null
              ? _MessageCard(icon: Icons.cloud_off_outlined, title: 'Finance workspace unavailable', message: error!)
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [
                        Kpi(label: 'Module catalog', value: '${modules.length}', note: 'Canonical + custom modules', icon: Icons.grid_view_outlined, accent: brandNavy),
                        Kpi(label: 'Custom modules', value: '$custom', note: 'Created by HIMATE admins', icon: Icons.extension_outlined, accent: brandSteel),
                        Kpi(label: 'Priced defaults', value: '$priced', note: 'Modules with catalog pricing', icon: Icons.sell_outlined, accent: brandGold),
                        const Kpi(label: 'Annual uplift', value: 'JAN 1', note: 'Default +10%, admin-overridable', icon: Icons.trending_up_rounded, accent: brandSuccess),
                      ],
                    ),
                    const SizedBox(height: 22),
                    LayoutBuilder(
                      builder: (context, c) {
                        final issuer = _IssuerProfileCard(profile: profile ?? {}, onEdit: editProfile);
                        const rules = _BillingRulesCard();
                        if (c.maxWidth < 920) return Column(children: [issuer, const SizedBox(height: 14), rules]);
                        return Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Expanded(child: issuer),
                          const SizedBox(width: 14),
                          const Expanded(child: rules),
                        ]);
                      },
                    ),
                    const SizedBox(height: 26),
                    _SectionHeader(
                      title: 'Canonical Module Catalog',
                      subtitle: 'The verified reference catalog stays centrally governed while custom modules can be added without changing the partner data model.',
                      trailing: _MiniCounter(label: '${filteredModules.length} shown'),
                    ),
                    const SizedBox(height: 12),
                    _FilterSurface(
                      child: LayoutBuilder(
                        builder: (context, c) {
                          final search = TextField(
                            onChanged: (v) => setState(() => query = v),
                            decoration: const InputDecoration(hintText: 'Search module catalog...', prefixIcon: Icon(Icons.search_rounded)),
                          );
                          final group = DropdownButtonFormField<String>(
                            value: groupFilter,
                            decoration: const InputDecoration(labelText: 'Menu group'),
                            items: [
                              const DropdownMenuItem(value: 'ALL', child: Text('All groups')),
                              for (final g in groups)
                                DropdownMenuItem(value: '${g['group_key']}', child: Text('${g['label']}')),
                            ],
                            onChanged: (v) => setState(() => groupFilter = v ?? 'ALL'),
                          );
                          if (c.maxWidth < 680) return Column(children: [search, const SizedBox(height: 10), group]);
                          return Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: group)]);
                        },
                      ),
                    ),
                    const SizedBox(height: 12),
                    LayoutBuilder(
                      builder: (context, c) {
                        final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1020 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                        return Wrap(
                          spacing: 12,
                          runSpacing: 12,
                          children: [
                            for (final m in filteredModules)
                              SizedBox(width: width, child: CatalogModuleCard(module: m, onTap: () => editCatalogModule(m))),
                          ],
                        );
                      },
                    ),
                  ],
                ),
    );
  }
}

class ImpactPage extends StatefulWidget {
  const ImpactPage({required this.api, super.key});
  final Api api;

  @override
  State<ImpactPage> createState() => _ImpactPageState();
}

class _ImpactPageState extends State<ImpactPage> {
  List<Map<String, dynamic>> definitions = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> summary = <Map<String, dynamic>>[];
  bool loading = true;
  String? error;

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final r = await Future.wait([
        widget.api.get('/api/v1/impact/definitions', force: true),
        widget.api.get('/api/v1/impact/summary', force: true),
      ]);
      definitions = items(r[0]);
      summary = items(r[1]);
    } catch (e) {
      error = e.toString();
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> addDefinition() async {
    final key = TextEditingController();
    final label = TextEditingController();
    final description = TextEditingController();
    final unit = TextEditingController(text: 'count');
    String aggregation = 'SUM';
    String scope = 'PARTNER';
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'New metric definition',
          subtitle: 'Create a stable impact metric used consistently across partners and reporting periods.',
          icon: Icons.add_chart_outlined,
          width: 700,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: TextField(controller: key, decoration: const InputDecoration(labelText: 'Metric key', hintText: 'culture.events')),
                second: TextField(controller: label, decoration: const InputDecoration(labelText: 'Display label')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: unit, decoration: const InputDecoration(labelText: 'Unit')),
                second: DropdownButtonFormField<String>(
                  value: aggregation,
                  decoration: const InputDecoration(labelText: 'Aggregation'),
                  items: const [
                    DropdownMenuItem(value: 'SUM', child: Text('SUM')),
                    DropdownMenuItem(value: 'LATEST', child: Text('LATEST')),
                    DropdownMenuItem(value: 'AVERAGE', child: Text('AVERAGE')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => aggregation = v); },
                ),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: scope,
                decoration: const InputDecoration(labelText: 'Scope'),
                items: const [
                  DropdownMenuItem(value: 'PARTNER', child: Text('PARTNER')),
                  DropdownMenuItem(value: 'GLOBAL', child: Text('GLOBAL')),
                  DropdownMenuItem(value: 'BOTH', child: Text('BOTH')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => scope = v); },
              ),
              const SizedBox(height: 12),
              TextField(controller: description, maxLines: 3, decoration: const InputDecoration(labelText: 'Description')),
            ],
          ),
          primaryLabel: 'Create metric',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok == true) {
      await widget.api.post('/api/v1/impact/definitions', {
        'metric_key': key.text.trim(),
        'label': label.text.trim(),
        'description': description.text.trim(),
        'unit': unit.text.trim(),
        'aggregation': aggregation,
        'scope': scope,
      });
      await load();
    }
    for (final controller in [key, label, description, unit]) { controller.dispose(); }
  }

  Future<void> addValue() async {
    if (definitions.isEmpty) return;
    String metricKey = '${definitions.first['metric_key']}';
    String provenance = 'MANUAL';
    final partner = TextEditingController();
    final start = TextEditingController(text: DateTime.now().toIso8601String().substring(0, 10));
    final end = TextEditingController(text: DateTime.now().toIso8601String().substring(0, 10));
    final numeric = TextEditingController();
    final source = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Record impact value',
          subtitle: 'Every value keeps period, provenance and source metadata for later evidence and reporting.',
          icon: Icons.insights_outlined,
          width: 700,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: metricKey,
                decoration: const InputDecoration(labelText: 'Metric'),
                items: [
                  for (final d in definitions)
                    DropdownMenuItem(value: '${d['metric_key']}', child: Text('${d['label']} · ${d['metric_key']}')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => metricKey = v); },
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: partner, decoration: const InputDecoration(labelText: 'Partner ID', hintText: 'Leave empty for global metric')),
                second: TextField(controller: numeric, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: 'Numeric value')),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: start, decoration: const InputDecoration(labelText: 'Period start', hintText: 'YYYY-MM-DD')),
                second: TextField(controller: end, decoration: const InputDecoration(labelText: 'Period end', hintText: 'YYYY-MM-DD')),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: provenance,
                decoration: const InputDecoration(labelText: 'Provenance'),
                items: const [
                  DropdownMenuItem(value: 'MANUAL', child: Text('MANUAL')),
                  DropdownMenuItem(value: 'SYSTEM', child: Text('SYSTEM')),
                  DropdownMenuItem(value: 'PARTNER_DECLARED', child: Text('PARTNER DECLARED')),
                  DropdownMenuItem(value: 'VERIFIED_DOCUMENT', child: Text('VERIFIED DOCUMENT')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => provenance = v); },
              ),
              const SizedBox(height: 12),
              TextField(controller: source, decoration: const InputDecoration(labelText: 'Source reference', hintText: 'Document, URL or source identifier')),
            ],
          ),
          primaryLabel: 'Record value',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok == true) {
      await widget.api.post('/api/v1/impact/values', {
        'partner_id': partner.text.trim(),
        'metric_key': metricKey,
        'period_start': start.text.trim(),
        'period_end': end.text.trim(),
        'numeric_value': double.tryParse(numeric.text),
        'provenance': provenance,
        'source_ref': source.text.trim(),
      });
      await load();
    }
    for (final controller in [partner, start, end, numeric, source]) { controller.dispose(); }
  }

  @override
  Widget build(BuildContext context) {
    if (loading) return const _BrandLoading();
    if (error != null) {
      return Content(
        eyebrow: 'IMPACT CONTROL',
        title: 'Impact & Reports',
        subtitle: 'Metric definitions, provenance and partner/global impact values.',
        child: _MessageCard(icon: Icons.error_outline_rounded, title: 'Impact data unavailable', message: error!),
      );
    }
    return Content(
      eyebrow: 'IMPACT CONTROL',
      title: 'Impact & Reports',
      subtitle: 'Global and partner impact metrics with explicit source provenance. Evidence files and PDF reports remain START-14–15.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          LayoutBuilder(
            builder: (context, c) {
              final actions = [
                OutlinedButton.icon(onPressed: addDefinition, icon: const Icon(Icons.add_chart_outlined), label: const Text('New metric')),
                FilledButton.icon(onPressed: definitions.isEmpty ? null : addValue, icon: const Icon(Icons.add_rounded), label: const Text('Record value')),
              ];
              if (c.maxWidth < 620) {
                return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                  SizedBox(width: double.infinity, child: actions[1]),
                  const SizedBox(height: 8),
                  SizedBox(width: double.infinity, child: actions[0]),
                ]);
              }
              return Row(mainAxisAlignment: MainAxisAlignment.end, children: [actions[0], const SizedBox(width: 10), actions[1]]);
            },
          ),
          const SizedBox(height: 18),
          _SectionHeader(title: 'Impact Summary', subtitle: 'Aggregated values follow each metric definition’s SUM, LATEST or AVERAGE rule.', trailing: _MiniCounter(label: '${summary.length} metrics')),
          const SizedBox(height: 12),
          LayoutBuilder(
            builder: (context, c) {
              final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1000 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final m in summary)
                    SizedBox(
                      width: width,
                      child: _InfoCard(
                        title: '${m['label'] ?? m['metric_key']}',
                        icon: Icons.insights_outlined,
                        children: [
                          _DefinitionRow(label: 'Value', value: '${m['numeric_value'] ?? '—'} ${m['unit'] ?? ''}'),
                          _DefinitionRow(label: 'Aggregation', value: '${m['aggregation'] ?? ''}'),
                          _DefinitionRow(label: 'Latest period', value: '${m['latest_period_end'] ?? '—'}'),
                          _DefinitionRow(label: 'Observations', value: '${m['observations'] ?? 0}'),
                        ],
                      ),
                    ),
                ],
              );
            },
          ),
          const SizedBox(height: 24),
          _SectionHeader(title: 'Metric Definitions', subtitle: 'Stable definitions are reused by manual entry, partner connectors and future verified-document workflows.', trailing: _MiniCounter(label: '${definitions.length} definitions')),
          const SizedBox(height: 12),
          LayoutBuilder(
            builder: (context, c) {
              final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1000 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final d in definitions)
                    SizedBox(
                      width: width,
                      child: _InfoCard(
                        title: '${d['label']}',
                        icon: Icons.stacked_line_chart_rounded,
                        children: [
                          _DefinitionRow(label: 'Key', value: '${d['metric_key']}'),
                          _DefinitionRow(label: 'Unit', value: '${d['unit']}'),
                          _DefinitionRow(label: 'Aggregation', value: '${d['aggregation']}'),
                          _DefinitionRow(label: 'Scope', value: '${d['scope']}'),
                          _DefinitionRow(label: 'Status', value: d['active'] == true ? 'Active' : 'Inactive'),
                        ],
                      ),
                    ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }
}

class SystemPage extends StatelessWidget {
  const SystemPage({required this.api, super.key});
  final Api api;

  Future<List<Map<String, dynamic>>> _load() async {
    final r = await Future.wait([
      api.get('/api/v1/system-health', force: true),
      api.get('/api/v1/provisioning/jobs', force: true),
      api.get('/api/v1/environments', force: true),
    ]);
    return r;
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<Map<String, dynamic>>>(
      future: _load(),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) return const _BrandLoading();
        if (snapshot.hasError || snapshot.data == null) {
          return Content(
            eyebrow: 'PLATFORM OPERATIONS',
            title: 'System & Operations',
            subtitle: 'Independent services behind one authenticated public gateway.',
            child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Operations data unavailable', message: '${snapshot.error}'),
          );
        }

        final health = snapshot.data![0];
        final provisioning = items(snapshot.data![1]);
        final environments = items(snapshot.data![2]);
        final services = items({'items': health['services']});
        final partners = items({'items': health['partners']});
        final overall = '${health['status'] ?? 'UNKNOWN'}';

        return Content(
          eyebrow: 'PLATFORM OPERATIONS',
          title: 'System & Operations',
          subtitle: 'Provisioning, partner environments, connectors and central health across the containerized HIMATE control plane.',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _OperationsHero(status: overall, environment: 'control plane', version: 'START-09–13'),
              const SizedBox(height: 22),
              _SectionHeader(
                title: 'Service Health',
                subtitle: 'Readiness and liveness are monitored independently for each microservice.',
                trailing: _MiniCounter(label: '${services.length} services'),
              ),
              const SizedBox(height: 12),
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final s in services)
                    ServiceCard(name: _humanize('${s['name'] ?? 'service'}'), status: '${s['status'] ?? 'UNKNOWN'}'),
                ],
              ),
              const SizedBox(height: 24),
              _SectionHeader(
                title: 'Partner Health',
                subtitle: 'Connector, environment, provisioning and platform-version state aggregated per partner.',
                trailing: _MiniCounter(label: '${partners.length} partners'),
              ),
              const SizedBox(height: 12),
              LayoutBuilder(
                builder: (context, c) {
                  final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1000 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                  return Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      for (final p in partners)
                        SizedBox(
                          width: width,
                          child: _InfoCard(
                            title: '${p['partner_id']}',
                            icon: Icons.monitor_heart_outlined,
                            children: [
                              _DefinitionRow(label: 'Overall', value: '${p['overall_status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Connector', value: '${p['connector_health'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Environment', value: '${p['environment_status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Provisioning', value: '${p['provisioning_status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Version', value: '${p['platform_version'] ?? '—'}'),
                            ],
                          ),
                        ),
                    ],
                  );
                },
              ),
              const SizedBox(height: 24),
              _SectionHeader(
                title: 'Provisioning Engine',
                subtitle: 'Idempotent jobs can resume after interruption without creating duplicate partner infrastructure.',
                trailing: _MiniCounter(label: '${provisioning.length} jobs'),
              ),
              const SizedBox(height: 12),
              LayoutBuilder(
                builder: (context, c) {
                  final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1000 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                  return Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      for (final j in provisioning)
                        SizedBox(
                          width: width,
                          child: _InfoCard(
                            title: '${j['partner_id']}',
                            icon: Icons.precision_manufacturing_outlined,
                            children: [
                              _DefinitionRow(label: 'Status', value: '${j['status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Current step', value: '${j['current_step'] ?? '—'}'),
                              _DefinitionRow(label: 'System', value: '${j['system_name'] ?? '—'}'),
                              _DefinitionRow(label: 'Release', value: '${j['desired_release'] ?? '—'}'),
                            ],
                          ),
                        ),
                    ],
                  );
                },
              ),
              const SizedBox(height: 24),
              _SectionHeader(
                title: 'Partner Environments',
                subtitle: 'Staging and production records retain hostname, version, deployment and environment state.',
                trailing: _MiniCounter(label: '${environments.length} environments'),
              ),
              const SizedBox(height: 12),
              LayoutBuilder(
                builder: (context, c) {
                  final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1000 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                  return Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      for (final e in environments)
                        SizedBox(
                          width: width,
                          child: _InfoCard(
                            title: '${e['partner_id']} · ${e['kind']}',
                            icon: Icons.dns_outlined,
                            children: [
                              _DefinitionRow(label: 'Hostname', value: '${e['hostname'] ?? '—'}'),
                              _DefinitionRow(label: 'Environment', value: '${e['environment_status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Deployment', value: '${e['deployment_status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Version', value: '${e['platform_version'] ?? '—'}'),
                            ],
                          ),
                        ),
                    ],
                  );
                },
              ),
              const SizedBox(height: 24),
              LayoutBuilder(
                builder: (context, c) {
                  const architecture = _ArchitectureCard();
                  const controls = _OperationsControlsCard();
                  if (c.maxWidth < 900) {
                    return const Column(children: [architecture, SizedBox(height: 14), controls]);
                  }
                  return const Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(child: architecture),
                      SizedBox(width: 14),
                      Expanded(child: controls),
                    ],
                  );
                },
              ),
            ],
          ),
        );
      },
    );
  }
}


String _humanize(String value) {
  return value
      .toLowerCase()
      .split('_')
      .where((e) => e.isNotEmpty)
      .map((e) => e[0].toUpperCase() + e.substring(1))
      .join(' ');
}

class ResponsiveFieldPair extends StatelessWidget {
  const ResponsiveFieldPair({
    required this.first,
    required this.second,
    this.breakpoint = 620,
    this.gap = 12,
    super.key,
  });

  final Widget first;
  final Widget second;
  final double breakpoint;
  final double gap;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < breakpoint) {
          return Column(
            children: [
              SizedBox(width: double.infinity, child: first),
              SizedBox(height: gap),
              SizedBox(width: double.infinity, child: second),
            ],
          );
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: first),
            SizedBox(width: gap),
            Expanded(child: second),
          ],
        );
      },
    );
  }
}


class BrandDialog extends StatelessWidget {
  const BrandDialog({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.child,
    required this.primaryLabel,
    required this.onPrimary,
    this.width = 620,
    super.key,
  });

  final String title, subtitle, primaryLabel;
  final IconData icon;
  final Widget child;
  final VoidCallback onPrimary;
  final double width;

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 24),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: width, maxHeight: MediaQuery.of(context).size.height * .88),
        child: Container(
          decoration: BoxDecoration(
            color: brandWhite,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: brandMist),
            boxShadow: [BoxShadow(color: brandNavy.withOpacity(.16), blurRadius: 44, offset: const Offset(0, 20))],
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                padding: const EdgeInsets.fromLTRB(22, 20, 18, 18),
                decoration: const BoxDecoration(
                  border: Border(bottom: BorderSide(color: brandMist)),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 42,
                      height: 42,
                      decoration: BoxDecoration(color: brandGold.withOpacity(.12), borderRadius: BorderRadius.circular(11)),
                      child: Icon(icon, color: brandGold, size: 21),
                    ),
                    const SizedBox(width: 13),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(title, style: Theme.of(context).textTheme.titleLarge),
                          const SizedBox(height: 4),
                          Text(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 12, height: 1.4)),
                        ],
                      ),
                    ),
                    IconButton(onPressed: () => Navigator.pop(context, false), icon: const Icon(Icons.close_rounded)),
                  ],
                ),
              ),
              Flexible(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(22),
                  child: child,
                ),
              ),
              Container(
                padding: const EdgeInsets.fromLTRB(20, 14, 20, 18),
                decoration: const BoxDecoration(border: Border(top: BorderSide(color: brandMist))),
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final actions = [
                      TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')),
                      FilledButton(onPressed: onPrimary, child: Text(primaryLabel)),
                    ];
                    if (constraints.maxWidth < 420) {
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          SizedBox(width: double.infinity, child: actions[1]),
                          const SizedBox(height: 8),
                          SizedBox(width: double.infinity, child: actions[0]),
                        ],
                      );
                    }
                    return Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [actions[0], const SizedBox(width: 8), actions[1]],
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _FilterSurface extends StatelessWidget {
  const _FilterSurface({required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: brandWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: brandMist),
      ),
      child: child,
    );
  }
}

class _MiniCounter extends StatelessWidget {
  const _MiniCounter({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(
        color: brandNavy.withOpacity(.055),
        borderRadius: BorderRadius.circular(99),
        border: Border.all(color: brandNavy.withOpacity(.07)),
      ),
      child: Text(label, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w700)),
    );
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) {
    final value = label.toUpperCase();
    final tone = value == 'LIVE' || value == 'ACTIVE' || value == 'OK' || value == 'HEALTHY'
        ? brandSuccess
        : value.contains('MAINTENANCE') || value.contains('PROVISION')
            ? brandWarning
            : value.contains('SUSPENDED') || value.contains('ARCHIVED')
                ? brandDanger
                : brandSteel;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
      decoration: BoxDecoration(color: tone.withOpacity(.08), borderRadius: BorderRadius.circular(99), border: Border.all(color: tone.withOpacity(.15))),
      child: Text(_humanize(label), style: TextStyle(color: tone, fontSize: 9, fontWeight: FontWeight.w800, letterSpacing: .25)),
    );
  }
}

class PartnerCard extends StatefulWidget {
  const PartnerCard({required this.partner, required this.onTap, super.key});
  final Map<String, dynamic> partner;
  final VoidCallback onTap;

  @override
  State<PartnerCard> createState() => _PartnerCardState();
}

class _PartnerCardState extends State<PartnerCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) {
    final p = widget.partner;
    return MouseRegion(
      onEnter: (_) => setState(() => hover = true),
      onExit: (_) => setState(() => hover = false),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        transform: Matrix4.translationValues(0, hover ? -3 : 0, 0),
        decoration: BoxDecoration(
          color: brandWhite,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: hover ? brandGold.withOpacity(.42) : brandMist),
          boxShadow: [BoxShadow(color: brandNavy.withOpacity(hover ? .08 : .035), blurRadius: hover ? 22 : 12, offset: Offset(0, hover ? 9 : 5))],
        ),
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: widget.onTap,
            borderRadius: BorderRadius.circular(14),
            child: Padding(
              padding: const EdgeInsets.all(17),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      _PartnerLogo(url: '${p['logo_url'] ?? ''}'),
                      const Spacer(),
                      if (p['reference_partner'] == true)
                        const Tooltip(message: 'Reference partner', child: Icon(Icons.workspace_premium_rounded, color: brandGold, size: 21)),
                    ],
                  ),
                  const SizedBox(height: 16),
                  Text('${p['display_name']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 19, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 4),
                  Text('${p['category_name']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 11)),
                  const SizedBox(height: 7),
                  Text(
                    '${p['primary_domain'] ?? ''}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: brandTextSoft, fontSize: 9.5),
                  ),
                  const SizedBox(height: 11),
                  Row(children: [
                    _StatusPill(label: '${p['lifecycle']}'),
                    const Spacer(),
                    _StatusPill(label: '${p['system_health'] ?? 'UNKNOWN'}'),
                  ]),
                  const SizedBox(height: 11),
                  Wrap(
                    spacing: 6,
                    runSpacing: 6,
                    children: [
                      _PartnerMetric(label: 'MODULES', value: '${p['active_modules'] ?? 0}'),
                      _PartnerMetric(label: '30 DAYS', value: '${p['currency'] ?? 'USD'} ${number(p['service_value_30d']).toStringAsFixed(0)}'),
                      _PartnerMetric(label: 'VERSION', value: '${p['platform_version']?.toString().isNotEmpty == true ? p['platform_version'] : '—'}'),
                    ],
                  ),
                  const SizedBox(height: 12),
                  const Divider(height: 1),
                  const SizedBox(height: 12),
                  Row(children: [
                    Expanded(child: Text('${p['id']}', style: const TextStyle(color: brandTextSoft, fontSize: 9.5))),
                    const Text('Open workspace', style: TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w700)),
                    const SizedBox(width: 5),
                    AnimatedSlide(offset: hover ? const Offset(.12, 0) : Offset.zero, duration: const Duration(milliseconds: 160), child: const Icon(Icons.arrow_forward_rounded, color: brandGold, size: 16)),
                  ]),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _PartnerLogo extends StatelessWidget {
  const _PartnerLogo({required this.url});
  final String url;

  bool get _safeToLoad {
    final value = url.trim();
    if (value.isEmpty) return false;
    if (value.startsWith('/')) return true;
    final parsed = Uri.tryParse(value);
    if (parsed == null) return false;
    if (!parsed.hasScheme) return true;
    return parsed.scheme == Uri.base.scheme && parsed.host == Uri.base.host && parsed.port == Uri.base.port;
  }

  @override
  Widget build(BuildContext context) {
    final fallback = Container(
      width: 42,
      height: 42,
      decoration: BoxDecoration(color: brandNavy.withOpacity(.055), borderRadius: BorderRadius.circular(11)),
      child: const Icon(Icons.apartment_rounded, color: brandNavy, size: 21),
    );
    if (!_safeToLoad) return fallback;
    return Container(
      width: 42,
      height: 42,
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: brandWhite,
        borderRadius: BorderRadius.circular(11),
        border: Border.all(color: brandMist),
      ),
      child: Image.network(
        url.trim(),
        fit: BoxFit.contain,
        errorBuilder: (_, __, ___) => fallback,
        semanticLabel: 'Partner logo',
      ),
    );
  }
}


class _PartnerMetric extends StatelessWidget {
  const _PartnerMetric({required this.label, required this.value});
  final String label, value;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 5),
        decoration: BoxDecoration(
          color: brandNavy.withOpacity(.035),
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: brandMist),
        ),
        child: Text(
          '$label  $value',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(color: brandTextSoft, fontSize: 8.3, fontWeight: FontWeight.w600),
        ),
      );
}


class NewPartnerCard extends StatefulWidget {
  const NewPartnerCard({required this.onTap, super.key});
  final VoidCallback onTap;

  @override
  State<NewPartnerCard> createState() => _NewPartnerCardState();
}

class _NewPartnerCardState extends State<NewPartnerCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => hover = true),
      onExit: (_) => setState(() => hover = false),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: widget.onTap,
          borderRadius: BorderRadius.circular(14),
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 180),
            constraints: const BoxConstraints(minHeight: 224),
            decoration: BoxDecoration(
              color: hover ? brandGold.withOpacity(.055) : brandWhite,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: brandGold.withOpacity(hover ? .75 : .35), width: 1.1),
            ),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(width: 46, height: 46, decoration: BoxDecoration(color: brandGold.withOpacity(.12), shape: BoxShape.circle), child: const Icon(Icons.add_rounded, color: brandGold, size: 26)),
                  const SizedBox(height: 11),
                  const Text('NEW PARTNER', style: TextStyle(color: brandNavy, fontSize: 11, fontWeight: FontWeight.w800, letterSpacing: 1.3)),
                  const SizedBox(height: 5),
                  const Text('Create a new partner workspace', style: TextStyle(color: brandTextSoft, fontSize: 10.5)),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _WorkspaceSpec {
  const _WorkspaceSpec(this.title, this.icon, this.subtitle, this.active);
  final String title, subtitle;
  final IconData icon;
  final bool active;
}

class WorkspaceCard extends StatelessWidget {
  const WorkspaceCard({required this.spec, this.onTap, super.key});
  final _WorkspaceSpec spec;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final card = Container(
      constraints: const BoxConstraints(minHeight: 112),
      padding: const EdgeInsets.all(15),
      decoration: BoxDecoration(
        color: brandWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: brandMist),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [
            Container(width: 34, height: 34, decoration: BoxDecoration(color: (spec.active ? brandGold : brandSteel).withOpacity(.10), borderRadius: BorderRadius.circular(9)), child: Icon(spec.icon, color: spec.active ? brandGold : brandSteel, size: 18)),
            const Spacer(),
            _MiniCounter(label: spec.active ? 'AVAILABLE' : 'PLANNED'),
          ]),
          const SizedBox(height: 11),
          Text(spec.title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 12.5)),
          const SizedBox(height: 3),
          Text(spec.subtitle, maxLines: 2, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.6, height: 1.35)),
        ],
      ),
    );
    if (onTap == null) return card;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: card,
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title, required this.subtitle, this.trailing});
  final String title, subtitle;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final copy = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title, style: Theme.of(context).textTheme.titleLarge),
        const SizedBox(height: 4),
        Text(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 11.5, height: 1.4)),
      ],
    );
    if (trailing == null) return copy;
    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < 620) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [copy, const SizedBox(height: 10), trailing!],
          );
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [Expanded(child: copy), const SizedBox(width: 12), trailing!],
        );
      },
    );
  }
}

class PartnerModuleCard extends StatefulWidget {
  const PartnerModuleCard({required this.module, required this.onTap, super.key});
  final Map<String, dynamic> module;
  final VoidCallback onTap;

  @override
  State<PartnerModuleCard> createState() => _PartnerModuleCardState();
}

class _PartnerModuleCardState extends State<PartnerModuleCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) {
    final m = widget.module;
    final included = m['included_in_base'] == true;
    final visible = m['visible'] == true;
    return MouseRegion(
      onEnter: (_) => setState(() => hover = true),
      onExit: (_) => setState(() => hover = false),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: widget.onTap,
          borderRadius: BorderRadius.circular(12),
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 170),
            padding: const EdgeInsets.all(15),
            decoration: BoxDecoration(
              color: brandWhite,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: hover ? brandGold.withOpacity(.42) : brandMist),
              boxShadow: hover ? [BoxShadow(color: brandNavy.withOpacity(.06), blurRadius: 18, offset: const Offset(0, 7))] : const [],
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Expanded(child: Text('${m['label']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 13))),
                  const SizedBox(width: 8),
                  _StatusPill(label: '${m['status']}'),
                ]),
                const SizedBox(height: 5),
                Text('${m['group_label']} · ${m['key']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
                const SizedBox(height: 13),
                Wrap(
                  spacing: 7,
                  runSpacing: 7,
                  children: [
                    _TinyFlag(icon: visible ? Icons.visibility_outlined : Icons.visibility_off_outlined, label: visible ? 'VISIBLE' : 'HIDDEN', active: visible),
                    _TinyFlag(icon: included ? Icons.inventory_2_outlined : Icons.add_card_outlined, label: included ? 'BASE' : 'EXTRA', active: included),
                  ],
                ),
                const SizedBox(height: 13),
                Row(children: [
                  const Text('Monthly', style: TextStyle(color: brandTextSoft, fontSize: 9.5)),
                  const Spacer(),
                  Text(included ? 'Included' : money(m['partner_price']), style: const TextStyle(color: brandNavy, fontSize: 16, fontWeight: FontWeight.w600)),
                  const SizedBox(width: 7),
                  const Icon(Icons.edit_outlined, color: brandGold, size: 16),
                ]),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _TinyFlag extends StatelessWidget {
  const _TinyFlag({required this.icon, required this.label, required this.active});
  final IconData icon;
  final String label;
  final bool active;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 5),
    decoration: BoxDecoration(color: (active ? brandSuccess : brandSteel).withOpacity(.07), borderRadius: BorderRadius.circular(7)),
    child: Row(mainAxisSize: MainAxisSize.min, children: [
      Icon(icon, size: 12, color: active ? brandSuccess : brandSteel),
      const SizedBox(width: 4),
      Text(label, style: TextStyle(fontSize: 8.5, color: active ? brandSuccess : brandSteel, fontWeight: FontWeight.w700)),
    ]),
  );
}

class _PartnerDetailsCard extends StatelessWidget {
  const _PartnerDetailsCard({required this.partner});
  final Map<String, dynamic> partner;

  String value(dynamic input) => '${input ?? ''}'.trim().isEmpty ? '—' : '${input ?? ''}';

  @override
  Widget build(BuildContext context) => _InfoCard(
    title: 'Company Data',
    icon: Icons.apartment_outlined,
    children: [
      _DefinitionRow(label: 'Legal name', value: value(partner['legal_name'])),
      _DefinitionRow(label: 'Brand / DBA', value: value(partner['brand_name'])),
      _DefinitionRow(label: 'Category', value: value(partner['category_name'])),
      _DefinitionRow(label: 'Registration', value: value(partner['registration_number'])),
      _DefinitionRow(label: 'Tax ID', value: value(partner['tax_id'])),
      _DefinitionRow(label: 'Primary contact', value: value(partner['contact_name'])),
      _DefinitionRow(label: 'Contact email', value: value(partner['contact_email'])),
      _DefinitionRow(label: 'Primary domain', value: value(partner['primary_domain'])),
      _DefinitionRow(label: 'Staging domain', value: value(partner['staging_domain'])),
      _DefinitionRow(label: 'Website', value: value(partner['website'])),
    ],
  );
}

class _CommercialSummaryCard extends StatelessWidget {
  const _CommercialSummaryCard({required this.terms, required this.billing, required this.license, required this.onEdit});
  final Map<String, dynamic> terms, billing, license;
  final VoidCallback onEdit;

  @override
  Widget build(BuildContext context) => _InfoCard(
    title: 'Pricing & Subscription',
    icon: Icons.payments_outlined,
    action: IconButton(onPressed: onEdit, tooltip: 'Edit commercial terms', icon: const Icon(Icons.edit_outlined, size: 18)),
    children: [
      _DefinitionRow(label: 'Activation fee', value: terms['activation_fee_waived'] == true ? 'Waived' : money(terms['activation_fee'])),
      _DefinitionRow(label: 'License status', value: _humanize('${license['status'] ?? 'NOT_PAID'}')),
      _DefinitionRow(label: 'License paid', value: '${money(license['paid_amount'])} / ${money(license['required_amount'])}'),
      _DefinitionRow(label: 'Base monthly fee', value: money(billing['effective_base_fee'])),
      _DefinitionRow(label: 'Extra modules', value: money(billing['extra_module_fee'])),
      _DefinitionRow(label: 'Current total', value: money(billing['current_total']), emphasis: true),
      _DefinitionRow(label: 'Annual increase', value: '${terms['annual_increase_percent'] ?? 10}% · January 1'),
      _DefinitionRow(label: 'Next cycle', value: '${billing['next_billing_date'] ?? '—'}'),
      const _DefinitionRow(label: 'Billing rule', value: 'Activation-date anchored · 30 days'),
    ],
  );
}

class _InfoCard extends StatelessWidget {
  const _InfoCard({required this.title, required this.icon, required this.children, this.action});
  final String title;
  final IconData icon;
  final List<Widget> children;
  final Widget? action;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(18),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Container(width: 36, height: 36, decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(9)), child: Icon(icon, color: brandGold, size: 19)),
          const SizedBox(width: 10),
          Expanded(child: Text(title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
          if (action != null) action!,
        ]),
        const SizedBox(height: 14),
        ...children,
      ]),
    ),
  );
}

class _DefinitionRow extends StatelessWidget {
  const _DefinitionRow({required this.label, required this.value, this.emphasis = false});
  final String label, value;
  final bool emphasis;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 7),
    child: Row(children: [
      Expanded(child: Text(label, style: const TextStyle(color: brandTextSoft, fontSize: 10.5))),
      const SizedBox(width: 12),
      Flexible(child: Text(value, textAlign: TextAlign.right, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(color: brandNavy, fontSize: emphasis ? 13 : 11, fontWeight: emphasis ? FontWeight.w800 : FontWeight.w600))),
    ]),
  );
}

class _RuleItem {
  const _RuleItem(this.icon, this.label, this.value);
  final IconData icon;
  final String label, value;
}

class _RuleStrip extends StatelessWidget {
  const _RuleStrip({required this.items});
  final List<_RuleItem> items;

  @override
  Widget build(BuildContext context) => Wrap(
    spacing: 8,
    runSpacing: 8,
    children: [
      for (final item in items)
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
          decoration: BoxDecoration(color: brandNavy.withOpacity(.04), borderRadius: BorderRadius.circular(9), border: Border.all(color: brandMist)),
          child: Row(mainAxisSize: MainAxisSize.min, children: [
            Icon(item.icon, color: brandGold, size: 15),
            const SizedBox(width: 7),
            Text('${item.label}: ', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
            Text(item.value, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w700)),
          ]),
        ),
    ],
  );
}

class _DocumentPanel extends StatelessWidget {
  const _DocumentPanel({required this.documents, required this.onAdd});
  final List<Map<String, dynamic>> documents;
  final VoidCallback onAdd;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(18),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          const Expanded(child: Text('Documents', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
          _MiniCounter(label: '${documents.length} RECORDS'),
        ]),
        const SizedBox(height: 12),
        if (documents.isEmpty)
          _EmptyInline(icon: Icons.folder_open_outlined, title: 'No documents registered', actionLabel: 'Register document', onTap: onAdd)
        else
          for (var i = 0; i < documents.length && i < 5; i++) ...[
            _DocumentRow(document: documents[i]),
            if (i < documents.length - 1 && i < 4) const Divider(height: 1),
          ],
      ]),
    ),
  );
}

class _DocumentRow extends StatelessWidget {
  const _DocumentRow({required this.document});
  final Map<String, dynamic> document;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 10),
    child: Row(children: [
      Container(width: 34, height: 34, decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(8)), child: const Icon(Icons.description_outlined, color: brandGold, size: 17)),
      const SizedBox(width: 10),
      Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text('${document['name']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 11.5, fontWeight: FontWeight.w600)),
        const SizedBox(height: 2),
        Text(_humanize('${document['kind']}'), style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
      ])),
      if ('${document['storage_url'] ?? ''}'.isNotEmpty) const Icon(Icons.link_rounded, color: brandSteel, size: 16),
    ]),
  );
}

class _InvoicePanel extends StatelessWidget {
  const _InvoicePanel({required this.invoices});
  final List<Map<String, dynamic>> invoices;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(18),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          const Expanded(child: Text('Invoices', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
          _MiniCounter(label: '${invoices.length} RECORDS'),
        ]),
        const SizedBox(height: 12),
        if (invoices.isEmpty)
          const _EmptyInline(icon: Icons.receipt_long_outlined, title: 'No invoice records yet')
        else
          for (var i = 0; i < invoices.length && i < 5; i++) ...[
            _InvoiceRow(invoice: invoices[i]),
            if (i < invoices.length - 1 && i < 4) const Divider(height: 1),
          ],
      ]),
    ),
  );
}

class _InvoiceRow extends StatelessWidget {
  const _InvoiceRow({required this.invoice});
  final Map<String, dynamic> invoice;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: 10),
    child: Row(children: [
      Container(width: 34, height: 34, decoration: BoxDecoration(color: brandNavy.withOpacity(.055), borderRadius: BorderRadius.circular(8)), child: const Icon(Icons.receipt_long_outlined, color: brandNavy, size: 17)),
      const SizedBox(width: 10),
      Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text('${invoice['id']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 11.5, fontWeight: FontWeight.w600)),
        const SizedBox(height: 2),
        Text('${invoice['invoice_date'] ?? ''}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
      ])),
      Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
        Text(money(invoice['total']), style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w600, fontSize: 14)),
        const SizedBox(height: 2),
        _StatusPill(label: '${invoice['status'] ?? 'DRAFT'}'),
      ]),
    ]),
  );
}

class _EmptyInline extends StatelessWidget {
  const _EmptyInline({required this.icon, required this.title, this.actionLabel, this.onTap});
  final IconData icon;
  final String title;
  final String? actionLabel;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) => Container(
    width: double.infinity,
    padding: const EdgeInsets.symmetric(vertical: 24, horizontal: 14),
    decoration: BoxDecoration(color: brandIvory, borderRadius: BorderRadius.circular(10), border: Border.all(color: brandMist)),
    child: Column(children: [
      Icon(icon, color: brandSteel, size: 24),
      const SizedBox(height: 8),
      Text(title, style: const TextStyle(color: brandNavy, fontSize: 11.5, fontWeight: FontWeight.w600)),
      if (actionLabel != null && onTap != null) ...[
        const SizedBox(height: 8),
        TextButton(onPressed: onTap, child: Text(actionLabel!)),
      ],
    ]),
  );
}

class _DialogSectionLabel extends StatelessWidget {
  const _DialogSectionLabel(this.label);
  final String label;

  @override
  Widget build(BuildContext context) => Align(
    alignment: Alignment.centerLeft,
    child: Text(label, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w800, letterSpacing: 1.6)),
  );
}

class _IssuerProfileCard extends StatelessWidget {
  const _IssuerProfileCard({required this.profile, required this.onEdit});
  final Map<String, dynamic> profile;
  final VoidCallback onEdit;

  String clean(dynamic value) => '${value ?? ''}'.trim().isEmpty ? 'Not configured' : '${value ?? ''}';

  @override
  Widget build(BuildContext context) => _InfoCard(
    title: 'HIMATE Issuer Profile',
    icon: Icons.account_balance_outlined,
    action: IconButton(onPressed: onEdit, tooltip: 'Edit billing profile', icon: const Icon(Icons.edit_outlined, size: 18)),
    children: [
      _DefinitionRow(label: 'Legal name', value: clean(profile['legal_name'])),
      _DefinitionRow(label: 'Billing email', value: clean(profile['email'])),
      _DefinitionRow(label: 'Tax ID', value: clean(profile['tax_id'])),
      _DefinitionRow(label: 'Bank', value: clean(profile['bank_name'])),
      _DefinitionRow(label: 'IBAN', value: clean(profile['iban'])),
      _DefinitionRow(label: 'SWIFT / BIC', value: clean(profile['swift'])),
    ],
  );
}

class _BillingRulesCard extends StatelessWidget {
  const _BillingRulesCard();

  @override
  Widget build(BuildContext context) => const _InfoCard(
    title: 'Commercial Rules',
    icon: Icons.rule_folder_outlined,
    children: [
      _DefinitionRow(label: 'Service period', value: '30 days from activation'),
      _DefinitionRow(label: 'Renewal', value: 'Every 30 days'),
      _DefinitionRow(label: 'Invoice trigger', value: 'Partner cycle boundary'),
      _DefinitionRow(label: 'Annual base-fee uplift', value: 'January 1'),
      _DefinitionRow(label: 'Default uplift', value: '10% · admin-overridable'),
      _DefinitionRow(label: 'Extra modules', value: 'Consolidated into main invoice'),
      _DefinitionRow(label: 'External payment provider', value: 'Not configured'),
    ],
  );
}

class CatalogModuleCard extends StatelessWidget {
  const CatalogModuleCard({required this.module, required this.onTap, super.key});
  final Map<String, dynamic> module;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final system = module['system'] == true;
    final availability = '${module['availability'] ?? 'ACTIVE'}';
    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(10),
        child: Padding(
          padding: const EdgeInsets.all(15),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Container(width: 36, height: 36, decoration: BoxDecoration(color: (system ? brandNavy : brandGold).withOpacity(.08), borderRadius: BorderRadius.circular(9)), child: Icon(system ? Icons.verified_outlined : Icons.extension_outlined, color: system ? brandNavy : brandGold, size: 18)),
              const Spacer(),
              _StatusPill(label: availability),
            ]),
            const SizedBox(height: 12),
            Text('${module['label']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 13)),
            const SizedBox(height: 4),
            Text('${module['group_label']}', style: const TextStyle(color: brandSteel, fontSize: 10, fontWeight: FontWeight.w600)),
            const SizedBox(height: 3),
            Text('${module['key']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.2)),
            const SizedBox(height: 12),
            Row(children: [
              Text('v${module['version'] ?? '1.0.0'} → ${module['latest_version'] ?? '1.0.0'}', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
              const Spacer(),
              Text(money(module['default_monthly_price']), style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w600, fontSize: 16)),
              const SizedBox(width: 7),
              const Icon(Icons.edit_outlined, color: brandGold, size: 15),
            ]),
          ]),
        ),
      ),
    );
  }
}

class _OperationsHero extends StatelessWidget {
  const _OperationsHero({required this.status, required this.environment, required this.version});
  final String status, environment, version;

  @override
  Widget build(BuildContext context) {
    final healthy = status.toLowerCase() == 'ok' || status.toLowerCase() == 'healthy';
    return Container(
      padding: const EdgeInsets.all(22),
      decoration: BoxDecoration(
        gradient: const LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [brandNavyDeep, brandNavy, brandNavySoft]),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: brandGold.withOpacity(.18)),
      ),
      child: LayoutBuilder(builder: (context, c) {
        final content = [
          BrandMark(size: 46),
          const SizedBox(width: 16),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(healthy ? 'Platform operational' : 'Platform requires attention', style: const TextStyle(color: brandWhite, fontSize: 24, fontWeight: FontWeight.w600)),
              const SizedBox(height: 5),
              Text('Environment: $environment · Version: $version', style: const TextStyle(color: Color(0xFFB8C6D6), fontSize: 11)),
            ]),
          ),
          _StatusPill(label: status),
        ];
        if (c.maxWidth < 620) {
          return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [content[0], content[1], content[2]]),
            const SizedBox(height: 14),
            content[3],
          ]);
        }
        return Row(children: content);
      }),
    );
  }
}

class _ArchitectureCard extends StatelessWidget {
  const _ArchitectureCard();

  @override
  Widget build(BuildContext context) => const _InfoCard(
    title: 'Architecture',
    icon: Icons.account_tree_outlined,
    children: [
      _DefinitionRow(label: 'Public ingress', value: 'HIMATE API Gateway'),
      _DefinitionRow(label: 'Identity boundary', value: 'Gateway session service'),
      _DefinitionRow(label: 'Partner domain', value: 'Independent Go service'),
      _DefinitionRow(label: 'Catalog domain', value: 'Independent Go service'),
      _DefinitionRow(label: 'Billing domain', value: 'Independent Go service'),
      _DefinitionRow(label: 'Persistence', value: 'PostgreSQL · service-owned schemas'),
    ],
  );
}

class _OperationsControlsCard extends StatelessWidget {
  const _OperationsControlsCard();

  @override
  Widget build(BuildContext context) => const _InfoCard(
    title: 'Operational Controls',
    icon: Icons.shield_outlined,
    children: [
      _DefinitionRow(label: 'Containerization', value: 'Enabled'),
      _DefinitionRow(label: 'Horizontal scaling', value: 'Stateless service design'),
      _DefinitionRow(label: 'Private services', value: 'Internal network only'),
      _DefinitionRow(label: 'Partner databases', value: 'Separate from HIMATE control plane'),
      _DefinitionRow(label: 'Connector model', value: 'Pre-defined API exchange'),
      _DefinitionRow(label: 'Backups / restore', value: 'Scheduled for START-21'),
    ],
  );
}

class Content extends StatelessWidget {
  const Content({required this.title, required this.subtitle, required this.child, this.actions = const [], this.eyebrow, super.key});
  final String title, subtitle;
  final String? eyebrow;
  final Widget child;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final narrow = constraints.maxWidth < 760;
        final padding = constraints.maxWidth < 520 ? 16.0 : constraints.maxWidth < 1050 ? 22.0 : 28.0;
        final header = Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (eyebrow != null) ...[
              Text(eyebrow!, style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: 17, fontWeight: FontWeight.w600)),
              const SizedBox(height: 2),
            ],
            Text(title, style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: narrow ? 36 : 42, fontWeight: FontWeight.w600, height: .98)),
            const SizedBox(height: 6),
            ConstrainedBox(constraints: const BoxConstraints(maxWidth: 760), child: Text(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 12.5, height: 1.45))),
          ],
        );

        return Scrollbar(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(padding, 24, padding, 40),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (narrow)
                  Column(crossAxisAlignment: CrossAxisAlignment.start, children: [header, if (actions.isNotEmpty) ...[const SizedBox(height: 16), Wrap(spacing: 9, runSpacing: 9, children: actions)]])
                else
                  Row(crossAxisAlignment: CrossAxisAlignment.start, children: [Expanded(child: header), if (actions.isNotEmpty) ...[const SizedBox(width: 20), Wrap(spacing: 9, runSpacing: 9, children: actions)]]),
                const SizedBox(height: 22),
                child,
              ],
            ),
          ),
        );
      },
    );
  }
}

class Kpi extends StatefulWidget {
  const Kpi({required this.label, required this.value, required this.note, this.icon = Icons.auto_graph_outlined, this.accent = brandNavy, super.key});
  final String label, value, note;
  final IconData icon;
  final Color accent;
  @override
  State<Kpi> createState() => _KpiState();
}

class _KpiState extends State<Kpi> {
  bool hover = false;
  @override
  Widget build(BuildContext context) => MouseRegion(
    onEnter: (_) => setState(() => hover = true),
    onExit: (_) => setState(() => hover = false),
    child: AnimatedContainer(
      duration: const Duration(milliseconds: 180),
      curve: Curves.easeOut,
      width: double.infinity,
      height: 132,
      transform: Matrix4.translationValues(0, hover ? -3 : 0, 0),
      decoration: BoxDecoration(
        color: brandWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: hover ? widget.accent.withOpacity(.24) : brandMist),
        boxShadow: [BoxShadow(color: brandNavy.withOpacity(hover ? .085 : .035), blurRadius: hover ? 22 : 12, offset: Offset(0, hover ? 9 : 5))],
      ),
      child: Padding(
        padding: const EdgeInsets.all(15),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [Icon(widget.icon, color: widget.accent, size: 22), const Spacer(), Container(width: 5, height: 5, decoration: BoxDecoration(color: widget.accent, shape: BoxShape.circle))]),
          const Spacer(),
          Text(widget.label, style: const TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w600)),
          const SizedBox(height: 2),
          FittedBox(fit: BoxFit.scaleDown, alignment: Alignment.centerLeft, child: Text(widget.value, style: const TextStyle(color: brandNavy, fontSize: 25, fontWeight: FontWeight.w600))),
          Text(widget.note, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.3)),
        ]),
      ),
    ),
  );
}

class ServiceCard extends StatelessWidget {
  const ServiceCard({required this.name, required this.status, super.key});
  final String name, status;

  @override
  Widget build(BuildContext context) {
    final ok = status.toLowerCase() == 'ok';
    final tone = ok ? brandSuccess : brandWarning;
    return SizedBox(
      width: 290,
      height: 118,
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(17),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Container(width: 36, height: 36, decoration: BoxDecoration(color: brandNavy.withOpacity(.055), borderRadius: BorderRadius.circular(9)), child: const Icon(Icons.dns_outlined, color: brandNavy, size: 19)),
              const Spacer(),
              Container(padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5), decoration: BoxDecoration(color: tone.withOpacity(.08), borderRadius: BorderRadius.circular(99)), child: Text(status.toUpperCase(), style: TextStyle(color: tone, fontSize: 9, fontWeight: FontWeight.w700))),
            ]),
            const Spacer(),
            Text(name, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14)),
            const SizedBox(height: 3),
            Text(ok ? 'Service responding normally' : 'Awaiting healthy response', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
          ]),
        ),
      ),
    );
  }
}

class _BrandLoading extends StatelessWidget {
  const _BrandLoading();
  @override
  Widget build(BuildContext context) => const Center(
    child: Column(mainAxisSize: MainAxisSize.min, children: [
      BrandMark(size: 42),
      SizedBox(height: 16),
      SizedBox(width: 26, height: 26, child: CircularProgressIndicator(strokeWidth: 2.1, color: brandGold)),
    ]),
  );
}

class _MessageCard extends StatelessWidget {
  const _MessageCard({required this.icon, required this.title, required this.message});
  final IconData icon;
  final String title, message;

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(24),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Container(width: 46, height: 46, decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(11)), child: Icon(icon, color: brandGold, size: 22)),
        const SizedBox(width: 15),
        Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 16)),
          const SizedBox(height: 6),
          Text(message, style: const TextStyle(color: brandTextSoft, height: 1.45)),
        ])),
      ]),
    ),
  );
}