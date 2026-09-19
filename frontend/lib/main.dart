import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/browser_client.dart';
import 'package:http/http.dart' as http;

void main() {
  WidgetsFlutterBinding.ensureInitialized();
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
  const serif = 'Georgia';
  const sans = 'Arial';
  final scheme = ColorScheme.fromSeed(
    seedColor: brandNavy,
    brightness: Brightness.light,
    primary: brandNavy,
    secondary: brandGold,
    surface: brandWhite,
    error: brandDanger,
  );
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: brandIvory,
    fontFamily: sans,
    visualDensity: VisualDensity.standard,
    splashFactory: InkSparkle.splashFactory,
    textTheme: const TextTheme(
      displaySmall: TextStyle(fontFamily: serif, color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.9, height: 1.05),
      headlineLarge: TextStyle(fontFamily: serif, color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.7, height: 1.08),
      headlineMedium: TextStyle(fontFamily: serif, color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.45, height: 1.12),
      headlineSmall: TextStyle(fontFamily: serif, color: brandNavy, fontWeight: FontWeight.w600, letterSpacing: -.25, height: 1.15),
      titleLarge: TextStyle(color: brandNavy, fontWeight: FontWeight.w700),
      titleMedium: TextStyle(color: brandNavy, fontWeight: FontWeight.w700),
      bodyLarge: TextStyle(color: brandCharcoal, height: 1.5),
      bodyMedium: TextStyle(color: brandCharcoal, height: 1.45),
      bodySmall: TextStyle(color: brandTextSoft, height: 1.4),
      labelLarge: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, letterSpacing: .1),
    ),
    cardTheme: CardThemeData(
      color: brandWhite,
      elevation: 0,
      margin: EdgeInsets.zero,
      shadowColor: brandNavy.withOpacity(.08),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: const BorderSide(color: brandMist),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: brandWhite,
      labelStyle: const TextStyle(color: brandTextSoft),
      hintStyle: const TextStyle(color: Color(0xFF98A2B3)),
      prefixIconColor: brandSteel,
      suffixIconColor: brandSteel,
      contentPadding: const EdgeInsets.symmetric(horizontal: 15, vertical: 15),
      enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandMist)),
      focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandSteel, width: 1.5)),
      errorBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandDanger)),
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandMist)),
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
        padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 20, vertical: 16)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(9))),
        textStyle: WidgetStateProperty.all(const TextStyle(fontWeight: FontWeight.w700)),
        elevation: WidgetStateProperty.all(0),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.hovered) ? brandSteel : brandNavy),
        side: WidgetStateProperty.resolveWith((states) => BorderSide(color: states.contains(WidgetState.hovered) ? brandSteel : brandMist)),
        overlayColor: WidgetStateProperty.all(brandSteel.withOpacity(.06)),
        padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 18, vertical: 15)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(9))),
        textStyle: WidgetStateProperty.all(const TextStyle(fontWeight: FontWeight.w700)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.hovered) ? brandSteel : brandNavy),
        overlayColor: WidgetStateProperty.all(brandSteel.withOpacity(.05)),
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
    appBarTheme: const AppBarTheme(
      backgroundColor: brandWhite,
      foregroundColor: brandNavy,
      elevation: 0,
      surfaceTintColor: Colors.transparent,
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

class Api {
  Api() : client = BrowserClient()..withCredentials = true;
  final BrowserClient client;

  Future<Map<String, dynamic>> get(String path) => request('GET', path);
  Future<Map<String, dynamic>> post(String path, [Map<String, dynamic>? body]) => request('POST', path, body);
  Future<Map<String, dynamic>> put(String path, Map<String, dynamic> body) => request('PUT', path, body);
  Future<Map<String, dynamic>> patch(String path, Map<String, dynamic> body) => request('PATCH', path, body);

  Future<Map<String, dynamic>> request(String method, String path, [Map<String, dynamic>? body]) async {
    final headers = <String, String>{'Accept': 'application/json'};
    if (body != null) headers['Content-Type'] = 'application/json';
    late http.Response response;
    final uri = Uri.parse(path);
    if (method == 'POST') {
      response = await client.post(uri, headers: headers, body: jsonEncode(body ?? <String, dynamic>{}));
    } else if (method == 'PUT') {
      response = await client.put(uri, headers: headers, body: jsonEncode(body));
    } else if (method == 'PATCH') {
      response = await client.patch(uri, headers: headers, body: jsonEncode(body));
    } else {
      response = await client.get(uri, headers: headers);
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
  Map<String, dynamic>? user;
  bool loading = true;
  @override
  void initState() { super.initState(); restore(); }
  Future<void> restore() async {
    try { user = await api.get('/api/v1/auth/me'); }
    on ApiError catch (e) { if (e.status != 401) rethrow; }
    finally { if (mounted) setState(() => loading = false); }
  }
  Future<void> login(String email, String password) async { user = await api.post('/api/v1/auth/login', {'email': email, 'password': password}); if (mounted) setState(() {}); }
  Future<void> logout() async { await api.post('/api/v1/auth/logout'); user = null; if (mounted) setState(() {}); }
  @override
  Widget build(BuildContext context) => MaterialApp(
    debugShowCheckedModeBanner: false,
    title: 'HIMATE System',
    theme: buildBrandTheme(),
    home: loading
      ? const Scaffold(backgroundColor: brandNavyDeep, body: Center(child: SizedBox(width: 30, height: 30, child: CircularProgressIndicator(strokeWidth: 2.5, color: brandGold))))
      : user == null ? LoginPage(onLogin: login) : Shell(api: api, user: user!, onLogout: logout),
  );
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
    setState(() {
      busy = true;
      error = null;
    });
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
      backgroundColor: brandIvory,
      body: LayoutBuilder(
        builder: (context, constraints) {
          final desktop = constraints.maxWidth >= 980;
          if (!desktop) {
            return _MobileLogin(
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
            );
          }
          return Row(
            children: [
              Expanded(
                flex: 47,
                child: _HeritagePanel(
                  child: SafeArea(
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(54, 42, 54, 44),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const HimateLogo(onDark: true, width: 245),
                          const Spacer(),
                          const Text(
                            'Culture\nConnects\nPeople',
                            style: TextStyle(fontFamily: 'Georgia', color: brandWhite, fontSize: 48, height: .98, fontWeight: FontWeight.w500, letterSpacing: -1.4),
                          ),
                          const SizedBox(height: 20),
                          const _LetterspacedLabel('BUILDING A BRIGHTER\nCULTURAL TOMORROW', color: Color(0xFFD9E2EC)),
                          const SizedBox(height: 34),
                          const _HeroValue(icon: Icons.groups_2_outlined, label: 'STRONGER COMMUNITIES'),
                          const SizedBox(height: 15),
                          const _HeroValue(icon: Icons.bar_chart_rounded, label: 'MORE OPPORTUNITIES'),
                          const SizedBox(height: 15),
                          const _HeroValue(icon: Icons.shield_outlined, label: 'GREATER IMPACT'),
                          const Spacer(),
                          const Row(
                            children: [
                              SizedBox(width: 34, child: Divider(color: brandGold, thickness: 1.4)),
                              SizedBox(width: 12),
                              _LetterspacedLabel('HERITAGE MEETS INNOVATION', color: Color(0xFFD9E2EC), fontSize: 9),
                            ],
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
              Expanded(
                flex: 53,
                child: Container(
                  color: brandIvory,
                  child: Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.symmetric(horizontal: 48, vertical: 40),
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 430),
                        child: _LoginCard(
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
                    ),
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _MobileLogin extends StatelessWidget {
  const _MobileLogin({
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

  final TextEditingController email;
  final TextEditingController password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot, onSso;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        const Positioned.fill(child: _HeritagePanel()),
        Positioned.fill(child: DecoratedBox(decoration: BoxDecoration(color: brandNavyDeep.withOpacity(.35)))),
        SafeArea(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(20, 24, 20, 32),
            child: Column(
              children: [
                const Align(alignment: Alignment.centerLeft, child: HimateLogo(onDark: true, width: 210)),
                const SizedBox(height: 46),
                ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 460),
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
              ],
            ),
          ),
        ),
      ],
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

  final TextEditingController email;
  final TextEditingController password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot, onSso;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(30, 32, 30, 26),
      decoration: BoxDecoration(
        color: brandWhite,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: brandMist),
        boxShadow: [BoxShadow(color: brandNavy.withOpacity(.10), blurRadius: 40, offset: const Offset(0, 18))],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Welcome back', textAlign: TextAlign.center, style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontSize: 27)),
          const SizedBox(height: 7),
          const Text('Sign in to your HIMATE System account', textAlign: TextAlign.center, style: TextStyle(color: brandTextSoft, fontSize: 13)),
          const SizedBox(height: 28),
          TextField(
            controller: email,
            keyboardType: TextInputType.emailAddress,
            autofillHints: const [AutofillHints.email],
            style: const TextStyle(color: brandCharcoal),
            decoration: const InputDecoration(hintText: 'Email address', prefixIcon: Icon(Icons.mail_outline_rounded, size: 19)),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: password,
            obscureText: obscure,
            autofillHints: const [AutofillHints.password],
            onSubmitted: (_) => onSubmit(),
            style: const TextStyle(color: brandCharcoal),
            decoration: InputDecoration(
              hintText: 'Password',
              prefixIcon: const Icon(Icons.lock_outline_rounded, size: 19),
              suffixIcon: IconButton(
                onPressed: onTogglePassword,
                tooltip: obscure ? 'Show password' : 'Hide password',
                icon: Icon(obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined, size: 19),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              SizedBox(
                height: 34,
                child: Row(
                  children: [
                    Checkbox(value: remember, onChanged: onRemember, activeColor: brandNavy, visualDensity: VisualDensity.compact),
                    const Text('Remember me', style: TextStyle(color: brandCharcoal, fontSize: 12)),
                  ],
                ),
              ),
              const Spacer(),
              TextButton(
                onPressed: onForgot,
                style: TextButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 5)),
                child: const Text('Forgot password?', style: TextStyle(fontSize: 12)),
              ),
            ],
          ),
          if (error != null) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(11),
              decoration: BoxDecoration(
                color: brandDanger.withOpacity(.06),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: brandDanger.withOpacity(.18)),
              ),
              child: Row(
                children: [
                  const Icon(Icons.error_outline_rounded, color: brandDanger, size: 18),
                  const SizedBox(width: 8),
                  Expanded(child: Text(error!, style: const TextStyle(color: brandDanger, fontSize: 12))),
                ],
              ),
            ),
          ],
          const SizedBox(height: 15),
          FilledButton(
            onPressed: busy ? null : onSubmit,
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(50)),
            child: AnimatedSwitcher(
              duration: const Duration(milliseconds: 180),
              child: busy
                  ? const SizedBox(key: ValueKey('busy'), width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2.1, color: brandWhite))
                  : const Row(
                      key: ValueKey('ready'),
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [Text('Sign in'), SizedBox(width: 12), Icon(Icons.arrow_forward_rounded, size: 18)],
                    ),
            ),
          ),
          const SizedBox(height: 18),
          const Row(
            children: [
              Expanded(child: Divider(color: brandMist)),
              Padding(padding: EdgeInsets.symmetric(horizontal: 12), child: Text('or continue with', style: TextStyle(color: brandTextSoft, fontSize: 11))),
              Expanded(child: Divider(color: brandMist)),
            ],
          ),
          const SizedBox(height: 14),
          OutlinedButton.icon(onPressed: onSso, icon: const Icon(Icons.account_balance_outlined, size: 18), label: const Text('Sign in with SSO')),
          const SizedBox(height: 24),
          const Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.verified_user_outlined, color: brandGold, size: 17),
              SizedBox(width: 7),
              Text('Secure', style: TextStyle(color: brandTextSoft, fontSize: 10.5)),
              Padding(padding: EdgeInsets.symmetric(horizontal: 7), child: Text('•', style: TextStyle(color: brandMist))),
              Text('Trusted', style: TextStyle(color: brandTextSoft, fontSize: 10.5)),
              Padding(padding: EdgeInsets.symmetric(horizontal: 7), child: Text('•', style: TextStyle(color: brandMist))),
              Flexible(child: Text('Built for a brighter tomorrow', overflow: TextOverflow.ellipsis, style: TextStyle(color: brandTextSoft, fontSize: 10.5))),
            ],
          ),
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
    final textColor = onDark ? const Color(0xFFF3E3BD) : brandNavy;
    if (compact) return SizedBox(width: width, child: const BrandMark(size: 38));
    return SizedBox(
      width: width,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          BrandMark(size: width * .22),
          SizedBox(width: width * .055),
          Flexible(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                FittedBox(
                  fit: BoxFit.scaleDown,
                  child: Text(
                    'HIMATE',
                    style: TextStyle(
                      fontFamily: 'Georgia',
                      color: textColor,
                      fontWeight: FontWeight.w600,
                      fontSize: width * .175,
                      letterSpacing: -1,
                      shadows: onDark ? const [Shadow(color: Colors.black45, blurRadius: 6, offset: Offset(0, 2))] : const [],
                    ),
                  ),
                ),
                Text(
                  'S Y S T E M',
                  style: TextStyle(
                    color: onDark ? brandWhite : brandNavy,
                    fontWeight: FontWeight.w600,
                    fontSize: width * .053,
                    letterSpacing: width * .017,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class BrandMark extends StatelessWidget {
  const BrandMark({this.size = 34, super.key});
  final double size;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: size,
      height: size,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          for (final factor in const [.42, .58, .76, 1.0])
            Container(
              width: size * .17,
              height: size * factor,
              decoration: BoxDecoration(
                gradient: const LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color(0xFFFFE3A7), brandGold, Color(0xFF9A6C25)]),
                borderRadius: BorderRadius.circular(size * .055),
                border: Border.all(color: const Color(0xFFB6812E), width: size * .02),
                boxShadow: [BoxShadow(color: Colors.black.withOpacity(.20), blurRadius: size * .07, offset: Offset(0, size * .035))],
              ),
            ),
        ],
      ),
    );
  }
}

class _HeroValue extends StatelessWidget {
  const _HeroValue({required this.icon, required this.label});
  final IconData icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 36,
          height: 36,
          decoration: BoxDecoration(border: Border.all(color: brandGold.withOpacity(.9)), shape: BoxShape.circle, color: brandNavy.withOpacity(.38)),
          child: Icon(icon, color: brandGold, size: 18),
        ),
        const SizedBox(width: 13),
        _LetterspacedLabel(label, color: const Color(0xFFE8EDF3), fontSize: 9.5),
      ],
    );
  }
}

class _LetterspacedLabel extends StatelessWidget {
  const _LetterspacedLabel(this.text, {required this.color, this.fontSize = 10});
  final String text;
  final Color color;
  final double fontSize;

  @override
  Widget build(BuildContext context) {
    return Text(text, style: TextStyle(color: color, fontSize: fontSize, fontWeight: FontWeight.w600, letterSpacing: 2.6, height: 1.65));
  }
}

class _HeritagePanel extends StatelessWidget {
  const _HeritagePanel({this.child});
  final Widget? child;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        const DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color(0xFF061426), brandNavy, Color(0xFF123859)]),
          ),
        ),
        CustomPaint(painter: _HeritagePainter()),
        if (child != null) child!,
      ],
    );
  }
}

class _HeritagePainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final haze = Paint()
      ..shader = const LinearGradient(
        begin: Alignment.centerLeft,
        end: Alignment.centerRight,
        colors: [Colors.transparent, Color(0x143F6D95), Color(0x337B674B)],
      ).createShader(Rect.fromLTWH(size.width * .42, 0, size.width * .58, size.height));
    canvas.drawRect(Rect.fromLTWH(size.width * .42, 0, size.width * .58, size.height), haze);

    final columnPaint = Paint()
      ..shader = LinearGradient(
        colors: [const Color(0xFF1A3550).withOpacity(.18), const Color(0xFFC7B189).withOpacity(.34), const Color(0xFFF2E6CE).withOpacity(.45)],
      ).createShader(Rect.fromLTWH(size.width * .56, 0, size.width * .44, size.height));
    for (var i = 0; i < 4; i++) {
      final x = size.width * (.61 + i * .105);
      final w = size.width * .052;
      canvas.drawRRect(RRect.fromRectAndRadius(Rect.fromLTWH(x, size.height * .22, w, size.height * .68), const Radius.circular(5)), columnPaint);
      canvas.drawRect(Rect.fromLTWH(x - w * .18, size.height * .21, w * 1.36, size.height * .026), columnPaint);
      canvas.drawRect(Rect.fromLTWH(x - w * .20, size.height * .89, w * 1.40, size.height * .026), columnPaint);
    }

    final blueArc = Paint()..style = PaintingStyle.stroke..strokeWidth = 1.1..color = const Color(0xFF3B78B8).withOpacity(.62);
    final goldArc = Paint()..style = PaintingStyle.stroke..strokeWidth = 1.4..color = brandGold.withOpacity(.92);
    final p1 = Path()..moveTo(size.width * .26, -10)..cubicTo(size.width * .62, size.height * .18, size.width * .36, size.height * .52, size.width * .88, size.height * .34);
    canvas.drawPath(p1, blueArc);
    final p2 = Path()..moveTo(size.width * .12, size.height * .78)..cubicTo(size.width * .45, size.height * .64, size.width * .56, size.height * .32, size.width * 1.04, size.height * .24);
    canvas.drawPath(p2, goldArc);
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

class NavSpec {
  const NavSpec(this.label, this.icon, this.subtitle);
  final String label;
  final IconData icon;
  final String subtitle;
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
      case 3: return const PlannedPage(title: 'Impact & Reports', subtitle: 'Metrics and partner impact become functional in START-13–15.', icon: Icons.show_chart_rounded);
      case 4: return const PlannedPage(title: 'Website & Marketing', subtitle: 'HIMATE CMS and public marketing tools are planned for START-16–17.', icon: Icons.campaign_outlined);
      case 5: return SystemPage(api: widget.api);
      default: return const PlannedPage(title: 'Administration', subtitle: 'Roles, permissions and advanced audit controls are planned for START-18–19.', icon: Icons.admin_panel_settings_outlined);
    }
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final desktop = constraints.maxWidth >= 980;
        if (!desktop) {
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
                width: collapsed ? 82 : 248,
                decoration: const BoxDecoration(
                  gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color(0xFF061426), brandNavy, Color(0xFF0A2C4C)]),
                ),
                child: SafeArea(
                  child: _SidebarContent(
                    nav: nav,
                    selected: selected,
                    collapsed: collapsed,
                    user: widget.user,
                    onSelect: (i) => setState(() => selected = i),
                    onToggle: () => setState(() => collapsed = !collapsed),
                    onLogout: widget.onLogout,
                  ),
                ),
              ),
              Expanded(
                child: Column(
                  children: [
                    Container(
                      height: 68,
                      padding: const EdgeInsets.symmetric(horizontal: 24),
                      decoration: const BoxDecoration(color: brandWhite, border: Border(bottom: BorderSide(color: brandMist))),
                      child: Row(
                        children: [
                          Expanded(
                            child: Align(
                              alignment: Alignment.centerLeft,
                              child: ConstrainedBox(
                                constraints: const BoxConstraints(maxWidth: 370),
                                child: TextField(
                                  readOnly: true,
                                  onTap: () => ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(content: Text('Global search will be activated in a later functional cycle.'), behavior: SnackBarBehavior.floating),
                                  ),
                                  decoration: const InputDecoration(isDense: true, hintText: 'Search anywhere...', prefixIcon: Icon(Icons.search_rounded, size: 19)),
                                ),
                              ),
                            ),
                          ),
                          const SizedBox(width: 18),
                          _TopIconButton(icon: Icons.notifications_none_rounded, hasDot: true, onTap: () {}),
                          const SizedBox(width: 8),
                          PopupMenuButton<String>(
                            tooltip: 'Admin account',
                            onSelected: (value) { if (value == 'logout') widget.onLogout(); },
                            itemBuilder: (_) => const [
                              PopupMenuItem(value: 'logout', child: Row(children: [Icon(Icons.logout_rounded, size: 18), SizedBox(width: 10), Text('Sign out')])),
                            ],
                            child: Row(
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
          height: 116,
          child: Stack(
            alignment: Alignment.center,
            children: [
              Padding(
                padding: EdgeInsets.symmetric(horizontal: collapsed ? 16 : 18),
                child: HimateLogo(onDark: true, compact: collapsed, width: collapsed ? 38 : 180),
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
                            Text('${user['name'] ?? 'Admin User'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandWhite, fontWeight: FontWeight.w700, fontSize: 11.5)),
                            const SizedBox(height: 2),
                            Text('${user['email'] ?? 'System Administrator'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: Color(0xFF91A4B8), fontSize: 9.5)),
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
                constraints: const BoxConstraints(minHeight: 48),
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
                              style: TextStyle(color: active ? const Color(0xFFF2D79F) : const Color(0xFFD9E2EC), fontWeight: active ? FontWeight.w700 : FontWeight.w500, fontSize: 12.2),
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
          return Content(
            title: 'Welcome to HIMATE System',
            subtitle: 'Manage partners, programs and cultural impact — all in one place.',
            child: _MessageCard(
              icon: Icons.cloud_off_outlined,
              title: 'Dashboard data is temporarily unavailable',
              message: '${snapshot.error}',
            ),
          );
        }

        final d = snapshot.data ?? <String, dynamic>{};
        final p = Map<String, dynamic>.from(d['partners'] ?? <String, dynamic>{});
        final m = Map<String, dynamic>.from(d['modules'] ?? <String, dynamic>{});
        final sys = Map<String, dynamic>.from(d['system'] ?? <String, dynamic>{});
        final hour = DateTime.now().hour;
        final greeting = hour < 12 ? 'Good morning,' : hour < 18 ? 'Good afternoon,' : 'Good evening,';

        return Content(
          eyebrow: greeting,
          title: 'Welcome to HIMATE System',
          subtitle: 'Manage partners, modules and cultural impact — all in one place.',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Wrap(
                spacing: 14,
                runSpacing: 14,
                children: [
                  Kpi(label: 'Active Partners', value: '${p['live'] ?? 0}', note: '${p['total'] ?? 0} partner records', icon: Icons.groups_2_outlined, accent: const Color(0xFF0B5DA8)),
                  Kpi(label: 'Module Catalog', value: '${m['catalog_total'] ?? 0}', note: 'Reference + custom modules', icon: Icons.description_outlined, accent: brandNavy),
                  Kpi(label: 'Architecture', value: 'MICRO', note: '${sys['architecture'] ?? 'microservices'}', icon: Icons.bar_chart_rounded, accent: brandGold),
                  Kpi(label: 'System Status', value: '${sys['status'] ?? 'unknown'}'.toUpperCase(), note: '${sys['version'] ?? ''}', icon: Icons.verified_user_outlined, accent: brandSuccess),
                ],
              ),
              const SizedBox(height: 18),
              LayoutBuilder(
                builder: (context, constraints) {
                  if (constraints.maxWidth < 920) {
                    return const Column(children: [_ImpactPanel(), SizedBox(height: 16), _ActivityPanel()]);
                  }
                  return const Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(flex: 7, child: _ImpactPanel()),
                      SizedBox(width: 16),
                      Expanded(flex: 4, child: _ActivityPanel()),
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

class _ImpactPanel extends StatelessWidget {
  const _ImpactPanel();

  @override
  Widget build(BuildContext context) => SizedBox(
    height: 290,
    child: Card(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(18, 17, 18, 15),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(children: [
              const Expanded(child: Text('Program Impact', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 15))),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
                decoration: BoxDecoration(border: Border.all(color: brandMist), borderRadius: BorderRadius.circular(8)),
                child: const Row(children: [Text('START-13', style: TextStyle(color: brandTextSoft, fontSize: 10.5)), SizedBox(width: 4), Icon(Icons.keyboard_arrow_down_rounded, size: 16, color: brandTextSoft)]),
              ),
            ]),
            const SizedBox(height: 12),
            const Expanded(child: _ImpactChart()),
            const SizedBox(height: 8),
            const Text('Impact metrics will populate from verified partner data in START-13.', style: TextStyle(color: brandTextSoft, fontSize: 10.5)),
          ],
        ),
      ),
    ),
  );
}

class _ImpactChart extends StatelessWidget {
  const _ImpactChart();
  @override
  Widget build(BuildContext context) => CustomPaint(painter: _ImpactChartPainter(), child: const SizedBox.expand());
}

class _ImpactChartPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final grid = Paint()..color = brandMist.withOpacity(.85)..strokeWidth = 1;
    for (var i = 1; i < 6; i++) {
      final y = size.height * i / 6;
      canvas.drawLine(Offset(0, y), Offset(size.width, y), grid);
    }
    for (var i = 1; i < 12; i++) {
      final x = size.width * i / 12;
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), grid);
    }

    final values = <double>[.16, .25, .22, .34, .46, .39, .51, .56, .50, .67, .72, .69, .82];
    final path = Path();
    for (var i = 0; i < values.length; i++) {
      final x = size.width * i / (values.length - 1);
      final y = size.height * (1 - values[i]);
      if (i == 0) path.moveTo(x, y); else path.lineTo(x, y);
    }
    canvas.drawPath(
      path,
      Paint()
        ..color = brandNavy
        ..strokeWidth = 2.1
        ..style = PaintingStyle.stroke
        ..strokeCap = StrokeCap.round
        ..strokeJoin = StrokeJoin.round,
    );
    final dot = Paint()..color = brandNavy;
    for (var i = 0; i < values.length; i++) {
      canvas.drawCircle(Offset(size.width * i / (values.length - 1), size.height * (1 - values[i])), 2.6, dot);
    }
  }
  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

class _ActivityPanel extends StatelessWidget {
  const _ActivityPanel();

  @override
  Widget build(BuildContext context) => SizedBox(
    height: 290,
    child: Card(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(18, 17, 18, 15),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Row(children: [
              Expanded(child: Text('Recent Activity', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 15))),
              Text('Audit-ready', style: TextStyle(color: brandSteel, fontSize: 10.5, fontWeight: FontWeight.w600)),
            ]),
            const SizedBox(height: 14),
            const _ActivityRow(icon: Icons.person_add_alt_1_outlined, title: 'Partner activity', subtitle: 'Will appear from audited partner events', tone: Color(0xFF1D6FC2)),
            const Divider(height: 17),
            const _ActivityRow(icon: Icons.description_outlined, title: 'Module updates', subtitle: 'Catalog changes will be recorded here', tone: brandGold),
            const Divider(height: 17),
            const _ActivityRow(icon: Icons.payments_outlined, title: 'Billing events', subtitle: 'Invoice lifecycle events are prepared', tone: brandSuccess),
            const Spacer(),
            const Text('The activity feed becomes authoritative when the audit service is implemented.', style: TextStyle(color: brandTextSoft, fontSize: 10.2, height: 1.4)),
          ],
        ),
      ),
    ),
  );
}

class _ActivityRow extends StatelessWidget {
  const _ActivityRow({required this.icon, required this.title, required this.subtitle, required this.tone});
  final IconData icon;
  final String title, subtitle;
  final Color tone;

  @override
  Widget build(BuildContext context) => Row(
    children: [
      Container(width: 34, height: 34, decoration: BoxDecoration(color: tone.withOpacity(.10), shape: BoxShape.circle), child: Icon(icon, color: tone, size: 17)),
      const SizedBox(width: 10),
      Expanded(
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text(title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w600, fontSize: 11.5)),
          const SizedBox(height: 2),
          Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
        ]),
      ),
    ],
  );
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
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final r = await Future.wait([
        widget.api.get('/api/v1/partners'),
        widget.api.get('/api/v1/partner-categories'),
      ]);
      partners = items(r[0]);
      categories = items(r[1]);
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
              Row(
                children: [
                  Expanded(child: TextField(controller: displayName, decoration: const InputDecoration(labelText: 'Display name *'))),
                  const SizedBox(width: 12),
                  Expanded(child: TextField(controller: legalName, decoration: const InputDecoration(labelText: 'Legal name'))),
                ],
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
              Row(
                children: [
                  Expanded(child: TextField(controller: contactName, decoration: const InputDecoration(labelText: 'Primary contact'))),
                  const SizedBox(width: 12),
                  Expanded(child: TextField(controller: contactEmail, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Contact email'))),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(child: TextField(controller: country, decoration: const InputDecoration(labelText: 'Country'))),
                  const SizedBox(width: 12),
                  Expanded(child: TextField(controller: primaryDomain, decoration: const InputDecoration(labelText: 'Primary domain', hintText: 'example.org'))),
                ],
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
          MaterialPageRoute(builder: (_) => PartnerWorkspace(api: widget.api, partner: created)),
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

  List<Map<String, dynamic>> get filtered {
    final q = query.trim().toLowerCase();
    return partners.where((p) {
      final searchOk = q.isEmpty ||
          '${p['display_name']}'.toLowerCase().contains(q) ||
          '${p['legal_name']}'.toLowerCase().contains(q) ||
          '${p['category_name']}'.toLowerCase().contains(q) ||
          '${p['id']}'.toLowerCase().contains(q);
      final categoryOk = categoryFilter == 'ALL' || '${p['category_id']}' == categoryFilter;
      final lifecycleOk = lifecycleFilter == 'ALL' || '${p['lifecycle']}' == lifecycleFilter;
      return searchOk && categoryOk && lifecycleOk;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    final live = partners.where((p) => p['lifecycle'] == 'LIVE').length;
    final prospects = partners.where((p) => p['lifecycle'] == 'PROSPECT').length;
    final reference = partners.where((p) => p['reference_partner'] == true).length;

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
                        Kpi(label: 'Partner records', value: '${partners.length}', note: 'All lifecycle states', icon: Icons.apartment_outlined, accent: brandNavy),
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
                            onChanged: (v) => setState(() => query = v),
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
                            onChanged: (v) => setState(() => categoryFilter = v ?? 'ALL'),
                          );
                          final lifecycle = DropdownButtonFormField<String>(
                            value: lifecycleFilter,
                            decoration: const InputDecoration(labelText: 'Lifecycle'),
                            items: [
                              const DropdownMenuItem(value: 'ALL', child: Text('All lifecycle states')),
                              for (final state in lifecycleOptions) DropdownMenuItem(value: state, child: Text(_humanize(state))),
                            ],
                            onChanged: (v) => setState(() => lifecycleFilter = v ?? 'ALL'),
                          );
                          if (compact) {
                            return Column(children: [search, const SizedBox(height: 10), category, const SizedBox(height: 10), lifecycle]);
                          }
                          return Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: category), const SizedBox(width: 10), Expanded(child: lifecycle)]);
                        },
                      ),
                    ),
                    const SizedBox(height: 20),
                    Row(
                      children: [
                        Text('Partner portfolio', style: Theme.of(context).textTheme.titleLarge),
                        const SizedBox(width: 10),
                        _MiniCounter(label: '${filtered.length} shown'),
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
                                  MaterialPageRoute(builder: (_) => PartnerWorkspace(api: widget.api, partner: p)),
                                ),
                              ),
                            ),
                          SizedBox(width: width, child: NewPartnerCard(onTap: addPartner)),
                        ];
                        return Wrap(spacing: 14, runSpacing: 14, children: cards);
                      },
                    ),
                  ],
                ),
    );
  }
}

class PartnerWorkspace extends StatefulWidget {
  const PartnerWorkspace({required this.api, required this.partner, super.key});
  final Api api;
  final Map<String, dynamic> partner;

  @override
  State<PartnerWorkspace> createState() => _PartnerWorkspaceState();
}

class _PartnerWorkspaceState extends State<PartnerWorkspace> {
  late Map<String, dynamic> partner;
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> documents = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> invoices = <Map<String, dynamic>>[];
  Map<String, dynamic>? billing;
  Map<String, dynamic>? terms;
  bool loading = true;
  String? error;
  String moduleQuery = '';
  String moduleState = 'ALL';

  static const workspaceCards = <_WorkspaceSpec>[
    _WorkspaceSpec('Overview', Icons.dashboard_customize_outlined, 'Partner health and commercial snapshot', true),
    _WorkspaceSpec('Company Data', Icons.apartment_outlined, 'Legal identity, contacts and lifecycle', true),
    _WorkspaceSpec('System & Environment', Icons.dns_outlined, 'Domains and deployment environment', true),
    _WorkspaceSpec('Modules', Icons.grid_view_outlined, 'Entitlements, visibility and pricing', true),
    _WorkspaceSpec('Pricing & Subscription', Icons.payments_outlined, 'Activation fee and recurring terms', true),
    _WorkspaceSpec('Finance & Documents', Icons.folder_copy_outlined, 'Invoices and commercial evidence', true),
    _WorkspaceSpec('Statistics', Icons.insights_outlined, 'Partner performance metrics', false),
    _WorkspaceSpec('Evidence', Icons.verified_outlined, 'Impact evidence library', false),
    _WorkspaceSpec('Branding & Website', Icons.palette_outlined, 'Partner-facing design and CMS', false),
    _WorkspaceSpec('Users & Contacts', Icons.group_outlined, 'Partner administrators and contacts', false),
    _WorkspaceSpec('Integrations', Icons.hub_outlined, 'Connector and provider registry', false),
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
        widget.api.get('/api/v1/billing/partners/$id/documents'),
        widget.api.get('/api/v1/billing/partners/$id/invoices'),
      ]);
      partner = r[0];
      modules = items(r[1]);
      billing = r[2];
      terms = r[3];
      documents = items(r[4]);
      invoices = items(r[5]);
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

  Future<void> editPartner() async {
    final display = TextEditingController(text: '${partner['display_name'] ?? ''}');
    final legal = TextEditingController(text: '${partner['legal_name'] ?? ''}');
    final contact = TextEditingController(text: '${partner['contact_name'] ?? ''}');
    final email = TextEditingController(text: '${partner['contact_email'] ?? ''}');
    final country = TextEditingController(text: '${partner['country'] ?? ''}');
    final primary = TextEditingController(text: '${partner['primary_domain'] ?? ''}');
    final staging = TextEditingController(text: '${partner['staging_domain'] ?? ''}');
    final notes = TextEditingController(text: '${partner['notes'] ?? ''}');
    String lifecycle = '${partner['lifecycle'] ?? 'PROSPECT'}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Company Data',
          subtitle: 'Edit partner identity, lifecycle, contacts and environment references.',
          icon: Icons.apartment_outlined,
          width: 720,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(children: [
                Expanded(child: TextField(controller: display, decoration: const InputDecoration(labelText: 'Display name'))),
                const SizedBox(width: 12),
                Expanded(child: TextField(controller: legal, decoration: const InputDecoration(labelText: 'Legal name'))),
              ]),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: lifecycle,
                decoration: const InputDecoration(labelText: 'Lifecycle'),
                items: [
                  for (final value in _PartnersPageState.lifecycleOptions)
                    DropdownMenuItem(value: value, child: Text(_humanize(value))),
                ],
                onChanged: (v) { if (v != null) setLocal(() => lifecycle = v); },
              ),
              const SizedBox(height: 12),
              Row(children: [
                Expanded(child: TextField(controller: contact, decoration: const InputDecoration(labelText: 'Primary contact'))),
                const SizedBox(width: 12),
                Expanded(child: TextField(controller: email, decoration: const InputDecoration(labelText: 'Contact email'))),
              ]),
              const SizedBox(height: 12),
              Row(children: [
                Expanded(child: TextField(controller: country, decoration: const InputDecoration(labelText: 'Country'))),
                const SizedBox(width: 12),
                Expanded(child: TextField(controller: primary, decoration: const InputDecoration(labelText: 'Primary domain'))),
              ]),
              const SizedBox(height: 12),
              TextField(controller: staging, decoration: const InputDecoration(labelText: 'Staging domain')),
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
        'lifecycle': lifecycle,
        'contact_name': contact.text.trim(),
        'contact_email': email.text.trim(),
        'country': country.text.trim(),
        'primary_domain': primary.text.trim(),
        'staging_domain': staging.text.trim(),
        'notes': notes.text.trim(),
      });
      await load();
      if (mounted) success('Partner data updated.');
    }
    for (final c in [display, legal, contact, email, country, primary, staging, notes]) {
      c.dispose();
    }
  }

  Future<void> editTerms() async {
    final activation = TextEditingController(text: number(terms?['activation_fee']).toStringAsFixed(2));
    final base = TextEditingController(text: number(terms?['base_monthly_fee']).toStringAsFixed(2));
    final uplift = TextEditingController(text: number(terms?['annual_increase_percent']).toStringAsFixed(2));
    final effective = TextEditingController(text: '${terms?['price_effective_from'] ?? ''}');
    final reason = TextEditingController(text: '${terms?['activation_fee_reason'] ?? ''}');
    bool waived = terms?['activation_fee_waived'] == true;
    String currency = '${terms?['currency'] ?? 'USD'}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Pricing & Subscription',
          subtitle: 'Commercial terms remain partner-specific while invoice day and service-cycle rules stay standardized.',
          icon: Icons.payments_outlined,
          width: 700,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    value: currency,
                    decoration: const InputDecoration(labelText: 'Currency'),
                    items: const [
                      DropdownMenuItem(value: 'USD', child: Text('USD')),
                      DropdownMenuItem(value: 'EUR', child: Text('EUR')),
                      DropdownMenuItem(value: 'GBP', child: Text('GBP')),
                    ],
                    onChanged: (v) { if (v != null) setLocal(() => currency = v); },
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(child: TextField(controller: activation, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: 'Activation fee'))),
              ]),
              const SizedBox(height: 8),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: waived,
                onChanged: (v) => setLocal(() => waived = v),
                title: const Text('Activation fee waived'),
                subtitle: const Text('Use for an existing/reference partner where no activation transaction applies.'),
              ),
              if (waived) ...[
                const SizedBox(height: 8),
                TextField(controller: reason, decoration: const InputDecoration(labelText: 'Waiver reason')),
              ],
              const SizedBox(height: 12),
              Row(children: [
                Expanded(child: TextField(controller: base, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: 'Base monthly fee'))),
                const SizedBox(width: 12),
                Expanded(child: TextField(controller: uplift, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: 'Annual increase %'))),
              ]),
              const SizedBox(height: 12),
              TextField(controller: effective, decoration: const InputDecoration(labelText: 'Price effective from', hintText: 'YYYY-MM-DD')),
              const SizedBox(height: 12),
              const _RuleStrip(
                items: [
                  _RuleItem(Icons.calendar_today_outlined, 'Invoice day', '1st of each month'),
                  _RuleItem(Icons.timelapse_outlined, 'Service cycle', '30 days'),
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
      await widget.api.put('/api/v1/billing/partners/${partner['id']}/terms', {
        'currency': currency,
        'activation_fee': double.tryParse(activation.text) ?? 0,
        'activation_fee_waived': waived,
        'activation_fee_reason': reason.text.trim(),
        'base_monthly_fee': double.tryParse(base.text) ?? 0,
        'annual_increase_percent': double.tryParse(uplift.text) ?? 10,
        'price_effective_from': effective.text.trim(),
      });
      await load();
      if (mounted) success('Commercial terms updated.');
    }

    for (final c in [activation, base, uplift, effective, reason]) {
      c.dispose();
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
          subtitle: 'Attach commercial metadata now; binary evidence storage will be connected in a later evidence cycle.',
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
                  DropdownMenuItem(value: 'PAYMENT_EVIDENCE', child: Text('Payment evidence')),
                  DropdownMenuItem(value: 'OTHER', child: Text('Other')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => kind = v); },
              ),
              const SizedBox(height: 12),
              TextField(controller: name, decoration: const InputDecoration(labelText: 'Document name *')),
              const SizedBox(height: 12),
              TextField(controller: url, decoration: const InputDecoration(labelText: 'Storage URL / reference')),
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

  Future<void> editModule(Map<String, dynamic> module) async {
    String state = '${module['status']}';
    bool visible = module['visible'] == true;
    bool included = module['included_in_base'] == true;
    final price = TextEditingController(text: number(module['partner_price']).toStringAsFixed(2));

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: '${module['label']}',
          subtitle: 'Control entitlement, partner visibility and monthly pricing without removing the underlying module code or data.',
          icon: Icons.grid_view_outlined,
          width: 650,
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
              const SizedBox(height: 8),
              TextField(
                controller: price,
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                decoration: const InputDecoration(labelText: 'Partner monthly price (USD)'),
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
        '/api/v1/partners/${partner['id']}/modules/${module['key']}',
        {
          'status': state,
          'visible': visible,
          'included_in_base': included,
          'partner_price': double.tryParse(price.text) ?? 0,
          'reason': 'HIMATE admin update',
        },
      );
      await load();
      if (mounted) success('Module configuration updated.');
    }
    price.dispose();
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
        title: const HimateLogo(width: 170),
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
                  subtitle: '${partner['category_name']} · ${partner['country']} · ${_humanize('${partner['lifecycle']}')}',
                  actions: [
                    OutlinedButton.icon(onPressed: editPartner, icon: const Icon(Icons.edit_outlined), label: const Text('Company data')),
                    FilledButton.icon(onPressed: editTerms, icon: const Icon(Icons.payments_outlined), label: const Text('Commercial terms')),
                  ],
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: 12,
                        runSpacing: 12,
                        children: [
                          Kpi(label: 'Current recurring', value: money(billing?['current_total']), note: 'Base + active extra modules', icon: Icons.account_balance_wallet_outlined, accent: brandGold),
                          Kpi(label: 'Active modules', value: '$active', note: '${modules.length} module records', icon: Icons.grid_view_outlined, accent: brandNavy),
                          Kpi(label: 'Base package', value: '$baseIncluded', note: 'Included module entitlements', icon: Icons.inventory_2_outlined, accent: brandSteel),
                          Kpi(label: 'Maintenance', value: '$maintenance', note: 'Temporarily restricted modules', icon: Icons.build_outlined, accent: brandWarning),
                        ],
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
                                SizedBox(width: width, child: WorkspaceCard(spec: spec)),
                            ],
                          );
                        },
                      ),
                      const SizedBox(height: 26),
                      LayoutBuilder(
                        builder: (context, c) {
                          final company = _PartnerDetailsCard(partner: partner);
                          final termsCard = _CommercialSummaryCard(terms: terms ?? {}, billing: billing ?? {}, onEdit: editTerms);
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
                      _SectionHeader(
                        title: 'Partner Modules',
                        subtitle: 'Entitlement, visibility, base-package inclusion and partner-specific pricing.',
                        trailing: _MiniCounter(label: '${filteredModules.length} shown'),
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
                      _SectionHeader(
                        title: 'Finance & Documents',
                        subtitle: 'Commercial evidence and internal invoice records for this partner.',
                        trailing: FilledButton.icon(onPressed: addDocument, icon: const Icon(Icons.note_add_outlined), label: const Text('Register document')),
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
    final address = TextEditingController(text: '${profile?['address'] ?? ''}');
    final tax = TextEditingController(text: '${profile?['tax_id'] ?? ''}');
    final email = TextEditingController(text: '${profile?['email'] ?? ''}');
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
            Row(children: [
              Expanded(child: TextField(controller: legal, decoration: const InputDecoration(labelText: 'Legal name'))),
              const SizedBox(width: 12),
              Expanded(child: TextField(controller: tax, decoration: const InputDecoration(labelText: 'Tax ID'))),
            ]),
            const SizedBox(height: 12),
            TextField(controller: address, decoration: const InputDecoration(labelText: 'Company address')),
            const SizedBox(height: 12),
            TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Billing email')),
            const SizedBox(height: 18),
            const _DialogSectionLabel('BANKING DETAILS'),
            const SizedBox(height: 10),
            Row(children: [
              Expanded(child: TextField(controller: bank, decoration: const InputDecoration(labelText: 'Bank name'))),
              const SizedBox(width: 12),
              Expanded(child: TextField(controller: bankAddress, decoration: const InputDecoration(labelText: 'Bank address'))),
            ]),
            const SizedBox(height: 12),
            TextField(controller: account, decoration: const InputDecoration(labelText: 'Account number')),
            const SizedBox(height: 12),
            Row(children: [
              Expanded(child: TextField(controller: iban, decoration: const InputDecoration(labelText: 'IBAN'))),
              const SizedBox(width: 12),
              Expanded(child: TextField(controller: swift, decoration: const InputDecoration(labelText: 'SWIFT / BIC'))),
            ]),
          ],
        ),
        primaryLabel: 'Save billing profile',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );

    if (ok == true) {
      await widget.api.put('/api/v1/billing/profile', {
        'legal_name': legal.text.trim(),
        'address': address.text.trim(),
        'tax_id': tax.text.trim(),
        'email': email.text.trim(),
        'bank_name': bank.text.trim(),
        'bank_address': bankAddress.text.trim(),
        'account_number': account.text.trim(),
        'iban': iban.text.trim(),
        'swift': swift.text.trim(),
      });
      await load();
      if (mounted) success('Billing profile updated.');
    }

    for (final c in [legal, address, tax, email, bank, bankAddress, account, iban, swift]) {
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
              Row(children: [
                Expanded(child: TextField(controller: label, decoration: const InputDecoration(labelText: 'Module name *'))),
                const SizedBox(width: 12),
                Expanded(child: TextField(controller: key, decoration: const InputDecoration(labelText: 'Stable key *', hintText: 'group.module_name'))),
              ]),
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
                              SizedBox(width: width, child: CatalogModuleCard(module: m)),
                          ],
                        );
                      },
                    ),
                  ],
                ),
    );
  }
}

class SystemPageclass SystemPage extends StatelessWidget {
  const SystemPage({required this.api, super.key});
  final Api api;
  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: api.get('/api/v1/health'),
      builder: (context, snapshot) {
        final serviceRaw = snapshot.data?['services'];
        final services = serviceRaw is Map ? Map<String, dynamic>.from(serviceRaw) : <String, dynamic>{};
        return Content(
          title: 'System & Operations',
          subtitle: 'Independent Go services behind one authenticated public gateway.',
          child: Wrap(spacing: 16, runSpacing: 16, children: [
            ServiceCard(name: 'API Gateway', status: '${snapshot.data?['status'] ?? 'loading'}'),
            ServiceCard(name: 'Identity', status: '${services['identity'] ?? 'loading'}'),
            ServiceCard(name: 'Partner Service', status: '${services['partners'] ?? 'loading'}'),
            ServiceCard(name: 'Catalog Service', status: '${services['catalog'] ?? 'loading'}'),
            ServiceCard(name: 'Billing Service', status: '${services['billing'] ?? 'loading'}'),
          ]),
        );
      },
    );
  }
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
              Text(eyebrow!, style: const TextStyle(fontFamily: 'Georgia', color: brandNavy, fontSize: 14)),
              const SizedBox(height: 3),
            ],
            Text(title, style: Theme.of(context).textTheme.headlineMedium),
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
      width: 248,
      height: 118,
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
          FittedBox(fit: BoxFit.scaleDown, alignment: Alignment.centerLeft, child: Text(widget.value, style: const TextStyle(fontFamily: 'Georgia', color: brandNavy, fontSize: 25, fontWeight: FontWeight.w600))),
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
