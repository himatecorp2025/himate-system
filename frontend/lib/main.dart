import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/browser_client.dart';
import 'package:http/http.dart' as http;

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const HimateApp());
}

const brandInk = Color(0xFF03111F);
const brandNavy = Color(0xFF071B33);
const brandNavy2 = Color(0xFF0D2B4B);
const brandBlue = Color(0xFF17466F);
const brandSteel = Color(0xFF426784);
const brandGold = Color(0xFFD5A23F);
const brandGold2 = Color(0xFFF0D47D);
const brandCanvas = Color(0xFFF4F7FB);
const brandText = Color(0xFF132033);
const brandMuted = Color(0xFF6A778A);
const brandBorder = Color(0xFFDDE5EF);
const brandSuccess = Color(0xFF1E7D5A);
const brandWarning = Color(0xFFB56A19);
const brandDanger = Color(0xFFB53A3A);
const navy = brandNavy;
const gold = brandGold;
const canvas = brandCanvas;
const muted = brandMuted;
const success = brandSuccess;
const _logoBase64 = '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';
final _logoBytes = base64Decode(_logoBase64);

ThemeData buildBrandTheme() {
  final scheme = ColorScheme.fromSeed(seedColor: brandNavy, brightness: Brightness.light, primary: brandNavy, secondary: brandGold, surface: Colors.white, error: brandDanger);
  return ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    scaffoldBackgroundColor: brandCanvas,
    fontFamily: 'Arial',
    textTheme: const TextTheme(
      headlineMedium: TextStyle(color: brandText, fontWeight: FontWeight.w800, letterSpacing: -.6),
      headlineSmall: TextStyle(color: brandText, fontWeight: FontWeight.w800),
      titleLarge: TextStyle(color: brandText, fontWeight: FontWeight.w700),
      titleMedium: TextStyle(color: brandText, fontWeight: FontWeight.w700),
      bodyLarge: TextStyle(color: brandText, height: 1.45),
      bodyMedium: TextStyle(color: brandText, height: 1.4),
    ),
    cardTheme: CardThemeData(color: Colors.white, elevation: 0, margin: EdgeInsets.zero, shadowColor: brandNavy.withOpacity(.08), shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: const BorderSide(color: brandBorder))),
    inputDecorationTheme: InputDecorationTheme(
      filled: true, fillColor: Colors.white, labelStyle: const TextStyle(color: brandMuted), hintStyle: const TextStyle(color: brandMuted),
      prefixIconColor: brandSteel, suffixIconColor: brandSteel, contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 17),
      enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(14), borderSide: const BorderSide(color: brandBorder)),
      focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(14), borderSide: const BorderSide(color: brandGold, width: 1.6)),
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(14), borderSide: const BorderSide(color: brandBorder)),
    ),
    filledButtonTheme: FilledButtonThemeData(style: FilledButton.styleFrom(backgroundColor: brandNavy, foregroundColor: Colors.white, padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16), shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)), textStyle: const TextStyle(fontWeight: FontWeight.w700))),
    outlinedButtonTheme: OutlinedButtonThemeData(style: OutlinedButton.styleFrom(foregroundColor: brandNavy, side: const BorderSide(color: brandBorder), padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16), shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)), textStyle: const TextStyle(fontWeight: FontWeight.w700))),
    appBarTheme: const AppBarTheme(backgroundColor: brandInk, foregroundColor: Colors.white, elevation: 0),
    dividerColor: brandBorder,
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
      ? const Scaffold(backgroundColor: brandInk, body: Center(child: SizedBox(width: 30, height: 30, child: CircularProgressIndicator(strokeWidth: 2.5, color: brandGold))))
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
  String? error;

  @override
  void dispose() { email.dispose(); password.dispose(); super.dispose(); }

  Future<void> submit() async {
    if (email.text.trim().isEmpty || password.text.isEmpty) return;
    setState(() { busy = true; error = null; });
    try { await widget.onLogin(email.text.trim(), password.text); }
    catch (e) { if (mounted) setState(() => error = e.toString()); }
    finally { if (mounted) setState(() => busy = false); }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: brandInk,
      body: LayoutBuilder(builder: (context, constraints) {
        final desktop = constraints.maxWidth >= 980;
        final form = _LoginForm(
          email: email, password: password, obscure: obscure, busy: busy, error: error,
          onToggle: () => setState(() => obscure = !obscure), onSubmit: submit,
        );
        if (!desktop) {
          return Container(
            decoration: const BoxDecoration(gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [brandInk, brandNavy, brandNavy2])),
            child: SafeArea(child: Center(child: SingleChildScrollView(
              padding: const EdgeInsets.all(22),
              child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 520), child: Column(children: [const BrandLogo(width: 290), const SizedBox(height: 24), form])),
            ))),
          );
        }
        return Row(children: [
          Expanded(flex: 11, child: Container(
            padding: const EdgeInsets.all(56),
            decoration: const BoxDecoration(gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color(0xFF020B14), brandNavy, Color(0xFF0A3156)])),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const BrandLogo(width: 350),
              const Spacer(),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                decoration: BoxDecoration(color: brandGold.withOpacity(.12), borderRadius: BorderRadius.circular(999), border: Border.all(color: brandGold.withOpacity(.34))),
                child: const Text('HIMATE CONTROL PLANE', style: TextStyle(color: brandGold2, fontSize: 11, fontWeight: FontWeight.w800, letterSpacing: 1.6)),
              ),
              const SizedBox(height: 22),
              const Text('Art Drives a\nBetter Tomorrow', style: TextStyle(color: Colors.white, fontSize: 46, height: 1.02, fontWeight: FontWeight.w800, letterSpacing: -1.6)),
              const SizedBox(height: 18),
              const SizedBox(width: 560, child: Text('One secure operating layer for partners, modules, licensing and the systems that power cultural organizations.', style: TextStyle(color: Color(0xFFC8D5E5), fontSize: 17, height: 1.55))),
              const SizedBox(height: 30),
              const Wrap(spacing: 10, runSpacing: 10, children: [
                _FeaturePill(icon: Icons.shield_outlined, text: 'Secure'),
                _FeaturePill(icon: Icons.hub_outlined, text: 'Scalable'),
                _FeaturePill(icon: Icons.auto_graph_outlined, text: 'Impact-led'),
              ]),
            ]),
          )),
          Expanded(flex: 9, child: Container(
            color: brandCanvas,
            child: Center(child: SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: 48, vertical: 36),
              child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 500), child: form),
            )),
          )),
        ]);
      }),
    );
  }
}

class _LoginForm extends StatelessWidget {
  const _LoginForm({required this.email, required this.password, required this.obscure, required this.busy, required this.error, required this.onToggle, required this.onSubmit});
  final TextEditingController email, password;
  final bool obscure, busy;
  final String? error;
  final VoidCallback onToggle, onSubmit;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(30),
    decoration: BoxDecoration(
      color: Colors.white, borderRadius: BorderRadius.circular(24), border: Border.all(color: brandBorder),
      boxShadow: [BoxShadow(color: brandNavy.withOpacity(.08), blurRadius: 38, offset: const Offset(0, 18))],
    ),
    child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      const Row(children: [BrandMark(size: 34), SizedBox(width: 12), Expanded(child: Text('HIMATE SYSTEM', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w800, letterSpacing: 1.5)))]),
      const SizedBox(height: 28),
      Text('Administrator sign in', style: Theme.of(context).textTheme.headlineSmall),
      const SizedBox(height: 8),
      const Text('Use the administrator credentials configured for this environment.', style: TextStyle(color: brandMuted, height: 1.45)),
      const SizedBox(height: 26),
      TextField(controller: email, keyboardType: TextInputType.emailAddress, autofillHints: const [AutofillHints.email], style: const TextStyle(color: brandText), decoration: const InputDecoration(labelText: 'Email address', prefixIcon: Icon(Icons.alternate_email))),
      const SizedBox(height: 14),
      TextField(controller: password, obscureText: obscure, autofillHints: const [AutofillHints.password], onSubmitted: (_) => onSubmit(), style: const TextStyle(color: brandText),
        decoration: InputDecoration(labelText: 'Password', prefixIcon: const Icon(Icons.lock_outline), suffixIcon: IconButton(onPressed: onToggle, icon: Icon(obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined)))),
      if (error != null) ...[
        const SizedBox(height: 14),
        Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(color: brandDanger.withOpacity(.07), borderRadius: BorderRadius.circular(12), border: Border.all(color: brandDanger.withOpacity(.18))),
          child: Row(children: [const Icon(Icons.error_outline, color: brandDanger, size: 19), const SizedBox(width: 9), Expanded(child: Text(error!, style: const TextStyle(color: brandDanger)))]),
        ),
      ],
      const SizedBox(height: 20),
      FilledButton(
        onPressed: busy ? null : onSubmit, style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(54)),
        child: busy ? const SizedBox(width: 21, height: 21, child: CircularProgressIndicator(strokeWidth: 2.2, color: Colors.white))
          : const Row(mainAxisAlignment: MainAxisAlignment.center, children: [Text('Sign in securely'), SizedBox(width: 10), Icon(Icons.arrow_forward_rounded, size: 18)]),
      ),
      const SizedBox(height: 15),
      const Text('Private HIMATE administration environment', textAlign: TextAlign.center, style: TextStyle(color: brandMuted, fontSize: 12)),
    ]),
  );
}

class _FeaturePill extends StatelessWidget {
  const _FeaturePill({required this.icon, required this.text});
  final IconData icon; final String text;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
    decoration: BoxDecoration(color: Colors.white.withOpacity(.06), borderRadius: BorderRadius.circular(12), border: Border.all(color: Colors.white.withOpacity(.10))),
    child: Row(mainAxisSize: MainAxisSize.min, children: [Icon(icon, color: brandGold2, size: 17), const SizedBox(width: 7), Text(text, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 12))]),
  );
}

class BrandLogo extends StatelessWidget {
  const BrandLogo({required this.width, super.key});
  final double width;
  @override
  Widget build(BuildContext context) => Container(
    width: width, padding: const EdgeInsets.all(8),
    decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(14), boxShadow: [BoxShadow(color: Colors.black.withOpacity(.16), blurRadius: 20, offset: const Offset(0, 8))]),
    child: ClipRRect(borderRadius: BorderRadius.circular(8), child: Image.memory(_logoBytes, fit: BoxFit.contain, filterQuality: FilterQuality.high)),
  );
}

class BrandMark extends StatelessWidget {
  const BrandMark({this.size = 34, super.key});
  final double size;
  @override
  Widget build(BuildContext context) => SizedBox(
    width: size, height: size,
    child: Row(crossAxisAlignment: CrossAxisAlignment.end, mainAxisAlignment: MainAxisAlignment.center, children: [
      for (final factor in const [.46, .62, .78, 1.0])
        Container(
          width: size * .13, height: size * factor, margin: EdgeInsets.symmetric(horizontal: size * .025),
          decoration: BoxDecoration(
            gradient: const LinearGradient(begin: Alignment.topCenter, end: Alignment.bottomCenter, colors: [brandBlue, brandNavy]),
            borderRadius: BorderRadius.circular(size * .08), border: Border.all(color: brandGold, width: size * .035),
          ),
        ),
    ]),
  );
}

class NavSpec {
  const NavSpec(this.label, this.icon, this.subtitle);
  final String label; final IconData icon; final String subtitle;
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
    NavSpec('Partners', Icons.apartment_outlined, 'Partner control'),
    NavSpec('Licensing & Finance', Icons.account_balance_wallet_outlined, 'Commercial management'),
    NavSpec('Impact & Reports', Icons.insights_outlined, 'Metrics and reporting'),
    NavSpec('Website & Marketing', Icons.campaign_outlined, 'Public brand & growth'),
    NavSpec('System & Operations', Icons.dns_outlined, 'Infrastructure health'),
    NavSpec('Administration', Icons.admin_panel_settings_outlined, 'Roles and control'),
  ];

  Widget page() {
    switch (selected) {
      case 0: return DashboardPage(api: widget.api);
      case 1: return PartnersPage(api: widget.api);
      case 2: return FinancePage(api: widget.api);
      case 3: return const PlannedPage(title: 'Impact & Reports', subtitle: 'Metrics and partner impact become functional in START-13–15.', icon: Icons.insights_outlined);
      case 4: return const PlannedPage(title: 'Website & Marketing', subtitle: 'HIMATE CMS and public marketing tools are planned for START-16–17.', icon: Icons.campaign_outlined);
      case 5: return SystemPage(api: widget.api);
      default: return const PlannedPage(title: 'Administration', subtitle: 'Roles, permissions and advanced audit controls are planned for START-18–19.', icon: Icons.admin_panel_settings_outlined);
    }
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(builder: (context, constraints) {
      final desktop = constraints.maxWidth >= 1020;
      if (!desktop) {
        return Scaffold(
          appBar: AppBar(
            title: Row(children: [const BrandMark(size: 28), const SizedBox(width: 10), Expanded(child: Text(nav[selected].label, style: const TextStyle(fontWeight: FontWeight.w700)))]),
            actions: [IconButton(onPressed: widget.onLogout, tooltip: 'Sign out', icon: const Icon(Icons.logout_rounded))],
          ),
          drawer: Drawer(
            backgroundColor: brandInk,
            child: SafeArea(child: Column(children: [
              const Padding(padding: EdgeInsets.fromLTRB(18, 18, 18, 12), child: BrandLogo(width: 230)),
              const Divider(color: Color(0xFF19344F), height: 20),
              Expanded(child: ListView.builder(
                padding: const EdgeInsets.symmetric(horizontal: 10),
                itemCount: nav.length,
                itemBuilder: (context, i) => ListTile(
                  onTap: () { setState(() => selected = i); Navigator.pop(context); },
                  selected: selected == i, selectedTileColor: Colors.white.withOpacity(.09),
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
                  leading: Icon(nav[i].icon, color: selected == i ? brandGold2 : const Color(0xFF9FB0C2)),
                  title: Text(nav[i].label, style: TextStyle(color: selected == i ? Colors.white : const Color(0xFFD2DCE8), fontWeight: FontWeight.w700)),
                  subtitle: Text(nav[i].subtitle, style: const TextStyle(color: Color(0xFF7E93A8), fontSize: 11)),
                ),
              )),
            ])),
          ),
          body: page(),
        );
      }
      return Scaffold(
        body: Row(children: [
          AnimatedContainer(
            duration: const Duration(milliseconds: 220), curve: Curves.easeOutCubic, width: collapsed ? 88 : 286,
            decoration: const BoxDecoration(gradient: LinearGradient(begin: Alignment.topLeft, end: Alignment.bottomRight, colors: [Color(0xFF020B14), brandNavy, Color(0xFF0B2948)])),
            child: SafeArea(child: Column(children: [
              Padding(
                padding: EdgeInsets.fromLTRB(collapsed ? 15 : 18, 18, collapsed ? 15 : 18, 8),
                child: Row(children: [
                  if (!collapsed) const Expanded(child: BrandLogo(width: 205)) else const Expanded(child: Center(child: BrandMark(size: 38))),
                  if (!collapsed) const SizedBox(width: 8),
                  IconButton(
                    onPressed: () => setState(() => collapsed = !collapsed),
                    tooltip: collapsed ? 'Show menu' : 'Hide menu',
                    style: IconButton.styleFrom(backgroundColor: Colors.white.withOpacity(.07), foregroundColor: Colors.white),
                    icon: Icon(collapsed ? Icons.keyboard_double_arrow_right_rounded : Icons.keyboard_double_arrow_left_rounded),
                  ),
                ]),
              ),
              Padding(padding: EdgeInsets.fromLTRB(collapsed ? 12 : 14, 12, collapsed ? 12 : 14, 8), child: Container(height: 1, color: Colors.white.withOpacity(.08))),
              Expanded(child: ListView.separated(
                padding: EdgeInsets.symmetric(horizontal: collapsed ? 11 : 14, vertical: 8),
                itemCount: nav.length,
                separatorBuilder: (_, __) => const SizedBox(height: 7),
                itemBuilder: (context, i) {
                  final active = selected == i;
                  final tile = Material(
                    color: Colors.transparent,
                    child: InkWell(
                      onTap: () => setState(() => selected = i), borderRadius: BorderRadius.circular(14),
                      child: AnimatedContainer(
                        duration: const Duration(milliseconds: 170), constraints: const BoxConstraints(minHeight: 54),
                        padding: EdgeInsets.symmetric(horizontal: collapsed ? 0 : 14, vertical: 10),
                        decoration: BoxDecoration(
                          color: active ? Colors.white.withOpacity(.10) : Colors.transparent,
                          borderRadius: BorderRadius.circular(14),
                          border: Border.all(color: active ? brandGold.withOpacity(.42) : Colors.transparent),
                        ),
                        child: collapsed
                          ? Center(child: Icon(nav[i].icon, color: active ? brandGold2 : const Color(0xFF9FB0C2), size: 23))
                          : Row(children: [
                              Icon(nav[i].icon, color: active ? brandGold2 : const Color(0xFF9FB0C2), size: 22),
                              const SizedBox(width: 13),
                              Expanded(child: Text(nav[i].label, style: TextStyle(color: active ? Colors.white : const Color(0xFFD2DCE8), fontWeight: active ? FontWeight.w700 : FontWeight.w600, fontSize: 13.5))),
                              if (active) Container(width: 4, height: 22, decoration: BoxDecoration(color: brandGold, borderRadius: BorderRadius.circular(99))),
                            ]),
                      ),
                    ),
                  );
                  return collapsed ? Tooltip(message: nav[i].label, child: tile) : tile;
                },
              )),
              Padding(
                padding: EdgeInsets.all(collapsed ? 10 : 14),
                child: collapsed
                  ? Tooltip(message: 'Sign out', child: IconButton(onPressed: widget.onLogout, style: IconButton.styleFrom(backgroundColor: Colors.white.withOpacity(.07), foregroundColor: Colors.white70), icon: const Icon(Icons.logout_rounded)))
                  : Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(color: Colors.white.withOpacity(.06), borderRadius: BorderRadius.circular(15), border: Border.all(color: Colors.white.withOpacity(.08))),
                      child: Row(children: [
                        Container(width: 34, height: 34, decoration: BoxDecoration(gradient: const LinearGradient(colors: [brandGold2, brandGold]), borderRadius: BorderRadius.circular(11)), child: const Icon(Icons.person_outline, color: brandNavy, size: 20)),
                        const SizedBox(width: 10),
                        Expanded(child: Text('${widget.user['name'] ?? widget.user['email'] ?? 'Administrator'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 12))),
                        IconButton(onPressed: widget.onLogout, tooltip: 'Sign out', icon: const Icon(Icons.logout_rounded, size: 19), color: Colors.white70),
                      ]),
                    ),
              ),
            ])),
          ),
          Expanded(child: Container(
            color: brandCanvas,
            child: Column(children: [
              Container(
                height: 76, padding: const EdgeInsets.symmetric(horizontal: 30),
                decoration: const BoxDecoration(color: Colors.white, border: Border(bottom: BorderSide(color: brandBorder))),
                child: Row(children: [
                  Expanded(child: Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text(nav[selected].label, style: const TextStyle(color: brandNavy, fontSize: 19, fontWeight: FontWeight.w800)),
                    const SizedBox(height: 2),
                    Text(nav[selected].subtitle, style: const TextStyle(color: brandMuted, fontSize: 12)),
                  ])),
                  const _SecurePill(),
                ]),
              ),
              Expanded(child: page()),
            ]),
          )),
        ]),
      );
    });
  }
}

class _SecurePill extends StatelessWidget {
  const _SecurePill();
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
    decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(999), border: Border.all(color: brandGold.withOpacity(.24))),
    child: const Row(children: [Icon(Icons.lock_outline, color: brandNavy, size: 13), SizedBox(width: 6), Text('SECURE ADMIN', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 9.5, letterSpacing: .7))]),
  );
}

class PlannedPage extends StatelessWidget {
  const PlannedPage({required this.title, required this.subtitle, required this.icon, super.key});
  final String title, subtitle; final IconData icon;
  @override
  Widget build(BuildContext context) => Content(
    title: title, subtitle: subtitle,
    child: Card(child: Padding(
      padding: const EdgeInsets.all(28),
      child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Container(width: 54, height: 54, decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(16)), child: Icon(icon, color: brandNavy, size: 28)),
        const SizedBox(width: 16),
        const Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('Workspace prepared', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 18)),
          SizedBox(height: 7),
          Text('The responsive navigation and visual system are already in place. Functional implementation will be added in its scheduled START cycle.', style: TextStyle(color: brandMuted, height: 1.5)),
        ])),
      ]),
    )),
  );
}

class DashboardPage extends StatelessWidget {
  const DashboardPage({required this.api, super.key});
  final Api api;
  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>>(
      future: api.get('/api/v1/dashboard/summary'),
      builder: (context, snapshot) {
        if (!snapshot.hasData) return const Center(child: CircularProgressIndicator());
        final d = snapshot.data!;
        final p = Map<String, dynamic>.from(d['partners'] ?? <String, dynamic>{});
        final m = Map<String, dynamic>.from(d['modules'] ?? <String, dynamic>{});
        final s = Map<String, dynamic>.from(d['system'] ?? <String, dynamic>{});
        return Content(
          title: 'HIMATE overview',
          subtitle: 'START-04–08 control plane · Go + Flutter · containerized microservices',
          child: Wrap(
            spacing: 16,
            runSpacing: 16,
            children: [
              Kpi(label: 'Partners', value: '${p['total'] ?? 0}', note: '${p['live'] ?? 0} live'),
              Kpi(label: 'Module catalog', value: '${m['catalog_total'] ?? 0}', note: '38 verified Klavierhaus reference modules'),
              Kpi(label: 'Architecture', value: 'MICROSERVICES', note: 'Gateway · Partners · Catalog · Billing'),
              Kpi(label: 'System', value: '${s['status'] ?? 'unknown'}'.toUpperCase(), note: '${s['version'] ?? ''}'),
            ],
          ),
        );
      },
    );
  }
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

  @override
  void initState() { super.initState(); load(); }
  Future<void> load() async {
    final r = await Future.wait([widget.api.get('/api/v1/partners'), widget.api.get('/api/v1/partner-categories')]);
    partners = items(r[0]); categories = items(r[1]);
    if (mounted) setState(() => loading = false);
  }

  Future<void> addCategory() async {
    final c = TextEditingController();
    final ok = await showDialog<bool>(context: context, builder: (context) => AlertDialog(title: const Text('Add partner category'), content: TextField(controller: c, decoration: const InputDecoration(labelText: 'Category name')), actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Add'))]));
    if (ok == true && c.text.trim().isNotEmpty) { await widget.api.post('/api/v1/partner-categories', {'name': c.text.trim()}); await load(); }
  }

  Future<void> addPartner() async {
    if (categories.isEmpty) return;
    final name = TextEditingController();
    String category = '${categories.first['id']}';
    final ok = await showDialog<bool>(context: context, builder: (context) => StatefulBuilder(builder: (context, setLocal) => AlertDialog(title: const Text('New Partner'), content: SizedBox(width: 500, child: Column(mainAxisSize: MainAxisSize.min, children: [TextField(controller: name, decoration: const InputDecoration(labelText: 'Partner name *')), const SizedBox(height: 12), DropdownButtonFormField<String>(value: category, decoration: const InputDecoration(labelText: 'Category'), items: [for (final c in categories) DropdownMenuItem(value: '${c['id']}', child: Text('${c['name']}'))], onChanged: (v) { if (v != null) setLocal(() => category = v); })])), actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Create'))])));
    if (ok == true && name.text.trim().isNotEmpty) { await widget.api.post('/api/v1/partners', {'display_name': name.text.trim(), 'legal_name': name.text.trim(), 'category_id': category, 'lifecycle': 'PROSPECT', 'country': 'United States'}); await load(); }
  }

  @override
  Widget build(BuildContext context) {
    return Content(
      title: 'Partners',
      subtitle: 'Partner cards, lifecycle and extensible partner categories.',
      actions: [OutlinedButton.icon(onPressed: addCategory, icon: const Icon(Icons.category_outlined), label: const Text('Add category')), FilledButton.icon(onPressed: addPartner, icon: const Icon(Icons.add_business), label: const Text('New Partner'))],
      child: loading ? const Center(child: CircularProgressIndicator()) : Wrap(spacing: 16, runSpacing: 16, children: [
        for (final p in partners) SizedBox(width: 330, height: 190, child: Card(child: InkWell(borderRadius: BorderRadius.circular(18), onTap: () => Navigator.push(context, MaterialPageRoute(builder: (_) => PartnerWorkspace(api: widget.api, partner: p))), child: Padding(padding: const EdgeInsets.all(20), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Row(children: [Expanded(child: Text('${p['display_name']}', style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 20))), if (p['reference_partner'] == true) const Icon(Icons.workspace_premium, color: gold)]), const SizedBox(height: 6), Text('${p['category_name']}', style: const TextStyle(color: muted)), const Spacer(), Chip(label: Text('${p['lifecycle']}')), const SizedBox(height: 8), const Row(children: [Text('Open workspace', style: TextStyle(fontWeight: FontWeight.w700)), Spacer(), Icon(Icons.arrow_forward)])]))))),
        SizedBox(width: 330, height: 190, child: Card(child: InkWell(borderRadius: BorderRadius.circular(18), onTap: addPartner, child: const Center(child: Column(mainAxisSize: MainAxisSize.min, children: [Icon(Icons.add_circle_outline, size: 42, color: gold), SizedBox(height: 10), Text('NEW PARTNER', style: TextStyle(fontWeight: FontWeight.w700))]))))),
      ]),
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
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  Map<String, dynamic>? billing;
  bool loading = true;

  static const workspaceCards = [
    'Overview', 'Company Data', 'System & Environment', 'Modules', 'Pricing & Subscription', 'Finance & Documents',
    'Statistics', 'Evidence', 'Branding & Website', 'Users & Contacts', 'Integrations', 'Audit History'
  ];

  @override
  void initState() { super.initState(); load(); }
  Future<void> load() async {
    final id = '${widget.partner['id']}';
    final r = await Future.wait([widget.api.get('/api/v1/partners/$id/modules'), widget.api.get('/api/v1/billing/partners/$id/summary')]);
    modules = items(r[0]); billing = r[1];
    if (mounted) setState(() => loading = false);
  }

  Future<void> editModule(Map<String, dynamic> module) async {
    String state = '${module['status']}';
    bool visible = module['visible'] == true;
    bool included = module['included_in_base'] == true;
    final price = TextEditingController(text: number(module['partner_price']).toStringAsFixed(2));
    final ok = await showDialog<bool>(context: context, builder: (context) => StatefulBuilder(builder: (context, setLocal) => AlertDialog(title: Text('${module['label']}'), content: SizedBox(width: 520, child: Column(mainAxisSize: MainAxisSize.min, children: [DropdownButtonFormField<String>(value: state, decoration: const InputDecoration(labelText: 'State'), items: const [DropdownMenuItem(value: 'ACTIVE', child: Text('ACTIVE')), DropdownMenuItem(value: 'NOT_LICENSED', child: Text('NOT LICENSED')), DropdownMenuItem(value: 'MAINTENANCE', child: Text('MAINTENANCE'))], onChanged: (v) { if (v != null) setLocal(() => state = v); }), SwitchListTile(value: visible, onChanged: (v) => setLocal(() => visible = v), title: const Text('Visible for partner')), SwitchListTile(value: included, onChanged: (v) => setLocal(() => included = v), title: const Text('Included in base package')), TextField(controller: price, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: 'Partner monthly price (USD)'))])), actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Save'))])));
    if (ok == true) {
      await widget.api.patch('/api/v1/partners/${widget.partner['id']}/modules/${module['key']}', {'status': state, 'visible': visible, 'included_in_base': included, 'partner_price': double.tryParse(price.text) ?? 0, 'reason': 'HIMATE admin update'});
      await load();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('${widget.partner['display_name']}')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('${widget.partner['display_name']}', style: Theme.of(context).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(height: 6),
          Text('${widget.partner['id']} · ${widget.partner['category_name']} · ${widget.partner['lifecycle']}', style: const TextStyle(color: muted)),
          const SizedBox(height: 20),
          Wrap(spacing: 12, runSpacing: 12, children: [for (final title in workspaceCards) SizedBox(width: 215, height: 105, child: Card(child: Padding(padding: const EdgeInsets.all(14), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Icon(title == 'Modules' ? Icons.grid_view_outlined : Icons.dashboard_customize_outlined, color: gold), const Spacer(), Text(title, style: const TextStyle(fontWeight: FontWeight.w700))]))))]),
          const SizedBox(height: 24),
          if (loading) const Center(child: CircularProgressIndicator()) else ...[
            Wrap(spacing: 16, runSpacing: 16, children: [Kpi(label: 'Base monthly', value: money(billing?['effective_base_fee']), note: 'January 1 annual uplift'), Kpi(label: 'Extra modules', value: money(billing?['extra_module_fee']), note: 'Same invoice day'), Kpi(label: 'Current total', value: money(billing?['current_total']), note: '30-day cycle · invoice day 1')]),
            const SizedBox(height: 24),
            Text('Modules (${modules.length})', style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700)),
            const SizedBox(height: 12),
            Wrap(spacing: 12, runSpacing: 12, children: [for (final m in modules) SizedBox(width: 330, child: Card(child: InkWell(onTap: () => editModule(m), borderRadius: BorderRadius.circular(18), child: Padding(padding: const EdgeInsets.all(16), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Row(children: [Expanded(child: Text('${m['label']}', style: const TextStyle(fontWeight: FontWeight.w700))), const Icon(Icons.edit_outlined, size: 18)]), const SizedBox(height: 4), Text('${m['group_label']} · ${m['status']}', style: const TextStyle(color: muted, fontSize: 12)), const SizedBox(height: 10), Row(children: [Text(m['visible'] == true ? 'VISIBLE' : 'HIDDEN'), const Spacer(), Text(m['included_in_base'] == true ? 'BASE' : money(m['partner_price']), style: const TextStyle(fontWeight: FontWeight.w700))])])))))])
          ]
        ]),
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
  Map<String, dynamic>? profile;
  bool loading = true;
  @override
  void initState() { super.initState(); load(); }
  Future<void> load() async {
    final r = await Future.wait([widget.api.get('/api/v1/modules'), widget.api.get('/api/v1/billing/profile')]);
    modules = items(r[0]); profile = r[1]; if (mounted) setState(() => loading = false);
  }

  Future<void> editProfile() async {
    final legal = TextEditingController(text: '${profile?['legal_name'] ?? ''}');
    final address = TextEditingController(text: '${profile?['address'] ?? ''}');
    final tax = TextEditingController(text: '${profile?['tax_id'] ?? ''}');
    final email = TextEditingController(text: '${profile?['email'] ?? ''}');
    final bank = TextEditingController(text: '${profile?['bank_name'] ?? ''}');
    final iban = TextEditingController(text: '${profile?['iban'] ?? ''}');
    final swift = TextEditingController(text: '${profile?['swift'] ?? ''}');
    final ok = await showDialog<bool>(context: context, builder: (context) => AlertDialog(title: const Text('HIMATE billing profile'), content: SizedBox(width: 600, child: SingleChildScrollView(child: Column(children: [TextField(controller: legal, decoration: const InputDecoration(labelText: 'Legal name')), const SizedBox(height: 10), TextField(controller: address, decoration: const InputDecoration(labelText: 'Address')), const SizedBox(height: 10), TextField(controller: tax, decoration: const InputDecoration(labelText: 'Tax ID')), const SizedBox(height: 10), TextField(controller: email, decoration: const InputDecoration(labelText: 'Billing email')), const SizedBox(height: 10), TextField(controller: bank, decoration: const InputDecoration(labelText: 'Bank name')), const SizedBox(height: 10), TextField(controller: iban, decoration: const InputDecoration(labelText: 'IBAN')), const SizedBox(height: 10), TextField(controller: swift, decoration: const InputDecoration(labelText: 'SWIFT / BIC'))]))), actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Save'))]));
    if (ok == true) { await widget.api.put('/api/v1/billing/profile', {'legal_name': legal.text, 'address': address.text, 'tax_id': tax.text, 'email': email.text, 'bank_name': bank.text, 'bank_address': '', 'account_number': '', 'iban': iban.text, 'swift': swift.text}); await load(); }
  }

  @override
  Widget build(BuildContext context) {
    return Content(
      title: 'Licensing & Finance',
      subtitle: 'Module catalog, commercial rules and HIMATE issuer profile.',
      actions: [FilledButton.icon(onPressed: editProfile, icon: const Icon(Icons.edit_outlined), label: const Text('Billing profile'))],
      child: loading ? const Center(child: CircularProgressIndicator()) : Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Wrap(spacing: 16, runSpacing: 16, children: [Kpi(label: 'Module catalog', value: '${modules.length}', note: 'Reference + custom'), const Kpi(label: 'Annual uplift', value: 'JAN 1', note: 'Default +10%, admin-overridable'), const Kpi(label: 'Service cycle', value: '30 DAYS', note: 'Invoice issue day: 1')]),
        const SizedBox(height: 24),
        Text('Canonical module catalog', style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700)),
        const SizedBox(height: 12),
        Wrap(spacing: 12, runSpacing: 12, children: [for (final m in modules) SizedBox(width: 300, child: Card(child: Padding(padding: const EdgeInsets.all(14), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text('${m['label']}', style: const TextStyle(fontWeight: FontWeight.w700)), const SizedBox(height: 4), Text('${m['group_label']} · ${m['key']}', style: const TextStyle(color: muted, fontSize: 11)), const SizedBox(height: 10), Text(money(m['default_monthly_price']), style: const TextStyle(fontWeight: FontWeight.w700))]))))])
      ]),
    );
  }
}

class SystemPage extends StatelessWidget {
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
  const Content({required this.title, required this.subtitle, required this.child, this.actions = const [], super.key});
  final String title, subtitle;
  final Widget child;
  final List<Widget> actions;
  @override
  Widget build(BuildContext context) => SingleChildScrollView(padding: const EdgeInsets.all(28), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Row(crossAxisAlignment: CrossAxisAlignment.start, children: [Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(title, style: Theme.of(context).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.w700)), const SizedBox(height: 6), Text(subtitle, style: const TextStyle(color: muted))])), if (actions.isNotEmpty) Wrap(spacing: 10, children: actions)]), const SizedBox(height: 24), child]));
}

class Kpi extends StatelessWidget {
  const Kpi({required this.label, required this.value, required this.note, super.key});
  final String label, value, note;
  @override
  Widget build(BuildContext context) => SizedBox(width: 290, height: 135, child: Card(child: Padding(padding: const EdgeInsets.all(18), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(label, style: const TextStyle(color: muted)), const SizedBox(height: 8), FittedBox(fit: BoxFit.scaleDown, alignment: Alignment.centerLeft, child: Text(value, style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w700))), const Spacer(), Text(note, maxLines: 2, overflow: TextOverflow.ellipsis, style: const TextStyle(color: muted, fontSize: 12))]))));
}

class ServiceCard extends StatelessWidget {
  const ServiceCard({required this.name, required this.status, super.key});
  final String name, status;
  @override
  Widget build(BuildContext context) => SizedBox(width: 300, height: 120, child: Card(child: Padding(padding: const EdgeInsets.all(18), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(name, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 17)), const Spacer(), Row(children: [Icon(status == 'ok' ? Icons.check_circle : Icons.warning_amber, color: status == 'ok' ? success : Colors.orange), const SizedBox(width: 8), Text(status.toUpperCase())])]))));
}
