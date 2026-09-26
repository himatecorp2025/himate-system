// ignore_for_file: deprecated_member_use
import 'dart:async';
import 'dart:convert';
import 'dart:html' as html;
import 'dart:math' as math;
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter/material.dart' as material show Text;
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter/services.dart';
import 'package:flutter_web_plugins/url_strategy.dart';
import 'package:http/browser_client.dart';
import 'package:http/http.dart' as http;
import 'package:intl/intl.dart' as intl;
import 'package:intl/date_symbol_data_local.dart' show initializeDateFormatting;
import 'package:google_fonts/google_fonts.dart';

part 'cms_page.dart';
part 'contact_leads.dart';
part 'design_guide.dart';
part 'seo_panel.dart';
part 'start22_connector.dart';
part 'administration_rbac.dart';
part 'brand_assets.dart';
part 'backups_panel.dart';
part 'domains_deployments.dart';
part 'secrets_admin.dart';
part 'localization.dart';
part 'profile_account.dart';
part 'module_control_plane.dart';
part 'notifications_panel.dart';
part 'partner_portal.dart';
part 'partner_design.dart';
part 'commercial_automation_ui.dart';
part 'compliance_archives.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await initializeDateFormatting('en_US');
  await initializeDateFormatting('hu_HU');
  usePathUrlStrategy();
  final path = Uri.base.path;
  final partnerPortal = path == '/partner/login' || path == '/partner/app' || path.startsWith('/partner/app/');
  runApp(partnerPortal ? const PartnerPortalApp() : const HimateApp());
}

const brandNavy = Color(0xFF72B7FF);
const brandSteel = Color(0xFF28A8FF);
const brandGold = Color(0xFFE2B95B);
const brandIvory = Color(0xFF020914);
const brandMist = Color(0xFF173653);
const brandCharcoal = Color(0xFFF1F6FC);
const brandWhite = Color(0xFFFFFFFF);
const brandSuccess = Color(0xFF46D9AD);
const brandWarning = Color(0xFFF3BD55);
const brandDanger = Color(0xFFFF7878);
const brandNavyDeep = Color(0xFF020914);
const brandNavySoft = Color(0xFF0A3158);
const brandTextSoft = Color(0xFFA9C0D8);
const brandSurface = Color(0xFF07182A);
const brandSurfaceRaised = Color(0xFF0B2540);
const brandIonBlue = Color(0xFF19B5FF);

const navy = brandNavy;
const gold = brandGold;
const canvas = brandIvory;
const muted = brandTextSoft;
const success = brandSuccess;



Map<String,dynamic> central9CanonicalPackage(String planKey) {
  return switch (planKey.toUpperCase().trim()) {
    'STARTER' => <String,dynamic>{'name':'Starter','price':990,'entitlement':'10 modules'},
    'BUSINESS' => <String,dynamic>{'name':'Business','price':1490,'entitlement':'20 modules'},
    'FLEX' => <String,dynamic>{'name':'Premium','price':2490,'entitlement':'Unlimited'},
    _ => <String,dynamic>{'name':planKey,'price':0,'entitlement':'—'},
  };
}

String central9CanonicalPackagePrice(String planKey) {
  final package = central9CanonicalPackage(planKey);
  final price = (package['price'] as num?)?.toInt() ?? 0;
  return '\$' + intl.NumberFormat('#,##0', 'en_US').format(price) + ' + VAT';
}

List<Map<String,dynamic>> central8PartnerPresetRows(
  List<Map<String,dynamic>> rows, {
  String lifecycle = 'ALL',
  bool reference = false,
}) {
  return rows.where((row) {
    if (lifecycle != 'ALL' && '${row['lifecycle'] ?? ''}' != lifecycle) return false;
    if (reference && row['reference_partner'] != true) return false;
    return true;
  }).toList();
}

Future<String?> promptMfaCode(BuildContext context, Map<String, dynamic> challenge) async {
  final code = TextEditingController();
  final setup = challenge['mfa_setup'] == true;
  final secret = challenge['secret']?.toString() ?? '';
  return showDialog<String>(
    context: context,
    barrierDismissible: false,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Multi-factor authentication'),
      content: SizedBox(
        width: 430,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(setup
                ? 'Add this account to your authenticator app, then enter the current six-digit code.'
                : 'Enter the current six-digit code from your authenticator app.'),
            if (setup && secret.isNotEmpty) ...[
              const SizedBox(height: 14),
              const Text('Setup key'),
              const SizedBox(height: 6),
              SelectableText(secret),
            ],
            const SizedBox(height: 18),
            TextField(
              controller: code,
              autofocus: true,
              keyboardType: TextInputType.number,
              maxLength: 6,
              decoration: const InputDecoration(labelText: 'Authentication code'),
              onSubmitted: (value) {
                final normalized = value.trim();
                if (normalized.length == 6) Navigator.of(dialogContext).pop(normalized);
              },
            ),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(dialogContext).pop(), child: const Text('Cancel')),
        FilledButton(
          onPressed: () {
            final normalized = code.text.trim();
            if (normalized.length == 6) Navigator.of(dialogContext).pop(normalized);
          },
          child: const Text('Verify'),
        ),
      ],
    ),
  );
}

ThemeData buildBrandTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: brandIonBlue,
    brightness: Brightness.dark,
    primary: brandIonBlue,
    secondary: brandGold,
    surface: brandSurfaceRaised,
    error: brandDanger,
  );
  final base = GoogleFonts.interTextTheme(ThemeData.dark().textTheme);
  final display = GoogleFonts.cormorantGaramondTextTheme(ThemeData.dark().textTheme);
  return ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    colorScheme: scheme,
    scaffoldBackgroundColor: brandNavyDeep,
    visualDensity: VisualDensity.standard,
    splashFactory: InkSparkle.splashFactory,
    textTheme: base.copyWith(
      displaySmall: display.displaySmall?.copyWith(color: brandWhite, fontWeight: FontWeight.w600, letterSpacing: -.7, height: 1.02),
      headlineLarge: display.headlineLarge?.copyWith(color: brandWhite, fontWeight: FontWeight.w600, letterSpacing: -.45, height: 1.03),
      headlineMedium: display.headlineMedium?.copyWith(color: brandWhite, fontWeight: FontWeight.w600, letterSpacing: -.3, height: 1.05),
      headlineSmall: display.headlineSmall?.copyWith(color: brandWhite, fontWeight: FontWeight.w600, letterSpacing: -.15, height: 1.08),
      titleLarge: display.titleLarge?.copyWith(color: brandWhite, fontWeight: FontWeight.w700),
      titleMedium: base.titleMedium?.copyWith(color: brandWhite, fontWeight: FontWeight.w700),
      bodyLarge: base.bodyLarge?.copyWith(color: brandCharcoal, height: 1.5),
      bodyMedium: base.bodyMedium?.copyWith(color: brandCharcoal, height: 1.45),
      bodySmall: base.bodySmall?.copyWith(color: brandTextSoft, height: 1.4),
      labelLarge: base.labelLarge?.copyWith(color: brandWhite, fontWeight: FontWeight.w700, letterSpacing: .05),
    ),
    cardTheme: CardThemeData(
      color: brandSurfaceRaised,
      elevation: 0,
      margin: EdgeInsets.zero,
      shadowColor: brandIonBlue.withOpacity(.18),
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: brandIonBlue.withOpacity(.24), width: 1.0),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: brandSurface,
      labelStyle: base.bodyMedium?.copyWith(color: brandTextSoft),
      hintStyle: base.bodyMedium?.copyWith(color: brandTextSoft.withOpacity(.72)),
      prefixIconColor: brandIonBlue,
      suffixIconColor: brandIonBlue,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
      enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: BorderSide(color: brandMist.withOpacity(.9))),
      focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandIonBlue, width: 1.5)),
      errorBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandDanger)),
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(10), borderSide: const BorderSide(color: brandMist)),
    ),
    checkboxTheme: CheckboxThemeData(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(3)),
      fillColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.selected) ? brandIonBlue : brandSurface),
      checkColor: WidgetStateProperty.all(brandNavyDeep),
      side: const BorderSide(color: brandIonBlue, width: 1.4),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: ButtonStyle(
        backgroundColor: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.disabled)) return brandIonBlue.withOpacity(.34);
          if (states.contains(WidgetState.hovered)) return const Color(0xFF39C2FF);
          if (states.contains(WidgetState.pressed)) return const Color(0xFF0E8FD5);
          return brandIonBlue;
        }),
        foregroundColor: WidgetStateProperty.all(brandNavyDeep),
        overlayColor: WidgetStateProperty.all(brandWhite.withOpacity(.08)),
        padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 21, vertical: 16)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(9))),
        textStyle: WidgetStateProperty.all(base.labelLarge?.copyWith(fontWeight: FontWeight.w800)),
        elevation: WidgetStateProperty.all(0),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.hovered) ? brandWhite : brandNavy),
        side: WidgetStateProperty.resolveWith((states) => BorderSide(color: states.contains(WidgetState.hovered) ? brandIonBlue : brandMist)),
        backgroundColor: WidgetStateProperty.all(brandSurface.withOpacity(.72)),
        overlayColor: WidgetStateProperty.all(brandIonBlue.withOpacity(.08)),
        padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 20, vertical: 15)),
        shape: WidgetStateProperty.all(RoundedRectangleBorder(borderRadius: BorderRadius.circular(9))),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.resolveWith((states) => states.contains(WidgetState.hovered) ? brandWhite : brandNavy),
        overlayColor: WidgetStateProperty.all(brandIonBlue.withOpacity(.07)),
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: ButtonStyle(
        foregroundColor: WidgetStateProperty.all(brandNavy),
        overlayColor: WidgetStateProperty.all(brandIonBlue.withOpacity(.08)),
      ),
    ),
    dividerColor: brandMist,
    scrollbarTheme: ScrollbarThemeData(
      thumbColor: WidgetStateProperty.all(brandIonBlue.withOpacity(.42)),
      radius: const Radius.circular(12),
      thickness: WidgetStateProperty.all(6),
    ),
    appBarTheme: AppBarTheme(
      backgroundColor: brandSurface,
      foregroundColor: brandWhite,
      elevation: 0,
      surfaceTintColor: Colors.transparent,
      titleTextStyle: display.titleLarge?.copyWith(color: brandWhite, fontWeight: FontWeight.w600),
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

  Future<Map<String, dynamic>> _fetchGet(String path, Duration maxAge) {
    final pending = _inflight[path];
    if (pending != null) return pending;
    final future = request('GET', path).then((data) {
      _cache[path] = _ApiCacheEntry(data, DateTime.now().add(maxAge));
      return data;
    }).whenComplete(() => _inflight.remove(path));
    _inflight[path] = future;
    return future;
  }

  Future<Map<String, dynamic>> get(
    String path, {
    Duration maxAge = const Duration(seconds: 30),
    bool force = false,
  }) {
    if (force) {
      return request('GET', path).then((data) {
        _cache[path] = _ApiCacheEntry(data, DateTime.now().add(maxAge));
        return data;
      });
    }

    final cached = _cache[path];
    if (cached != null) {
      if (DateTime.now().isAfter(cached.expiresAt) && !_inflight.containsKey(path)) {
        unawaited(_fetchGet(path, maxAge).catchError((_) => cached.data));
      }
      return Future<Map<String, dynamic>>.value(cached.data);
    }

    final pending = _inflight[path];
    if (pending != null) return pending;
    return _fetchGet(path, maxAge);
  }

  void prefetch(Iterable<String> paths, {Duration maxAge = const Duration(seconds: 45)}) {
    for (final path in paths) {
      unawaited(get(path, maxAge: maxAge).catchError((_) => <String, dynamic>{}));
    }
  }

  Map<String, dynamic>? peek(String path) => _cache[path]?.data;

  void _invalidateMutation(String path) {
    final prefixes = <String>{};
    void add(String prefix) => prefixes.add(prefix);

    // Every successful mutation can affect one or more Central backend read
    // models. Never let the browser keep a pre-mutation screen snapshot.
    add('/api/v1/central');

    if (path.startsWith('/partner/api/v1')) {
      add('/partner/api/v1');
    } else if (path.startsWith('/api/v1/partners') || path.startsWith('/api/v1/partner-categories')) {
      add('/api/v1/partners');
      add('/api/v1/partner-categories');
      add('/api/v1/dashboard');
    } else if (path.startsWith('/api/v1/modules') || path.startsWith('/api/v1/module-groups')) {
      add('/api/v1/modules');
      add('/api/v1/module-groups');
      add('/api/v1/partners');
      add('/api/v1/dashboard');
    } else if (path.startsWith('/api/v1/billing')) {
      add('/api/v1/billing');
      add('/api/v1/partners');
      add('/api/v1/dashboard');
    } else if (path.startsWith('/api/v1/impact') || path.startsWith('/api/v1/evidence') || path.startsWith('/api/v1/reports')) {
      add('/api/v1/impact');
      add('/api/v1/evidence');
      add('/api/v1/reports');
      add('/api/v1/dashboard');
    } else if (path.startsWith('/api/v1/cms')) {
      add('/api/v1/cms');
    } else if (path.startsWith('/api/v1/contact/inquiries')) {
      add('/api/v1/contact/inquiries');
    } else if (path.startsWith('/api/v1/backups')) {
      add('/api/v1/backups');
      add('/api/v1/system-health');
      add('/api/v1/dashboard');
    } else if (path.startsWith('/api/v1/provisioning') || path.startsWith('/api/v1/environments') || path.startsWith('/api/v1/connectors')) {
      add('/api/v1/provisioning');
      add('/api/v1/environments');
      add('/api/v1/connectors');
      add('/api/v1/system-health');
      add('/api/v1/dashboard');
    } else if (path.startsWith('/api/v1/admin')) {
      add('/api/v1/admin');
      add('/api/v1/audit');
    } else if (path.startsWith('/api/v1/notifications')) {
      add('/api/v1/notifications');
    } else if (path.startsWith('/api/v1/auth')) {
      if (path.endsWith('/logout')) {
        clearCache();
        return;
      }
      add('/api/v1/auth');
    } else {
      add('/api/v1/dashboard');
    }

    for (final prefix in prefixes) {
      clearCache(prefix);
    }
  }

  Future<Map<String, dynamic>> post(String path, [Map<String, dynamic>? body]) async {
    final result = await request('POST', path, body);
    _invalidateMutation(path);
    return result;
  }

  Future<Map<String, dynamic>> put(String path, Map<String, dynamic> body) async {
    final result = await request('PUT', path, body);
    _invalidateMutation(path);
    return result;
  }

  Future<Map<String, dynamic>> patch(String path, Map<String, dynamic> body) async {
    final result = await request('PATCH', path, body);
    _invalidateMutation(path);
    return result;
  }

  Future<Map<String, dynamic>> delete(String path) async {
    final result = await request('DELETE', path);
    _invalidateMutation(path);
    return result;
  }

  Future<Map<String, dynamic>> multipart(
    String path,
    Map<String, String> fields,
    Uint8List bytes,
    String filename,
  ) async {
    final request = http.MultipartRequest('POST', Uri.parse(path));
    request.headers['Accept'] = 'application/json';
    request.fields.addAll(fields);
    request.files.add(http.MultipartFile.fromBytes('file', bytes, filename: filename));
    final streamed = await client.send(request).timeout(const Duration(seconds: 90));
    final response = await http.Response.fromStream(streamed);
    Map<String, dynamic> data = <String, dynamic>{};
    if (response.body.trim().isNotEmpty) {
      final decoded = jsonDecode(response.body);
      if (decoded is Map) data = Map<String, dynamic>.from(decoded);
    }
    if (response.statusCode >= 200 && response.statusCode < 300) {
      _invalidateMutation(path);
      return data;
    }
    final error = data['error'];
    if (error is Map && error['message'] != null) {
      throw ApiError(response.statusCode, '${error['message']}');
    }
    throw ApiError(response.statusCode, 'Upload failed (${response.statusCode})');
  }

  void clearCache([String? prefix]) {
    if (prefix == null) {
      _cache.clear();
      return;
    }
    _cache.removeWhere((key, _) => key.startsWith(prefix));
  }

  Future<Map<String, dynamic>> request(String method, String path, [Map<String, dynamic>? body]) async {
    final headers = <String, String>{
      'Accept': 'application/json',
      'X-Himate-Locale': HimateI18n.activeLocale,
    };
    if (body != null) headers['Content-Type'] = 'application/json';
    late http.Response response;
    final uri = Uri.parse(path);
    final timeout = (path.startsWith('/api/v1/central/') || path.startsWith('/api/v1/dashboard/'))
        ? const Duration(milliseconds: 950)
        : const Duration(seconds: 4);
    if (method == 'POST') {
      response = await client.post(uri, headers: headers, body: jsonEncode(body ?? <String, dynamic>{})).timeout(timeout);
    } else if (method == 'PUT') {
      response = await client.put(uri, headers: headers, body: jsonEncode(body)).timeout(timeout);
    } else if (method == 'PATCH') {
      response = await client.patch(uri, headers: headers, body: jsonEncode(body)).timeout(timeout);
    } else if (method == 'DELETE') {
      response = await client.delete(uri, headers: headers).timeout(timeout);
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

Future<html.File?> pickBrowserFile(String accept) async {
  final input = html.FileUploadInputElement()..accept = accept;
  input.click();
  await input.onChange.first;
  final files = input.files;
  if (files == null || files.isEmpty) return null;
  return files.first;
}

Future<Uint8List> readBrowserFile(html.File file) async {
  final reader = html.FileReader();
  reader.readAsArrayBuffer(file);
  await reader.onLoad.first;
  final result = reader.result;
  if (result is ByteBuffer) return result.asUint8List();
  if (result is Uint8List) return result;
  throw StateError('Could not read selected file.');
}

void openBrowserDownload(String path) {
  html.window.open(path, '_blank');
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
  String anonymousLocale = 'en_US';

  Timer? _restoreFallback;
  String? _pendingDeepLink;
  bool _deepLinkHandled = false;

  @override
  void initState() {
    super.initState();
    unawaited(_loadPublishedBrandAssets());
    final storedLocale = html.window.localStorage['himate_locale'];
    if (storedLocale == 'hu_HU' || storedLocale == 'en_US') {
      anonymousLocale = storedLocale!;
    }
    final path = Uri.base.path;
    if (path == '/app' || path.startsWith('/app/')) {
      if (path != '/app') _pendingDeepLink = path;
      _restoreFallback = Timer(const Duration(seconds: 3), () {
        if (mounted && loading) {
          setState(() => loading = false);
        }
      });
      restore();
    } else if (path == '/login') {
      // Paint the login form immediately, then reuse any valid HttpOnly
      // session in the background. A remembered user never needs to retype
      // the password simply because the login URL was opened again.
      loading = false;
      unawaited(_restoreLoginSession());
    } else {
      loading = false;
    }
  }

  @override
  void dispose() {
    _restoreFallback?.cancel();
    super.dispose();
  }


  bool _can(String permission) {
    final current = user;
    if (current == null) return false;
    final roles = current['roles'];
    if (roles is List && roles.map((e) => e.toString()).contains('platform_admin')) return true;
    final permissions = current['permissions'];
    if (permissions is! List) return false;
    final values = permissions.map((e) => e.toString()).toSet();
    return values.contains('*') || values.contains(permission);
  }

  void _warmControlPlane() {
    if (user == null) return;
    final path = Uri.base.path;
    String? target;
    if (path == '/app' || path == '/app/') {
      if (_can('dashboard.read')) target = '/api/v1/dashboard/summary';
    } else if (path == '/app/partners') {
      if (_can('partners.read')) target = '/api/v1/central/partners?limit=24&offset=0';
    } else if (path.startsWith('/app/partners/')) {
      if (_can('partners.read')) {
        final id = path.substring('/app/partners/'.length).split('/').first;
        if (id.isNotEmpty) target = '/api/v1/central/partners/$id';
      }
    } else if (path == '/app/modules') {
      if (_can('catalog.read')) target = '/api/v1/central/modules';
    } else if (path == '/app/packages') {
      if (_can('billing.read')) target = '/api/v1/central/packages';
    } else if (path == '/app/finance') {
      if (_can('billing.read')) target = '/api/v1/central/finance';
    } else if (path == '/app/impact') {
      if (_can('impact.read') || _can('evidence.read') || _can('reports.read')) {
        target = '/api/v1/central/impact';
      }
    }
    if (target != null) {
      api.prefetch([target], maxAge: const Duration(seconds: 5));
    }
  }

  Future<void> _loadPublishedBrandAssets() async {
    try {
      final response = await api
          .get('/public/v1/cms/design', force: true)
          .timeout(const Duration(seconds: 2));
      final raw = response['design'];
      if (raw is Map) {
        applyPublishedBrandAssets(Map<String, dynamic>.from(raw));
        if (mounted) setState(() {});
      }
    } catch (_) {
      // Brand customization is optional; built-in assets remain the safe fallback.
    }
  }

  Future<void> _restoreLoginSession() async {
    try {
      final restored = await api
          .get('/api/v1/auth/me', force: true)
          .timeout(const Duration(seconds: 2));
      if (!mounted) return;
      user = restored;
      _warmControlPlane();
      setState(() {});
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        navigatorKey.currentState?.pushNamedAndRemoveUntil('/app', (route) => false);
      });
    } catch (_) {
      // No valid session is normal on the public login route; keep the form
      // visible and do not surface an error.
    }
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
      if (user != null) _warmControlPlane();
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

  String get effectiveLocaleCode {
    final preferred = user?['preferred_locale']?.toString();
    if (preferred == 'hu_HU' || preferred == 'en_US') return preferred!;
    return anonymousLocale;
  }

  void setAnonymousLocale(String value) {
    final normalized = value == 'hu_HU' ? 'hu_HU' : 'en_US';
    html.window.localStorage['himate_locale'] = normalized;
    HimateI18n.activeLocale = normalized;
    api.clearCache();
    if (mounted) setState(() => anonymousLocale = normalized);
  }

  void updateSignedInUser(Map<String, dynamic> next) {
    user = Map<String, dynamic>.from(next);
    final preferred = user?['preferred_locale']?.toString();
    if (preferred == 'hu_HU' || preferred == 'en_US') {
      anonymousLocale = preferred!;
      HimateI18n.activeLocale = preferred;
      html.window.localStorage['himate_locale'] = preferred;
    }
    api.clearCache();
    if (mounted) setState(() {});
  }

  Widget loginPage() => LoginPage(
        onLogin: login,
        onRequestPasswordReset: requestPasswordReset,
        onConfirmPasswordReset: confirmPasswordReset,
        localeCode: effectiveLocaleCode,
        onLocaleChanged: setAnonymousLocale,
      );

  Future<void> login(String email, String password, bool remember) async {
    var response = await api.post('/api/v1/auth/login', {
      'email': email,
      'password': password,
      'remember': remember,
    });
    if (response['mfa_required'] == true) {
      final context = navigatorKey.currentContext;
      if (context == null) throw Exception('MFA dialog is unavailable.');
      final code = await promptMfaCode(context, response);
      if (code == null) throw Exception('Multi-factor authentication was cancelled.');
      response = await api.post('/api/v1/auth/mfa/verify', {
        'challenge_id': response['challenge_id'],
        'code': code,
      });
    }
    user = response;
    final preferred = user?['preferred_locale']?.toString();
    if (preferred == 'hu_HU' || preferred == 'en_US') {
      anonymousLocale = preferred!;
      html.window.localStorage['himate_locale'] = preferred;
    }
    if (!mounted) return;
    setState(() {});
    _warmControlPlane();
    final target = _pendingDeepLink;
    navigatorKey.currentState?.pushNamedAndRemoveUntil('/app', (route) => false);
    if (target != null && !_deepLinkHandled) {
      _deepLinkHandled = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        navigatorKey.currentState?.pushNamed(target);
      });
    }
  }

  Future<void> requestPasswordReset(String email) async {
    await api.post('/api/v1/auth/password-reset/request', {'email': email.trim()});
  }

  Future<void> confirmPasswordReset(String token, String newPassword) async {
    await api.post('/api/v1/auth/password-reset/confirm', {
      'token': token.trim(),
      'new_password': newPassword,
    });
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
            mainAxisSize: MainAxisSize.max,
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
    HimateI18n.activeLocale = effectiveLocaleCode;
    final path = Uri.base.path;
    final initial = path == '/app' || path.startsWith('/app/') ? '/app' : '/login';

    return MaterialApp(
      navigatorKey: navigatorKey,
      debugShowCheckedModeBanner: false,
      title: 'HIMATE System',
      theme: buildBrandTheme(),
      locale: himateLocaleFromCode(effectiveLocaleCode),
      supportedLocales: himateSupportedLocales,
      localizationsDelegates: const [
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      initialRoute: initial,
      routes: {
        // The public login route must never be gated by protected-route
        // session restoration. This guarantees that a failed /auth/me request
        // cannot strand visitors behind a global spinner.
        '/login': (_) => user == null
            ? loginPage()
            : _SignedInRedirect(onContinue: () {
                navigatorKey.currentState?.pushNamedAndRemoveUntil('/app', (route) => false);
              }),
        '/app': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout),
        '/app/partners': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 1),
        '/app/modules': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 2),
        '/app/packages': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 3),
        '/app/finance': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 4),
        '/app/impact': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 5),
        '/app/website': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 6),
        '/app/system': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 7),
        '/app/admin': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 8),
        '/app/archives': (_) => loading
            ? loadingScreen()
            : user == null
                ? loginPage()
                : Shell(api: api, user: user!, onUserChanged: updateSignedInUser, onLogout: logout, initialSelected: 9),
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
                  ? loginPage()
                  : PartnerRouteLoader(api: api, partnerId: partnerId, initialSection: section),
        );
      },
      onUnknownRoute: (_) => MaterialPageRoute(
        settings: const RouteSettings(name: '/login'),
        builder: (_) => user == null
            ? loginPage()
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
      future: api.get('/api/v1/partners/$partnerId', maxAge: const Duration(seconds: 20)),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done && snapshot.data == null) {
          return const Content(
            eyebrow: 'PLATFORM OPERATIONS',
            title: 'System & Operations',
            subtitle: 'Independent services behind one authenticated public gateway.',
            child: _MessageCard(
              icon: Icons.sync_rounded,
              title: 'Refreshing operations data',
              message: 'No cached operations snapshot is available yet. The page is ready and data will appear automatically.',
            ),
          );
        }
        if (snapshot.hasError || snapshot.data == null) {
          return Scaffold(
            backgroundColor: brandIvory,
            appBar: AppBar(
              leading: IconButton(
                tooltip: uiLiteral('Back to Partners'),
                onPressed: () => Navigator.of(context).pushNamedAndRemoveUntil('/app/partners', (route) => false),
                icon: const Icon(Icons.arrow_back_rounded),
              ),
            ),
            body: Padding(
              padding: const EdgeInsets.all(24),
              child: _MessageCard(icon: Icons.error_outline_rounded, title: 'Partner could not be opened', message: '${snapshot.error ?? 'Partner not found'}'),
            ),
          );
        }
        return PartnerWorkspace(
          api: api,
          partner: snapshot.data!,
          initialSection: initialSection,
          onBack: () => Navigator.of(context).pushNamedAndRemoveUntil('/app/partners', (route) => false),
        );
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
                  const LText(
                    'You are already signed in.',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: brandNavy, fontSize: 22, fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 8),
                  const LText(
                    'Continue to the HIMATE administration platform.',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: brandTextSoft),
                  ),
                  const SizedBox(height: 20),
                  FilledButton.icon(
                    onPressed: onContinue,
                    icon: const Icon(Icons.arrow_forward_rounded),
                    label: const LText('Open admin platform'),
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
  const LoginPage({
    required this.onLogin,
    required this.onRequestPasswordReset,
    required this.onConfirmPasswordReset,
    required this.localeCode,
    required this.onLocaleChanged,
    super.key,
  });
  final Future<void> Function(String email, String password, bool remember) onLogin;
  final Future<void> Function(String email) onRequestPasswordReset;
  final Future<void> Function(String token, String newPassword) onConfirmPasswordReset;
  final String localeCode;
  final ValueChanged<String> onLocaleChanged;

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
  void initState() {
    super.initState();
    final token = Uri.base.queryParameters['reset_token']?.trim() ?? '';
    if (token.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) unawaited(showResetPassword(token));
      });
    }
  }

  Future<void> forgotPassword() async {
    final controller = TextEditingController(text: email.text.trim());
    final submitted = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: LText(tr(dialogContext, 'resetRequestTitle')),
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              LText(tr(dialogContext, 'resetRequestBody')),
              const SizedBox(height: 18),
              TextField(
                controller: controller,
                keyboardType: TextInputType.emailAddress,
                autofocus: true,
                decoration: InputDecoration(labelText: tr(dialogContext, 'emailAddress')),
                onSubmitted: (value) => Navigator.of(dialogContext).pop(value.trim()),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(dialogContext).pop(), child: const LText('Cancel')),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(controller.text.trim()),
            child: LText(tr(dialogContext, 'resetSend')),
          ),
        ],
      ),
    );
    controller.dispose();
    if (submitted == null || submitted.isEmpty || !mounted) return;
    try {
      await widget.onRequestPasswordReset(submitted);
      if (!mounted) return;
      info(tr(context, 'resetSent'));
    } catch (e) {
      if (!mounted) return;
      info(e.toString());
    }
  }

  Future<void> showResetPassword(String token) async {
    final first = TextEditingController();
    final second = TextEditingController();
    final values = await showDialog<List<String>>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: LText(tr(dialogContext, 'resetNewPasswordTitle')),
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: first,
                obscureText: true,
                autofocus: true,
                decoration: InputDecoration(labelText: tr(dialogContext, 'resetNewPassword')),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: second,
                obscureText: true,
                decoration: InputDecoration(labelText: tr(dialogContext, 'resetConfirmPassword')),
                onSubmitted: (_) => Navigator.of(dialogContext).pop([first.text, second.text]),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(dialogContext).pop(), child: const LText('Cancel')),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop([first.text, second.text]),
            child: LText(tr(dialogContext, 'resetConfirmAction')),
          ),
        ],
      ),
    );
    first.dispose();
    second.dispose();
    if (values == null || values.length != 2 || !mounted) return;
    if (values[0] != values[1]) {
      info(tr(context, 'resetMismatch'));
      return;
    }
    try {
      await widget.onConfirmPasswordReset(token, values[0]);
      if (!mounted) return;
      html.window.history.replaceState(null, 'HIMATE', '/login');
      password.clear();
      info(tr(context, 'resetDone'));
    } catch (e) {
      if (!mounted) return;
      info(e.toString());
    }
  }

  @override
  void dispose() {
    email.dispose();
    password.dispose();
    super.dispose();
  }

  Future<void> submit() async {
    if (email.text.trim().isEmpty || password.text.isEmpty) {
      setState(() => error = tr(context, 'enterCredentials'));
      return;
    }
    setState(() { busy = true; error = null; });
    try {
      await widget.onLogin(email.text.trim(), password.text, remember);
      TextInput.finishAutofillContext(shouldSave: true);
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  void info(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: LText(message), behavior: SnackBarBehavior.floating, backgroundColor: brandNavy),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: brandNavyDeep,
      floatingActionButtonLocation: FloatingActionButtonLocation.endTop,
      floatingActionButton: SafeArea(
        child: _LoginLanguageSelector(
          value: widget.localeCode,
          onChanged: widget.onLocaleChanged,
        ),
      ),
      body: LayoutBuilder(
        builder: (context, constraints) {
          final compact = useCompactLoginForSize(Size(constraints.maxWidth, constraints.maxHeight));
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
                        onForgot: forgotPassword,
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
                        onForgot: forgotPassword,
                      ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _LoginLanguageSelector extends StatelessWidget {
  const _LoginLanguageSelector({required this.value, required this.onChanged});
  final String value;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: brandNavy.withOpacity(.84),
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10),
        child: DropdownButtonHideUnderline(
          child: DropdownButton<String>(
            value: value == 'hu_HU' ? 'hu_HU' : 'en_US',
            dropdownColor: brandNavy,
            iconEnabledColor: brandGold,
            style: const TextStyle(color: brandWhite, fontWeight: FontWeight.w700),
            items: [
              DropdownMenuItem(value: 'en_US', child: LText(HimateI18n.text('en_US', 'englishUS'))),
              DropdownMenuItem(value: 'hu_HU', child: LText(HimateI18n.text('hu_HU', 'hungarian'))),
            ],
            onChanged: (next) {
              if (next != null) onChanged(next);
            },
          ),
        ),
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
  });
  final TextEditingController email, password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot;
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
                HimateLogo(onDark: true, width: logoWidth, assetUrl: himateLoginWordmarkUrl),
                const Spacer(),
                LText(
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
  });
  final TextEditingController email, password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    final narrow = MediaQuery.of(context).size.width < 520;
    return SingleChildScrollView(
      padding: EdgeInsets.fromLTRB(narrow ? 18 : 30, 24, narrow ? 18 : 30, 34),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          HimateLogo(onDark: true, width: 188, assetUrl: himateLoginWordmarkUrl),
          SizedBox(height: narrow ? 58 : 90),
          LText(
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
  });

  final TextEditingController email, password;
  final bool busy, obscure, remember;
  final String? error;
  final VoidCallback onTogglePassword, onSubmit, onForgot;
  final ValueChanged<bool?> onRemember;

  @override
  Widget build(BuildContext context) {
    return AutofillGroup(
      child: Container(
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
          LText(
            tr(context, 'welcomeBack'),
            textAlign: TextAlign.center,
            style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: 39, fontWeight: FontWeight.w700, height: 1),
          ),
          const SizedBox(height: 8),
          LText(tr(context, 'signInSubtitle'), textAlign: TextAlign.center, style: GoogleFonts.inter(color: brandSteel, fontSize: 15.5)),
          const SizedBox(height: 28),
          TextField(
            controller: email,
            keyboardType: TextInputType.emailAddress,
            autofillHints: const [AutofillHints.username, AutofillHints.email],
            style: GoogleFonts.inter(color: brandCharcoal, fontSize: 16),
            decoration: InputDecoration(hintText: tr(context, 'emailAddress'), prefixIcon: const Icon(Icons.mail_outline_rounded, size: 22)),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: password,
            obscureText: obscure,
            autofillHints: const [AutofillHints.password],
            onSubmitted: (_) => onSubmit(),
            style: GoogleFonts.inter(color: brandCharcoal, fontSize: 16),
            decoration: InputDecoration(
              hintText: tr(context, 'password'),
              prefixIcon: const Icon(Icons.lock_outline_rounded, size: 22),
              suffixIcon: IconButton(
                onPressed: onTogglePassword,
                tooltip: obscure ? tr(context, 'showPassword') : tr(context, 'hidePassword'),
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
                  LText(tr(context, 'rememberMe'), style: GoogleFonts.inter(color: brandNavy, fontSize: 14.2)),
                ]),
              ),
              const Spacer(),
              TextButton(
                onPressed: onForgot,
                style: TextButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 5)),
                child: LText(tr(context, 'forgotPassword'), style: GoogleFonts.inter(fontSize: 14.2, fontWeight: FontWeight.w600)),
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
                Expanded(child: LText(error!, style: GoogleFonts.inter(color: brandDanger, fontSize: 11.5))),
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
                      LText(tr(context, 'signIn'), style: GoogleFonts.inter(fontWeight: FontWeight.w700)),
                      const SizedBox(width: 16),
                      const Icon(Icons.arrow_forward_rounded, size: 21),
                    ]),
            ),
          ),
          const SizedBox(height: 24),
          Row(mainAxisAlignment: MainAxisAlignment.center, children: [
            const Icon(Icons.verified_user_outlined, color: brandGold, size: 17),
            const SizedBox(width: 7),
            LText('Secure  •  Trusted  •  Built for a brighter tomorrow', style: GoogleFonts.inter(color: brandTextSoft, fontSize: 11.8)),
          ]),
        ],
      ),
      ),
    );
  }
}

const double _himateWordmarkAspectRatio = 2048 / 682;

class HimateLogo extends StatelessWidget {
  const HimateLogo({
    super.key,
    this.width = 300,
    this.compact = false,
    this.shadow = true,
    this.onDark = false,
    this.assetUrl,
  });

  final double width;
  final bool compact;
  final bool shadow;
  final bool onDark;
  final String? assetUrl;

  @override
  Widget build(BuildContext context) {
    final targetWidth = compact ? width : width;
    final targetHeight = compact ? width : width / _himateWordmarkAspectRatio;
    final resolvedAssetUrl = assetUrl ?? (compact ? himateRuntimeIconUrl : himateRuntimeWordmarkUrl);

    final image = Image.network(
      resolvedAssetUrl,
      width: targetWidth,
      height: targetHeight,
      fit: BoxFit.contain,
      filterQuality: FilterQuality.high,
      errorBuilder: (_, __, ___) => Center(
        child: LText(
          compact ? 'H' : 'HIMATE',
          style: TextStyle(
            fontSize: compact ? targetHeight * .48 : targetHeight * .34,
            color: onDark ? brandGold : brandNavy,
            fontWeight: FontWeight.w800,
            letterSpacing: compact ? 0 : 3.0,
          ),
        ),
      ),
    );

    return SizedBox(
      width: targetWidth,
      height: targetHeight,
      child: DecoratedBox(
        decoration: BoxDecoration(
          boxShadow: shadow
              ? [
                  BoxShadow(
                    color: Colors.black.withValues(alpha: .12),
                    blurRadius: 20,
                    offset: const Offset(0, 8),
                  ),
                ]
              : const [],
        ),
        child: image,
      ),
    );
  }
}

class BrandMark extends StatelessWidget {
  const BrandMark({required this.size, super.key});
  final double size;

  @override
  Widget build(BuildContext context) {
    return SizedBox.square(
      dimension: size,
      child: Image.network(
        himateRuntimeIconUrl,
        fit: BoxFit.contain,
        filterQuality: FilterQuality.high,
        errorBuilder: (_, __, ___) => DecoratedBox(
          decoration: BoxDecoration(
            color: brandNavy,
            borderRadius: BorderRadius.circular(size * .24),
            border: Border.all(color: brandGold.withValues(alpha: .8)),
          ),
          child: Center(
            child: LText(
              'H',
              style: TextStyle(
                color: brandGold,
                fontSize: size * .52,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
        ),
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
    return LText(
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

bool useCompactLoginForSize(Size size) => size.width < 820 || size.height < 720;

int responsiveGridColumnsForWidth(double width) {
  if (width < 620) return 1;
  if (width < 980) return 2;
  return 4;
}

bool shouldStackContentActions(double width, int actionCount) =>
    width < 920 || (actionCount > 2 && width < 1180);

class Shell extends StatefulWidget {
  const Shell({
    required this.api,
    required this.user,
    required this.onUserChanged,
    required this.onLogout,
    this.initialSelected = 0,
    super.key,
  });
  final Api api;
  final Map<String, dynamic> user;
  final ValueChanged<Map<String, dynamic>> onUserChanged;
  final Future<void> Function() onLogout;
  final int initialSelected;

  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  late int selected;
  bool collapsed = false;
  final Map<int, Widget> _pageCache = <int, Widget>{};

  @override
  void initState() {
    super.initState();
    selected = widget.initialSelected.clamp(0, navCount - 1);
  }

  static const int navCount = 10;

  List<NavSpec> navFor(BuildContext context) => <NavSpec>[
    NavSpec(tr(context,'nav.dashboard'), Icons.dashboard_outlined, tr(context,'nav.dashboardSub')),
    NavSpec(tr(context,'nav.partners'), Icons.groups_2_outlined, tr(context,'nav.partnersSub')),
    const NavSpec('Modules', Icons.hub_outlined, 'Registry, dependencies & partner usage'),
    const NavSpec('Packages', Icons.inventory_2_outlined, 'Central plans, prices & included modules'),
    NavSpec(tr(context,'nav.finance'), Icons.account_balance_wallet_outlined, tr(context,'nav.financeSub')),
    NavSpec(tr(context,'nav.impact'), Icons.show_chart_rounded, tr(context,'nav.impactSub')),
    NavSpec(tr(context,'nav.website'), Icons.campaign_outlined, tr(context,'nav.websiteSub')),
    NavSpec(tr(context,'nav.system'), Icons.settings_suggest_outlined, tr(context,'nav.systemSub')),
    NavSpec(tr(context,'nav.admin'), Icons.admin_panel_settings_outlined, tr(context,'nav.adminSub')),
    const NavSpec('Archives', Icons.inventory_2_outlined, 'Seven-year compliance evidence vault'),
  ];

  void accountAction(BuildContext context, String value) {
    if (value == 'profile') {
      unawaited(showAccountProfileDialog(
        context,
        api: widget.api,
        currentUser: widget.user,
        onUserChanged: widget.onUserChanged,
      ));
    } else if (value == 'logout') {
      unawaited(widget.onLogout());
    }
  }

  Future<void> openGlobalSearch(BuildContext context) async {
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (_) => _GlobalSearchDialog(api: widget.api),
    );
    if (!context.mounted || result == null) return;
    final deepLink = '${result['deep_link'] ?? ''}'.trim();
    if (deepLink.startsWith('/app/partners/')) {
      Navigator.of(context).pushNamed(deepLink);
      return;
    }
    final resource = '${result['resource'] ?? 'workspace'}';
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: LText('${uiLiteral('Result workspace')}: $resource'),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  bool can(String permission) {
    final roles = widget.user['roles'];
    if (roles is List && roles.map((e) => e.toString()).contains('platform_admin')) return true;
    final permissions = widget.user['permissions'];
    if (permissions is! List) return false;
    final values = permissions.map((e) => e.toString()).toSet();
    return values.contains('*') || values.contains(permission);
  }

  String _routeForIndex(int index) => switch (index) {
    0 => '/app',
    1 => '/app/partners',
    2 => '/app/modules',
    3 => '/app/packages',
    4 => '/app/finance',
    5 => '/app/impact',
    6 => '/app/website',
    7 => '/app/system',
    8 => '/app/admin',
    9 => '/app/archives',
    _ => '/app',
  };

  void _selectNav(int index) {
    if (!visibleNavIndexes().contains(index) || selected == index) return;
    setState(() => selected = index);
    final route = _routeForIndex(index);
    if (Uri.base.path != route) {
      html.window.history.replaceState(null, '', route);
    }
  }

  List<int> visibleNavIndexes() {
    final indexes = <int>[];
    if (can('dashboard.read')) indexes.add(0);
    if (can('partners.read')) indexes.add(1);
    if (can('catalog.read')) indexes.add(2);
    if (can('billing.read')) indexes.add(3);
    if (can('billing.read')) indexes.add(4);
    if (can('impact.read') || can('reports.read') || can('evidence.read')) indexes.add(5);
    if (can('cms.read') || can('contact.read')) indexes.add(6);
    if (can('health.read') || can('provisioning.read') || can('environments.read') || can('connectors.read') || can('backups.read')) indexes.add(7);
    if (can('administration.read') || can('audit.read')) indexes.add(8);
    if (can('audit.read')) indexes.add(9);
    if (indexes.isEmpty) indexes.add(0);
    return indexes;
  }

  Widget _pageForIndex(int index) {
    switch (index) {
      case 0: return DashboardPage(
        api: widget.api,
        canNavigate: (index) => visibleNavIndexes().contains(index),
        onNavigate: (index) {
          if (!visibleNavIndexes().contains(index)) return;
          _selectNav(index);
        },
      );
      case 1: return PartnersPage(api: widget.api);
      case 2: return ModuleControlPlanePage(api: widget.api);
      case 3: return PackagesPage(api: widget.api);
      case 4: return FinancePage(api: widget.api);
      case 5: return ImpactPage(api: widget.api);
      case 6: return WebsiteMarketingPage(api: widget.api);
      case 7: return SystemPage(api: widget.api);
      case 8: return AdministrationPage(api: widget.api, user: widget.user);
      case 9: return ComplianceArchivesPage(api: widget.api);
      default: return const SizedBox.shrink();
    }
  }

  Widget pageStack() {
    final visible = visibleNavIndexes().toSet();
    Widget cachedPage(int index) => _pageCache.putIfAbsent(index, () => _pageForIndex(index));
    return IndexedStack(
      index: selected,
      sizing: StackFit.expand,
      children: [
        for (var index = 0; index < navCount; index++)
          visible.contains(index) && (index == selected || _pageCache.containsKey(index))
              ? cachedPage(index)
              : const SizedBox.shrink(),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final layoutMode = shellLayoutForWidth(constraints.maxWidth);
        final mobile = layoutMode == ShellLayoutMode.mobile;
        final tablet = layoutMode == ShellLayoutMode.tablet;
        final visibleIndexes = visibleNavIndexes();
        final allNav = navFor(context);
        final visibleNav = <NavSpec>[for (final index in visibleIndexes) allNav[index]];
        final visibleSelected = visibleIndexes.indexOf(selected).clamp(0, visibleIndexes.length - 1);
        if (mobile) {
          return Scaffold(
            appBar: AppBar(
              toolbarHeight: 64,
              titleSpacing: 12,
              title: const HimateLogo(width: 170),
              actions: [
                IconButton(
                  tooltip: uiLiteral('Global search'),
                  onPressed: () => unawaited(openGlobalSearch(context)),
                  icon: const Icon(Icons.search_rounded),
                ),
                NotificationCenterButton(api: widget.api),
                PopupMenuButton<String>(
                  tooltip: tr(context,'account'),
                  onSelected: (value) => accountAction(context,value),
                  itemBuilder: (_) => [
                    PopupMenuItem(value:'profile',child:Row(children:[const Icon(Icons.account_circle_outlined,size:18),const SizedBox(width:10),LText(tr(context,'profile'))])),
                    PopupMenuItem(value:'logout',child:Row(children:[const Icon(Icons.logout_rounded,size:18),const SizedBox(width:10),LText(tr(context,'signOut'))])),
                  ],
                  child: Padding(padding: const EdgeInsets.symmetric(horizontal: 14), child: _Avatar(name: '${widget.user['name'] ?? 'Admin User'}')),
                ),
              ],
            ),
            drawer: Drawer(
              backgroundColor: brandNavyDeep,
              child: SafeArea(
                child: _SidebarContent(
                  nav: visibleNav,
                  selected: visibleSelected,
                  collapsed: false,
                  user: widget.user,
                  onSelect: (i) { _selectNav(visibleIndexes[i]); Navigator.pop(context); },
                  onToggle: null,
                  onLogout: widget.onLogout,
                ),
              ),
            ),
            body: pageStack(),
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
                    nav: visibleNav,
                    selected: visibleSelected,
                    collapsed: tablet || collapsed,
                    user: widget.user,
                    onSelect: (i) => _selectNav(visibleIndexes[i]),
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
                      decoration: const BoxDecoration(color: brandSurface, border: Border(bottom: BorderSide(color: brandMist))),
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
                                    onTap: () => unawaited(openGlobalSearch(context)),
                                    decoration: InputDecoration(isDense: true, hintText: uiLiteral('Search anywhere...'), prefixIcon: Icon(Icons.search_rounded, size: 19)),
                                  ),
                                ),
                              ),
                            )
                          else
                            const Spacer(),
                          if (tablet)
                            IconButton(
                              tooltip: uiLiteral('Global search'),
                              onPressed: () => unawaited(openGlobalSearch(context)),
                              icon: const Icon(Icons.search_rounded),
                            ),
                          const SizedBox(width: 18),
                          NotificationCenterButton(api: widget.api),
                          const SizedBox(width: 8),
                          PopupMenuButton<String>(
                            tooltip: tr(context,'account'),
                            onSelected: (value) => accountAction(context,value),
                            itemBuilder: (_) => [
                              PopupMenuItem(value:'profile',child:Row(children:[const Icon(Icons.account_circle_outlined,size:18),const SizedBox(width:10),LText(tr(context,'profile'))])),
                              PopupMenuItem(value:'logout',child:Row(children:[const Icon(Icons.logout_rounded,size:18),const SizedBox(width:10),LText(tr(context,'signOut'))])),
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
                                        child: LText(
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
                    Expanded(child: ColoredBox(color: brandIvory, child: pageStack())),
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
                            LText('${user['name'] ?? 'Admin User'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: GoogleFonts.inter(color: brandWhite, fontWeight: FontWeight.w700, fontSize: 11.5)),
                            const SizedBox(height: 2),
                            LText('${user['email'] ?? 'System Administrator'}', maxLines: 1, overflow: TextOverflow.ellipsis, style: GoogleFonts.inter(color: const Color(0xFF91A4B8), fontSize: 9.5)),
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
                            child: LText(
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
      child: LText(initials, style: const TextStyle(color: brandWhite, fontWeight: FontWeight.w700, fontSize: 10)),
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
                    LText('Workspace prepared', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 17)),
                    SizedBox(height: 7),
                    LText('The brand system and responsive shell are ready. Functional implementation remains in its scheduled START cycle.', style: TextStyle(color: brandTextSoft, height: 1.5)),
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

class _GlobalSearchDialog extends StatefulWidget {
  const _GlobalSearchDialog({required this.api});
  final Api api;

  @override
  State<_GlobalSearchDialog> createState() => _GlobalSearchDialogState();
}

class _GlobalSearchDialogState extends State<_GlobalSearchDialog> {
  final controller = TextEditingController();
  Timer? debounce;
  List<Map<String, dynamic>> results = <Map<String, dynamic>>[];
  bool loading = false;
  String? error;
  int generation = 0;

  @override
  void dispose() {
    debounce?.cancel();
    controller.dispose();
    super.dispose();
  }

  void changed(String value) {
    debounce?.cancel();
    final query = value.trim();
    if (query.length < 2) {
      setState(() {
        results = <Map<String, dynamic>>[];
        loading = false;
        error = null;
      });
      return;
    }
    debounce = Timer(const Duration(milliseconds: 240), () => search(query));
  }

  Future<void> search(String query) async {
    final current = ++generation;
    setState(() {
      loading = true;
      error = null;
    });
    try {
      final uri = Uri(path: '/api/v1/search', queryParameters: {'q': query, 'limit': '5'});
      final response = await widget.api.get(uri.toString(), force: true);
      if (!mounted || current != generation) return;
      setState(() {
        results = items(response);
        loading = false;
      });
    } catch (e) {
      if (!mounted || current != generation) return;
      setState(() {
        loading = false;
        error = e.toString();
      });
    }
  }

  IconData iconFor(String resource) {
    switch (resource) {
      case 'partners':
        return Icons.apartment_rounded;
      case 'catalog':
        return Icons.widgets_outlined;
      case 'contact':
        return Icons.mark_email_read_outlined;
      case 'cms':
        return Icons.language_rounded;
      case 'administration':
        return Icons.manage_accounts_outlined;
      case 'audit':
        return Icons.fact_check_outlined;
      default:
        return Icons.search_rounded;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 32),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 760, maxHeight: 620),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(22, 20, 22, 18),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                children: [
                  const Icon(Icons.manage_search_rounded, color: brandGold),
                  const SizedBox(width: 10),
                  Expanded(
                    child: LText(
                      uiLiteral('Global search'),
                      style: GoogleFonts.cormorantGaramond(fontSize: 24, fontWeight: FontWeight.w700, color: brandNavy),
                    ),
                  ),
                  IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close_rounded)),
                ],
              ),
              const SizedBox(height: 12),
              TextField(
                controller: controller,
                autofocus: true,
                onChanged: changed,
                decoration: InputDecoration(
                  hintText: uiLiteral('Search partners, modules, contacts, CMS and administration...'),
                  prefixIcon: const Icon(Icons.search_rounded),
                ),
              ),
              const SizedBox(height: 14),
              Flexible(
                child: Builder(
                  builder: (context) {
                    if (loading && results.isEmpty) {
                      return const Center(child: Padding(padding: EdgeInsets.all(30), child: CircularProgressIndicator()));
                    }
                    if (error != null) {
                      return _MessageCard(
                        icon: Icons.cloud_off_outlined,
                        title: uiLiteral('Search is temporarily unavailable'),
                        message: error!,
                      );
                    }
                    if (controller.text.trim().length < 2) {
                      return Center(
                        child: Padding(
                          padding: const EdgeInsets.all(30),
                          child: LText(
                            uiLiteral('Type at least two characters. Results are restricted to workspaces you are allowed to read.'),
                            textAlign: TextAlign.center,
                            style: GoogleFonts.inter(color: brandTextSoft, fontSize: 12),
                          ),
                        ),
                      );
                    }
                    if (results.isEmpty) {
                      return Center(
                        child: Padding(
                          padding: const EdgeInsets.all(30),
                          child: LText(uiLiteral('No permitted results found.'), style: GoogleFonts.inter(color: brandTextSoft)),
                        ),
                      );
                    }
                    return ListView.separated(
                      shrinkWrap: true,
                      itemCount: results.length,
                      separatorBuilder: (_, __) => const Divider(height: 1),
                      itemBuilder: (context, index) {
                        final result = results[index];
                        final resource = '${result['resource'] ?? ''}';
                        final deepLink = '${result['deep_link'] ?? ''}';
                        return ListTile(
                          leading: CircleAvatar(
                            backgroundColor: brandNavy.withOpacity(.07),
                            child: Icon(iconFor(resource), color: brandNavy, size: 19),
                          ),
                          title: LText(
                            '${result['title'] ?? result['id'] ?? ''}',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: GoogleFonts.inter(fontWeight: FontWeight.w600, color: brandNavy, fontSize: 12.5),
                          ),
                          subtitle: LText(
                            '${result['subtitle'] ?? resource}',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: GoogleFonts.inter(color: brandTextSoft, fontSize: 10.5),
                          ),
                          trailing: deepLink.startsWith('/app/partners/')
                              ? const Icon(Icons.open_in_new_rounded, size: 17)
                              : LText(resource.toUpperCase(), style: GoogleFonts.inter(fontSize: 8.5, fontWeight: FontWeight.w700, color: brandSteel)),
                          onTap: () => Navigator.pop(context, result),
                        );
                      },
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

String _dashboardCompact(dynamic value) {
  final number = value is num ? value.toDouble() : double.tryParse('$value') ?? 0;
  return intl.NumberFormat.compact(locale: HimateI18n.activeLocale == 'hu_HU' ? 'hu_HU' : 'en_US').format(number);
}

String _dashboardMoney(String currency, dynamic value) {
  final amount = value is num ? value.toDouble() : double.tryParse('$value') ?? 0;
  try {
    return intl.NumberFormat.simpleCurrency(
      name: currency.isEmpty ? null : currency,
      locale: HimateI18n.activeLocale == 'hu_HU' ? 'hu_HU' : 'en_US',
      decimalDigits: amount.abs() >= 1000 ? 0 : 2,
    ).format(amount);
  } catch (_) {
    return '$currency ${intl.NumberFormat('#,##0.##').format(amount)}'.trim();
  }
}

class DashboardPage extends StatelessWidget {
  const DashboardPage({
    required this.api,
    required this.canNavigate,
    required this.onNavigate,
    super.key,
  });
  final Api api;
  final bool Function(int index) canNavigate;
  final ValueChanged<int> onNavigate;

  @override
  Widget build(BuildContext context) {
    final year = DateTime.now().toUtc().year;
    final path = Uri(path: '/api/v1/dashboard/summary', queryParameters: {'year': '$year'}).toString();
    return FutureBuilder<Map<String, dynamic>>(
      future: api.get(path),
      initialData: api.peek(path),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting && snapshot.data == null) {
          return Content(
            eyebrow: uiLiteral('Loading live control-plane data'),
            title: uiLiteral('Welcome to HIMATE System'),
            subtitle: uiLiteral('The Go read model is assembling the first usable dashboard payload.'),
            child: ResponsiveKpiGrid(children: [
              Kpi(label: uiLiteral('Active Partners'), value: '—', note: uiLiteral('Loading authoritative value'), icon: Icons.groups_2_outlined, accent: const Color(0xFF0B5DA8)),
              Kpi(label: uiLiteral('Active Programs'), value: '—', note: uiLiteral('Loading authoritative value'), icon: Icons.description_outlined, accent: brandNavy),
              Kpi(label: uiLiteral('Revenue (YTD)'), value: '—', note: uiLiteral('Loading authoritative value'), icon: Icons.bar_chart_rounded, accent: brandGold),
              Kpi(label: uiLiteral('People Reached'), value: '—', note: uiLiteral('Loading authoritative value'), icon: Icons.groups_rounded, accent: brandNavy),
            ]),
          );
        }
        if (snapshot.hasError && snapshot.data == null) {
          return Content(
            title: uiLiteral('Welcome to HIMATE System'),
            subtitle: uiLiteral('Manage partners, programs and cultural impact — all in one place.'),
            child: _MessageCard(icon: Icons.cloud_off_outlined, title: uiLiteral('Dashboard data is temporarily unavailable'), message: '${snapshot.error}'),
          );
        }
        final d=snapshot.data??<String,dynamic>{};
        final p=Map<String,dynamic>.from(d['partners']??<String,dynamic>{});
        final m=Map<String,dynamic>.from(d['modules']??<String,dynamic>{});
        final billing=Map<String,dynamic>.from(d['billing']??<String,dynamic>{});
        final impact=Map<String,dynamic>.from(d['impact']??<String,dynamic>{});
        final activity=Map<String,dynamic>.from(d['activity']??<String,dynamic>{});
        final billingAuthorized=billing['authorized']!=false;
        final impactAuthorized=impact['authorized']!=false;
        final impactHasData=impact['has_data']==true;
        final revenueRows=items(billing);
        String revenueValue=billingAuthorized?'0':uiLiteral('Restricted');
        String revenueNote=billingAuthorized
            ?uiLiteral('No paid revenue recorded this year')
            :uiLiteral('Billing permission required');
        if(billingAuthorized&&revenueRows.length==1){
          final row=revenueRows.first;
          final currency='${row['currency']??''}';
          revenueValue=_dashboardMoney(currency,row['revenue_ytd']);
          revenueNote=uiLiteral('Paid activation + recurring revenue');
        }else if(billingAuthorized&&revenueRows.length>1){
          revenueValue=uiLiteral('Mixed');
          revenueNote=revenueRows
              .map((row)=>_dashboardMoney('${row['currency']??''}',row['revenue_ytd']))
              .join(' · ');
        }
        final people=impact['people_reached_ytd']??0;
        final peopleValue=impactAuthorized?_dashboardCompact(people):uiLiteral('Restricted');
        final peopleNote=impactAuthorized
            ?uiLiteral('Verified attendance metric · YTD')
            :uiLiteral('Impact permission required');
        final hour=DateTime.now().hour;
        final greeting=hour<12?uiLiteral('Good morning,'):hour<18?uiLiteral('Good afternoon,'):uiLiteral('Good evening,');
        return Content(
          eyebrow:greeting,
          title:uiLiteral('Welcome to HIMATE System'),
          subtitle:uiLiteral('Manage partners, programs, and cultural impact — all in one place.'),
          child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
            LayoutBuilder(builder:(context,c){
              final gap=14.0;
              final cols=c.maxWidth<620?2:4;
              final w=(c.maxWidth-gap*(cols-1))/cols;
              return Wrap(spacing:gap,runSpacing:gap,children:[
                SizedBox(width:w,child:Kpi(label:uiLiteral('Active Partners'),value:'${p['live']??0}',note:uiLiteral('${p['total']??0} partner records'),icon:Icons.groups_2_outlined,accent:const Color(0xFF0B5DA8),onTap:canNavigate(1)?()=>onNavigate(1):null)),
                SizedBox(width:w,child:Kpi(label:uiLiteral('Active Programs'),value:'${m['catalog_total']??0}',note:uiLiteral('Available program modules'),icon:Icons.description_outlined,accent:brandNavy,onTap:canNavigate(2)?()=>onNavigate(2):null)),
                SizedBox(width:w,child:Kpi(label:uiLiteral('Revenue (YTD)'),value:revenueValue,note:revenueNote,icon:Icons.bar_chart_rounded,accent:brandGold,onTap:canNavigate(4)?()=>onNavigate(4):null)),
                SizedBox(width:w,child:Kpi(label:uiLiteral('People Reached'),value:peopleValue,note:peopleNote,icon:Icons.groups_rounded,accent:brandNavy,onTap:canNavigate(5)?()=>onNavigate(5):null)),
              ]);
            }),
            const SizedBox(height:18),
            LayoutBuilder(builder:(context,c){
              final monthlyTrend=items(<String,dynamic>{'items':impact['trend']});
              final weeklyTrend=items(<String,dynamic>{'items':impact['weekly_trend']});
              final activities=items(activity);
              if(c.maxWidth<900)return Column(children:[
                _ImpactPanel(monthlyTrend:monthlyTrend,weeklyTrend:weeklyTrend,year:year,authorized:impactAuthorized,hasData:impactHasData),
                const SizedBox(height:16),
                _ActivityPanel(items:activities),
              ]);
              return Row(crossAxisAlignment:CrossAxisAlignment.start,children:[
                Expanded(flex:7,child:_ImpactPanel(monthlyTrend:monthlyTrend,weeklyTrend:weeklyTrend,year:year,authorized:impactAuthorized,hasData:impactHasData)),
                const SizedBox(width:16),
                Expanded(flex:4,child:_ActivityPanel(items:activities)),
              ]);
            }),
          ]),
        );
      },
    );
  }
}

class _ImpactPanel extends StatefulWidget {
  const _ImpactPanel({
    required this.monthlyTrend,
    required this.weeklyTrend,
    required this.year,
    required this.authorized,
    required this.hasData,
  });
  final List<Map<String,dynamic>> monthlyTrend;
  final List<Map<String,dynamic>> weeklyTrend;
  final int year;
  final bool authorized;
  final bool hasData;

  @override
  State<_ImpactPanel> createState()=>_ImpactPanelState();
}

class _ImpactPanelState extends State<_ImpactPanel> {
  bool weekly=false;

  @override
  Widget build(BuildContext context){
    final trend=weekly ? widget.weeklyTrend : widget.monthlyTrend;
    return SizedBox(
      height:330,
      child:Card(child:Padding(
        padding:const EdgeInsets.fromLTRB(22,20,22,16),
        child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
          Row(children:[
            Expanded(child:LText(uiLiteral('Program Impact'),style:GoogleFonts.cormorantGaramond(color:brandNavy,fontWeight:FontWeight.w700,fontSize:20))),
            if(widget.authorized)
              Container(
                height:34,
                padding:const EdgeInsets.symmetric(horizontal:9),
                decoration:BoxDecoration(border:Border.all(color:brandMist),borderRadius:BorderRadius.circular(7)),
                child:DropdownButtonHideUnderline(
                  child:DropdownButton<bool>(
                    value:weekly,
                    isDense:true,
                    icon:const Icon(Icons.keyboard_arrow_down_rounded,size:17),
                    style:GoogleFonts.inter(color:brandNavy,fontSize:10.5,fontWeight:FontWeight.w600),
                    items:[
                      DropdownMenuItem(value:false,child:LText(uiLiteral('Monthly'))),
                      DropdownMenuItem(value:true,child:LText(uiLiteral('Weekly'))),
                    ],
                    onChanged:(value){if(value!=null)setState(()=>weekly=value);},
                  ),
                ),
              ),
            const SizedBox(width:8),
            Container(
              padding:const EdgeInsets.symmetric(horizontal:12,vertical:8),
              decoration:BoxDecoration(border:Border.all(color:brandMist),borderRadius:BorderRadius.circular(7)),
              child:LText('${widget.year}',style:GoogleFonts.inter(color:brandNavy,fontSize:10.5,fontWeight:FontWeight.w600)),
            )
          ]),
          const SizedBox(height:12),
          Expanded(
            child:!widget.authorized
                ? Center(
                    child:LText(
                      uiLiteral('Impact permission required'),
                      style:GoogleFonts.inter(color:brandTextSoft,fontSize:11.5),
                    ),
                  )
                : !widget.hasData || trend.isEmpty
                    ? Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const Icon(Icons.insights_outlined, color: brandSteel, size: 26),
                            const SizedBox(height: 8),
                            LText(
                              uiLiteral(weekly ? 'No weekly impact data recorded yet.' : 'No monthly impact data recorded yet.'),
                              style: GoogleFonts.inter(color: brandTextSoft, fontSize: 11.5),
                            ),
                          ],
                        ),
                      )
                    : _ImpactChart(trend:trend),
          ),
        ]),
      )),
    );
  }
}

class _ImpactChart extends StatelessWidget {
  const _ImpactChart({required this.trend});
  final List<Map<String,dynamic>> trend;

  @override
  Widget build(BuildContext context)=>CustomPaint(
    painter:_ImpactChartPainter(
      trend.map((row)=>number(row['value'])).toList(),
      trend.map((row)=>'${row['label']??''}').toList(),
    ),
    child:const SizedBox.expand(),
  );
}

class _ImpactChartPainter extends CustomPainter {
  _ImpactChartPainter(this.values,this.labels);
  final List<double> values;
  final List<String> labels;

  @override
  void paint(Canvas canvas,Size size){
    const left=48.0,bottom=27.0,top=8.0;
    final chart=Rect.fromLTWH(left,top,size.width-left-5,size.height-top-bottom);
    final grid=Paint()..color=brandMist.withOpacity(.82)..strokeWidth=.8;
    for(var i=0;i<=4;i++){final y=chart.top+chart.height*i/4;canvas.drawLine(Offset(chart.left,y),Offset(chart.right,y),grid);}

    if (values.isEmpty) return;
    final vals=values;
    final names=labels.length==vals.length?labels:List<String>.generate(vals.length,(i)=>'${i+1}');
    final divisor=vals.length>1?vals.length-1:1;
    final labelStep=names.length<=12?1:((names.length-1)/11).ceil();
    for(var i=0;i<names.length;i+=labelStep){
      final x=chart.left+chart.width*i/divisor;
      canvas.drawLine(Offset(x,chart.top),Offset(x,chart.bottom),grid);
    }
    if((names.length-1)%labelStep!=0){
      canvas.drawLine(Offset(chart.right,chart.top),Offset(chart.right,chart.bottom),grid);
    }

    var maxValue=0.0;
    for(final value in vals){if(value>maxValue)maxValue=value;}
    if(maxValue<=0)maxValue=1;

    final line=Path();
    final area=Path();
    for(var i=0;i<vals.length;i++){
      final x=vals.length==1?chart.center.dx:chart.left+chart.width*i/divisor;
      final y=chart.bottom-chart.height*(vals[i]/maxValue);
      if(i==0){line.moveTo(x,y);area.moveTo(x,chart.bottom);area.lineTo(x,y);}else{line.lineTo(x,y);area.lineTo(x,y);}
    }
    area.lineTo(vals.length==1?chart.center.dx:chart.right,chart.bottom);area.close();
    canvas.drawPath(area,Paint()..color=const Color(0xFF2E5B87).withOpacity(.11));
    canvas.drawPath(line,Paint()..color=brandNavy..strokeWidth=2.2..style=PaintingStyle.stroke..strokeCap=StrokeCap.round..strokeJoin=StrokeJoin.round);
    final dot=Paint()..color=brandNavy;
    for(var i=0;i<vals.length;i++){
      final x=vals.length==1?chart.center.dx:chart.left+chart.width*i/divisor;
      canvas.drawCircle(Offset(x,chart.bottom-chart.height*(vals[i]/maxValue)),2.7,dot);
    }

    void drawAxisLabel(int index){
      final tp=TextPainter(text:TextSpan(text:names[index],style:GoogleFonts.inter(fontSize:8.5,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      final x=names.length==1?chart.center.dx:chart.left+chart.width*index/divisor;
      tp.paint(canvas,Offset(x-tp.width/2,chart.bottom+7));
    }
    for(var i=0;i<names.length;i+=labelStep){drawAxisLabel(i);}
    if((names.length-1)%labelStep!=0){drawAxisLabel(names.length-1);}

    for(var i=0;i<=4;i++){
      final value=maxValue*(4-i)/4;
      final label=intl.NumberFormat.compact().format(value);
      final tp=TextPainter(text:TextSpan(text:label,style:GoogleFonts.inter(fontSize:8,color:brandTextSoft)),textDirection:TextDirection.ltr)..layout();
      tp.paint(canvas,Offset(chart.left-tp.width-8,chart.top+chart.height*i/4-tp.height/2));
    }
  }

  @override
  bool shouldRepaint(covariant _ImpactChartPainter oldDelegate)=>
      oldDelegate.values.toString()!=values.toString()||oldDelegate.labels.toString()!=labels.toString();
}

class _ActivityPanel extends StatelessWidget {
  const _ActivityPanel({required this.items});
  final List<Map<String,dynamic>> items;

  static String titleFor(String action) {
    final value=action.trim();
    if(value.isEmpty)return uiLiteral('System activity');
    final humanized=value
        .toLowerCase()
        .split('_')
        .where((part)=>part.isNotEmpty)
        .map((part)=>part[0].toUpperCase()+part.substring(1))
        .join(' ');
    return uiLiteral(humanized);
  }

  static IconData iconFor(String resource) {
    switch(resource){
      case 'partners': return Icons.person_add_alt_1_outlined;
      case 'billing': return Icons.payments_outlined;
      case 'catalog': return Icons.widgets_outlined;
      case 'administration': return Icons.manage_accounts_outlined;
      case 'cms': return Icons.language_outlined;
      case 'impact': return Icons.insights_outlined;
      case 'evidence': return Icons.verified_outlined;
      case 'reports': return Icons.picture_as_pdf_outlined;
      default: return Icons.history_rounded;
    }
  }

  @override
  Widget build(BuildContext context)=>SizedBox(
    height:330,
    child:Card(child:Padding(
      padding:const EdgeInsets.fromLTRB(20,20,20,16),
      child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
        Row(children:[
          Expanded(child:LText(uiLiteral('Recent Activity'),style:GoogleFonts.cormorantGaramond(color:brandNavy,fontWeight:FontWeight.w700,fontSize:20))),
          LText(uiLiteral('Live audit feed'),style:GoogleFonts.inter(color:brandSteel,fontSize:9.5,fontWeight:FontWeight.w600)),
        ]),
        const SizedBox(height:10),
        Expanded(
          child:items.isEmpty
              ? Center(child:LText(uiLiteral('No permitted recent activity.'),style:GoogleFonts.inter(color:brandTextSoft,fontSize:11)))
              : ListView.separated(
                  padding:EdgeInsets.zero,
                  itemCount:items.length,
                  separatorBuilder:(_,__)=>const Divider(height:8),
                  itemBuilder:(context,index){
                    final item=items[index];
                    final resource='${item['resource']??''}';
                    final actor='${item['actor_name']??''}'.trim();
                    final partner='${item['partner_id']??''}'.trim();
                    final subtitle=[
                      if(actor.isNotEmpty)actor,
                      if(resource.isNotEmpty)resource,
                      if(partner.isNotEmpty)partner,
                    ].join(' · ');
                    return _ActivityRow(
                      icon:iconFor(resource),
                      title:titleFor('${item['action']??''}'),
                      subtitle:subtitle,
                      tone:resource=='billing'?brandSuccess:resource=='catalog'?brandGold:brandNavy,
                    );
                  },
                ),
        ),
      ]),
    )),
  );
}

class _ActivityRow extends StatelessWidget {
  const _ActivityRow({required this.icon,required this.title,required this.subtitle,required this.tone});
  final IconData icon;
  final String title,subtitle;
  final Color tone;

  @override
  Widget build(BuildContext context)=>Padding(
    padding:const EdgeInsets.symmetric(vertical:5),
    child:Row(children:[
      Container(width:36,height:36,decoration:BoxDecoration(color:tone.withOpacity(.10),shape:BoxShape.circle),child:Icon(icon,color:tone,size:17)),
      const SizedBox(width:10),
      Expanded(child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
        LText(title,maxLines:1,overflow:TextOverflow.ellipsis,style:GoogleFonts.inter(color:brandNavy,fontWeight:FontWeight.w600,fontSize:11.2)),
        const SizedBox(height:2),
        LText(subtitle,maxLines:1,overflow:TextOverflow.ellipsis,style:GoogleFonts.inter(color:brandTextSoft,fontSize:9.2)),
      ]))
    ]),
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
  Map<String, dynamic> partnerKpis = <String, dynamic>{};
  bool loading = false;
  bool categoriesLoading = true;
  String? categoryRegistryWarning;
  bool statsReady = false;
  bool hasMore = false;
  int _loadGeneration = 0;
  String? error;
  String query = '';
  String categoryFilter = 'ALL';
  String lifecycleFilter = 'ALL';
  String healthFilter = 'ALL';
  bool referenceOnly = false;
  static const int pageSize = 24;
  int offset = 0;
  int total = 0;
  final TextEditingController _searchController = TextEditingController();
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

  List<Map<String, dynamic>> _builtInPartnerCategories() {
    final hu = HimateI18n.activeLocale == 'hu_HU';
    const rows = <List<String>>[
      <String>['cat_001', 'Classical Music', 'Klasszikus zene', 'classical-music'],
      <String>['cat_002', 'Fine Art', 'Képzőművészet', 'fine-art'],
      <String>['cat_003', 'Gallery', 'Galéria', 'gallery'],
      <String>['cat_004', 'Theatre', 'Színház', 'theatre'],
      <String>['cat_005', 'Cultural Organization', 'Kulturális szervezet', 'cultural-organization'],
      <String>['cat_006', 'Other', 'Egyéb', 'other'],
    ];
    return rows
        .map((row) => <String, dynamic>{
              'id': row[0],
              'name': hu ? row[2] : row[1],
              'name_en': row[1],
              'name_hu': row[2],
              'slug': row[3],
              'system': true,
            })
        .toList();
  }

  List<Map<String, dynamic>> _mergePartnerCategories(Iterable<Map<String, dynamic>> remote) {
    final byID = <String, Map<String, dynamic>>{
      for (final item in _builtInPartnerCategories()) '${item['id']}': item,
    };
    for (final item in remote) {
      final id = '${item['id'] ?? ''}'.trim();
      if (id.isEmpty) continue;
      byID[id] = Map<String, dynamic>.from(item);
    }
    final systemOrder = <String, int>{
      'cat_001': 1,
      'cat_002': 2,
      'cat_003': 3,
      'cat_004': 4,
      'cat_005': 5,
      'cat_006': 6,
    };
    final result = byID.values.toList()
      ..sort((a, b) {
        final aid = '${a['id']}';
        final bid = '${b['id']}';
        final ao = systemOrder[aid];
        final bo = systemOrder[bid];
        if (ao != null && bo != null) return ao.compareTo(bo);
        if (ao != null) return -1;
        if (bo != null) return 1;
        return '${a['name']}'.toLowerCase().compareTo('${b['name']}'.toLowerCase());
      });
    return result;
  }

  @override
  void initState() {
    super.initState();
    categories = _builtInPartnerCategories();
    load(loadCategories: true);
  }

  @override
  void dispose() {
    _searchDebounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  Map<String, String> _partnerQueryParameters() {
    final params = <String, String>{
      'limit': '$pageSize',
      'offset': '$offset',
    };
    if (query.trim().isNotEmpty) params['q'] = query.trim();
    if (categoryFilter != 'ALL') params['category'] = categoryFilter;
    if (lifecycleFilter != 'ALL') params['lifecycle'] = lifecycleFilter;
    if (healthFilter != 'ALL') params['health'] = healthFilter;
    if (referenceOnly) params['reference'] = 'true';
    return params;
  }

  Uri _centralPartnerUri() =>
      Uri(path: '/api/v1/central/partners', queryParameters: _partnerQueryParameters());

  Uri _partnerExportUri() {
    final params = <String, String>{};
    if (query.trim().isNotEmpty) params['q'] = query.trim();
    if (categoryFilter != 'ALL') params['category'] = categoryFilter;
    if (lifecycleFilter != 'ALL') params['lifecycle'] = lifecycleFilter;
    if (healthFilter != 'ALL') params['health'] = healthFilter;
    if (referenceOnly) params['reference'] = 'true';
    return Uri(path: '/api/v1/partners/export.pdf', queryParameters: params.isEmpty ? null : params);
  }

  Future<void> load({bool reset = false, bool loadCategories = false}) async {
    if (reset) offset = 0;
    final generation = ++_loadGeneration;
    if (mounted) {
      setState(() {
        loading = true;
        error = null;
        statsReady = false;
        if (loadCategories) categoriesLoading = true;
      });
    }
    try {
      final model = await widget.api.get(
        _centralPartnerUri().toString(),
        force: loadCategories,
        maxAge: const Duration(seconds: 5),
      );
      if (!mounted || generation != _loadGeneration) return;
      final categoryRows = items(<String, dynamic>{'items': model['categories']});
      final pagination = model['pagination'] is Map
          ? Map<String, dynamic>.from(model['pagination'] as Map)
          : <String, dynamic>{};
      final kpis = model['kpis'] is Map
          ? Map<String, dynamic>.from(model['kpis'] as Map)
          : <String, dynamic>{};
      final meta = model['meta'] is Map
          ? Map<String, dynamic>.from(model['meta'] as Map)
          : <String, dynamic>{};
      final unavailable = meta['unavailable'] is List
          ? (meta['unavailable'] as List).map((e) => '$e').toSet()
          : <String>{};
      setState(() {
        partners = items(model);
        categories = _mergePartnerCategories(categoryRows);
        partnerKpis = kpis;
        total = (pagination['total'] as num?)?.toInt() ?? partners.length;
        hasMore = pagination['has_more'] == true;
        statsReady = true;
        loading = false;
        categoriesLoading = false;
        categoryRegistryWarning = unavailable.contains('partner_categories')
            ? 'The live category registry is temporarily unavailable. Built-in partner categories remain available.'
            : null;
      });
    } catch (e) {
      if (mounted && generation == _loadGeneration) {
        setState(() {
          error = e.toString();
          loading = false;
          categoriesLoading = false;
        });
      }
    }
  }

  void updateSearch(String value) {
    query = value;
    _searchDebounce?.cancel();
    _searchDebounce = Timer(const Duration(milliseconds: 240), () {
      if (mounted) load(reset: true);
    });
  }

  void applyPortfolioPreset({String lifecycle = 'ALL', bool reference = false}) {
    _searchDebounce?.cancel();
    _searchController.clear();
    setState(() {
      query = '';
      categoryFilter = 'ALL';
      lifecycleFilter = lifecycle;
      healthFilter = 'ALL';
      referenceOnly = reference;
      offset = 0;
    });
    unawaited(load(reset: true));
  }

  void clearReferenceFilter() {
    if (!referenceOnly) return;
    setState(() => referenceOnly = false);
    unawaited(load(reset: true));
  }

  void previousPage() {
    if (offset <= 0) return;
    offset = offset >= pageSize ? offset - pageSize : 0;
    unawaited(load());
  }

  void nextPage() {
    if (!hasMore) return;
    offset += pageSize;
    unawaited(load());
  }

  void success(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: LText(message), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
    );
  }

  Future<void> addCategory() async {
    final nameEN = TextEditingController();
    final nameHU = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'Add partner category',
        subtitle: 'Store both English and Hungarian business labels for every dynamic category.',
        icon: Icons.category_outlined,
        width: 640,
        child: ResponsiveFieldPair(
          first: TextField(
            controller: nameEN,
            autofocus: true,
            decoration: InputDecoration(labelText: uiLiteral('English category name *'), hintText: uiLiteral('e.g. Cultural Foundation')),
          ),
          second: TextField(
            controller: nameHU,
            decoration: InputDecoration(labelText: uiLiteral('Hungarian category name *'), hintText: uiLiteral('pl. Kulturális alapítvány')),
          ),
        ),
        primaryLabel: 'Add category',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );
    if (ok == true && nameEN.text.trim().isNotEmpty && nameHU.text.trim().isNotEmpty) {
      final created = await widget.api.post('/api/v1/partner-categories', {
        'name_en': nameEN.text.trim(),
        'name_hu': nameHU.text.trim(),
      });
      if (mounted) {
        setState(() {
          categories = _mergePartnerCategories(<Map<String, dynamic>>[...categories, created]);
          categoryRegistryWarning = null;
        });
        success('Partner category created.');
      }
    }
    nameEN.dispose();
    nameHU.dispose();
  }

  Future<void> addPartner() async {
    // The creation dialog must be available even when Catalog or supplementary
    // services are degraded. Partner master data is the primary record; modules,
    // licensing and provisioning are configured from the workspace afterwards.
    const pendingOnboardingKey = 'himate_pending_partner_onboarding';
    final pendingRequestId = (html.window.localStorage[pendingOnboardingKey] ?? '').trim();
    if (pendingRequestId.isNotEmpty) {
      try {
        final resumed = await widget.api.post('/api/v1/partner-onboarding/$pendingRequestId/resume', const <String, dynamic>{});
        final resumedPartner = resumed['partner'];
        if ('${resumed['status'] ?? ''}' == 'COMPLETE' && resumedPartner is Map) {
          final partner = Map<String, dynamic>.from(resumedPartner);
          html.window.localStorage.remove(pendingOnboardingKey);
          if (!mounted) return;
          unawaited(load(reset: true));
          success('Interrupted partner onboarding was resumed and completed.');
          Navigator.push(
            context,
            MaterialPageRoute(
              settings: RouteSettings(name: "/app/partners/${partner['id']}"),
              builder: (_) => PartnerWorkspace(api: widget.api, partner: partner),
            ),
          );
          return;
        }
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: LText('A previous partner onboarding is still incomplete. Retry after the dependent service recovers.'),
              behavior: SnackBarBehavior.floating,
              backgroundColor: brandWarning,
            ),
          );
        }
        return;
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: LText('${uiLiteral('Previous partner onboarding could not be resumed yet')}: $e'),
              behavior: SnackBarBehavior.floating,
              backgroundColor: brandWarning,
            ),
          );
        }
        return;
      }
    }

    var categoryOptions = _mergePartnerCategories(categories);
    if (categoryRegistryWarning != null && !categoriesLoading) {
      unawaited(load(loadCategories: true));
    }

    final displayName = TextEditingController();
    final legalName = TextEditingController();
    final brandName = TextEditingController();
    final registrationNumber = TextEditingController();
    final taxId = TextEditingController();
    final country = TextEditingController(text: 'United States');
    final stateRegion = TextEditingController();
    final city = TextEditingController();
    final postalCode = TextEditingController();
    final addressLine1 = TextEditingController();
    final addressLine2 = TextEditingController();
    final website = TextEditingController();
    final phone = TextEditingController();
    final primaryDomain = TextEditingController();
    final contactName = TextEditingController();
    final contactEmail = TextEditingController();
    final portalPassword = TextEditingController();
    final financeContactName = TextEditingController();
    final financeContactEmail = TextEditingController();
    final technicalContactName = TextEditingController();
    final technicalContactEmail = TextEditingController();
    final marketingContactName = TextEditingController();
    final marketingContactEmail = TextEditingController();
    final activationFee = TextEditingController(text: '0');
    final baseMonthlyFee = TextEditingController(text: '0');
    final minimumMonthlyCommitment = TextEditingController(text: '0');
    final quoteReference = TextEditingController();
    final notes = TextEditingController();
    final onboardingRequestId = 'onb_${DateTime.now().microsecondsSinceEpoch}';
    final onboardingDate = DateTime.now().toUtc().toIso8601String().substring(0, 10);

    html.File? partnerLogoFile;
    String category = '${categoryOptions.first['id']}';
    String currency = 'USD';
    bool portalPasswordObscure = true;
    String? modalCategoryWarning = categoryRegistryWarning;
    bool categoryRefreshStarted = false;
    bool dialogOpen = true;
    bool submitting = false;
    String? formError;
    String? stagedPartnerId;
    bool portalOwnerCreated = false;
    bool logoUploaded = false;
    bool billingTermsSaved = false;
    bool completionReady = false;
    int step = 0;

    bool validPortalPassword(String value) {
      return value.runes.length >= 12 &&
          RegExp(r'[a-z]').hasMatch(value) &&
          RegExp(r'[A-Z]').hasMatch(value) &&
          RegExp(r'[0-9]').hasMatch(value) &&
          RegExp(r'[^A-Za-z0-9\s]').hasMatch(value);
    }

    bool validEmail(String value) =>
        RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$').hasMatch(value.trim());

    bool validOptionalEmail(TextEditingController controller) =>
        controller.text.trim().isEmpty || validEmail(controller.text);

    Map<String, dynamic> partnerPayload({bool includeOnboardingRequest = true}) => {
      'display_name': displayName.text.trim(),
      'legal_name': legalName.text.trim(),
      'brand_name': brandName.text.trim().isEmpty ? displayName.text.trim() : brandName.text.trim(),
      'category_id': category,
      'lifecycle': 'PROSPECT',
      'registration_number': registrationNumber.text.trim(),
      'tax_id': taxId.text.trim(),
      'country': country.text.trim(),
      'state_region': stateRegion.text.trim(),
      'city': city.text.trim(),
      'postal_code': postalCode.text.trim(),
      'address_line1': addressLine1.text.trim(),
      'address_line2': addressLine2.text.trim(),
      'website': website.text.trim(),
      'phone': phone.text.trim(),
      'primary_domain': primaryDomain.text.trim(),
      'contact_name': contactName.text.trim(),
      'contact_email': contactEmail.text.trim(),
      'finance_contact_name': financeContactName.text.trim(),
      'finance_contact_email': financeContactEmail.text.trim(),
      'technical_contact_name': technicalContactName.text.trim(),
      'technical_contact_email': technicalContactEmail.text.trim(),
      'marketing_contact_name': marketingContactName.text.trim(),
      'marketing_contact_email': marketingContactEmail.text.trim(),
      'notes': notes.text.trim(),
      if (includeOnboardingRequest) 'onboarding_request_id': onboardingRequestId,
    };

    Map<String, dynamic> billingTermsPayload() {
      return {
        'currency': currency,
        'activation_fee': double.tryParse(activationFee.text) ?? 0,
        'activation_fee_waived': false,
        'activation_fee_reason': '',
        'base_monthly_fee': double.tryParse(baseMonthlyFee.text) ?? 0,
        'minimum_monthly_commitment': double.tryParse(minimumMonthlyCommitment.text) ?? 0,
        'quote_reference': quoteReference.text.trim(),
        'annual_increase_percent': 5,
        'price_effective_from': onboardingDate,
        'service_anchor_date': onboardingDate,
        'reason': 'New Partner master-data onboarding',
      };
    }

    final createdResult = await showDialog<Map<String, dynamic>>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) {
          if (!categoryRefreshStarted) {
            categoryRefreshStarted = true;
            unawaited(() async {
              try {
                final response = await widget.api.get(
                  '/api/v1/central/partners?limit=1&offset=0',
                  force: true,
                  maxAge: const Duration(seconds: 5),
                );
                final loaded = items(<String, dynamic>{'items': response['categories']});
                final merged = _mergePartnerCategories(loaded);
                final warning = loaded.isEmpty
                    ? 'The live category registry returned no rows. Built-in partner categories are shown.'
                    : null;
                if (!dialogOpen) return;
                if (mounted) {
                  setState(() {
                    categories = merged;
                    categoryRegistryWarning = warning;
                    categoriesLoading = false;
                  });
                }
                setLocal(() {
                  categoryOptions = merged;
                  modalCategoryWarning = warning;
                  if (!categoryOptions.any((item) => '${item['id']}' == category)) {
                    category = '${categoryOptions.first['id']}';
                  }
                });
              } catch (_) {
                if (!dialogOpen) return;
                final fallback = _mergePartnerCategories(categoryOptions);
                const warning =
                    'The live category registry is temporarily unavailable. Built-in partner categories remain available.';
                if (mounted) {
                  setState(() {
                    categories = fallback;
                    categoryRegistryWarning = warning;
                    categoriesLoading = false;
                  });
                }
                setLocal(() {
                  categoryOptions = fallback;
                  modalCategoryWarning = warning;
                });
              }
            }());
          }
          final canDismiss = !submitting && stagedPartnerId == null;
          return PopScope(
            canPop: completionReady || canDismiss,
            child: BrandDialog(
          key: const Key('new-partner-dialog'),
          title: 'New Partner',
          subtitle: 'Create the complete partner master record, billing identity, first Portal Owner and initial brand identity. Provisioning can be completed from the partner workspace.',
          icon: Icons.add_business_outlined,
          width: 900,
          dismissEnabled: canDismiss,
          onDismiss: () => Navigator.pop<Map<String, dynamic>>(dialogContext),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (formError != null) ...[
                _MessageCard(
                  icon: Icons.error_outline_rounded,
                  title: stagedPartnerId == null ? uiLiteral('Partner registration needs attention') : uiLiteral('Registration is not complete yet'),
                  message: formError!,
                ),
                const SizedBox(height: 12),
              ],
              if (submitting) ...[
                const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
                const SizedBox(height: 12),
              ],
              SizedBox(
                height: formError == null ? 620 : 540,
                child: Stepper(
              currentStep: step,
              type: StepperType.vertical,
              controlsBuilder: (context, details) => const SizedBox.shrink(),
              onStepTapped: (value) => setLocal(() => step = value),
              steps: [
                Step(
                  title: const LText('1 · Company & legal identity'),
                  isActive: step >= 0,
                  content: Column(
                    children: [
                      ResponsiveFieldPair(
                        first: TextField(
                          controller: displayName,
                          decoration: InputDecoration(labelText: uiLiteral('Display name *'), hintText: uiLiteral('Name shown inside HIMATE'), helperText: uiLiteral('Display names do not need to be unique.')),
                        ),
                        second: TextField(
                          controller: legalName,
                          decoration: InputDecoration(labelText: uiLiteral('Legal company name *')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(
                          controller: brandName,
                          decoration: InputDecoration(labelText: uiLiteral('Brand / DBA'), hintText: uiLiteral('Defaults to display name')),
                        ),
                        second: DropdownButtonFormField<String>(
                          value: category,
                          decoration: InputDecoration(labelText: uiLiteral('Partner category')),
                          items: [
                            for (final item in categoryOptions)
                              DropdownMenuItem(value: '${item['id']}', child: LText('${item['name']}')),
                          ],
                          onChanged: (value) {
                            if (value != null) setLocal(() => category = value);
                          },
                        ),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(
                          controller: registrationNumber,
                          decoration: InputDecoration(labelText: uiLiteral('Company / registration number *')),
                        ),
                        second: TextField(
                          controller: taxId,
                          decoration: InputDecoration(labelText: uiLiteral('Tax / VAT ID *')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(
                          controller: website,
                          keyboardType: TextInputType.url,
                          decoration: InputDecoration(labelText: uiLiteral('Website')),
                        ),
                        second: TextField(
                          controller: phone,
                          keyboardType: TextInputType.phone,
                          decoration: InputDecoration(labelText: uiLiteral('Company phone')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: primaryDomain,
                        decoration: InputDecoration(labelText: uiLiteral('Primary domain'), hintText: uiLiteral('example.com')),
                      ),
                      if (modalCategoryWarning != null) ...[
                        const SizedBox(height: 12),
                        _MessageCard(
                          icon: Icons.info_outline_rounded,
                          title: 'Built-in partner categories are available',
                          message: modalCategoryWarning!,
                        ),
                      ],
                    ],
                  ),
                ),
                Step(
                  title: const LText('2 · Registered office & contacts'),
                  isActive: step >= 1,
                  content: Column(
                    children: [
                      ResponsiveFieldPair(
                        first: TextField(controller: country, decoration: InputDecoration(labelText: uiLiteral('Country *'))),
                        second: TextField(controller: stateRegion, decoration: InputDecoration(labelText: uiLiteral('State / region'))),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(controller: city, decoration: InputDecoration(labelText: uiLiteral('City *'))),
                        second: TextField(controller: postalCode, decoration: InputDecoration(labelText: uiLiteral('Postal code *'))),
                      ),
                      const SizedBox(height: 12),
                      TextField(controller: addressLine1, decoration: InputDecoration(labelText: uiLiteral('Registered address line 1 *'))),
                      const SizedBox(height: 12),
                      TextField(controller: addressLine2, decoration: InputDecoration(labelText: uiLiteral('Registered address line 2'))),
                      const SizedBox(height: 18),
                      const _DialogSectionLabel('OPERATIONAL CONTACTS'),
                      const SizedBox(height: 10),
                      ResponsiveFieldPair(
                        first: TextField(controller: financeContactName, decoration: InputDecoration(labelText: uiLiteral('Finance / billing contact'))),
                        second: TextField(controller: financeContactEmail, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Finance / billing email'))),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(controller: technicalContactName, decoration: InputDecoration(labelText: uiLiteral('Technical contact'))),
                        second: TextField(controller: technicalContactEmail, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Technical email'))),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(controller: marketingContactName, decoration: InputDecoration(labelText: uiLiteral('Marketing contact'))),
                        second: TextField(controller: marketingContactEmail, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Marketing email'))),
                      ),
                    ],
                  ),
                ),
                Step(
                  title: const LText('3 · Partner Portal & branding'),
                  isActive: step >= 2,
                  content: Column(
                    children: [
                      ResponsiveFieldPair(
                        first: TextField(
                          controller: contactName,
                          decoration: InputDecoration(labelText: uiLiteral('Portal Owner / primary contact name *')),
                        ),
                        second: TextField(
                          controller: contactEmail,
                          keyboardType: TextInputType.emailAddress,
                          decoration: InputDecoration(labelText: uiLiteral('Partner Portal Owner email *')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: portalPassword,
                        obscureText: portalPasswordObscure,
                        enableSuggestions: false,
                        autocorrect: false,
                        decoration: InputDecoration(
                          labelText: uiLiteral('Initial Partner Portal password *'),
                          helperText: uiLiteral('Minimum 12 characters with lowercase, uppercase, number and special character.'),
                          prefixIcon: const Icon(Icons.lock_outline_rounded),
                          suffixIcon: IconButton(
                            tooltip: uiLiteral(portalPasswordObscure ? 'Show password' : 'Hide password'),
                            onPressed: () => setLocal(() => portalPasswordObscure = !portalPasswordObscure),
                            icon: Icon(portalPasswordObscure ? Icons.visibility_outlined : Icons.visibility_off_outlined),
                          ),
                        ),
                      ),
                      const SizedBox(height: 18),
                      const _DialogSectionLabel('PARTNER BRAND IDENTITY'),
                      const SizedBox(height: 10),
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.center,
                        children: [
                          Container(
                            width: 58,
                            height: 58,
                            decoration: BoxDecoration(
                              color: brandNavy.withOpacity(.05),
                              borderRadius: BorderRadius.circular(12),
                              border: Border.all(color: brandMist),
                            ),
                            child: const Icon(Icons.image_outlined, color: brandNavy),
                          ),
                          const SizedBox(width: 14),
                          Expanded(
                            child: LText(
                              partnerLogoFile?.name ?? 'No partner logo selected',
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(color: brandTextSoft),
                            ),
                          ),
                          const SizedBox(width: 12),
                          OutlinedButton.icon(
                            onPressed: () async {
                              final file = await pickBrowserFile('image/png,image/jpeg,image/webp');
                              if (file != null) setLocal(() => partnerLogoFile = file);
                            },
                            icon: const Icon(Icons.upload_file_rounded),
                            label: const LText('Choose logo'),
                          ),
                        ],
                      ),
                      const SizedBox(height: 10),
                      const LText(
                        'PNG, JPEG or WebP. The asset is stored in the partner-scoped media library and becomes the partner logo reference used by HIMATE and the Partner Portal.',
                        style: TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
                      ),
                    ],
                  ),
                ),
                Step(
                  title: const LText('4 · Commercial defaults'),
                  isActive: step >= 3,
                  content: Column(
                    children: [
                      ResponsiveFieldPair(
                        first: DropdownButtonFormField<String>(
                          value: currency,
                          decoration: InputDecoration(labelText: uiLiteral('Billing currency')),
                          items: const [
                            DropdownMenuItem(value: 'USD', child: LText('USD')),
                            DropdownMenuItem(value: 'EUR', child: LText('EUR')),
                            DropdownMenuItem(value: 'GBP', child: LText('GBP')),
                          ],
                          onChanged: (value) {
                            if (value != null) setLocal(() => currency = value);
                          },
                        ),
                        second: TextField(
                          controller: quoteReference,
                          decoration: InputDecoration(labelText: uiLiteral('Quote / offer reference')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      ResponsiveFieldPair(
                        first: TextField(
                          controller: activationFee,
                          keyboardType: const TextInputType.numberWithOptions(decimal: true),
                          decoration: InputDecoration(labelText: uiLiteral('Activation fee')),
                        ),
                        second: TextField(
                          controller: baseMonthlyFee,
                          keyboardType: const TextInputType.numberWithOptions(decimal: true),
                          decoration: InputDecoration(labelText: uiLiteral('Base monthly fee')),
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: minimumMonthlyCommitment,
                        keyboardType: const TextInputType.numberWithOptions(decimal: true),
                        decoration: InputDecoration(labelText: uiLiteral('Minimum monthly commitment')),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: notes,
                        maxLines: 3,
                        decoration: InputDecoration(labelText: uiLiteral('Internal partner notes')),
                      ),
                      const SizedBox(height: 12),
                      const _MessageCard(
                        icon: Icons.lock_clock_outlined,
                        title: 'Provisioning remains controlled',
                        message: 'Creating the partner does not launch production or bypass licensing. Agreements, invoices, payment evidence, modules and environments can be completed from the partner workspace.',
                      ),
                    ],
                  ),
                ),
                  ],
                ),
              ),
            ],
          ),
          primaryLabel: submitting
              ? (stagedPartnerId == null ? uiLiteral('Creating partner…') : uiLiteral('Completing setup…'))
              : (stagedPartnerId == null ? uiLiteral('Create partner') : uiLiteral('Retry setup')),
          onPrimary: () async {
            if (submitting) return;

            String? validationMessage;
            int validationStep = 0;
            if (displayName.text.trim().isEmpty) {
              validationMessage = 'Display name is required. This is the partner name shown inside HIMATE and Partner Portal.';
            } else if (legalName.text.trim().isEmpty) {
              validationMessage = 'Legal company name is required.';
            } else if (registrationNumber.text.trim().isEmpty) {
              validationMessage = 'Company / registration number is required.';
            } else if (taxId.text.trim().isEmpty) {
              validationMessage = 'Tax / VAT ID is required.';
            } else if (country.text.trim().isEmpty) {
              validationMessage = 'Country is required for the registered office.';
              validationStep = 1;
            } else if (city.text.trim().isEmpty) {
              validationMessage = 'City is required for the registered office.';
              validationStep = 1;
            } else if (postalCode.text.trim().isEmpty) {
              validationMessage = 'Postal code is required for the registered office.';
              validationStep = 1;
            } else if (addressLine1.text.trim().isEmpty) {
              validationMessage = 'Registered address line 1 is required.';
              validationStep = 1;
            } else if (contactName.text.trim().isEmpty) {
              validationMessage = 'Partner Portal Owner name is required.';
              validationStep = 2;
            } else if (contactEmail.text.trim().isEmpty) {
              validationMessage = 'Partner Portal Owner email is required.';
              validationStep = 2;
            } else if (portalPassword.text.isEmpty) {
              validationMessage = 'Initial Partner Portal password is required.';
              validationStep = 2;
            } else if (!validEmail(contactEmail.text)) {
              validationMessage = 'Partner Portal Owner email address is invalid.';
              validationStep = 2;
            } else if (!validOptionalEmail(financeContactEmail) ||
                !validOptionalEmail(technicalContactEmail) ||
                !validOptionalEmail(marketingContactEmail)) {
              validationMessage = 'One or more optional contact email addresses are invalid.';
              validationStep = 1;
            } else if (!validPortalPassword(portalPassword.text)) {
              validationMessage = 'Partner Portal password must be at least 12 characters and include lowercase, uppercase, a number and a special character.';
              validationStep = 2;
            }

            if (validationMessage != null) {
              setLocal(() {
                formError = validationMessage;
                step = validationStep;
              });
              return;
            }

            setLocal(() {
              submitting = true;
              formError = null;
            });

            try {
              html.window.localStorage[pendingOnboardingKey] = onboardingRequestId;
              final onboarding = await widget.api.post('/api/v1/partner-onboarding', {
                'request_id': onboardingRequestId,
                'partner': partnerPayload(),
                'portal_owner': {
                  'name': contactName.text.trim(),
                  'email': contactEmail.text.trim(),
                  'password': portalPassword.text,
                },
                'billing_terms': billingTermsPayload(),
              });
              final rawPartner = onboarding['partner'];
              if (rawPartner is! Map) {
                throw Exception('Onboarding completed without an authoritative partner readback.');
              }
              var created = Map<String, dynamic>.from(rawPartner);
              stagedPartnerId = '${created['id']}';
              portalOwnerCreated = onboarding['owner_done'] == true;
              billingTermsSaved = onboarding['billing_done'] == true;
              if ('${onboarding['status'] ?? ''}' != 'COMPLETE' || !portalOwnerCreated || !billingTermsSaved) {
                throw Exception('${onboarding['last_error'] ?? 'Partner onboarding is incomplete.'}');
              }

              final partnerId = stagedPartnerId!;

              if (!logoUploaded && partnerLogoFile != null && '${created['logo_url'] ?? ''}'.trim().isNotEmpty) {
                logoUploaded = true;
              }

              if (partnerLogoFile != null && !logoUploaded) {
                final logoFile = partnerLogoFile!;
                final bytes = await readBrowserFile(logoFile);
                final logo = await widget.api.multipart(
                  '/api/v1/partners/$partnerId/logo',
                  {
                    'alt_text': '${displayName.text.trim()} logo',
                    'purpose': 'logo',
                  },
                  bytes,
                  logoFile.name,
                );
                final updated = logo['partner'];
                if (updated is Map) {
                  created = Map<String, dynamic>.from(updated);
                }
                logoUploaded = true;
              }

              html.window.localStorage.remove(pendingOnboardingKey);
              if (dialogContext.mounted) {
                setLocal(() {
                  submitting = false;
                  completionReady = true;
                });
                Navigator.pop<Map<String, dynamic>>(dialogContext, created);
              }
            } catch (e) {
              if (!dialogContext.mounted) return;
              setLocal(() {
                submitting = false;
                formError = stagedPartnerId == null
                    ? '${uiLiteral('Partner could not be created')}: $e'
                    : '${uiLiteral('Partner')} ${stagedPartnerId!} ${uiLiteral('exists, but onboarding is not complete.')} $e. ${uiLiteral('Correct the data or service issue and press Retry setup. This window will stay open.')}';
              });
            }
          },
        ),
      );
        },
      ),
    );
    dialogOpen = false;

    if (createdResult != null && mounted) {
      final partnerId = '${createdResult['id']}';
      unawaited(load(reset: true));
      success('Partner master data, Portal Owner, logo and commercial defaults were saved.');
      Navigator.push(
        context,
        MaterialPageRoute(
          settings: RouteSettings(name: '/app/partners/$partnerId'),
          builder: (_) => PartnerWorkspace(api: widget.api, partner: createdResult),
        ),
      );
    }

    for (final controller in [
      displayName,
      legalName,
      brandName,
      registrationNumber,
      taxId,
      country,
      stateRegion,
      city,
      postalCode,
      addressLine1,
      addressLine2,
      website,
      phone,
      primaryDomain,
      contactName,
      contactEmail,
      portalPassword,
      financeContactName,
      financeContactEmail,
      technicalContactName,
      technicalContactEmail,
      marketingContactName,
      marketingContactEmail,
      activationFee,
      baseMonthlyFee,
      minimumMonthlyCommitment,
      quoteReference,
      notes,
    ]) {
      controller.dispose();
    }
  }

  List<Map<String, dynamic>> get filtered => partners;

  @override
  Widget build(BuildContext context) {
    final live = (partnerKpis['live_partners'] as num?)?.toInt() ?? 0;
    final prospects = (partnerKpis['prospects'] as num?)?.toInt() ?? 0;
    final reference = (partnerKpis['reference_partners'] as num?)?.toInt() ?? 0;
    final allRecords = (partnerKpis['partner_records'] as num?)?.toInt() ?? 0;

    return Content(
      eyebrow: 'PEOPLE  |  PROGRAMS  |  IMPACT',
      title: 'Partners',
      subtitle: 'A single premium workspace for every organization connected to the HIMATE ecosystem.',
      actions: [
        OutlinedButton.icon(
          onPressed: () => openBrowserDownload(_partnerExportUri().toString()),
          icon: const Icon(Icons.download_outlined),
          label: const LText('Export PDF'),
        ),
        OutlinedButton.icon(onPressed: addCategory, icon: const Icon(Icons.category_outlined), label: const LText('Add category')),
        FilledButton.icon(
          key: const Key('partners-new-partner-button'),
          onPressed: addPartner,
          icon: const Icon(Icons.add_business_outlined),
          label: const LText('New Partner'),
        ),
      ],
      child: error != null
          ? _MessageCard(icon: Icons.cloud_off_outlined, title: 'Partners could not be loaded', message: error!)
          : Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    ResponsiveKpiGrid(
                      children: [
                        Kpi(label: 'Partner records', value: '$allRecords', note: 'All lifecycle states', icon: Icons.apartment_outlined, accent: brandNavy, onTap: () => applyPortfolioPreset()),
                        Kpi(label: 'Live partners', value: '$live', note: 'Operational partner environments', icon: Icons.public_outlined, accent: brandSuccess, onTap: () => applyPortfolioPreset(lifecycle: 'LIVE')),
                        Kpi(label: 'Prospects', value: '$prospects', note: 'Pre-license pipeline', icon: Icons.handshake_outlined, accent: brandSteel, onTap: () => applyPortfolioPreset(lifecycle: 'PROSPECT')),
                        Kpi(label: 'Reference partners', value: '$reference', note: 'Reference implementation', icon: Icons.workspace_premium_outlined, accent: brandGold, onTap: () => applyPortfolioPreset(reference: true)),
                      ],
                    ),
                    const SizedBox(height: 20),
                    _FilterSurface(
                      child: LayoutBuilder(
                        builder: (context, c) {
                          final compact = c.maxWidth < 860;
                          final search = TextField(
                            controller: _searchController,
                            onChanged: updateSearch,
                            decoration: InputDecoration(
                              hintText: uiLiteral('Search partners...'),
                              prefixIcon: Icon(Icons.search_rounded),
                            ),
                          );
                          final category = DropdownButtonFormField<String>(
                            value: categoryFilter,
                            decoration: InputDecoration(labelText: uiLiteral('Category')),
                            items: [
                              const DropdownMenuItem(value: 'ALL', child: LText('All categories')),
                              for (final c in categories) DropdownMenuItem(value: '${c['id']}', child: LText('${c['name']}')),
                            ],
                            onChanged: (v) {
                              setState(() => categoryFilter = v ?? 'ALL');
                              load(reset: true);
                            },
                          );
                          final lifecycle = DropdownButtonFormField<String>(
                            value: lifecycleFilter,
                            decoration: InputDecoration(labelText: uiLiteral('Lifecycle')),
                            items: [
                              const DropdownMenuItem(value: 'ALL', child: LText('All lifecycle states')),
                              for (final state in lifecycleOptions) DropdownMenuItem(value: state, child: LText(_humanize(state))),
                            ],
                            onChanged: (v) {
                              setState(() => lifecycleFilter = v ?? 'ALL');
                              load(reset: true);
                            },
                          );
                          final health = DropdownButtonFormField<String>(
                            value: healthFilter,
                            decoration: InputDecoration(labelText: uiLiteral('Health')),
                            items: const [
                              DropdownMenuItem(value: 'ALL', child: LText('All health states')),
                              DropdownMenuItem(value: 'HEALTHY', child: LText('Healthy')),
                              DropdownMenuItem(value: 'WARNING', child: LText('Warning')),
                              DropdownMenuItem(value: 'OFFLINE', child: LText('Offline')),
                              DropdownMenuItem(value: 'UNKNOWN', child: LText('Unknown')),
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
                    if (referenceOnly) ...[
                      const SizedBox(height: 10),
                      Align(
                        alignment: Alignment.centerLeft,
                        child: InputChip(
                          label: const LText('Reference partners only'),
                          avatar: const Icon(Icons.workspace_premium_outlined, size: 16),
                          onDeleted: clearReferenceFilter,
                        ),
                      ),
                    ],
                    const SizedBox(height: 14),
                    if (loading) ...[
                      const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
                      const SizedBox(height: 12),
                    ],
                    Row(
                      children: [
                        LText('Partner portfolio', style: Theme.of(context).textTheme.titleLarge),
                        const SizedBox(width: 10),
                        _MiniCounter(
                          label: statsReady
                              ? '${partners.length} ${uiLiteral('shown')} · $total ${uiLiteral('matched')}'
                              : '${partners.length} ${uiLiteral('shown')}',
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    LayoutBuilder(
                      builder: (context, c) {
                        final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1040 ? (c.maxWidth - 14) / 2 : (c.maxWidth - 28) / 3;
                        final cards = <Widget>[
                          if (filtered.isEmpty)
                            SizedBox(
                              width: width,
                              child: const _MessageCard(
                                icon: Icons.inbox_outlined,
                                title: 'No partner data',
                                message: 'No partners match the current filters. Create a new partner or adjust the filters.',
                              ),
                            ),
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
                            if (offset > 0 || hasMore || (statsReady && total > pageSize)) ...[
                              const SizedBox(height: 18),
                              Wrap(
                                spacing: 10,
                                runSpacing: 10,
                                crossAxisAlignment: WrapCrossAlignment.center,
                                children: [
                                  OutlinedButton.icon(
                                    onPressed: offset > 0 && !loading ? previousPage : null,
                                    icon: const Icon(Icons.chevron_left_rounded),
                                    label: const LText('Previous'),
                                  ),
                                  _MiniCounter(
                                    label: statsReady
                                        ? '${uiLiteral('Page')} ${offset ~/ pageSize + 1} ${uiLiteral('of')} ${(total + pageSize - 1) ~/ pageSize}'
                                        : '${uiLiteral('Page')} ${offset ~/ pageSize + 1}',
                                  ),
                                  OutlinedButton.icon(
                                    onPressed: hasMore && !loading ? nextPage : null,
                                    icon: const Icon(Icons.chevron_right_rounded),
                                    label: const LText('Next'),
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
  const PartnerWorkspace({
    required this.api,
    required this.partner,
    this.initialSection,
    this.onBack,
    super.key,
  });
  final Api api;
  final Map<String, dynamic> partner;
  final String? initialSection;
  final VoidCallback? onBack;

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
  List<Map<String, dynamic>> portalUsers = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> billingEvents = <Map<String, dynamic>>[];
  Map<String, dynamic>? billing;
  Map<String, dynamic>? terms;
  Map<String, dynamic>? license;
  Map<String, dynamic>? agreement;
  Map<String, dynamic>? commercialStatus;
  Map<String, dynamic>? paymentProfile;
  Map<String, dynamic>? websiteAdapter;
  bool loading = true;
  bool supplementalLoading = true;
  String? error;
  String? supplementalError;
  String moduleQuery = '';
  String moduleState = 'ALL';
  final GlobalKey _overviewKey = GlobalKey();
  final GlobalKey _companyKey = GlobalKey();
  final GlobalKey _environmentKey = GlobalKey();
  final GlobalKey _pricingKey = GlobalKey();
  final GlobalKey _modulesKey = GlobalKey();
  final GlobalKey _financeKey = GlobalKey();
  final GlobalKey _statisticsKey = GlobalKey();
  final GlobalKey _usersKey = GlobalKey();
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
    _WorkspaceSpec('Users & Contacts', Icons.group_outlined, 'Partner Portal users and organization contacts', true),
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
    final hasPrimary =
        '${partner['id'] ?? ''}'.isNotEmpty && '${partner['display_name'] ?? ''}'.isNotEmpty;
    if (mounted) {
      setState(() {
        loading = !hasPrimary;
        supplementalLoading = true;
        error = null;
        supplementalError = null;
      });
    }
    if (hasPrimary) _scrollToInitialSection();
    final id = '${partner['id']}';
    try {
      final model = await widget.api.get(
        '/api/v1/central/partners/$id',
        maxAge: const Duration(seconds: 5),
      );
      if (!mounted) return;
      final core = model['partner'] is Map
          ? Map<String, dynamic>.from(model['partner'] as Map)
          : partner;
      final meta = model['meta'] is Map
          ? Map<String, dynamic>.from(model['meta'] as Map)
          : <String, dynamic>{};
      final unavailable = meta['unavailable'] is List
          ? (meta['unavailable'] as List).map((e) => '$e').toList()
          : <String>[];
      setState(() {
        partner = core;
        modules = items(<String, dynamic>{'items': model['modules']});
        documents = items(<String, dynamic>{'items': model['documents']});
        invoices = items(<String, dynamic>{'items': model['invoices']});
        subscriptions = items(<String, dynamic>{'items': model['subscriptions']});
        environments = items(<String, dynamic>{'items': model['environments']});
        provisioningJobs = items(<String, dynamic>{'items': model['provisioning_jobs']});
        impactSummary = items(<String, dynamic>{'items': model['impact_summary']});
        connectorCredentials = items(<String, dynamic>{'items': model['connector_credentials']});
        portalUsers = items(<String, dynamic>{'items': model['portal_users']});
        billingEvents = items(<String, dynamic>{'items': model['billing_events']});
        billing = model['billing'] is Map ? Map<String, dynamic>.from(model['billing'] as Map) : null;
        terms = model['terms'] is Map ? Map<String, dynamic>.from(model['terms'] as Map) : null;
        license = model['license'] is Map ? Map<String, dynamic>.from(model['license'] as Map) : null;
        agreement = model['agreement'] is Map ? Map<String, dynamic>.from(model['agreement'] as Map) : null;
        commercialStatus = model['commercial_status'] is Map
            ? Map<String, dynamic>.from(model['commercial_status'] as Map)
            : null;
        paymentProfile = model['payment_profile'] is Map
            ? Map<String, dynamic>.from(model['payment_profile'] as Map)
            : null;
        websiteAdapter = model['website_adapter'] is Map
            ? Map<String, dynamic>.from(model['website_adapter'] as Map)
            : null;
        loading = false;
        supplementalLoading = false;
        supplementalError = unavailable.isEmpty
            ? null
            : 'Some secondary services are temporarily unavailable: ${unavailable.join(', ')}. Available sections remain usable.';
      });
      _scrollToInitialSection();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        supplementalLoading = false;
        if (hasPrimary) {
          supplementalError =
              'The latest Go partner read model could not be refreshed. The already loaded partner record remains usable.';
        } else {
          error = e.toString();
        }
      });
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
      'users-and-contacts' => _usersKey,
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
      SnackBar(content: LText(message), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
    );
  }

  Map<String, dynamic>? get provisioningJob =>
      provisioningJobs.isEmpty ? null : provisioningJobs.first;

  String _partnerModuleSection(Map<String,dynamic> module) {
    return switch ('${module['group_key'] ?? ''}') {
      'finance_invoicing' => 'Finance & Invoicing',
      'marketing' => 'Marketing',
      'website_events' => 'Website & Events',
      _ => 'Technical Operation',
    };
  }

  Map<String,List<Map<String,dynamic>>> get groupedFilteredModules {
    final grouped = <String,List<Map<String,dynamic>>>{
      'Finance & Invoicing': <Map<String,dynamic>>[],
      'Technical Operation': <Map<String,dynamic>>[],
      'Marketing': <Map<String,dynamic>>[],
      'Website & Events': <Map<String,dynamic>>[],
    };
    for (final module in filteredModules) {
      grouped[_partnerModuleSection(module)]!.add(module);
    }
    return grouped;
  }

  Future<void> startProvisioning() async {
    final lifecycle = '${partner['lifecycle'] ?? ''}';
    if (!const {'READY_TO_PROVISION', 'PROVISIONING', 'CONFIGURATION'}.contains(lifecycle)) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: LText('Move the partner to READY TO PROVISION before starting provisioning.'),
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
          content: LText('${uiLiteral('Provisioning could not complete')}: $e'),
          behavior: SnackBarBehavior.floating,
          backgroundColor: brandDanger,
        ),
      );
    }
  }

  Future<void> createPortalUser() async {
    final name = TextEditingController();
    final email = TextEditingController();
    final password = TextEditingController();
    String role = portalUsers.isEmpty ? 'owner' : 'viewer';
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: portalUsers.isEmpty ? 'Create Partner Portal owner' : 'Add Partner Portal user',
          subtitle: 'Create an account scoped only to this partner. Portal roles cannot grant HIMATE control-plane access.',
          icon: Icons.person_add_alt_1_rounded,
          width: 650,
          primaryLabel: 'Create portal user',
          onPrimary: () => Navigator.pop(dialogContext, true),
          child: Column(children: [
            TextField(controller: name, decoration: InputDecoration(labelText: uiLiteral('Name'))),
            const SizedBox(height: 12),
            TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Email'))),
            const SizedBox(height: 12),
            TextField(controller: password, obscureText: true, decoration: InputDecoration(labelText: uiLiteral('Temporary password'))),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              value: role,
              decoration: InputDecoration(labelText: uiLiteral('Partner Portal role')),
              items: const [
                DropdownMenuItem(value: 'owner', child: LText('Owner')),
                DropdownMenuItem(value: 'admin', child: LText('Admin')),
                DropdownMenuItem(value: 'billing', child: LText('Billing')),
                DropdownMenuItem(value: 'viewer', child: LText('Viewer')),
              ],
              onChanged: (value) { if (value != null) setLocal(() => role = value); },
            ),
          ]),
        ),
      ),
    );
    if (ok == true) {
      try {
        await widget.api.post('/api/v1/partners/${partner['id']}/portal-users', {
          'name': name.text.trim(),
          'email': email.text.trim(),
          'password': password.text,
          'role': role,
        });
        await load();
        if (mounted) success('Partner Portal user created.');
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: LText('${uiLiteral('Portal user could not be created')}: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
          );
        }
      }
    }
    name.dispose(); email.dispose(); password.dispose();
  }

  Future<void> editPortalUser(Map<String, dynamic> target) async {
    final name = TextEditingController(text: '${target['name'] ?? ''}');
    final email = TextEditingController(text: '${target['email'] ?? ''}');
    String role = '${target['role'] ?? 'viewer'}';
    bool active = target['active'] != false;
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Edit Partner Portal user',
          subtitle: 'Role or status changes invalidate that partner-user session.',
          icon: Icons.manage_accounts_outlined,
          width: 650,
          primaryLabel: 'Save portal user',
          onPrimary: () => Navigator.pop(dialogContext, true),
          child: Column(children: [
            TextField(controller: name, decoration: InputDecoration(labelText: uiLiteral('Name'))),
            const SizedBox(height: 12),
            TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Email'))),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              value: role,
              decoration: InputDecoration(labelText: uiLiteral('Partner Portal role')),
              items: const [
                DropdownMenuItem(value: 'owner', child: LText('Owner')),
                DropdownMenuItem(value: 'admin', child: LText('Admin')),
                DropdownMenuItem(value: 'billing', child: LText('Billing')),
                DropdownMenuItem(value: 'viewer', child: LText('Viewer')),
              ],
              onChanged: (value) { if (value != null) setLocal(() => role = value); },
            ),
            const SizedBox(height: 8),
            SwitchListTile.adaptive(
              contentPadding: EdgeInsets.zero,
              value: active,
              title: const LText('Active portal access'),
              onChanged: (value) => setLocal(() => active = value),
            ),
          ]),
        ),
      ),
    );
    if (ok == true) {
      try {
        await widget.api.patch('/api/v1/partners/${partner['id']}/portal-users/${target['id']}', {
          'name': name.text.trim(),
          'email': email.text.trim(),
          'role': role,
          'active': active,
        });
        await load();
        if (mounted) success('Partner Portal user updated.');
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: LText('${uiLiteral('Portal user could not be updated')}: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
          );
        }
      }
    }
    name.dispose(); email.dispose();
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
            decoration: InputDecoration(labelText: uiLiteral('Environment')),
            items: const [
              DropdownMenuItem(value: 'STAGING', child: LText('STAGING')),
              DropdownMenuItem(value: 'PRODUCTION', child: LText('PRODUCTION')),
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
          title: const LText('Store this credential now'),
          content: SizedBox(
            width: 560,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const LText('For security, HIMATE stores only the token hash. This raw credential will not be shown again.'),
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
                    const SnackBar(content: LText('Credential copied.'), behavior: SnackBarBehavior.floating),
                  );
                }
              },
              icon: const Icon(Icons.copy_rounded),
              label: const LText('Copy'),
            ),
            FilledButton(onPressed: () => Navigator.pop(context), child: const LText('I stored it securely')),
          ],
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: LText('${uiLiteral('Credential operation failed')}: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
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
              TextField(controller: hostname, decoration: InputDecoration(labelText: uiLiteral('Hostname'))),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: version, decoration: InputDecoration(labelText: uiLiteral('Platform version'))),
                second: TextField(controller: desiredRelease, decoration: InputDecoration(labelText: uiLiteral('Desired release'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: activeRelease, decoration: InputDecoration(labelText: uiLiteral('Active release'))),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: deployment,
                  decoration: InputDecoration(labelText: uiLiteral('Deployment status')),
                  items: const [
                    DropdownMenuItem(value: 'NOT_DEPLOYED', child: LText('NOT DEPLOYED')),
                    DropdownMenuItem(value: 'QUEUED', child: LText('QUEUED')),
                    DropdownMenuItem(value: 'DEPLOYING', child: LText('DEPLOYING')),
                    DropdownMenuItem(value: 'DEPLOYED', child: LText('DEPLOYED')),
                    DropdownMenuItem(value: 'FAILED', child: LText('FAILED')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => deployment = v); },
                ),
                second: DropdownButtonFormField<String>(
                  value: environmentStatus,
                  decoration: InputDecoration(labelText: uiLiteral('Environment status')),
                  items: const [
                    DropdownMenuItem(value: 'CREATING', child: LText('CREATING')),
                    DropdownMenuItem(value: 'CONFIGURATION_REQUIRED', child: LText('CONFIGURATION REQUIRED')),
                    DropdownMenuItem(value: 'TESTING', child: LText('TESTING')),
                    DropdownMenuItem(value: 'READY', child: LText('READY')),
                    DropdownMenuItem(value: 'LIVE', child: LText('LIVE')),
                    DropdownMenuItem(value: 'SUSPENDED', child: LText('SUSPENDED')),
                    DropdownMenuItem(value: 'FAILED', child: LText('FAILED')),
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
            TextField(controller: hostname, decoration: InputDecoration(labelText: uiLiteral('Production hostname'))),
            const SizedBox(height: 12),
            TextField(controller: version, decoration: InputDecoration(labelText: uiLiteral('Platform version'))),
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
    bool testPartner = partner['test_partner'] == true;

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
                first: TextField(controller: display, decoration: InputDecoration(labelText: uiLiteral('Display name'))),
                second: TextField(controller: legal, decoration: InputDecoration(labelText: uiLiteral('Legal name'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: brand, decoration: InputDecoration(labelText: uiLiteral('Brand / DBA'))),
                second: DropdownButtonFormField<String>(
                  value: lifecycle,
                  decoration: InputDecoration(labelText: uiLiteral('Lifecycle')),
                  items: [
                    for (final value in _PartnersPageState.lifecycleOptions)
                      DropdownMenuItem(value: value, child: LText(_humanize(value))),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => lifecycle = v); },
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: lifecycleReason,
                decoration: InputDecoration(labelText: uiLiteral('Lifecycle change reason'), hintText: uiLiteral('Required for traceability when status changes')),
              ),
              const SizedBox(height: 8),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: testPartner,
                onChanged: partner['test_partner'] == true
                    ? null
                    : (value) => setLocal(() {
                          testPartner = value;
                          if (value) lifecycle = 'LIVE';
                        }),
                title: const LText('Golden Test Partner'),
                subtitle: LText(partner['test_partner'] == true
                    ? 'Permanent QA tenant. Remove it only through the dedicated future test-tenant purge workflow.'
                    : '38/38 modules stay active. Test revenue and impact remain visible here but are excluded from HIMATE platform aggregates.'),
              ),
              const SizedBox(height: 18),
              const _DialogSectionLabel('REGISTRATION & ADDRESS'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: registration, decoration: InputDecoration(labelText: uiLiteral('Registration number'))),
                second: TextField(controller: tax, decoration: InputDecoration(labelText: uiLiteral('Tax ID'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: country, decoration: InputDecoration(labelText: uiLiteral('Country'))),
                second: TextField(controller: stateRegion, decoration: InputDecoration(labelText: uiLiteral('State / region'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: city, decoration: InputDecoration(labelText: uiLiteral('City'))),
                second: TextField(controller: postal, decoration: InputDecoration(labelText: uiLiteral('Postal code'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: address1, decoration: InputDecoration(labelText: uiLiteral('Address line 1'))),
              const SizedBox(height: 12),
              TextField(controller: address2, decoration: InputDecoration(labelText: uiLiteral('Address line 2'))),
              const SizedBox(height: 18),
              const _DialogSectionLabel('CONTACTS'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: contact, decoration: InputDecoration(labelText: uiLiteral('Primary contact'))),
                second: TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Primary email'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: financeName, decoration: InputDecoration(labelText: uiLiteral('Finance contact'))),
                second: TextField(controller: financeEmail, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Finance email'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: technicalName, decoration: InputDecoration(labelText: uiLiteral('Technical contact'))),
                second: TextField(controller: technicalEmail, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Technical email'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: marketingName, decoration: InputDecoration(labelText: uiLiteral('Marketing contact'))),
                second: TextField(controller: marketingEmail, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Marketing email'))),
              ),
              const SizedBox(height: 18),
              const _DialogSectionLabel('WEB & ENVIRONMENT REFERENCES'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: website, decoration: InputDecoration(labelText: uiLiteral('Website'))),
                second: TextField(controller: phone, decoration: InputDecoration(labelText: uiLiteral('Phone'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: primary, decoration: InputDecoration(labelText: uiLiteral('Primary domain'))),
                second: TextField(controller: staging, decoration: InputDecoration(labelText: uiLiteral('Staging domain'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: logo, decoration: InputDecoration(labelText: uiLiteral('Logo URL / asset reference'))),
              const SizedBox(height: 12),
              TextField(controller: notes, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('Internal notes'))),
            ],
          ),
          primaryLabel: 'Save changes',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true) {
      final updated = await widget.api.patch('/api/v1/partners/${partner['id']}', {
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
        'test_partner': testPartner,
      });
      if (mounted) {
        setState(() => partner = updated);
        await load();
        if (mounted) {
          success(updated['test_partner'] == true
              ? 'Golden Test Partner active with full test entitlements.'
              : 'Partner data updated.');
        }
      }
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
    final providerCustomer = TextEditingController(text: '${paymentProfile?['provider_customer_id'] ?? ''}');
    final paymentMethod = TextEditingController(text: '${paymentProfile?['payment_method_id'] ?? ''}');
    final licenseNote = TextEditingController(text: '${license?['note'] ?? ''}');
    final base = TextEditingController(text: number(terms?['base_monthly_fee']).toStringAsFixed(2));
    final minimumMonthly = TextEditingController(text: number(terms?['minimum_monthly_commitment'] ?? 0).toStringAsFixed(2));
    final quoteReference = TextEditingController(text: '${terms?['quote_reference'] ?? ''}');
    final uplift = TextEditingController(text: number(terms?['annual_increase_percent'] ?? 5).toStringAsFixed(2));
    final effective = TextEditingController(text: '${terms?['price_effective_from'] ?? ''}');
    final anchor = TextEditingController(text: '${terms?['service_anchor_date'] ?? ''}');
    final waiverReason = TextEditingController(text: '${terms?['activation_fee_reason'] ?? ''}');
    final commercialReason = TextEditingController();
    bool waived = terms?['activation_fee_waived'] == true;
    bool autopay = paymentProfile?['autopay_enabled'] == true;
    bool collectActivationNow = false;
    String currency = '${terms?['currency'] ?? 'USD'}';
    String billingMode = '${terms?['billing_mode'] ?? 'PAID'}';
    String charityStatus = '${terms?['charity_status'] ?? 'NOT_REQUESTED'}';

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Pricing & Subscription',
          subtitle: 'Partner-specific license, provider-backed payment and recurring 30-day terms.',
          icon: Icons.payments_outlined,
          width: 760,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const _DialogSectionLabel('COMMERCIAL MODE'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: billingMode,
                  decoration: InputDecoration(labelText: uiLiteral('Billing mode')),
                  items: const [
                    DropdownMenuItem(value: 'PAID', child: LText('Paid')),
                    DropdownMenuItem(value: 'COMPLIMENTARY', child: LText('Complimentary')),
                    DropdownMenuItem(value: 'CHARITY', child: LText('Charity')),
                  ],
                  onChanged: (v) {
                    if (v == null) return;
                    setLocal(() {
                      billingMode = v;
                      if (v != 'CHARITY' && charityStatus == 'APPROVED') {
                        charityStatus = 'NOT_REQUESTED';
                      }
                    });
                  },
                ),
                second: DropdownButtonFormField<String>(
                  value: charityStatus,
                  decoration: InputDecoration(labelText: uiLiteral('Charity review status')),
                  items: const [
                    DropdownMenuItem(value: 'NOT_REQUESTED', child: LText('Not requested')),
                    DropdownMenuItem(value: 'PENDING', child: LText('Pending review')),
                    DropdownMenuItem(value: 'APPROVED', child: LText('Approved')),
                    DropdownMenuItem(value: 'REJECTED', child: LText('Rejected')),
                  ],
                  onChanged: (v) {
                    if (v == null) return;
                    setLocal(() {
                      charityStatus = v;
                      if (v == 'APPROVED') billingMode = 'CHARITY';
                    });
                  },
                ),
              ),
              if (billingMode == 'CHARITY' && charityStatus != 'APPROVED') ...[
                const SizedBox(height: 8),
                const _MessageCard(
                  icon: Icons.policy_outlined,
                  title: 'Charity requires HIMATE approval',
                  message: 'Set Charity review status to Approved only after the application or negotiated eligibility has been verified.',
                ),
              ],
              const SizedBox(height: 18),
              const _DialogSectionLabel('ACTIVATION & PAYMENT'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: currency,
                  decoration: InputDecoration(labelText: uiLiteral('Currency')),
                  items: const [
                    DropdownMenuItem(value: 'USD', child: LText('USD')),
                    DropdownMenuItem(value: 'EUR', child: LText('EUR')),
                    DropdownMenuItem(value: 'GBP', child: LText('GBP')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => currency = v); },
                ),
                second: TextField(
                  controller: activation,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Initial license / activation fee')),
                ),
              ),
              const SizedBox(height: 8),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: waived,
                onChanged: (v) => setLocal(() => waived = v),
                title: const LText('Activation fee waived'),
                subtitle: const LText('Use only where no provider transaction applies.'),
              ),
              if (waived) ...[
                const SizedBox(height: 8),
                TextField(controller: waiverReason, decoration: InputDecoration(labelText: uiLiteral('Waiver reason'))),
              ],
              const SizedBox(height: 18),
              const _DialogSectionLabel('PROVIDER-BACKED PAYMENT'),
              const SizedBox(height: 10),
              _DefinitionRow(label: 'License status', value: '${license?['status'] ?? 'NOT_PAID'}'),
              _DefinitionRow(label: 'Paid amount', value: '$currency ${number(license?['paid_amount']).toStringAsFixed(2)}'),
              _DefinitionRow(label: 'Provider reference', value: '${license?['payment_reference'] ?? '—'}'),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(
                  controller: providerCustomer,
                  decoration: InputDecoration(labelText: uiLiteral('Provider customer ID'), hintText: uiLiteral('Stripe customer ID')),
                ),
                second: TextField(
                  controller: paymentMethod,
                  decoration: InputDecoration(labelText: uiLiteral('Payment method ID'), hintText: uiLiteral('Saved payment method')),
                ),
              ),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: autopay,
                onChanged: (waived || billingMode != 'PAID') ? null : (v) => setLocal(() => autopay = v),
                title: const LText('Automatic recurring collection'),
                subtitle: const LText('Recurring invoices are charged off-session through the configured provider.'),
              ),
              if (!waived && '${license?['status'] ?? ''}' != 'PAID')
                SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  value: collectActivationNow,
                  onChanged: (v) => setLocal(() => collectActivationNow = v),
                  title: const LText('Collect activation license after save'),
                  subtitle: const LText('PAID is set only after the signed provider webhook is verified.'),
                ),
              const SizedBox(height: 12),
              TextField(controller: licenseNote, maxLines: 2, decoration: InputDecoration(labelText: uiLiteral('License note'))),
              const SizedBox(height: 18),
              const _DialogSectionLabel('RECURRING SERVICE'),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(
                  controller: base,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Individual base service fee')),
                ),
                second: TextField(
                  controller: minimumMonthly,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Minimum monthly commitment')),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: quoteReference, decoration: InputDecoration(labelText: uiLiteral('Quote / offer reference'))),
                second: TextField(
                  controller: uplift,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Annual increase %')),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: effective, decoration: InputDecoration(labelText: uiLiteral('Price effective from'), hintText: uiLiteral('YYYY-MM-DD'))),
                second: TextField(controller: anchor, decoration: InputDecoration(labelText: uiLiteral('Service activation / anchor date'), hintText: uiLiteral('YYYY-MM-DD'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: commercialReason, decoration: InputDecoration(labelText: uiLiteral('Change reason'), hintText: uiLiteral('Recorded in commercial price history'))),
              const SizedBox(height: 12),
              const _RuleStrip(items: [
                _RuleItem(Icons.verified_user_outlined, 'Payment authority', 'Signed provider webhook only'),
                _RuleItem(Icons.event_repeat_outlined, 'Renewal', 'Automatic 30-day collection'),
                _RuleItem(Icons.lock_clock_outlined, 'Provisioning', 'Opens only after verified PAID state'),
              ]),
            ],
          ),
          primaryLabel: 'Save commercial terms',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true) {
      final activationAmount = double.tryParse(activation.text) ?? 0;
      final effectiveWaived = waived || activationAmount == 0;
      final effectiveWaiverReason = effectiveWaived && waiverReason.text.trim().isEmpty
          ? (activationAmount == 0 ? 'Zero-dollar activation fee' : 'HIMATE administrator waiver')
          : waiverReason.text.trim();
      if (billingMode == 'CHARITY' && charityStatus != 'APPROVED') {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: LText('Charity billing mode requires Approved charity status.'),
              behavior: SnackBarBehavior.floating,
            ),
          );
        }
      } else {
        await widget.api.put('/api/v1/billing/partners/${partner['id']}/terms', {
          'currency': currency,
          'activation_fee': activationAmount,
          'activation_fee_waived': effectiveWaived,
          'activation_fee_reason': effectiveWaiverReason,
          'base_monthly_fee': double.tryParse(base.text) ?? 0,
          'minimum_monthly_commitment': double.tryParse(minimumMonthly.text) ?? 0,
          'quote_reference': quoteReference.text.trim(),
          'annual_increase_percent': double.tryParse(uplift.text) ?? 5,
          'price_effective_from': effective.text.trim(),
          'service_anchor_date': anchor.text.trim(),
          'reason': commercialReason.text.trim(),
        });
        await widget.api.patch('/api/v1/billing/partners/${partner['id']}/commercial-mode', {
          'billing_mode': billingMode,
          'charity_status': charityStatus,
          'reason': commercialReason.text.trim().isEmpty
              ? 'HIMATE administrator commercial-mode update'
              : commercialReason.text.trim(),
        });
        await widget.api.put('/api/v1/billing/partners/${partner['id']}/license', {
          'currency': currency,
          'required_amount': activationAmount,
          'note': licenseNote.text.trim(),
          'waived': effectiveWaived,
          'waiver_reason': effectiveWaiverReason,
        });
        if (!effectiveWaived && billingMode == 'PAID') {
          await widget.api.put('/api/v1/payments/partners/${partner['id']}/profile', {
            'provider_customer_id': providerCustomer.text.trim(),
            'payment_method_id': paymentMethod.text.trim(),
            'autopay_enabled': autopay,
          });
          if (collectActivationNow) {
            await widget.api.post('/api/v1/billing/partners/${partner['id']}/license/collect', {});
          }
        }
        await load();
        if (mounted) {
          success(collectActivationNow && !effectiveWaived && billingMode == 'PAID'
              ? 'Commercial terms saved and provider-backed activation collection initiated.'
              : 'Commercial terms and billing mode updated.');
        }
      }
    }

    for (final controller in [
      activation,
      providerCustomer,
      paymentMethod,
      licenseNote,
      base,
      minimumMonthly,
      quoteReference,
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
    final note = TextEditingController();
    String kind = 'CONTRACT';
    html.File? selectedFile;

    final ok = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Upload commercial document',
          subtitle: 'The file is stored in HIMATE Evidence/Storage first, checksum-verified, then linked to Billing.',
          icon: Icons.note_add_outlined,
          width: 680,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: kind,
                decoration: InputDecoration(labelText: uiLiteral('Document type')),
                items: const [
                  DropdownMenuItem(value: 'CONTRACT', child: LText('Contract')),
                  DropdownMenuItem(value: 'INVOICE', child: LText('Invoice')),
                  DropdownMenuItem(value: 'RECEIPT', child: LText('Receipt')),
                  DropdownMenuItem(value: 'PAYMENT_EVIDENCE', child: LText('Payment evidence')),
                  DropdownMenuItem(value: 'OTHER', child: LText('Other')),
                ],
                onChanged: (v) { if (v != null) setLocal(() { kind = v; selectedFile = null; }); },
              ),
              const SizedBox(height: 12),
              TextField(controller: name, decoration: InputDecoration(labelText: uiLiteral('Document name *'))),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: LText(
                      selectedFile?.name ?? 'No file selected',
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(color: brandTextSoft),
                    ),
                  ),
                  const SizedBox(width: 12),
                  OutlinedButton.icon(
                    onPressed: () async {
                      final file = await pickBrowserFile('application/pdf,image/png,image/jpeg,image/webp,text/plain');
                      if (file != null) setLocal(() => selectedFile = file);
                    },
                    icon: const Icon(Icons.upload_file_outlined),
                    label: const LText('Choose file'),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              TextField(controller: note, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('Notes'))),
              const SizedBox(height: 12),
              const _RuleStrip(items: [
                _RuleItem(Icons.shield_outlined, 'Storage', 'Evidence-backed · partner-scoped'),
                _RuleItem(Icons.fingerprint_outlined, 'Integrity', 'SHA-256 checked before Billing link'),
              ]),
            ],
          ),
          primaryLabel: 'Upload and register',
          onPrimary: () {
            if (name.text.trim().isEmpty || selectedFile == null) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: LText('Choose a file and enter a document name.'),
                  behavior: SnackBarBehavior.floating,
                ),
              );
              return;
            }
            Navigator.pop(context, true);
          },
        ),
      ),
    );

    if (ok == true && selectedFile != null) {
      final file = selectedFile!;
      final bytes = await readBrowserFile(file);
      final evidenceType = switch (kind) {
        'CONTRACT' => 'CONTRACT',
        'INVOICE' => 'INVOICE',
        _ => 'OTHER',
      };
      final evidence = await widget.api.multipart('/api/v1/evidence', {
        'partner_id': '${partner['id']}',
        'metric_key': '',
        'evidence_type': evidenceType,
        'title': name.text.trim(),
        'description': note.text.trim(),
        'period_start': '',
        'period_end': '',
      }, bytes, file.name);
      final evidenceId = '${evidence['id'] ?? ''}'.trim();
      if (evidenceId.isEmpty) {
        throw StateError('Evidence upload returned no ID.');
      }
      await widget.api.post('/api/v1/billing/partners/${partner['id']}/documents', {
        'kind': kind,
        'name': name.text.trim(),
        'storage_url': 'evidence://$evidenceId',
        'note': note.text.trim(),
        'mime_type': '${evidence['mime_type'] ?? ''}',
        'sha256': '${evidence['sha256'] ?? ''}',
        'size_bytes': evidence['size_bytes'] ?? 0,
      });
      await load();
      if (mounted) success('Commercial document uploaded and registered.');
    }

    for (final controller in [name, note]) {
      controller.dispose();
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
    final activationFee = TextEditingController(text: number(module['partner_activation_fee']).toStringAsFixed(2));
    final activationFeeEffectiveAt = TextEditingController();
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
                decoration: InputDecoration(labelText: uiLiteral('Module state')),
                items: [
                  const DropdownMenuItem(value: 'ACTIVE', child: LText('ACTIVE')),
                  if ('${module['status']}' == 'NOT_LICENSED')
                    const DropdownMenuItem(value: 'NOT_LICENSED', child: LText('NOT LICENSED')),
                  const DropdownMenuItem(value: 'MAINTENANCE', child: LText('MAINTENANCE')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => state = v); },
              ),
              if ('${module['status']}' == 'ACTIVE') ...[
                const SizedBox(height: 6),
                const LText(
                  'Paid-period deactivation is Billing-managed. Use Cancel at period end; access remains active until the current 30-day period closes.',
                  style: TextStyle(color: brandTextSoft, fontSize: 9.5),
                ),
              ],
              const SizedBox(height: 8),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: visible,
                onChanged: (v) => setLocal(() => visible = v),
                title: const LText('Visible for partner'),
                subtitle: const LText('Visibility is separate from module code existence.'),
              ),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: included,
                onChanged: (v) => setLocal(() => included = v),
                title: const LText('Included in base package'),
                subtitle: const LText('Modules outside the base package contribute to recurring fees.'),
              ),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                value: cancelAtPeriodEnd,
                onChanged: state == 'ACTIVE' ? (v) => setLocal(() => cancelAtPeriodEnd = v) : null,
                title: const LText('Cancel at period end'),
                subtitle: LText(
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
                  decoration: InputDecoration(labelText: uiLiteral('Partner 30-day price')),
                ),
                second: TextField(
                  controller: effectiveAt,
                  decoration: InputDecoration(labelText: uiLiteral('Price effective at'), hintText: uiLiteral('Optional RFC3339 timestamp')),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(
                  controller: activationFee,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Partner activation fee')),
                ),
                second: TextField(
                  controller: activationFeeEffectiveAt,
                  decoration: InputDecoration(labelText: uiLiteral('Activation fee effective at'), hintText: uiLiteral('Optional RFC3339 timestamp')),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: reason,
                decoration: InputDecoration(labelText: uiLiteral('Change reason'), hintText: uiLiteral('Recorded in module, price and subscription history')),
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
          'partner_activation_fee': double.tryParse(activationFee.text) ?? 0,
          'activation_fee_effective_at': activationFeeEffectiveAt.text.trim(),
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
    activationFee.dispose();
    activationFeeEffectiveAt.dispose();
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
        leading: IconButton(
          tooltip: uiLiteral('Back to Partners'),
          onPressed: widget.onBack ?? () => Navigator.pop(context),
          icon: const Icon(Icons.arrow_back_rounded),
        ),
        title: MediaQuery.sizeOf(context).width < 520
            ? const HimateLogo(compact: true, width: 34)
            : const HimateLogo(width: 170),
        actions: [
          if (partner['test_partner'] == true) ...[
            const _StatusPill(label: 'GOLDEN TEST'),
            const SizedBox(width: 8),
          ],
          _StatusPill(label: '${partner['lifecycle'] ?? 'PROSPECT'}'),
          const SizedBox(width: 12),
          IconButton(onPressed: editPartner, tooltip: uiLiteral('Edit partner'), icon: const Icon(Icons.edit_outlined)),
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
                    _localizedPartnerCategory(partner),
                    '${partner['country']}',
                    _humanize('${partner['lifecycle']}'),
                    if ('${partner['primary_domain'] ?? ''}'.isNotEmpty) '${partner['primary_domain']}',
                    '${uiLiteral('Health')}: ${uiLiteral(_humanize('${partner['system_health'] ?? 'UNKNOWN'}'))}',
                    '${uiLiteral('Version')}: ${'${partner['platform_version'] ?? ''}'.isEmpty ? '—' : partner['platform_version']}',
                    if (partner['test_partner'] == true) 'TEST DATA · excluded from platform aggregates',
                  ].join(' · '),
                  actions: [
                    OutlinedButton.icon(onPressed: editPartner, icon: const Icon(Icons.edit_outlined), label: const LText('Company data')),
                    FilledButton.icon(onPressed: terms != null && license != null ? editTerms : null, icon: const Icon(Icons.payments_outlined), label: const LText('Commercial terms')),
                  ],
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (supplementalLoading) ...[
                        const _MessageCard(
                          icon: Icons.sync_rounded,
                          title: 'Secondary data is loading',
                          message: 'The partner workspace is usable now. Billing, modules, impact and environment sections are loading independently.',
                        ),
                        const SizedBox(height: 12),
                      ],
                      if (supplementalError != null) ...[
                        _MessageCard(
                          icon: Icons.sync_problem_outlined,
                          title: 'Secondary data is loading independently',
                          message: supplementalError!,
                        ),
                        const SizedBox(height: 16),
                      ],
                      KeyedSubtree(
                        key: _overviewKey,
                        child: ResponsiveKpiGrid(
                          children: [
                            Kpi(label: 'Current recurring', value: money(billing?['current_total']), note: 'Base + active extra modules', icon: Icons.account_balance_wallet_outlined, accent: brandGold),
                            Kpi(label: 'Active modules', value: '$active', note: '${modules.length} ${uiLiteral('module records')}', icon: Icons.grid_view_outlined, accent: brandNavy),
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
                      const SizedBox(height: 14),
                      start223CommercialWorkflowPanel(),
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
                                label: LText(environments.any((e) => e['kind'] == 'PRODUCTION') ? 'Production settings' : 'Add production'),
                              ),
                              FilledButton.icon(
                                onPressed: startProvisioning,
                                icon: const Icon(Icons.precision_manufacturing_outlined),
                                label: LText(provisioningJob == null ? 'Start provisioning' : 'Resume provisioning'),
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
                                  tooltip: uiLiteral('Edit environment'),
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
                              decoration: InputDecoration(hintText: uiLiteral('Search modules...'), prefixIcon: Icon(Icons.search_rounded)),
                            );
                            final state = DropdownButtonFormField<String>(
                              value: moduleState,
                              decoration: InputDecoration(labelText: uiLiteral('State')),
                              items: const [
                                DropdownMenuItem(value: 'ALL', child: LText('All states')),
                                DropdownMenuItem(value: 'ACTIVE', child: LText('Active')),
                                DropdownMenuItem(value: 'NOT_LICENSED', child: LText('Not licensed')),
                                DropdownMenuItem(value: 'MAINTENANCE', child: LText('Maintenance')),
                              ],
                              onChanged: (v) => setState(() => moduleState = v ?? 'ALL'),
                            );
                            if (c.maxWidth < 680) return Column(children: [search, const SizedBox(height: 10), state]);
                            return Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: state)]);
                          },
                        ),
                      ),
                      const SizedBox(height: 12),
                      for (final entry in groupedFilteredModules.entries) ...[
                        _SectionHeader(
                          title: entry.key,
                          subtitle: entry.value.isEmpty
                              ? 'No modules in this category for this partner.'
                              : '${entry.value.length} module${entry.value.length == 1 ? '' : 's'} in this partner category.',
                          trailing: _MiniCounter(label: '${entry.value.length} MODULES'),
                        ),
                        const SizedBox(height: 10),
                        if (entry.value.isEmpty)
                          const _MessageCard(
                            icon: Icons.inbox_outlined,
                            title: 'No module entitlement',
                            message: 'There is no module to load in this category. The page will not retry an empty dataset.',
                          )
                        else
                          LayoutBuilder(
                            builder: (context, c) {
                              final width = c.maxWidth < 620 ? c.maxWidth : c.maxWidth < 1020 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                              return Wrap(
                                spacing: 12,
                                runSpacing: 12,
                                children: [
                                  for (final m in entry.value)
                                    SizedBox(width: width, child: PartnerModuleCard(module: m, onTap: () => editModule(m))),
                                ],
                              );
                            },
                          ),
                        const SizedBox(height: 18),
                      ],
                      const SizedBox(height: 26),
                      KeyedSubtree(
                        key: _financeKey,
                        child: _SectionHeader(
                          title: 'Finance & Documents',
                          subtitle: 'Commercial evidence and internal invoice records for this partner.',
                          trailing: FilledButton.icon(onPressed: addDocument, icon: const Icon(Icons.note_add_outlined), label: const LText('Register document')),
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
                        key: _usersKey,
                        child: _SectionHeader(
                          title: 'Partner Portal Access',
                          subtitle: 'Tenant-scoped partner identities. These users can never inherit HIMATE platform-administrator authority.',
                          trailing: FilledButton.icon(
                            onPressed: createPortalUser,
                            icon: const Icon(Icons.person_add_alt_1_rounded),
                            label: LText(portalUsers.isEmpty ? 'Create portal owner' : 'Add portal user'),
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      portalUsers.isEmpty
                          ? const _MessageCard(
                              icon: Icons.group_outlined,
                              title: 'No Partner Portal user yet',
                              message: 'Create the first Owner account to let this partner sign in at /partner/login.',
                            )
                          : LayoutBuilder(
                              builder: (context, c) {
                                final width = c.maxWidth < 680 ? c.maxWidth : (c.maxWidth - 12) / 2;
                                return Wrap(
                                  spacing: 12,
                                  runSpacing: 12,
                                  children: [
                                    for (final portalUser in portalUsers)
                                      SizedBox(
                                        width: width,
                                        child: _InfoCard(
                                          title: '${portalUser['name'] ?? 'Portal user'}',
                                          icon: Icons.person_outline_rounded,
                                          action: IconButton(
                                            tooltip: uiLiteral('Edit portal user'),
                                            onPressed: () => editPortalUser(portalUser),
                                            icon: const Icon(Icons.edit_outlined, size: 18),
                                          ),
                                          children: [
                                            _DefinitionRow(label: 'Email', value: '${portalUser['email'] ?? '—'}'),
                                            _DefinitionRow(label: 'Portal role', value: _humanize('${portalUser['role'] ?? 'viewer'}')),
                                            _DefinitionRow(label: 'Status', value: portalUser['active'] == true ? 'Active' : 'Inactive'),
                                            _DefinitionRow(label: 'Portal URL', value: '/partner/login'),
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
                            label: const LText('Generate / rotate credential'),
                          ),
                        ),
                      ),
                      const SizedBox(height: 12),
                      start223WebsiteAdapterPanel(),
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

class PackagesPage extends StatefulWidget {
  const PackagesPage({required this.api, super.key});
  final Api api;

  @override
  State<PackagesPage> createState() => _PackagesPageState();
}

class _PackagesPageState extends State<PackagesPage> {
  bool loading = true;
  bool analyticsLoading = true;
  String? error;
  String? analyticsError;
  List<Map<String, dynamic>> plans = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  Map<String,dynamic> analytics = <String,dynamic>{};

  @override
  void initState() {
    super.initState();
    unawaited(load());
  }

  Future<void> load() async {
    if (mounted) {
      setState(() {
        loading = true;
        analyticsLoading = true;
        error = null;
        analyticsError = null;
      });
    }
    try {
      final model = await widget.api.get(
        '/api/v1/central/packages',
        maxAge: const Duration(seconds: 5),
      );
      if (!mounted) return;
      final meta = model['meta'] is Map
          ? Map<String, dynamic>.from(model['meta'] as Map)
          : <String, dynamic>{};
      final unavailable = meta['unavailable'] is List
          ? (meta['unavailable'] as List).map((e) => '$e').toSet()
          : <String>{};
      setState(() {
        plans = items(<String, dynamic>{'items': model['plans']});
        modules = items(<String, dynamic>{'items': model['modules']});
        analytics = model['analytics'] is Map
            ? Map<String, dynamic>.from(model['analytics'] as Map)
            : <String, dynamic>{};
        loading = false;
        analyticsLoading = false;
        analyticsError = unavailable.contains('analytics')
            ? 'Package analytics is temporarily unavailable. Package definitions remain usable.'
            : null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        analyticsLoading = false;
        error = e.toString();
      });
    }
  }

  String _packageDescription(Map<String,dynamic> plan) {
    return switch ('${plan['plan_key']}') {
      'STARTER' => '10 HIMATE-defined modules for focused teams and first deployments.',
      'BUSINESS' => '20 HIMATE-defined modules for broader operating workflows.',
      'FLEX' => 'Unlimited access to every current and future eligible module.',
      _ => '',
    };
  }

  String _packageEntitlement(Map<String,dynamic> plan) =>
      '${plan['entitlement'] ?? '—'}';

  String moduleLabel(Map<String, dynamic> module) =>
      '${module['label'] ?? module['label_en'] ?? module['key'] ?? ''}';

  bool moduleReady(Map<String, dynamic> module) =>
      '${module['publication_status'] ?? ''}' == 'PUBLISHED' &&
      '${module['implementation_state'] ?? ''}' == 'READY';

  Future<void> editPackage(Map<String, dynamic> plan) async {
    final key = '${plan['plan_key']}';
    final limit = (plan['module_limit'] as num?)?.toInt() ?? 0;
    final mode = '${plan['selection_mode']}';
    final fixed = mode == 'FIXED';
    final unlimited = mode == 'UNLIMITED';
    final price = TextEditingController(text: number(plan['monthly_price']).toStringAsFixed(2));
    final effective = TextEditingController();
    final reason = TextEditingController();
    final selected = <String>{
      for (final value in (plan['fixed_module_keys'] is List ? plan['fixed_module_keys'] as List : const []))
        '$value',
    };

    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: '${plan['display_name']} package',
          subtitle: fixed
              ? 'HIMATE defines exactly $limit included modules. Price changes apply to all active customers from the effective date.'
              : unlimited
                  ? 'Premium is Unlimited: every current and future eligible module is included automatically. Price changes apply to all active customers from the effective date.'
                  : 'Partner-selectable package. Price changes apply to all active customers from the effective date.',
          icon: Icons.inventory_2_outlined,
          width: 820,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: TextField(
                  controller: price,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Monthly package price')),
                ),
                second: TextField(
                  controller: effective,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Price effective date'),
                    hintText: uiLiteral('YYYY-MM-DD · blank = today'),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              const _RuleStrip(items: [
                _RuleItem(Icons.trending_up_rounded, 'Annual uplift', '5% every January 1'),
                _RuleItem(Icons.history_rounded, 'Pricing', 'Effective-dated · audited'),
                _RuleItem(Icons.receipt_long_outlined, 'Existing invoices', 'Never rewritten'),
              ]),
              const SizedBox(height: 12),
              _DefinitionRow(label: 'Module limit', value: unlimited ? 'Unlimited' : '$limit'),
              _DefinitionRow(label: 'Selection mode', value: fixed ? 'HIMATE fixed package' : unlimited ? 'Automatic Unlimited entitlement' : 'Partner selectable'),
              _DefinitionRow(label: 'Annual uplift', value: '${plan['annual_increase_percent'] ?? 5}% · January 1'),
              if (fixed) ...[
                const SizedBox(height: 16),
                _SectionHeader(
                  title: 'Included modules',
                  subtitle: 'Select exactly $limit published and implementation-ready modules.',
                ),
                const SizedBox(height: 10),
                Align(
                  alignment: Alignment.centerLeft,
                  child: _MiniCounter(label: '${selected.length} / $limit SELECTED'),
                ),
                const SizedBox(height: 10),
                SizedBox(
                  height: 280,
                  child: SingleChildScrollView(
                    child: Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        for (final module in modules)
                          FilterChip(
                            selected: selected.contains('${module['key']}'),
                            onSelected: moduleReady(module)
                                ? (value) => setLocal(() {
                                      final moduleKey = '${module['key']}';
                                      if (value) {
                                        if (selected.length < limit) selected.add(moduleKey);
                                      } else {
                                        selected.remove(moduleKey);
                                      }
                                    })
                                : null,
                            label: LText(
                              moduleLabel(module),
                              style: const TextStyle(fontSize: 10),
                            ),
                            tooltip: moduleReady(module)
                                ? '${module['key']}'
                                : 'Not yet PUBLISHED + READY',
                          ),
                      ],
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 14),
              TextField(
                controller: reason,
                decoration: InputDecoration(
                  labelText: uiLiteral('Change reason'),
                  hintText: uiLiteral('Required for commercial audit trail'),
                ),
              ),
            ],
          ),
          primaryLabel: 'Save package',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );

    if (ok == true) {
      final monthly = double.tryParse(price.text.trim());
      if (monthly == null || monthly < 0) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: LText('Package price must be zero or greater.'), behavior: SnackBarBehavior.floating),
          );
        }
      } else if (fixed && selected.length != limit) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: LText('Select exactly $limit modules for $key.'), behavior: SnackBarBehavior.floating),
          );
        }
      } else {
        final payload = <String, dynamic>{
          'monthly_price': monthly,
          'reason': reason.text.trim().isEmpty ? 'HIMATE administrator package update' : reason.text.trim(),
          if (effective.text.trim().isNotEmpty) 'effective_at': effective.text.trim(),
          if (fixed) 'fixed_module_keys': selected.toList()..sort(),
        };
        await widget.api.patch('/api/v1/billing/plans/$key', payload);
        await load();
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: LText('$key package updated.'), behavior: SnackBarBehavior.floating),
          );
        }
      }
    }
    price.dispose();
    effective.dispose();
    reason.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (loading && plans.isEmpty) {
      return const Content(
        eyebrow: 'COMMERCIAL CONTROL PLANE',
        title: 'Packages',
        subtitle: 'Central subscription packages, prices and module entitlements.',
        child: _BrandLoading(),
      );
    }
    if (error != null && plans.isEmpty) {
      return Content(
        eyebrow: 'COMMERCIAL CONTROL PLANE',
        title: 'Packages',
        subtitle: 'Central subscription packages, prices and module entitlements.',
        actions: [OutlinedButton.icon(onPressed: load, icon: const Icon(Icons.refresh_rounded), label: const LText('Retry'))],
        child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Packages could not be loaded', message: error!),
      );
    }
    final analyticsPackages = analytics['packages'] is List
        ? (analytics['packages'] as List).whereType<Map>().map((e) => Map<String,dynamic>.from(e)).toList()
        : <Map<String,dynamic>>[];
    final analyticsPartners = analytics['partners'] is List
        ? (analytics['partners'] as List).whereType<Map>().map((e) => Map<String,dynamic>.from(e)).toList()
        : <Map<String,dynamic>>[];
    final activityMeasured = analytics['portal_activity_measured'] == true;

    return Content(
      eyebrow: 'COMMERCIAL CONTROL PLANE',
      title: 'Packages',
      subtitle: 'Starter, Business and Premium package control with usage and commercial analytics.',
      actions: [
        OutlinedButton.icon(
          onPressed: () => openBrowserDownload('/api/v1/billing/packages/export.pdf'),
          icon: const Icon(Icons.download_outlined),
          label: const LText('Export PDF'),
        ),
        OutlinedButton.icon(onPressed: loading ? null : load, icon: const Icon(Icons.refresh_rounded), label: const LText('Refresh')),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 720
                  ? constraints.maxWidth
                  : (constraints.maxWidth - 24) / 3;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final plan in plans)
                    SizedBox(
                      width: width,
                      child: _PackageOverviewCard(
                        name: '${plan['display_name'] ?? plan['plan_key']}',
                        price: '${plan['display_price'] ?? '—'}',
                        description: _packageDescription(plan),
                        entitlement: _packageEntitlement(plan),
                        active: plan['active'] == true,
                        onTap: () => unawaited(editPackage(plan)),
                        onEdit: () => unawaited(editPackage(plan)),
                      ),
                    ),
                ],
              );
            },
          ),
          const SizedBox(height: 24),
          _SectionHeader(
            title: 'Package Analytics',
            subtitle: 'Partner distribution, package usage, Portal activity and current commercial context from authoritative runtime data.',
            trailing: analyticsLoading ? const _MiniCounter(label: 'REFRESHING') : _MiniCounter(label: '${analyticsPartners.length} PARTNERS'),
          ),
          const SizedBox(height: 12),
          if (analyticsError != null && analytics.isEmpty)
            _MessageCard(
              icon: Icons.query_stats_outlined,
              title: 'Package analytics is temporarily unavailable',
              message: analyticsError!,
            )
          else if (analyticsLoading && analytics.isEmpty)
            const _MessageCard(
              icon: Icons.sync_rounded,
              title: 'Loading package analytics',
              message: 'Package cards remain usable while analytics loads independently.',
            )
          else if (analyticsPackages.isEmpty)
            const _MessageCard(
              icon: Icons.bar_chart_outlined,
              title: 'No package analytics yet',
              message: 'There is no package subscription data to chart. No retry loop is started for an empty dataset.',
            )
          else ...[
            _PackageAnalyticsChart(packages: analyticsPackages),
            const SizedBox(height: 14),
            if (!activityMeasured)
              const _MessageCard(
                icon: Icons.schedule_outlined,
                title: 'Portal active-time measurement has just been enabled',
                message: 'No historical online-hours estimate is invented. Five-minute authenticated activity buckets will populate this metric from the CENTRAL-8 deployment forward.',
              ),
            if (!activityMeasured) const SizedBox(height: 14),
            _InfoCard(
              title: 'Partner package usage',
              icon: Icons.groups_2_outlined,
              children: analyticsPartners.isEmpty
                  ? const [
                      _EmptyInline(
                        icon: Icons.inbox_outlined,
                        title: 'No partner subscriptions recorded',
                      ),
                    ]
                  : [
                      for (final partner in analyticsPartners.take(50))
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 8),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(children: [
                                Expanded(
                                  child: LText(
                                    '${partner['display_name'] ?? partner['partner_id']}',
                                    style: const TextStyle(color: brandNavy, fontSize: 13, fontWeight: FontWeight.w800),
                                  ),
                                ),
                                _StatusPill(label: '${partner['plan_name'] ?? partner['plan_key']}'),
                              ]),
                              const SizedBox(height: 5),
                              Wrap(
                                spacing: 7,
                                runSpacing: 7,
                                children: [
                                  _MiniCounter(label: '${partner['billing_frequency'] ?? '—'}'),
                                  _MiniCounter(label: '${partner['classification'] ?? '—'}'),
                                  _MiniCounter(label: '${partner['onboarding_state'] ?? '—'}'),
                                  _MiniCounter(label: '${partner['module_usage_events_30d'] ?? 0} MODULE USES / 30D'),
                                  _MiniCounter(
                                    label: partner['portal_activity_measured'] == true && number(partner['portal_active_hours_30d']) > 0
                                        ? '${number(partner['portal_active_hours_30d']).toStringAsFixed(1)} PORTAL HOURS / 30D'
                                        : 'NO PORTAL ACTIVITY RECORDED',
                                  ),
                                ],
                              ),
                              const SizedBox(height: 5),
                              LText(
                                [
                                  if ('${partner['legal_name'] ?? ''}'.trim().isNotEmpty) '${partner['legal_name']}',
                                  if ('${partner['country'] ?? ''}'.trim().isNotEmpty) '${partner['country']}',
                                  if ('${partner['quote_reference'] ?? ''}'.trim().isNotEmpty) 'Quote: ${partner['quote_reference']}',
                                ].join(' · '),
                                style: const TextStyle(color: brandTextSoft, fontSize: 10.5),
                              ),
                              const Divider(height: 18),
                            ],
                          ),
                        ),
                    ],
            ),
          ],
        ],
      ),
    );
  }
}

class _PackageOverviewCard extends StatefulWidget {
  const _PackageOverviewCard({
    required this.name,
    required this.price,
    required this.description,
    required this.entitlement,
    required this.active,
    required this.onTap,
    required this.onEdit,
  });
  final String name;
  final String price;
  final String description;
  final String entitlement;
  final bool active;
  final VoidCallback onTap;
  final VoidCallback onEdit;

  @override
  State<_PackageOverviewCard> createState() => _PackageOverviewCardState();
}

class _PackageOverviewCardState extends State<_PackageOverviewCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) => MouseRegion(
    onEnter: (_) => setState(() => hover = true),
    onExit: (_) => setState(() => hover = false),
    child: AnimatedContainer(
      duration: const Duration(milliseconds: 160),
      transform: Matrix4.translationValues(0, hover ? -3 : 0, 0),
      decoration: BoxDecoration(
        color: brandSurfaceRaised,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: hover ? brandGold.withOpacity(.85) : brandIonBlue.withOpacity(.30), width: hover ? 1.5 : 1.0),
        boxShadow: [BoxShadow(color: brandNavy.withOpacity(hover ? .14 : .075), blurRadius: hover ? 24 : 15, offset: Offset(0, hover ? 10 : 6))],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: widget.onTap,
          borderRadius: BorderRadius.circular(14),
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(color: brandGold.withOpacity(.12), borderRadius: BorderRadius.circular(11)),
                    child: const Icon(Icons.inventory_2_outlined, color: brandNavy, size: 23),
                  ),
                  const Spacer(),
                  IconButton(
                    tooltip: uiLiteral('Edit package'),
                    onPressed: widget.onEdit,
                    icon: const Icon(Icons.edit_outlined, size: 19),
                  ),
                ]),
                const SizedBox(height: 18),
                LText(widget.name, style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: 28, fontWeight: FontWeight.w700)),
                const SizedBox(height: 4),
                LText(widget.price, style: const TextStyle(color: brandTextSoft, fontSize: 13, fontWeight: FontWeight.w600)),
                const SizedBox(height: 14),
                LText(widget.description, style: const TextStyle(color: brandCharcoal, fontSize: 11.5, height: 1.45)),
                const SizedBox(height: 14),
                _RuleStrip(items: [
                  _RuleItem(Icons.widgets_outlined, 'Included', widget.entitlement),
                  _RuleItem(Icons.circle, 'Status', widget.active ? 'ACTIVE' : 'INACTIVE'),
                ]),
                const SizedBox(height: 18),
                Row(children: [
                  const LText('Package details', style: TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w800)),
                  const Spacer(),
                  AnimatedSlide(
                    offset: hover ? const Offset(.14, 0) : Offset.zero,
                    duration: const Duration(milliseconds: 150),
                    child: const Icon(Icons.arrow_forward_rounded, color: brandGold, size: 20),
                  ),
                ]),
              ],
            ),
          ),
        ),
      ),
    ),
  );
}

class _PackageAnalyticsChart extends StatelessWidget {
  const _PackageAnalyticsChart({required this.packages});
  final List<Map<String,dynamic>> packages;

  @override
  Widget build(BuildContext context) {
    final maxPartners = packages.fold<double>(0, (m, p) => math.max(m, number(p['active_partner_count'])));
    final maxUsage = packages.fold<double>(0, (m, p) => math.max(m, number(p['module_usage_events_30d'])));
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const LText('Package distribution & usage', style: TextStyle(color: brandNavy, fontSize: 15, fontWeight: FontWeight.w800)),
            const SizedBox(height: 16),
            for (final package in packages) ...[
              Row(children: [
                SizedBox(
                  width: 90,
                  child: LText('${package['display_name']}', style: const TextStyle(color: brandNavy, fontSize: 11, fontWeight: FontWeight.w700)),
                ),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      LinearProgressIndicator(
                        value: maxPartners <= 0 ? 0 : number(package['active_partner_count']) / maxPartners,
                        minHeight: 9,
                        borderRadius: BorderRadius.circular(99),
                        color: brandNavy,
                        backgroundColor: brandMist,
                      ),
                      const SizedBox(height: 5),
                      LText(
                        '${package['active_partner_count'] ?? 0} active partners · ${package['module_usage_events_30d'] ?? 0} module uses / 30d · ${number(package['portal_active_hours_30d']).toStringAsFixed(1)} Portal hours / 30d',
                        style: const TextStyle(color: brandTextSoft, fontSize: 9.5),
                      ),
                      if (maxUsage > 0) ...[
                        const SizedBox(height: 5),
                        LinearProgressIndicator(
                          value: number(package['module_usage_events_30d']) / maxUsage,
                          minHeight: 5,
                          borderRadius: BorderRadius.circular(99),
                          color: brandGold,
                          backgroundColor: brandMist,
                        ),
                      ],
                    ],
                  ),
                ),
              ]),
              const SizedBox(height: 16),
            ],
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
  Map<String, dynamic>? profile;
  Map<String, dynamic> overview = <String, dynamic>{};
  Map<String, dynamic> financeKpis = <String, dynamic>{};
  List<Map<String, dynamic>> invoices = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> partners = <Map<String, dynamic>>[];
  String invoiceFilter = 'ALL';
  String revenuePeriod = 'MONTHLY';
  String revenuePlan = 'ALL';
  final GlobalKey onboardingKey = GlobalKey();
  bool loading = false;
  String? error;

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) {
      setState(() {
        loading = true;
        error = null;
      });
    }
    try {
      final model = await widget.api.get(
        '/api/v1/central/finance',
        maxAge: const Duration(seconds: 5),
      );
      if (!mounted) return;
      setState(() {
        profile = model['profile'] is Map
            ? Map<String, dynamic>.from(model['profile'] as Map)
            : null;
        overview = model['overview'] is Map
            ? Map<String, dynamic>.from(model['overview'] as Map)
            : <String, dynamic>{};
        financeKpis = model['kpis'] is Map
            ? Map<String, dynamic>.from(model['kpis'] as Map)
            : <String, dynamic>{};
        invoices = items(<String, dynamic>{'items': model['invoices']});
        partners = items(<String, dynamic>{'items': model['partners']});
        loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        error = e.toString();
      });
    }
  }

  void success(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: LText(message), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
    );
  }

  void failure(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: LText(message), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
    );
  }

  List<Map<String, dynamic>> get currencyRows {
    final raw = overview['currencies'];
    if (raw is! List) return <Map<String, dynamic>>[];
    return raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  List<Map<String, dynamic>> get onboardingRows {
    final raw = overview['onboarding'];
    if (raw is! Map || raw['items'] is! List) return <Map<String, dynamic>>[];
    return (raw['items'] as List).whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  List<Map<String, dynamic>> get filteredInvoices => invoiceFilter == 'ALL'
      ? invoices
      : invoices.where((invoice) => '${invoice['workflow_status'] ?? invoice['status'] ?? ''}' == invoiceFilter).toList();

  String partnerName(String id) {
    for (final partner in partners) {
      if ('${partner['id'] ?? ''}' == id) {
        final display = '${partner['display_name'] ?? ''}'.trim();
        if (display.isNotEmpty) return display;
      }
    }
    return id;
  }

  int workflowCount(String key) => currencyRows.fold<int>(0, (sum, row) => sum + (row[key] is num ? (row[key] as num).toInt() : int.tryParse('${row[key]}') ?? 0));

  String moneyAcrossCurrencies(String key) {
    final nonZero = currencyRows.where((row) => number(row[key]) != 0).toList();
    if (nonZero.isEmpty) return '\$0.00';
    if (nonZero.length == 1) {
      final row = nonZero.first;
      return '${row['currency'] ?? 'USD'} ${number(row[key]).toStringAsFixed(2)}';
    }
    return '${nonZero.length} currencies';
  }

  String get chartCurrency => currencyRows.isEmpty ? 'USD' : '${currencyRows.first['currency'] ?? 'USD'}';

  String get revenuePlanKey => switch (revenuePlan) {
    'Starter' => 'STARTER',
    'Business' => 'BUSINESS',
    'Premium' => 'FLEX',
    _ => 'ALL',
  };

  List<Map<String, dynamic>> get chartRows {
    final byPlan = revenuePlanKey != 'ALL';
    final key = revenuePeriod == 'WEEKLY'
        ? (byPlan ? 'weekly_paid_by_plan' : 'weekly_paid')
        : (byPlan ? 'monthly_paid_by_plan' : 'monthly_paid');
    final raw = overview[key];
    if (raw is! List) return <Map<String, dynamic>>[];
    return raw
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .where((row) =>
            '${row['currency'] ?? ''}' == chartCurrency &&
            (!byPlan || '${row['plan_key'] ?? ''}' == revenuePlanKey))
        .map((row) => <String,dynamic>{
              ...row,
              'period': row['period'] ?? row['month'] ?? '',
            })
        .toList();
  }

  String get financeExportPath {
    final params = <String,String>{};
    if (invoiceFilter != 'ALL') params['status'] = invoiceFilter;
    if (revenuePlanKey != 'ALL') params['plan_key'] = revenuePlanKey;
    return Uri(path: '/api/v1/billing/finance/export.pdf', queryParameters: params.isEmpty ? null : params).toString();
  }

  Future<void> createManualInvoice({String? partnerID}) async {
    String selectedPartner = partnerID ?? (partners.isNotEmpty ? '${partners.first['id'] ?? ''}' : '');
    final description = TextEditingController(text: 'HIMATE service');
    final net = TextEditingController();
    final currency = TextEditingController(text: 'USD');
    final start = TextEditingController();
    final end = TextEditingController();
    final due = TextEditingController();
    final notes = TextEditingController();

    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setLocal) => BrandDialog(
          title: 'Create invoice draft',
          subtitle: 'Manual invoices are always created as DRAFT. Approval is required before sending or collecting payment.',
          icon: Icons.receipt_long_outlined,
          width: 720,
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            DropdownButtonFormField<String>(
              value: selectedPartner.isEmpty ? null : selectedPartner,
              isExpanded: true,
              decoration: InputDecoration(labelText: uiLiteral('Partner')),
              items: [
                for (final p in partners)
                  DropdownMenuItem(
                    value: '${p['id']}',
                    child: LText('${p['display_name'] ?? p['id']}'),
                  ),
              ],
              onChanged: (value) => setLocal(() => selectedPartner = value ?? ''),
            ),
            const SizedBox(height: 12),
            TextField(controller: description, decoration: InputDecoration(labelText: uiLiteral('Description'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(
                controller: net,
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                decoration: InputDecoration(labelText: uiLiteral('Net amount')),
              ),
              second: TextField(controller: currency, decoration: InputDecoration(labelText: uiLiteral('Currency'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: start, decoration: InputDecoration(labelText: uiLiteral('Service period start'), hintText: 'YYYY-MM-DD')),
              second: TextField(controller: end, decoration: InputDecoration(labelText: uiLiteral('Service period end'), hintText: 'YYYY-MM-DD')),
            ),
            const SizedBox(height: 12),
            TextField(controller: due, decoration: InputDecoration(labelText: uiLiteral('Due date'), hintText: 'YYYY-MM-DD')),
            const SizedBox(height: 12),
            TextField(controller: notes, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('Notes'))),
          ]),
          primaryLabel: 'Create draft',
          onPrimary: () => Navigator.pop(dialogContext, true),
        ),
      ),
    );

    if (ok == true) {
      final amount = double.tryParse(net.text.trim().replaceAll(',', '.'));
      if (selectedPartner.isEmpty || amount == null || amount <= 0 || description.text.trim().isEmpty) {
        failure('Partner, description and a positive net amount are required.');
      } else {
        try {
          await widget.api.post('/api/v1/billing/invoices', {
            'partner_id': selectedPartner,
            'currency': currency.text.trim().toUpperCase(),
            'description': description.text.trim(),
            'net_amount': amount,
            'service_period_start': start.text.trim(),
            'service_period_end_exclusive': end.text.trim(),
            'due_date': due.text.trim(),
            'notes': notes.text.trim(),
          });
          invoiceFilter = 'DRAFT';
          await load();
          if (mounted) success('Invoice draft created.');
        } catch (e) {
          if (mounted) failure(e.toString());
        }
      }
    }

    for (final controller in [description, net, currency, start, end, due, notes]) {
      controller.dispose();
    }
  }

  Future<void> invoiceAction(Map<String, dynamic> invoice, String action) async {
    final reason = TextEditingController();
    final paymentReference = TextEditingController();
    final label = switch (action) {
      'approve' => 'Approve invoice',
      'send' => 'Send invoice',
      'mark-paid' => 'Mark paid',
      'cancel' => 'Cancel invoice',
      _ => 'Update invoice',
    };
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: label,
        subtitle: '${invoice['id']} · ${partnerName('${invoice['partner_id']}')} · ${invoice['currency']} ${number(invoice['gross_total'] ?? invoice['total']).toStringAsFixed(2)}',
        icon: action == 'cancel' ? Icons.cancel_outlined : Icons.receipt_long_outlined,
        width: 560,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(controller: reason, maxLines: 2, decoration: InputDecoration(labelText: uiLiteral('Reason / note'))),
          if (action == 'mark-paid') ...[
            const SizedBox(height: 12),
            TextField(controller: paymentReference, decoration: InputDecoration(labelText: uiLiteral('Payment reference'))),
          ],
        ]),
        primaryLabel: label,
        onPrimary: () => Navigator.pop(dialogContext, true),
      ),
    );
    if (ok == true) {
      try {
        await widget.api.post('/api/v1/billing/invoices/${invoice['id']}/$action', {
          'reason': reason.text.trim(),
          'payment_reference': paymentReference.text.trim(),
        });
        await load();
        if (mounted) success('$label completed.');
      } catch (e) {
        if (mounted) failure(e.toString());
      }
    }
    reason.dispose();
    paymentReference.dispose();
  }

  Future<void> onboardingTransition(Map<String, dynamic> row, String nextState, {String? reason}) async {
    try {
      await widget.api.patch('/api/v1/billing/partners/${row['partner_id']}/onboarding', {
        'state': nextState,
        'classification': '${row['classification'] ?? 'UNCLASSIFIED'}',
        'reason': reason ?? 'Central-6 administrator onboarding workflow',
      });
      await load();
      if (mounted) success('Onboarding moved to ${_humanize(nextState)}.');
    } catch (e) {
      if (mounted) failure(e.toString());
    }
  }

  Future<void> classifyOnboarding(Map<String, dynamic> row) async {
    String classification = 'PAID';
    final reason = TextEditingController();
    final nominal = TextEditingController();
    final evidence = TextEditingController();
    final currency = TextEditingController(text: 'USD');

    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setLocal) => BrandDialog(
          title: 'Classify partner',
          subtitle: 'Paid partners continue through invoice and payment. Charity, Sponsored and Complimentary partners use documented zero-dollar support instead of an invoice.',
          icon: Icons.rule_folder_outlined,
          width: 680,
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            DropdownButtonFormField<String>(
              value: classification,
              decoration: InputDecoration(labelText: uiLiteral('Commercial classification')),
              items: const [
                DropdownMenuItem(value: 'PAID', child: LText('Paid')),
                DropdownMenuItem(value: 'CHARITY', child: LText('Charity')),
                DropdownMenuItem(value: 'SPONSORED', child: LText('Sponsored')),
                DropdownMenuItem(value: 'COMPLIMENTARY', child: LText('Complimentary')),
              ],
              onChanged: (value) => setLocal(() => classification = value ?? 'PAID'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: reason,
              maxLines: 3,
              decoration: InputDecoration(
                labelText: uiLiteral(classification == 'PAID' ? 'Classification note' : 'Support / waiver reason'),
              ),
            ),
            if (classification != 'PAID') ...[
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(
                  controller: nominal,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  decoration: InputDecoration(labelText: uiLiteral('Nominal supported value')),
                ),
                second: TextField(controller: currency, decoration: InputDecoration(labelText: uiLiteral('Currency'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: evidence, decoration: InputDecoration(labelText: uiLiteral('Evidence reference'))),
            ],
          ]),
          primaryLabel: 'Save classification',
          onPrimary: () => Navigator.pop(dialogContext, true),
        ),
      ),
    );

    if (ok == true) {
      if (classification != 'PAID' && reason.text.trim().isEmpty) {
        failure('A documented support / waiver reason is required.');
      } else {
        try {
          await widget.api.patch('/api/v1/billing/partners/${row['partner_id']}/onboarding', {
            'state': 'CLASSIFIED',
            'classification': classification,
            'reason': reason.text.trim(),
            'nominal_value': double.tryParse(nominal.text.trim().replaceAll(',', '.')) ?? 0,
            'currency': currency.text.trim().toUpperCase(),
            'evidence_reference': evidence.text.trim(),
          });
          await load();
          if (mounted) success('Partner classification saved.');
        } catch (e) {
          if (mounted) failure(e.toString());
        }
      }
    }

    for (final controller in [reason, nominal, evidence, currency]) {
      controller.dispose();
    }
  }

  Widget onboardingAction(Map<String, dynamic> row) {
    final state = '${row['state'] ?? ''}';
    final classification = '${row['classification'] ?? 'UNCLASSIFIED'}';
    switch (state) {
      case 'REGISTERED':
        return FilledButton.icon(
          onPressed: () => onboardingTransition(row, 'PENDING_REVIEW'),
          icon: const Icon(Icons.fact_check_outlined),
          label: const LText('Start review'),
        );
      case 'PENDING_REVIEW':
        return FilledButton.icon(
          onPressed: () => classifyOnboarding(row),
          icon: const Icon(Icons.rule_folder_outlined),
          label: const LText('Classify'),
        );
      case 'CLASSIFIED':
        if (classification == 'PAID') {
          return FilledButton.icon(
            onPressed: () => createManualInvoice(partnerID: '${row['partner_id']}'),
            icon: const Icon(Icons.receipt_long_outlined),
            label: const LText('Create invoice'),
          );
        }
        return FilledButton.icon(
          onPressed: () => onboardingTransition(row, 'ADMIN_APPROVAL', reason: 'Zero-dollar support documentation verified'),
          icon: const Icon(Icons.verified_outlined),
          label: const LText('Send to approval'),
        );
      case 'ADMIN_APPROVAL':
        return FilledButton.icon(
          onPressed: () => onboardingTransition(row, 'ACTIVE', reason: 'Final HIMATE administrator approval'),
          icon: const Icon(Icons.check_circle_outline_rounded),
          label: const LText('Activate partner'),
        );
      default:
        return _StatusPill(label: _humanize(state).toUpperCase());
    }
  }

  Widget financeChart() {
    final rows = chartRows;
    final windowLabel = revenuePeriod == 'WEEKLY' ? 'last 4 weeks' : 'last 12 months';
    final planLabel = revenuePlan == 'ALL' ? 'All revenue' : revenuePlan;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(18),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          LayoutBuilder(builder: (context, constraints) {
            final controls = Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                SizedBox(
                  width: 132,
                  child: DropdownButtonFormField<String>(
                    value: revenuePeriod,
                    isDense: true,
                    decoration: const InputDecoration(labelText: 'Period'),
                    items: const [
                      DropdownMenuItem(value: 'WEEKLY', child: LText('Weekly')),
                      DropdownMenuItem(value: 'MONTHLY', child: LText('Monthly')),
                    ],
                    onChanged: (value) {
                      if (value != null) setState(() => revenuePeriod = value);
                    },
                  ),
                ),
                SizedBox(
                  width: 150,
                  child: DropdownButtonFormField<String>(
                    value: revenuePlan,
                    isDense: true,
                    decoration: const InputDecoration(labelText: 'Package'),
                    items: const [
                      DropdownMenuItem(value: 'ALL', child: LText('All')),
                      DropdownMenuItem(value: 'Starter', child: LText('Starter')),
                      DropdownMenuItem(value: 'Business', child: LText('Business')),
                      DropdownMenuItem(value: 'Premium', child: LText('Premium')),
                    ],
                    onChanged: (value) {
                      if (value != null) setState(() => revenuePlan = value);
                    },
                  ),
                ),
              ],
            );
            if (constraints.maxWidth < 720) {
              return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                LText('Paid revenue · $windowLabel · $planLabel', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 14)),
                const SizedBox(height: 12),
                controls,
              ]);
            }
            return Row(children: [
              Expanded(child: LText('Paid revenue · $windowLabel · $planLabel', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 14))),
              controls,
              const SizedBox(width: 8),
              _MiniCounter(label: chartCurrency),
            ]);
          }),
          const SizedBox(height: 18),
          if (rows.isEmpty)
            const _MessageCard(
              icon: Icons.bar_chart_outlined,
              title: 'No paid revenue in this view',
              message: 'There is no ledger data for the selected period/package. The chart stays empty instead of retrying indefinitely.',
            )
          else
            SizedBox(
              height: 205,
              child: LayoutBuilder(builder: (context, constraints) {
                final maxValue = rows.fold<double>(0, (max, row) => math.max(max, number(row['paid'])));
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    for (final row in rows)
                      Expanded(
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 3),
                          child: Column(mainAxisAlignment: MainAxisAlignment.end, children: [
                            LText(
                              number(row['paid']) == 0 ? '—' : number(row['paid']).toStringAsFixed(0),
                              style: const TextStyle(color: brandTextSoft, fontSize: 8.5),
                            ),
                            const SizedBox(height: 4),
                            AnimatedContainer(
                              duration: const Duration(milliseconds: 180),
                              height: maxValue <= 0 ? 2 : math.max(2, 135 * number(row['paid']) / maxValue),
                              decoration: BoxDecoration(
                                color: revenuePlanKey == 'ALL' ? brandGold.withOpacity(.78) : brandNavy.withOpacity(.78),
                                borderRadius: const BorderRadius.vertical(top: Radius.circular(5)),
                              ),
                            ),
                            const SizedBox(height: 5),
                            LText(
                              '${row['period'] ?? ''}',
                              maxLines: 1,
                              overflow: TextOverflow.fade,
                              style: const TextStyle(color: brandTextSoft, fontSize: 8.2),
                            ),
                          ]),
                        ),
                      ),
                  ],
                );
              }),
            ),
        ]),
      ),
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
    final vatRate = TextEditingController(text: '${profile?['vat_rate_percent'] ?? 0}');
    final vatJurisdiction = TextEditingController(text: '${profile?['vat_jurisdiction'] ?? 'GB'}');
    final taxLabel = TextEditingController(text: '${profile?['tax_label'] ?? 'VAT'}');

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
              first: TextField(controller: legal, decoration: InputDecoration(labelText: uiLiteral('Legal name'))),
              second: TextField(controller: registration, decoration: InputDecoration(labelText: uiLiteral('Registration number'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: tax, decoration: InputDecoration(labelText: uiLiteral('Tax ID'))),
              second: TextField(controller: contactName, decoration: InputDecoration(labelText: uiLiteral('Billing contact'))),
            ),
            const SizedBox(height: 18),
            const _DialogSectionLabel('VAT & TAX POLICY'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: TextField(
                controller: vatRate,
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                decoration: InputDecoration(labelText: uiLiteral('VAT rate %')),
              ),
              second: TextField(controller: vatJurisdiction, decoration: InputDecoration(labelText: uiLiteral('VAT jurisdiction'))),
            ),
            const SizedBox(height: 12),
            TextField(controller: taxLabel, decoration: InputDecoration(labelText: uiLiteral('Tax label'))),
            const SizedBox(height: 6),
            const LText('Set VAT rate to 0 while HIMATE is outside the applicable VAT charging regime. Package prices remain net and checkout adds the configured tax rate.', style: TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.4)),
            const SizedBox(height: 18),
            TextField(controller: address, decoration: InputDecoration(labelText: uiLiteral('Company address'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Billing email'))),
              second: TextField(controller: phone, keyboardType: TextInputType.phone, decoration: InputDecoration(labelText: uiLiteral('Billing phone'))),
            ),
            const SizedBox(height: 18),
            const _DialogSectionLabel('BANKING DETAILS'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: TextField(controller: bank, decoration: InputDecoration(labelText: uiLiteral('Bank name'))),
              second: TextField(controller: bankAddress, decoration: InputDecoration(labelText: uiLiteral('Bank address'))),
            ),
            const SizedBox(height: 12),
            TextField(controller: account, decoration: InputDecoration(labelText: uiLiteral('Account number'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: iban, decoration: InputDecoration(labelText: uiLiteral('IBAN'))),
              second: TextField(controller: swift, decoration: InputDecoration(labelText: uiLiteral('SWIFT / BIC'))),
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
        'vat_rate_percent': double.tryParse(vatRate.text.trim().replaceAll(',', '.')) ?? 0,
        'vat_jurisdiction': vatJurisdiction.text.trim(),
        'tax_label': taxLabel.text.trim(),
      });
      await load();
      if (mounted) success('Billing profile updated.');
    }

    for (final c in [legal, registration, address, tax, contactName, email, phone, bank, bankAddress, account, iban, swift, vatRate, vatJurisdiction, taxLabel]) {
      c.dispose();
    }
  }

  @override
  Widget build(BuildContext context) {
    final onboarding = overview['onboarding'] is Map
        ? Map<String, dynamic>.from(overview['onboarding'] as Map)
        : <String, dynamic>{};
    final draftCount = workflowCount('draft');
    final sentCount = workflowCount('sent');
    final approvedCount = workflowCount('approved');
    final paidCount = workflowCount('paid');
    final pendingOnboarding = onboarding['pending'] is num
        ? (onboarding['pending'] as num).toInt()
        : int.tryParse('${onboarding['pending'] ?? 0}') ?? 0;
    final visibleInvoices = filteredInvoices;

    void scrollToOnboarding() {
      final target = onboardingKey.currentContext;
      if (target != null) {
        Scrollable.ensureVisible(target, duration: const Duration(milliseconds: 260), curve: Curves.easeOut);
      }
    }

    return Content(
      eyebrow: 'CENTRAL-6 · COMMERCIAL CONTROL',
      title: 'Licensing & Finance',
      subtitle: 'Partner onboarding, invoice approval, payment status and auditable finance controls. A partner reaches Portal access only after final HIMATE approval.',
      actions: [
        OutlinedButton.icon(
          onPressed: () => openBrowserDownload(financeExportPath),
          icon: const Icon(Icons.download_outlined),
          label: const LText('Export PDF'),
        ),
        OutlinedButton.icon(
          onPressed: editProfile,
          icon: const Icon(Icons.account_balance_outlined),
          label: const LText('Billing profile'),
        ),
        FilledButton.icon(
          onPressed: partners.isEmpty ? null : () => createManualInvoice(),
          icon: const Icon(Icons.add_card_outlined),
          label: const LText('New invoice'),
        ),
      ],
      child: error != null
          ? _MessageCard(icon: Icons.cloud_off_outlined, title: 'Finance workspace unavailable', message: error!)
          : Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (loading) const LinearProgressIndicator(minHeight: 2),
                ResponsiveKpiGrid(
                  children: [
                    Kpi(
                      label: 'Draft invoices',
                      value: '$draftCount',
                      note: 'Awaiting Central approval',
                      icon: Icons.edit_note_outlined,
                      accent: brandSteel,
                      onTap: () => setState(() => invoiceFilter = 'DRAFT'),
                    ),
                    Kpi(
                      label: 'Outstanding',
                      value: moneyAcrossCurrencies('outstanding'),
                      note: '${approvedCount + sentCount} approved / sent invoices',
                      icon: Icons.outbox_outlined,
                      accent: brandGold,
                      onTap: () => setState(() => invoiceFilter = sentCount > 0 ? 'SENT' : 'APPROVED'),
                    ),
                    Kpi(
                      label: 'Paid YTD',
                      value: moneyAcrossCurrencies('paid_ytd'),
                      note: '$paidCount paid invoices',
                      icon: Icons.payments_outlined,
                      accent: brandSuccess,
                      onTap: () => setState(() => invoiceFilter = 'PAID'),
                    ),
                    Kpi(
                      label: 'Pending onboarding',
                      value: '$pendingOnboarding',
                      note: 'Registration → review → approval',
                      icon: Icons.fact_check_outlined,
                      accent: brandNavy,
                      onTap: scrollToOnboarding,
                    ),
                  ],
                ),
                const SizedBox(height: 20),
                financeChart(),
                const SizedBox(height: 24),
                Row(
                  children: [
                    const Expanded(
                      child: _SectionHeader(
                        title: 'Invoice approval queue',
                        subtitle: 'Draft → Approved → Sent → Paid / Cancelled. Collection is blocked until the invoice is Sent.',
                      ),
                    ),
                    _MiniCounter(label: '${visibleInvoices.length} shown'),
                  ],
                ),
                const SizedBox(height: 10),
                Wrap(
                  spacing: 7,
                  runSpacing: 7,
                  children: [
                    for (final status in const ['ALL', 'DRAFT', 'APPROVED', 'SENT', 'PAID', 'CANCELLED'])
                      FilterChip(
                        selected: invoiceFilter == status,
                        label: LText(status == 'ALL' ? 'All' : _humanize(status)),
                        onSelected: (_) => setState(() => invoiceFilter = status),
                      ),
                  ],
                ),
                const SizedBox(height: 12),
                if (visibleInvoices.isEmpty)
                  const _MessageCard(
                    icon: Icons.receipt_long_outlined,
                    title: 'No invoices in this state',
                    message: 'Create a manual draft or wait for the recurring billing cycle to generate a new draft.',
                  )
                else
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        children: [
                          for (var i = 0; i < visibleInvoices.length; i++) ...[
                            Builder(builder: (context) {
                              final invoice = visibleInvoices[i];
                              final workflow = '${invoice['workflow_status'] ?? invoice['status'] ?? 'DRAFT'}'.toUpperCase();
                              final id = '${invoice['id'] ?? ''}';
                              final partnerID = '${invoice['partner_id'] ?? ''}';
                              final gross = number(invoice['gross_total'] ?? invoice['total']);
                              return Padding(
                                padding: const EdgeInsets.symmetric(vertical: 8),
                                child: LayoutBuilder(builder: (context, constraints) {
                                  final details = Column(
                                    crossAxisAlignment: CrossAxisAlignment.start,
                                    children: [
                                      Row(children: [
                                        const Icon(Icons.receipt_long_outlined, size: 18, color: brandGold),
                                        const SizedBox(width: 9),
                                        Expanded(
                                          child: LText(
                                            id,
                                            maxLines: 1,
                                            overflow: TextOverflow.ellipsis,
                                            style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 11.5),
                                          ),
                                        ),
                                      ]),
                                      const SizedBox(height: 5),
                                      LText(
                                        partnerName(partnerID),
                                        style: const TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w600),
                                      ),
                                      const SizedBox(height: 2),
                                      LText(
                                        '${invoice['service_period_start'] ?? ''} — ${invoice['service_period_end_exclusive'] ?? ''}',
                                        style: const TextStyle(color: brandTextSoft, fontSize: 9.2),
                                      ),
                                      const SizedBox(height: 2),
                                      LText(
                                        '${invoice['currency'] ?? 'USD'} ${gross.toStringAsFixed(2)} gross · net ${number(invoice['net_total']).toStringAsFixed(2)} · tax ${number(invoice['tax_amount']).toStringAsFixed(2)}',
                                        style: const TextStyle(color: brandTextSoft, fontSize: 9.2),
                                      ),
                                    ],
                                  );
                                  final actions = Wrap(
                                    spacing: 7,
                                    runSpacing: 7,
                                    alignment: WrapAlignment.end,
                                    children: [
                                      _StatusPill(label: workflow),
                                      if (workflow == 'DRAFT')
                                        FilledButton.tonalIcon(
                                          onPressed: () => invoiceAction(invoice, 'approve'),
                                          icon: const Icon(Icons.verified_outlined, size: 16),
                                          label: const LText('Approve'),
                                        ),
                                      if (workflow == 'APPROVED')
                                        FilledButton.icon(
                                          onPressed: () => invoiceAction(invoice, 'send'),
                                          icon: const Icon(Icons.send_outlined, size: 16),
                                          label: const LText('Send'),
                                        ),
                                      if (workflow == 'SENT')
                                        FilledButton.tonalIcon(
                                          onPressed: () => invoiceAction(invoice, 'mark-paid'),
                                          icon: const Icon(Icons.payments_outlined, size: 16),
                                          label: const LText('Mark paid'),
                                        ),
                                      if (workflow != 'DRAFT')
                                        OutlinedButton.icon(
                                          onPressed: () => openBrowserDownload('/api/v1/billing/invoices/$id/pdf'),
                                          icon: const Icon(Icons.picture_as_pdf_outlined, size: 16),
                                          label: const LText('PDF'),
                                        ),
                                      if (workflow != 'PAID' && workflow != 'CANCELLED')
                                        TextButton.icon(
                                          onPressed: () => invoiceAction(invoice, 'cancel'),
                                          icon: const Icon(Icons.cancel_outlined, size: 16),
                                          label: const LText('Cancel invoice'),
                                        ),
                                    ],
                                  );
                                  if (constraints.maxWidth < 820) {
                                    return Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [details, const SizedBox(height: 12), actions],
                                    );
                                  }
                                  return Row(
                                    crossAxisAlignment: CrossAxisAlignment.center,
                                    children: [
                                      Expanded(child: details),
                                      const SizedBox(width: 18),
                                      Flexible(child: actions),
                                    ],
                                  );
                                }),
                              );
                            }),
                            if (i < visibleInvoices.length - 1) const Divider(height: 1),
                          ],
                        ],
                      ),
                    ),
                  ),
                const SizedBox(height: 28),
                Container(
                  key: onboardingKey,
                  child: Row(
                    children: [
                      const Expanded(
                        child: _SectionHeader(
                          title: 'Partner onboarding',
                          subtitle: 'Registered → Pending Review → Classified → invoice/payment or documented support → Admin Approval → Active.',
                        ),
                      ),
                      _MiniCounter(label: '$pendingOnboarding pending'),
                    ],
                  ),
                ),
                const SizedBox(height: 12),
                if (onboardingRows.isEmpty)
                  const _MessageCard(
                    icon: Icons.check_circle_outline_rounded,
                    title: 'No pending onboarding',
                    message: 'All current partner registrations have completed the Central-6 commercial approval lifecycle.',
                  )
                else
                  Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      for (final row in onboardingRows)
                        SizedBox(
                          width: MediaQuery.sizeOf(context).width < 760 ? double.infinity : 360,
                          child: Card(
                            child: Padding(
                              padding: const EdgeInsets.all(16),
                              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                                Row(children: [
                                  Expanded(
                                    child: LText(
                                      '${row['display_name'] ?? row['partner_id']}',
                                      style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 13),
                                    ),
                                  ),
                                  _StatusPill(label: '${row['state'] ?? 'REGISTERED'}'),
                                ]),
                                const SizedBox(height: 10),
                                _DefinitionRow(label: 'Classification', value: _humanize('${row['classification'] ?? 'UNCLASSIFIED'}')),
                                _DefinitionRow(label: 'Portal access', value: row['portal_enabled'] == true ? 'Enabled' : 'Blocked until Active'),
                                _DefinitionRow(label: 'Partner ID', value: '${row['partner_id'] ?? ''}'),
                                const SizedBox(height: 12),
                                onboardingAction(row),
                              ]),
                            ),
                          ),
                        ),
                    ],
                  ),
                const SizedBox(height: 28),
                LayoutBuilder(
                  builder: (context, constraints) {
                    final issuer = _IssuerProfileCard(profile: profile ?? {}, onEdit: editProfile);
                    const rules = _BillingRulesCard();
                    if (constraints.maxWidth < 920) {
                      return Column(children: [issuer, const SizedBox(height: 14), rules]);
                    }
                    return Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Expanded(child: issuer),
                        const SizedBox(width: 14),
                        const Expanded(child: rules),
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
  List<Map<String, dynamic>> evidence = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> reports = <Map<String, dynamic>>[];
  int evidenceTotal = 0;
  int evidenceOffset = 0;
  static const int evidenceLimit = 12;
  String evidenceQuery = '';
  String evidenceTypeFilter = '';
  String evidenceStatusFilter = '';
  String evidencePeriodStart = '';
  String evidencePeriodEnd = '';
  bool loading = false;
  String? error;

  @override
  void initState() {
    super.initState();
    load();
  }

  String evidencePath() {
    final query = <String, String>{
      'evidence_limit': '$evidenceLimit',
      'evidence_offset': '$evidenceOffset',
    };
    if (evidenceQuery.trim().isNotEmpty) query['evidence_query'] = evidenceQuery.trim();
    if (evidenceTypeFilter.isNotEmpty) query['evidence_type'] = evidenceTypeFilter;
    if (evidenceStatusFilter.isNotEmpty) query['evidence_status'] = evidenceStatusFilter;
    if (evidencePeriodStart.trim().isNotEmpty) query['evidence_period_start'] = evidencePeriodStart.trim();
    if (evidencePeriodEnd.trim().isNotEmpty) query['evidence_period_end'] = evidencePeriodEnd.trim();
    return Uri(path: '/api/v1/central/impact', queryParameters: query).toString();
  }

  Future<void> load() async {
    if (mounted) {
      setState(() {
        loading = true;
        error = null;
      });
    }
    try {
      final model = await widget.api.get(
        evidencePath(),
        maxAge: const Duration(seconds: 5),
      );
      if (!mounted) return;
      setState(() {
        definitions = items(<String, dynamic>{'items': model['definitions']});
        summary = items(<String, dynamic>{'items': model['summary']});
        evidence = items(<String, dynamic>{'items': model['evidence']});
        reports = items(<String, dynamic>{'items': model['reports']});
        evidenceTotal = (model['evidence_total'] as num?)?.toInt() ?? evidence.length;
        loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        error = e.toString();
      });
    }
  }

  Future<void> addDefinition() async {
    final key = TextEditingController();
    final labelEN = TextEditingController();
    final labelHU = TextEditingController();
    final descriptionEN = TextEditingController();
    final descriptionHU = TextEditingController();
    final unit = TextEditingController(text: 'count');
    String aggregation = 'SUM';
    String scope = 'PARTNER';
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'New metric definition',
          subtitle: 'Dynamic business metrics store English and Hungarian labels and descriptions independently.',
          icon: Icons.add_chart_outlined,
          width: 760,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: key, decoration: InputDecoration(labelText: uiLiteral('Metric key'), hintText: uiLiteral('culture.events'))),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: labelEN, decoration: InputDecoration(labelText: uiLiteral('English display label *'))),
                second: TextField(controller: labelHU, decoration: InputDecoration(labelText: uiLiteral('Hungarian display label *'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: unit, decoration: InputDecoration(labelText: uiLiteral('Unit'))),
                second: DropdownButtonFormField<String>(
                  value: aggregation,
                  decoration: InputDecoration(labelText: uiLiteral('Aggregation')),
                  items: const [
                    DropdownMenuItem(value: 'SUM', child: LText('SUM')),
                    DropdownMenuItem(value: 'LATEST', child: LText('LATEST')),
                    DropdownMenuItem(value: 'AVERAGE', child: LText('AVERAGE')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => aggregation = v); },
                ),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: scope,
                decoration: InputDecoration(labelText: uiLiteral('Scope')),
                items: const [
                  DropdownMenuItem(value: 'PARTNER', child: LText('PARTNER')),
                  DropdownMenuItem(value: 'GLOBAL', child: LText('GLOBAL')),
                  DropdownMenuItem(value: 'BOTH', child: LText('BOTH')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => scope = v); },
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: descriptionEN, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('English description'))),
                second: TextField(controller: descriptionHU, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('Hungarian description'))),
              ),
            ],
          ),
          primaryLabel: 'Create metric',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok == true && labelEN.text.trim().isNotEmpty && labelHU.text.trim().isNotEmpty) {
      await widget.api.post('/api/v1/impact/definitions', {
        'metric_key': key.text.trim(),
        'label_en': labelEN.text.trim(),
        'label_hu': labelHU.text.trim(),
        'description_en': descriptionEN.text.trim(),
        'description_hu': descriptionHU.text.trim(),
        'unit': unit.text.trim(),
        'aggregation': aggregation,
        'scope': scope,
      });
      await load();
    }
    for (final controller in [key, labelEN, labelHU, descriptionEN, descriptionHU, unit]) { controller.dispose(); }
  }

  Future<void> addValue() async {
    if (definitions.isEmpty) return;
    String metricKey = '${definitions.first['metric_key']}';
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
          subtitle: 'Manual values remain distinct from connector and verified-document provenance.',
          icon: Icons.insights_outlined,
          width: 700,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: metricKey,
                decoration: InputDecoration(labelText: uiLiteral('Metric')),
                items: [
                  for (final d in definitions)
                    DropdownMenuItem(value: '${d['metric_key']}', child: LText('${d['label']} · ${d['metric_key']}')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => metricKey = v); },
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: partner, decoration: InputDecoration(labelText: uiLiteral('Partner ID'), hintText: uiLiteral('Leave empty for global metric'))),
                second: TextField(controller: numeric, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: InputDecoration(labelText: uiLiteral('Numeric value'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: start, decoration: InputDecoration(labelText: uiLiteral('Period start'), hintText: uiLiteral('YYYY-MM-DD'))),
                second: TextField(controller: end, decoration: InputDecoration(labelText: uiLiteral('Period end'), hintText: uiLiteral('YYYY-MM-DD'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: source, decoration: InputDecoration(labelText: uiLiteral('Source reference'))),
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
        'provenance': 'MANUAL',
        'source_ref': source.text.trim(),
      });
      await load();
    }
    for (final controller in [partner, start, end, numeric, source]) { controller.dispose(); }
  }

  Future<void> addBaseline() async {
    if (definitions.isEmpty) return;
    String metricKey = '${definitions.first['metric_key']}';
    final partner = TextEditingController();
    final start = TextEditingController(text: DateTime.now().toIso8601String().substring(0, 10));
    final end = TextEditingController(text: DateTime.now().toIso8601String().substring(0, 10));
    final numeric = TextEditingController();
    final source = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Set metric baseline',
          subtitle: 'Store an explicit baseline period and value for reproducible comparison.',
          icon: Icons.flag_outlined,
          width: 700,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: metricKey,
                decoration: InputDecoration(labelText: uiLiteral('Metric')),
                items: [
                  for (final d in definitions)
                    DropdownMenuItem(value: '${d['metric_key']}', child: LText('${d['label']} · ${d['metric_key']}')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => metricKey = v); },
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: partner, decoration: InputDecoration(labelText: uiLiteral('Partner ID'), hintText: uiLiteral('Leave empty for global baseline'))),
                second: TextField(controller: numeric, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: InputDecoration(labelText: uiLiteral('Baseline value'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: start, decoration: InputDecoration(labelText: uiLiteral('Baseline period start'), hintText: uiLiteral('YYYY-MM-DD'))),
                second: TextField(controller: end, decoration: InputDecoration(labelText: uiLiteral('Baseline period end'), hintText: uiLiteral('YYYY-MM-DD'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: source, decoration: InputDecoration(labelText: uiLiteral('Source reference'))),
            ],
          ),
          primaryLabel: 'Save baseline',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok == true) {
      await widget.api.put('/api/v1/impact/baselines', {
        'partner_id': partner.text.trim(),
        'metric_key': metricKey,
        'period_start': start.text.trim(),
        'period_end': end.text.trim(),
        'numeric_value': double.tryParse(numeric.text),
        'provenance': 'MANUAL',
        'source_ref': source.text.trim(),
      });
      await load();
    }
    for (final controller in [partner, start, end, numeric, source]) { controller.dispose(); }
  }

  Future<void> addEvidence() async {
    final partner = TextEditingController();
    final title = TextEditingController();
    final description = TextEditingController();
    final start = TextEditingController(text: DateTime.now().toIso8601String().substring(0, 10));
    final end = TextEditingController(text: DateTime.now().toIso8601String().substring(0, 10));
    final sourceUrl = TextEditingController();
    final declaration = TextEditingController();
    String evidenceType = 'PDF';
    String metricKey = '';
    html.File? selectedFile;

    final ok = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) {
          final fileBacked = evidenceType != 'URL' && evidenceType != 'PARTNER_DECLARATION';
          return BrandDialog(
            title: 'Upload Evidence',
            subtitle: 'Evidence is partner-scoped, checksum-backed and explicitly verified before it can support VERIFIED_DOCUMENT provenance.',
            icon: Icons.verified_outlined,
            width: 760,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ResponsiveFieldPair(
                  first: TextField(controller: partner, decoration: InputDecoration(labelText: uiLiteral('Partner ID *'))),
                  second: DropdownButtonFormField<String>(
                    value: evidenceType,
                    decoration: InputDecoration(labelText: uiLiteral('Evidence type')),
                    items: const [
                      DropdownMenuItem(value: 'PDF', child: LText('PDF')),
                      DropdownMenuItem(value: 'IMAGE', child: LText('Image')),
                      DropdownMenuItem(value: 'INVOICE', child: LText('Invoice')),
                      DropdownMenuItem(value: 'CONTRACT', child: LText('Contract')),
                      DropdownMenuItem(value: 'SCREENSHOT', child: LText('Screenshot')),
                      DropdownMenuItem(value: 'REPORT', child: LText('External report')),
                      DropdownMenuItem(value: 'URL', child: LText('URL reference')),
                      DropdownMenuItem(value: 'PARTNER_DECLARATION', child: LText('Partner declaration')),
                      DropdownMenuItem(value: 'OTHER', child: LText('Other')),
                    ],
                    onChanged: (v) { if (v != null) setLocal(() { evidenceType = v; selectedFile = null; }); },
                  ),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: metricKey,
                  decoration: InputDecoration(labelText: uiLiteral('Linked metric')),
                  items: [
                    const DropdownMenuItem(value: '', child: LText('No metric link')),
                    for (final d in definitions)
                      DropdownMenuItem(value: '${d['metric_key']}', child: LText('${d['label']} · ${d['metric_key']}')),
                  ],
                  onChanged: (v) { if (v != null) setLocal(() => metricKey = v); },
                ),
                const SizedBox(height: 12),
                TextField(controller: title, decoration: InputDecoration(labelText: uiLiteral('Evidence title *'))),
                const SizedBox(height: 12),
                TextField(controller: description, maxLines: 2, decoration: InputDecoration(labelText: uiLiteral('Description'))),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: TextField(controller: start, decoration: InputDecoration(labelText: uiLiteral('Period start'))),
                  second: TextField(controller: end, decoration: InputDecoration(labelText: uiLiteral('Period end'))),
                ),
                const SizedBox(height: 12),
                if (fileBacked)
                  Row(
                    children: [
                      Expanded(child: LText(selectedFile?.name ?? 'No file selected', style: const TextStyle(color: brandTextSoft))),
                      const SizedBox(width: 12),
                      OutlinedButton.icon(
                        onPressed: () async {
                          final file = await pickBrowserFile('application/pdf,image/png,image/jpeg,image/webp,text/plain');
                          if (file != null) setLocal(() => selectedFile = file);
                        },
                        icon: const Icon(Icons.upload_file_outlined),
                        label: const LText('Choose file'),
                      ),
                    ],
                  ),
                if (evidenceType == 'URL')
                  TextField(controller: sourceUrl, decoration: InputDecoration(labelText: uiLiteral('HTTP(S) source URL *'))),
                if (evidenceType == 'PARTNER_DECLARATION')
                  TextField(controller: declaration, maxLines: 4, decoration: InputDecoration(labelText: uiLiteral('Partner declaration *'))),
                const SizedBox(height: 12),
                const _RuleStrip(items: [
                  _RuleItem(Icons.security_outlined, 'Validation', 'Content-sniffed · max 20 MiB · SHA-256'),
                  _RuleItem(Icons.link_off_outlined, 'URL safety', 'URL evidence is referenced, never fetched'),
                ]),
              ],
            ),
            primaryLabel: 'Create evidence',
            onPrimary: () {
              final fileBackedNow = evidenceType != 'URL' && evidenceType != 'PARTNER_DECLARATION';
              if (partner.text.trim().isEmpty || title.text.trim().isEmpty ||
                  (fileBackedNow && selectedFile == null) ||
                  (evidenceType == 'URL' && sourceUrl.text.trim().isEmpty) ||
                  (evidenceType == 'PARTNER_DECLARATION' && declaration.text.trim().isEmpty)) {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: LText('Complete the required Evidence fields.'), behavior: SnackBarBehavior.floating),
                );
                return;
              }
              Navigator.pop(context, true);
            },
          );
        },
      ),
    );

    if (ok == true) {
      if (evidenceType == 'URL' || evidenceType == 'PARTNER_DECLARATION') {
        await widget.api.post('/api/v1/evidence', {
          'partner_id': partner.text.trim(),
          'metric_key': metricKey,
          'evidence_type': evidenceType,
          'title': title.text.trim(),
          'description': description.text.trim(),
          'period_start': start.text.trim(),
          'period_end': end.text.trim(),
          'source_url': sourceUrl.text.trim(),
          'declaration_text': declaration.text.trim(),
        });
      } else {
        final file = selectedFile!;
        final bytes = await readBrowserFile(file);
        await widget.api.multipart('/api/v1/evidence', {
          'partner_id': partner.text.trim(),
          'metric_key': metricKey,
          'evidence_type': evidenceType,
          'title': title.text.trim(),
          'description': description.text.trim(),
          'period_start': start.text.trim(),
          'period_end': end.text.trim(),
        }, bytes, file.name);
      }
      await load();
    }
    for (final controller in [partner, title, description, start, end, sourceUrl, declaration]) { controller.dispose(); }
  }

  Future<void> verifyEvidence(Map<String, dynamic> item) async {
    await widget.api.patch('/api/v1/evidence/${item['id']}', {'verification_status': 'VERIFIED'});
    await load();
  }

  Future<void> recordVerifiedValue(Map<String, dynamic> item) async {
    final metricKey = '${item['metric_key'] ?? ''}';
    if (metricKey.isEmpty || item['verification_status'] != 'VERIFIED') return;
    final numeric = TextEditingController();
    final start = TextEditingController(text: '${item['period_start'] ?? DateTime.now().toIso8601String().substring(0, 10)}');
    final end = TextEditingController(text: '${item['period_end'] ?? DateTime.now().toIso8601String().substring(0, 10)}');
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'Record verified metric',
        subtitle: 'This observation will use VERIFIED_DOCUMENT provenance and is cryptographically tied to the selected Evidence record.',
        icon: Icons.fact_check_outlined,
        width: 620,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(enabled: false, decoration: InputDecoration(labelText: uiLiteral('Evidence'), hintText: '${item['id']} · ${item['title']}')),
            const SizedBox(height: 12),
            TextField(enabled: false, decoration: InputDecoration(labelText: uiLiteral('Metric'), hintText: metricKey)),
            const SizedBox(height: 12),
            TextField(controller: numeric, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: InputDecoration(labelText: uiLiteral('Verified numeric value'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: start, decoration: InputDecoration(labelText: uiLiteral('Period start'))),
              second: TextField(controller: end, decoration: InputDecoration(labelText: uiLiteral('Period end'))),
            ),
          ],
        ),
        primaryLabel: 'Record verified value',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );
    if (ok == true) {
      await widget.api.post('/api/v1/impact/values', {
        'partner_id': '${item['partner_id']}',
        'metric_key': metricKey,
        'period_start': start.text.trim(),
        'period_end': end.text.trim(),
        'numeric_value': double.tryParse(numeric.text),
        'provenance': 'VERIFIED_DOCUMENT',
        'evidence_id': '${item['id']}',
      });
      await load();
    }
    numeric.dispose(); start.dispose(); end.dispose();
  }

  Future<void> generateReport() async {
    String reportType = 'PARTNER_IMPACT';
    final title = TextEditingController();
    final partnerIds = TextEditingController();
    final now = DateTime.now();
    final start = TextEditingController(text: DateTime(now.year, now.month - 1, now.day).toIso8601String().substring(0, 10));
    final end = TextEditingController(text: now.toIso8601String().substring(0, 10));
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Generate PDF report',
          subtitle: 'The report freezes metrics, sources and Evidence references into a reproducible snapshot before PDF rendering.',
          icon: Icons.picture_as_pdf_outlined,
          width: 720,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: reportType,
                decoration: InputDecoration(labelText: uiLiteral('Report type')),
                items: const [
                  DropdownMenuItem(value: 'PARTNER_IMPACT', child: LText('Partner Impact Report')),
                  DropdownMenuItem(value: 'MULTI_PARTNER', child: LText('Multi-Partner Report')),
                  DropdownMenuItem(value: 'HIMATE_GLOBAL', child: LText('HIMATE Global Impact Report')),
                ],
                onChanged: (v) { if (v != null) setLocal(() => reportType = v); },
              ),
              const SizedBox(height: 12),
              TextField(controller: title, decoration: InputDecoration(labelText: uiLiteral('Report title'), hintText: uiLiteral('Optional · default title follows report type'))),
              if (reportType != 'HIMATE_GLOBAL') ...[
                const SizedBox(height: 12),
                TextField(
                  controller: partnerIds,
                  decoration: InputDecoration(
                    labelText: reportType == 'PARTNER_IMPACT' ? 'Partner ID *' : 'Partner IDs *',
                    hintText: reportType == 'MULTI_PARTNER' ? 'ptr_000001, ptr_000002' : 'ptr_000001',
                  ),
                ),
              ],
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: start, decoration: InputDecoration(labelText: uiLiteral('Period start'))),
                second: TextField(controller: end, decoration: InputDecoration(labelText: uiLiteral('Period end'))),
              ),
              const SizedBox(height: 12),
              const _RuleStrip(items: [
                _RuleItem(Icons.inventory_2_outlined, 'Snapshot', 'Frozen before PDF generation'),
                _RuleItem(Icons.replay_outlined, 'Reproducible', 'Regeneration never rereads live metrics'),
              ]),
            ],
          ),
          primaryLabel: 'Queue report',
          onPrimary: () => Navigator.pop(context, true),
        ),
      ),
    );
    if (ok == true) {
      final ids = partnerIds.text.split(',').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
      final created = await widget.api.post('/api/v1/reports', {
        'report_type': reportType,
        'title': title.text.trim(),
        'partner_ids': reportType == 'HIMATE_GLOBAL' ? <String>[] : ids,
        'period_start': start.text.trim(),
        'period_end': end.text.trim(),
      });
      await _waitReport('${created['id']}');
      await load();
    }
    title.dispose(); partnerIds.dispose(); start.dispose(); end.dispose();
  }

  Future<void> _waitReport(String id) async {
    for (var i = 0; i < 15; i++) {
      await Future<void>.delayed(const Duration(seconds: 1));
      final item = await widget.api.get('/api/v1/reports/$id', force: true);
      final status = '${item['status']}';
      if (status == 'READY' || status == 'FAILED') return;
    }
  }

  Future<void> regenerateReport(Map<String, dynamic> item) async {
    await widget.api.post('/api/v1/reports/${item['id']}/regenerate');
    await load();
  }

  Future<void> checkEvidenceIntegrity(Map<String, dynamic> item) async {
    try {
      final result = await widget.api.get('/api/v1/evidence/${item['id']}/integrity', force: true);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: LText('Evidence integrity: ${result['status']} · ${shortHash(result['sha256'])}'), behavior: SnackBarBehavior.floating),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: LText('Evidence integrity check failed: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
        );
      }
    }
  }

  Future<void> applyEvidenceFilters() async {
    evidenceOffset = 0;
    await load();
  }

  String shortHash(dynamic value) {
    final raw = '${value ?? ''}';
    return raw.length > 14 ? '${raw.substring(0, 14)}…' : raw;
  }

  @override
  Widget build(BuildContext context) {
    if (error != null) {
      return Content(
        eyebrow: 'IMPACT CONTROL',
        title: 'Impact & Reports',
        subtitle: 'Metrics, Evidence and reproducible reports.',
        child: _MessageCard(icon: Icons.error_outline_rounded, title: 'Impact data unavailable', message: error!),
      );
    }
    return Content(
      eyebrow: 'IMPACT CONTROL',
      title: 'Impact & Reports',
      subtitle: 'Global and partner metrics, auditable Evidence and reproducible PDF reporting.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              final actions = <Widget>[
                OutlinedButton.icon(
                  onPressed: () => openBrowserDownload('/api/v1/impact/export.pdf'),
                  icon: const Icon(Icons.download_outlined),
                  label: const LText('Export PDF'),
                ),
                OutlinedButton.icon(onPressed: addDefinition, icon: const Icon(Icons.add_chart_outlined), label: const LText('New metric')),
                OutlinedButton.icon(onPressed: definitions.isEmpty ? null : addBaseline, icon: const Icon(Icons.flag_outlined), label: const LText('Set baseline')),
                OutlinedButton.icon(onPressed: addEvidence, icon: const Icon(Icons.verified_outlined), label: const LText('Upload Evidence')),
                OutlinedButton.icon(onPressed: generateReport, icon: const Icon(Icons.picture_as_pdf_outlined), label: const LText('Generate Report')),
                FilledButton.icon(onPressed: definitions.isEmpty ? null : addValue, icon: const Icon(Icons.add_rounded), label: const LText('Record value')),
              ];
              return Wrap(
                alignment: WrapAlignment.end,
                spacing: 10,
                runSpacing: 8,
                children: actions,
              );
            },
          ),
          const SizedBox(height: 18),
          _SectionHeader(title: 'Impact Summary', subtitle: 'Aggregated values follow each metric definition’s SUM, LATEST or AVERAGE rule.', trailing: _MiniCounter(label: '${summary.length} metrics')),
          const SizedBox(height: 12),
          if (summary.isEmpty)
            const _MessageCard(
              icon: Icons.insights_outlined,
              title: 'No impact data',
              message: 'No impact observations have been recorded yet.',
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 620 ? constraints.maxWidth : constraints.maxWidth < 1000 ? (constraints.maxWidth - 12) / 2 : (constraints.maxWidth - 24) / 3;
                return Wrap(
                  spacing: 12, runSpacing: 12,
                  children: [
                    for (final m in summary)
                      SizedBox(
                      width: width,
                      child: _InfoCard(
                        title: '${m['label'] ?? m['metric_key']}',
                        icon: Icons.insights_outlined,
                        children: [
                          _DefinitionRow(label: 'Value', value: '${m['numeric_value'] ?? '—'} ${m['unit'] ?? ''}'),
                          _DefinitionRow(label: 'Baseline', value: '${m['baseline_numeric_value'] ?? '—'} ${m['unit'] ?? ''}'),
                          _DefinitionRow(label: 'Delta', value: '${m['delta_from_baseline'] ?? '—'} ${m['unit'] ?? ''}'),
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
          _SectionHeader(title: 'Evidence Library', subtitle: 'Partner-scoped proof with metric/period linkage, verification state and SHA-256 integrity.', trailing: _MiniCounter(label: '$evidenceTotal records')),
          const SizedBox(height: 12),
          LayoutBuilder(
            builder: (context, constraints) {
              final fieldWidth = constraints.maxWidth < 700 ? constraints.maxWidth : 210.0;
              return Wrap(
                spacing: 10,
                runSpacing: 10,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  SizedBox(
                    width: constraints.maxWidth < 700 ? constraints.maxWidth : 280,
                    child: TextField(
                      onChanged: (value) => evidenceQuery = value,
                      onSubmitted: (_) => applyEvidenceFilters(),
                      decoration: InputDecoration(labelText: uiLiteral('Search Evidence'), hintText: uiLiteral('Title, file, partner, metric, URL…'), prefixIcon: Icon(Icons.search_rounded)),
                    ),
                  ),
                  SizedBox(
                    width: fieldWidth,
                    child: DropdownButtonFormField<String>(
                      value: evidenceTypeFilter,
                      decoration: InputDecoration(labelText: uiLiteral('Type')),
                      items: const [
                        DropdownMenuItem(value: '', child: LText('All types')),
                        DropdownMenuItem(value: 'PDF', child: LText('PDF')),
                        DropdownMenuItem(value: 'IMAGE', child: LText('Image')),
                        DropdownMenuItem(value: 'INVOICE', child: LText('Invoice')),
                        DropdownMenuItem(value: 'CONTRACT', child: LText('Contract')),
                        DropdownMenuItem(value: 'SCREENSHOT', child: LText('Screenshot')),
                        DropdownMenuItem(value: 'REPORT', child: LText('Report')),
                        DropdownMenuItem(value: 'URL', child: LText('URL')),
                        DropdownMenuItem(value: 'PARTNER_DECLARATION', child: LText('Partner declaration')),
                        DropdownMenuItem(value: 'OTHER', child: LText('Other')),
                      ],
                      onChanged: (v) { if (v != null) setState(() => evidenceTypeFilter = v); },
                    ),
                  ),
                  SizedBox(
                    width: fieldWidth,
                    child: DropdownButtonFormField<String>(
                      value: evidenceStatusFilter,
                      decoration: InputDecoration(labelText: uiLiteral('Verification')),
                      items: const [
                        DropdownMenuItem(value: '', child: LText('All statuses')),
                        DropdownMenuItem(value: 'UNVERIFIED', child: LText('Unverified')),
                        DropdownMenuItem(value: 'VERIFIED', child: LText('Verified')),
                        DropdownMenuItem(value: 'REJECTED', child: LText('Rejected')),
                      ],
                      onChanged: (v) { if (v != null) setState(() => evidenceStatusFilter = v); },
                    ),
                  ),
                  SizedBox(width: fieldWidth, child: TextField(onChanged: (v) => evidencePeriodStart = v, decoration: InputDecoration(labelText: uiLiteral('Period from'), hintText: uiLiteral('YYYY-MM-DD')))),
                  SizedBox(width: fieldWidth, child: TextField(onChanged: (v) => evidencePeriodEnd = v, decoration: InputDecoration(labelText: uiLiteral('Period to'), hintText: uiLiteral('YYYY-MM-DD')))),
                  FilledButton.icon(onPressed: applyEvidenceFilters, icon: const Icon(Icons.filter_alt_outlined), label: const LText('Apply')),
                ],
              );
            },
          ),
          const SizedBox(height: 12),
          if (evidence.isEmpty)
            const _MessageCard(icon: Icons.verified_outlined, title: 'No Evidence yet', message: 'Upload a PDF, image, invoice, contract, screenshot, URL or partner declaration.')
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 700 ? constraints.maxWidth : constraints.maxWidth < 1100 ? (constraints.maxWidth - 12) / 2 : (constraints.maxWidth - 24) / 3;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final item in evidence)
                      SizedBox(
                        width: width,
                        child: Card(
                          child: Padding(
                            padding: const EdgeInsets.all(18),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(children: [
                                  const Icon(Icons.verified_outlined, color: brandGold),
                                  const SizedBox(width: 10),
                                  Expanded(child: LText('${item['title']}', style: const TextStyle(fontWeight: FontWeight.w700, color: brandNavy))),
                                ]),
                                const SizedBox(height: 14),
                                _DefinitionRow(label: 'Partner', value: '${item['partner_id']}'),
                                _DefinitionRow(label: 'Type', value: '${item['evidence_type']}'),
                                _DefinitionRow(label: 'Metric', value: '${item['metric_key'] == '' ? '—' : item['metric_key']}'),
                                _DefinitionRow(label: 'Period', value: '${item['period_start'] ?? '—'} → ${item['period_end'] ?? '—'}'),
                                _DefinitionRow(label: 'Verification', value: '${item['verification_status']}'),
                                _DefinitionRow(label: 'Uploaded by', value: '${item['uploaded_by'] == '' ? '—' : item['uploaded_by']}'),
                                _DefinitionRow(label: 'Uploaded', value: '${item['created_at'] ?? '—'}'),
                                _DefinitionRow(label: 'Reports', value: (item['report_ids'] is List && (item['report_ids'] as List).isNotEmpty) ? (item['report_ids'] as List).join(', ') : '—'),
                                _DefinitionRow(label: 'SHA-256', value: shortHash(item['sha256'])),
                                const SizedBox(height: 12),
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: [
                                    if (item['has_file'] == true)
                                      OutlinedButton.icon(
                                        onPressed: () => openBrowserDownload('/api/v1/evidence/${item['id']}/preview'),
                                        icon: const Icon(Icons.visibility_outlined),
                                        label: const LText('Preview'),
                                      ),
                                    if (item['has_file'] == true)
                                      OutlinedButton.icon(
                                        onPressed: () => openBrowserDownload('/api/v1/evidence/${item['id']}/download'),
                                        icon: const Icon(Icons.download_outlined),
                                        label: const LText('Download'),
                                      ),
                                    if (item['has_file'] == true)
                                      OutlinedButton.icon(
                                        onPressed: () => checkEvidenceIntegrity(item),
                                        icon: const Icon(Icons.security_outlined),
                                        label: const LText('Integrity'),
                                      ),
                                    if ('${item['source_url'] ?? ''}'.isNotEmpty)
                                      OutlinedButton.icon(
                                        onPressed: () => html.window.open('${item['source_url']}', '_blank'),
                                        icon: const Icon(Icons.open_in_new_rounded),
                                        label: const LText('Open URL'),
                                      ),
                                    if (item['verification_status'] != 'VERIFIED')
                                      FilledButton.icon(
                                        onPressed: () => verifyEvidence(item),
                                        icon: const Icon(Icons.fact_check_outlined),
                                        label: const LText('Verify'),
                                      ),
                                    if (item['verification_status'] == 'VERIFIED' && '${item['metric_key'] ?? ''}'.isNotEmpty)
                                      FilledButton.icon(
                                        onPressed: () => recordVerifiedValue(item),
                                        icon: const Icon(Icons.add_chart_rounded),
                                        label: const LText('Verified metric'),
                                      ),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                  ],
                );
              },
            ),
          if (evidenceTotal > 0) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: LText(
                    'Showing ${evidenceOffset + 1}–${(evidenceOffset + evidence.length) > evidenceTotal ? evidenceTotal : evidenceOffset + evidence.length} of $evidenceTotal',
                    style: const TextStyle(color: brandTextSoft, fontSize: 11.5, fontWeight: FontWeight.w600),
                  ),
                ),
                OutlinedButton(
                  onPressed: evidenceOffset > 0 ? () async { evidenceOffset = (evidenceOffset - evidenceLimit).clamp(0, evidenceTotal).toInt(); await load(); } : null,
                  child: const LText('Previous'),
                ),
                const SizedBox(width: 8),
                OutlinedButton(
                  onPressed: evidenceOffset + evidence.length < evidenceTotal ? () async { evidenceOffset += evidenceLimit; await load(); } : null,
                  child: const LText('Next'),
                ),
              ],
            ),
          ],
          const SizedBox(height: 24),
          _SectionHeader(title: 'Reports', subtitle: 'Partner, multi-partner and HIMATE Global PDFs generated from frozen, auditable snapshots.', trailing: _MiniCounter(label: '${reports.length} reports')),
          const SizedBox(height: 12),
          if (reports.isEmpty)
            const _MessageCard(icon: Icons.picture_as_pdf_outlined, title: 'No reports yet', message: 'Generate a report to freeze impact metrics, data sources and Evidence references into a reproducible snapshot.')
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 700 ? constraints.maxWidth : constraints.maxWidth < 1100 ? (constraints.maxWidth - 12) / 2 : (constraints.maxWidth - 24) / 3;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final item in reports)
                      SizedBox(
                        width: width,
                        child: Card(
                          child: Padding(
                            padding: const EdgeInsets.all(18),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(children: [
                                  const Icon(Icons.picture_as_pdf_outlined, color: brandGold),
                                  const SizedBox(width: 10),
                                  Expanded(child: LText('${item['title']}', style: const TextStyle(fontWeight: FontWeight.w700, color: brandNavy))),
                                ]),
                                const SizedBox(height: 14),
                                _DefinitionRow(label: 'Report ID', value: '${item['id']}'),
                                _DefinitionRow(label: 'Type', value: '${item['report_type']}'),
                                _DefinitionRow(label: 'Period', value: '${item['period_start']} → ${item['period_end']}'),
                                _DefinitionRow(label: 'Status', value: '${item['status']}'),
                                _DefinitionRow(label: 'Template', value: '${item['template_version'] ?? '—'}'),
                                _DefinitionRow(label: 'Snapshot', value: shortHash(item['snapshot_sha256'])),
                                _DefinitionRow(label: 'Evidence', value: '${(item['evidence_ids'] is List) ? (item['evidence_ids'] as List).length : 0} linked'),
                                _DefinitionRow(label: 'PDF SHA-256', value: shortHash(item['pdf_sha256'])),
                                if ('${item['last_error'] ?? ''}'.isNotEmpty)
                                  _DefinitionRow(label: 'Error', value: '${item['last_error']}'),
                                const SizedBox(height: 12),
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 8,
                                  children: [
                                    if (item['download_ready'] == true)
                                      FilledButton.icon(
                                        onPressed: () => openBrowserDownload('/api/v1/reports/${item['id']}/download'),
                                        icon: const Icon(Icons.download_outlined),
                                        label: const LText('Download PDF'),
                                      ),
                                    if (item['download_ready'] == true)
                                      OutlinedButton.icon(
                                        onPressed: () => regenerateReport(item),
                                        icon: const Icon(Icons.replay_outlined),
                                        label: const LText('Regenerate snapshot'),
                                      ),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                  ],
                );
              },
            ),
          const SizedBox(height: 24),
          _SectionHeader(title: 'Metric Definitions', subtitle: 'Stable definitions reused by manual entry, connectors and verified-document workflows.', trailing: _MiniCounter(label: '${definitions.length} definitions')),
          const SizedBox(height: 12),
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 620 ? constraints.maxWidth : constraints.maxWidth < 1000 ? (constraints.maxWidth - 12) / 2 : (constraints.maxWidth - 24) / 3;
              return Wrap(
                spacing: 12, runSpacing: 12,
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
      api.get('/api/v1/system-health/snapshot'),
      api.get('/api/v1/provisioning/jobs'),
      api.get('/api/v1/environments'),
      api.get('/api/v1/backups/summary'),
    ]);
    return r;
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<Map<String, dynamic>>>(
      future: _load(),
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done && snapshot.data == null) {
          return const Content(
            eyebrow: 'PLATFORM OPERATIONS',
            title: 'System & Operations',
            subtitle: 'Independent services behind one authenticated public gateway.',
            child: _MessageCard(
              icon: Icons.storage_outlined,
              title: 'No cached operations data yet',
              message: 'The workspace is ready. The latest background health snapshot will appear automatically when available.',
            ),
          );
        }
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
        final backupResponse = snapshot.data![3];
        final backupSummary = items(backupResponse);
        final backupProvider = '${backupResponse['provider'] ?? 'unknown'}';
        final services = items({'items': health['services']});
        final partners = items({'items': health['partners']});
        final backupPartnerIds = <String>{
          for (final p in partners)
            if ('${p['partner_id'] ?? ''}'.trim().isNotEmpty &&
                '${p['database_health'] ?? ''}' == 'OK' &&
                ('${p['storage_health'] ?? ''}' == 'READY' || '${p['storage_health'] ?? ''}' == 'OK'))
              '${p['partner_id']}',
          for (final e in environments)
            if ('${e['partner_id'] ?? ''}'.trim().isNotEmpty)
              '${e['partner_id']}',
          for (final b in backupSummary)
            if ('${b['partner_id'] ?? ''}'.trim().isNotEmpty)
              '${b['partner_id']}',
        }.toList()..sort();
        final overall = '${health['status'] ?? 'UNKNOWN'}';

        return Content(
          eyebrow: 'PLATFORM OPERATIONS',
          title: 'System & Operations',
          subtitle: 'Provisioning, partner environments, connectors, backups and central health across the containerized HIMATE control plane.',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _OperationsHero(status: overall, environment: 'control plane', version: 'START-09–22'),
              const SizedBox(height: 22),
              _SectionHeader(
                title: 'Service Health',
                subtitle: 'Readiness and liveness are monitored independently for each microservice.',
                trailing: _MiniCounter(label: '${services.length} services'),
              ),
              const SizedBox(height: 12),
              if (services.isEmpty)
                const _MessageCard(
                  icon: Icons.dns_outlined,
                  title: 'No service health data',
                  message: 'No service-health snapshot is available yet.',
                )
              else
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
                              _DefinitionRow(label: 'Partner DB', value: '${p['database_health'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Storage', value: '${p['storage_health'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Hostname / runtime', value: '${p['hostname_status'] ?? 'UNKNOWN'}'),
                              _DefinitionRow(label: 'Data sync', value: '${p['sync_status'] ?? 'NEVER'}'),
                              _DefinitionRow(label: 'Last sync', value: '${p['last_sync_at'] ?? '—'}'),
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
              Start22ConnectorPanel(api: api),
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
              DomainsDeploymentsPanel(
                api: api,
                initialEnvironments: environments,
              ),
              const SizedBox(height: 24),
              BackupsPanel(
                api: api,
                initialSummary: backupSummary,
                partnerIds: backupPartnerIds,
                initialProvider: backupProvider,
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



class AdministrationPage extends StatefulWidget {
  const AdministrationPage({required this.api, required this.user, super.key});
  final Api api;
  final Map<String, dynamic> user;

  @override
  State<AdministrationPage> createState() => _AdministrationPageState();
}

class _AdministrationPageState extends State<AdministrationPage> {
  final searchController = TextEditingController();
  Timer? _searchTimer;
  List<Map<String, dynamic>> events = <Map<String, dynamic>>[];
  bool loading = false;
  String? error;
  String resource = 'ALL';
  String method = 'ALL';
  String outcome = 'ALL';
  int total = 0;
  int offset = 0;
  final int pageSize = 50;
  int _generation = 0;

  static const resources = <String>[
    'ALL',
    'backups',
    'partners',
    'billing',
    'catalog',
    'provisioning',
    'environments',
    'connectors',
    'impact',
    'evidence',
    'reports',
    'cms',
  ];

  @override
  void initState() {
    super.initState();
    load(reset: true);
  }

  @override
  void dispose() {
    _searchTimer?.cancel();
    searchController.dispose();
    super.dispose();
  }

  Uri _uri() {
    final params = <String, String>{
      'limit': '$pageSize',
      'offset': '$offset',
    };
    final q = searchController.text.trim();
    if (q.isNotEmpty) params['q'] = q;
    if (resource != 'ALL') params['resource'] = resource;
    if (method != 'ALL') params['method'] = method;
    if (outcome != 'ALL') params['outcome'] = outcome;
    return Uri(path: '/api/v1/audit/events', queryParameters: params);
  }

  Future<void> load({bool reset = false}) async {
    if (reset) offset = 0;
    final generation = ++_generation;
    if (mounted) setState(() => error = null);
    try {
      final response = await widget.api.get(_uri().toString());
      if (!mounted || generation != _generation) return;
      setState(() {
        events = items(response);
        total = (response['total'] as num?)?.toInt() ?? events.length;
        loading = false;
      });
    } catch (e) {
      if (!mounted || generation != _generation) return;
      setState(() {
        error = e.toString();
        loading = false;
      });
    }
  }

  void searchChanged(String _) {
    _searchTimer?.cancel();
    _searchTimer = Timer(const Duration(milliseconds: 280), () => load(reset: true));
  }

  void setFilter(VoidCallback update) {
    setState(update);
    load(reset: true);
  }

  String _timestamp(dynamic value) {
    final raw = value?.toString() ?? '';
    final parsed = DateTime.tryParse(raw)?.toLocal();
    if (parsed == null) return raw.isEmpty ? '—' : raw;
    String two(int v) => v.toString().padLeft(2, '0');
    return '${parsed.year}-${two(parsed.month)}-${two(parsed.day)} ${two(parsed.hour)}:${two(parsed.minute)}:${two(parsed.second)}';
  }

  Color _outcomeColor(String value) {
    switch (value.toUpperCase()) {
      case 'SUCCESS':
        return brandSuccess;
      case 'FAILED':
        return brandDanger;
      default:
        return brandSteel;
    }
  }

  Widget _auditEventCard(Map<String, dynamic> event) {
    final eventOutcome = '${event['outcome'] ?? 'UNKNOWN'}';
    final tone = _outcomeColor(eventOutcome);
    final actorName = '${event['actor_name'] ?? ''}'.trim();
    final actorId = '${event['actor_id'] ?? ''}'.trim();
    final partnerId = '${event['partner_id'] ?? ''}'.trim();
    final requestId = '${event['request_id'] ?? ''}'.trim();
    final path = '${event['path'] ?? ''}'.trim();
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 38,
                  height: 38,
                  decoration: BoxDecoration(color: tone.withOpacity(.09), borderRadius: BorderRadius.circular(10)),
                  child: Icon(eventOutcome == 'SUCCESS' ? Icons.check_circle_outline_rounded : Icons.error_outline_rounded, color: tone, size: 20),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: 7,
                        runSpacing: 6,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          _StatusPill(label: '${event['method'] ?? 'UNKNOWN'}'),
                          _StatusPill(label: eventOutcome),
                          LText(
                            _humanize('${event['resource'] ?? 'api'}'),
                            style: const TextStyle(color: brandNavy, fontSize: 13, fontWeight: FontWeight.w700),
                          ),
                        ],
                      ),
                      const SizedBox(height: 5),
                      SelectableText(
                        path.isEmpty ? '—' : path,
                        style: const TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.35),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 10),
                LText(_timestamp(event['created_at']), style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
              ],
            ),
            const SizedBox(height: 14),
            const Divider(height: 1),
            const SizedBox(height: 7),
            _DefinitionRow(label: 'Actor', value: actorName.isNotEmpty ? '$actorName · $actorId' : (actorId.isEmpty ? '—' : actorId)),
            if (partnerId.isNotEmpty) _DefinitionRow(label: 'Partner', value: partnerId),
            _DefinitionRow(label: 'HTTP status', value: '${event['status'] ?? '—'}'),
            _DefinitionRow(label: 'Duration', value: '${event['duration_ms'] ?? 0} ms'),
            if (requestId.isNotEmpty) _DefinitionRow(label: 'Request ID', value: requestId),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final currentPage = offset ~/ pageSize + 1;
    final pageCount = total == 0 ? 1 : (total + pageSize - 1) ~/ pageSize;
    final roles = widget.user['roles'] is List
        ? (widget.user['roles'] as List).map((e) => e.toString()).join(', ')
        : '${widget.user['roles'] ?? 'platform_admin'}';

    return Content(
      eyebrow: 'ADMINISTRATION',
      title: 'Administration',
      subtitle: 'Central audit history, administrator lifecycle, roles and backend-enforced permissions across the HIMATE control plane.',
      actions: [
        OutlinedButton.icon(
          onPressed: loading ? null : () => load(),
          icon: const Icon(Icons.refresh_rounded),
          label: const LText('Refresh'),
        ),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _RuleStrip(items: [
            const _RuleItem(Icons.lock_clock_outlined, 'Capture', 'Mutating admin API calls'),
            const _RuleItem(Icons.speed_outlined, 'Write path', 'Asynchronous queue'),
            _RuleItem(Icons.person_outline_rounded, 'Current actor', '${widget.user['name'] ?? 'Administrator'}'),
            _RuleItem(Icons.admin_panel_settings_outlined, 'Role', roles),
          ]),
          const SizedBox(height: 22),
          _SectionHeader(
            title: 'Administrative Event Stream',
            subtitle: 'Search by actor, path, request ID, resource or partner. Filters are server-side and pagination keeps the audit page fast as history grows.',
            trailing: _MiniCounter(label: '$total matched'),
          ),
          const SizedBox(height: 12),
          _FilterSurface(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final search = TextField(
                  controller: searchController,
                  onChanged: searchChanged,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Search audit history'),
                    hintText: uiLiteral('Actor, request ID, resource, path or partner'),
                    prefixIcon: Icon(Icons.search_rounded),
                  ),
                );
                final resourceFilter = DropdownButtonFormField<String>(
                  value: resource,
                  decoration: InputDecoration(labelText: uiLiteral('Resource')),
                  items: [
                    for (final value in resources)
                      DropdownMenuItem(value: value, child: LText(value == 'ALL' ? 'All resources' : _humanize(value))),
                  ],
                  onChanged: (value) {
                    if (value != null) setFilter(() => resource = value);
                  },
                );
                final methodFilter = DropdownButtonFormField<String>(
                  value: method,
                  decoration: InputDecoration(labelText: uiLiteral('Method')),
                  items: const [
                    DropdownMenuItem(value: 'ALL', child: LText('All methods')),
                    DropdownMenuItem(value: 'POST', child: LText('POST')),
                    DropdownMenuItem(value: 'PATCH', child: LText('PATCH')),
                    DropdownMenuItem(value: 'PUT', child: LText('PUT')),
                    DropdownMenuItem(value: 'DELETE', child: LText('DELETE')),
                  ],
                  onChanged: (value) {
                    if (value != null) setFilter(() => method = value);
                  },
                );
                final outcomeFilter = DropdownButtonFormField<String>(
                  value: outcome,
                  decoration: InputDecoration(labelText: uiLiteral('Outcome')),
                  items: const [
                    DropdownMenuItem(value: 'ALL', child: LText('All outcomes')),
                    DropdownMenuItem(value: 'SUCCESS', child: LText('Success')),
                    DropdownMenuItem(value: 'FAILED', child: LText('Failed')),
                  ],
                  onChanged: (value) {
                    if (value != null) setFilter(() => outcome = value);
                  },
                );

                if (constraints.maxWidth < 720) {
                  return Column(
                    children: [
                      search,
                      const SizedBox(height: 10),
                      resourceFilter,
                      const SizedBox(height: 10),
                      ResponsiveFieldPair(first: methodFilter, second: outcomeFilter),
                    ],
                  );
                }
                return Column(
                  children: [
                    Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: resourceFilter)]),
                    const SizedBox(height: 10),
                    Row(children: [Expanded(child: methodFilter), const SizedBox(width: 10), Expanded(child: outcomeFilter)]),
                  ],
                );
              },
            ),
          ),
          const SizedBox(height: 16),
          if (error != null)
            _MessageCard(
              icon: Icons.error_outline_rounded,
              title: 'Audit history could not be loaded',
              message: error!,
            )
          else if (events.isEmpty)
            const _MessageCard(
              icon: Icons.history_toggle_off_rounded,
              title: 'No matching audit events',
              message: 'Mutating administration actions will appear here automatically. Adjust the filters if you are looking for earlier activity.',
            )
          else ...[
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 780
                    ? constraints.maxWidth
                    : constraints.maxWidth < 1240
                        ? (constraints.maxWidth - 12) / 2
                        : (constraints.maxWidth - 24) / 3;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final event in events)
                      SizedBox(width: width, child: _auditEventCard(event)),
                  ],
                );
              },
            ),
            const SizedBox(height: 16),
            _FilterSurface(
              child: Row(
                children: [
                  _MiniCounter(label: 'Page $currentPage of $pageCount'),
                  const Spacer(),
                  OutlinedButton.icon(
                    onPressed: offset > 0 && !loading
                        ? () {
                            offset = (offset - pageSize).clamp(0, total);
                            load();
                          }
                        : null,
                    icon: const Icon(Icons.chevron_left_rounded),
                    label: const LText('Previous'),
                  ),
                  const SizedBox(width: 8),
                  FilledButton.icon(
                    onPressed: offset + events.length < total && !loading
                        ? () {
                            offset += pageSize;
                            load();
                          }
                        : null,
                    icon: const Icon(Icons.chevron_right_rounded),
                    label: const LText('Next'),
                  ),
                ],
              ),
            ),
          ],
          if (loading && events.isNotEmpty) ...[
            const SizedBox(height: 12),
            const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
          ],
          const SizedBox(height: 28),
          PlatformSecretsPanel(api: widget.api, currentUser: widget.user),
          const SizedBox(height: 28),
          CompanySettingsPanel(api: widget.api, currentUser: widget.user),
          const SizedBox(height: 28),
          AccessControlPanel(api: widget.api, currentUser: widget.user),
        ],
      ),
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

String _localizedPartnerCategory(Map<String, dynamic> partner) {
  final key = HimateI18n.activeLocale == 'hu_HU' ? 'category_name_hu' : 'category_name_en';
  final localized = '${partner[key] ?? ''}'.trim();
  if (localized.isNotEmpty) return localized;
  return '${partner['category_name'] ?? ''}'.trim();
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


class ResponsiveActionBar extends StatelessWidget {
  const ResponsiveActionBar({
    required this.actions,
    this.leading,
    this.breakpoint = 620,
    this.gap = 10,
    super.key,
  });

  final Widget? leading;
  final List<Widget> actions;
  final double breakpoint;
  final double gap;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < breakpoint) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (leading != null) ...[
                leading!,
                SizedBox(height: gap),
              ],
              for (var i = 0; i < actions.length; i++) ...[
                SizedBox(width: double.infinity, child: actions[i]),
                if (i < actions.length - 1) SizedBox(height: gap),
              ],
            ],
          );
        }
        return Row(
          children: [
            if (leading != null) Expanded(child: leading!),
            if (leading != null && actions.isNotEmpty) SizedBox(width: gap),
            if (actions.isNotEmpty)
              Flexible(
                child: Align(
                  alignment: Alignment.centerRight,
                  child: Wrap(
                    alignment: WrapAlignment.end,
                    spacing: gap,
                    runSpacing: gap,
                    children: actions,
                  ),
                ),
              ),
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
    this.dismissEnabled = true,
    this.onDismiss,
    super.key,
  });

  final String title, subtitle, primaryLabel;
  final IconData icon;
  final Widget child;
  final VoidCallback onPrimary;
  final double width;
  final bool dismissEnabled;
  final VoidCallback? onDismiss;

  @override
  Widget build(BuildContext context) {
    final viewport = MediaQuery.sizeOf(context);
    final mediaPhone = viewport.width < 520;
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: EdgeInsets.symmetric(
        horizontal: mediaPhone ? 10 : 20,
        vertical: mediaPhone ? 12 : 24,
      ),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: width,
          maxHeight: viewport.height * (mediaPhone ? .94 : .88),
        ),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final compact = constraints.maxWidth < 520;

            final header = Container(
              width: double.infinity,
              padding: EdgeInsets.fromLTRB(
                compact ? 16 : 22,
                compact ? 14 : 20,
                compact ? 10 : 18,
                compact ? 12 : 18,
              ),
              decoration: const BoxDecoration(
                border: Border(bottom: BorderSide(color: brandMist)),
              ),
              child: compact
                  ? Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              width: 36,
                              height: 36,
                              decoration: BoxDecoration(
                                color: brandGold.withOpacity(.12),
                                borderRadius: BorderRadius.circular(11),
                              ),
                              child: Icon(icon, color: brandGold, size: 19),
                            ),
                            const Spacer(),
                            IconButton(
                              onPressed: dismissEnabled ? (onDismiss ?? () => Navigator.pop(context, false)) : null,
                              icon: const Icon(Icons.close_rounded),
                            ),
                          ],
                        ),
                        const SizedBox(height: 10),
                        LText(title, style: Theme.of(context).textTheme.titleMedium),
                        const SizedBox(height: 4),
                        LText(
                          subtitle,
                          style: const TextStyle(
                            color: brandTextSoft,
                            fontSize: 11,
                            height: 1.4,
                          ),
                        ),
                      ],
                    )
                  : Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Container(
                          width: 42,
                          height: 42,
                          decoration: BoxDecoration(
                            color: brandGold.withOpacity(.12),
                            borderRadius: BorderRadius.circular(11),
                          ),
                          child: Icon(icon, color: brandGold, size: 21),
                        ),
                        const SizedBox(width: 13),
                        Expanded(
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              LText(title, style: Theme.of(context).textTheme.titleLarge),
                              const SizedBox(height: 4),
                              LText(
                                subtitle,
                                style: const TextStyle(
                                  color: brandTextSoft,
                                  fontSize: 12,
                                  height: 1.4,
                                ),
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                    onPressed: dismissEnabled ? (onDismiss ?? () => Navigator.pop(context, false)) : null,
                          icon: const Icon(Icons.close_rounded),
                        ),
                      ],
                    ),
            );

            final footer = Container(
              width: double.infinity,
              padding: EdgeInsets.fromLTRB(
                compact ? 14 : 20,
                compact ? 10 : 14,
                compact ? 14 : 20,
                compact ? 12 : 18,
              ),
              decoration: const BoxDecoration(
                border: Border(top: BorderSide(color: brandMist)),
              ),
              child: ResponsiveActionBar(
                breakpoint: 480,
                actions: [
                  TextButton(
                    onPressed: dismissEnabled ? (onDismiss ?? () => Navigator.pop(context, false)) : null,
                    child: const LText('Cancel'),
                  ),
                  FilledButton(
                    onPressed: onPrimary,
                    child: LText(primaryLabel),
                  ),
                ],
              ),
            );

            return Container(
              decoration: BoxDecoration(
                color: brandSurfaceRaised,
                borderRadius: BorderRadius.circular(16),
                border: Border.all(color: brandIonBlue.withOpacity(.24)),
                boxShadow: [
                  BoxShadow(
                    color: brandNavy.withOpacity(.16),
                    blurRadius: 44,
                    offset: const Offset(0, 20),
                  ),
                ],
              ),
              child: Column(
                mainAxisSize: MainAxisSize.max,
                children: [
                  header,
                  Flexible(
                    child: SingleChildScrollView(
                      padding: EdgeInsets.all(compact ? 16 : 22),
                      child: child,
                    ),
                  ),
                  footer,
                ],
              ),
            );
          },
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
        color: brandSurfaceRaised,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: brandIonBlue.withOpacity(.24), width: 1.15),
        boxShadow: [BoxShadow(color: brandNavy.withOpacity(.055), blurRadius: 14, offset: const Offset(0, 5))],
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
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 180),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
        decoration: BoxDecoration(
          color: brandNavy.withOpacity(.055),
          borderRadius: BorderRadius.circular(99),
          border: Border.all(color: brandNavy.withOpacity(.07)),
        ),
        child: LText(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w700)),
      ),
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
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 180),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
        decoration: BoxDecoration(color: tone.withOpacity(.08), borderRadius: BorderRadius.circular(99), border: Border.all(color: tone.withOpacity(.15))),
        child: LText(_humanize(label), maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: tone, fontSize: 9, fontWeight: FontWeight.w800, letterSpacing: .25)),
      ),
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
          color: brandSurfaceRaised,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: hover ? brandGold.withOpacity(.78) : brandIonBlue.withOpacity(.24), width: hover ? 1.5 : 1.1),
          boxShadow: [BoxShadow(color: brandNavy.withOpacity(hover ? .14 : .075), blurRadius: hover ? 24 : 15, offset: Offset(0, hover ? 10 : 6))],
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
                      if (p['test_partner'] == true) ...[
                        const _StatusPill(label: 'TEST'),
                        const SizedBox(width: 7),
                      ],
                      if (p['reference_partner'] == true)
                        Tooltip(message: 'Reference partner', child: Icon(Icons.workspace_premium_rounded, color: brandGold, size: 21)),
                    ],
                  ),
                  const SizedBox(height: 16),
                  LText('${p['display_name']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 19, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 4),
                  LText(_localizedPartnerCategory(p), maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 11)),
                  const SizedBox(height: 7),
                  LText(
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
                    Expanded(child: LText('${p['id']}', style: const TextStyle(color: brandTextSoft, fontSize: 9.5))),
                    const LText('Open workspace', style: TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w700)),
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
        child: LText(
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
              color: hover ? brandGold.withOpacity(.08) : brandSurfaceRaised,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: brandGold.withOpacity(hover ? .75 : .35), width: 1.1),
            ),
            child: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(width: 46, height: 46, decoration: BoxDecoration(color: brandGold.withOpacity(.12), shape: BoxShape.circle), child: const Icon(Icons.add_rounded, color: brandGold, size: 26)),
                  const SizedBox(height: 11),
                  const LText('NEW PARTNER', style: TextStyle(color: brandNavy, fontSize: 11, fontWeight: FontWeight.w800, letterSpacing: 1.3)),
                  const SizedBox(height: 5),
                  const LText('Create a new partner workspace', style: TextStyle(color: brandTextSoft, fontSize: 10.5)),
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
        color: brandSurfaceRaised,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: brandIonBlue.withOpacity(.22)),
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
          LText(spec.title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 12.5)),
          const SizedBox(height: 3),
          LText(spec.subtitle, maxLines: 2, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.6, height: 1.35)),
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
        LText(title, style: Theme.of(context).textTheme.titleLarge),
        const SizedBox(height: 4),
        LText(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 11.5, height: 1.4)),
      ],
    );
    if (trailing == null) return copy;
    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < 760) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              copy,
              const SizedBox(height: 10),
              SizedBox(width: double.infinity, child: trailing!),
            ],
          );
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(child: copy),
            const SizedBox(width: 12),
            Flexible(child: Align(alignment: Alignment.centerRight, child: trailing!)),
          ],
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
              color: brandSurfaceRaised,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: hover ? brandGold.withOpacity(.72) : brandIonBlue.withOpacity(.22)),
              boxShadow: hover ? [BoxShadow(color: brandNavy.withOpacity(.06), blurRadius: 18, offset: const Offset(0, 7))] : const [],
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Expanded(child: LText('${m['label']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 13))),
                  const SizedBox(width: 8),
                  _StatusPill(label: '${m['status']}'),
                ]),
                const SizedBox(height: 5),
                LText('${m['group_label']} · ${m['key']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
                const SizedBox(height: 13),
                Wrap(
                  spacing: 7,
                  runSpacing: 7,
                  children: [
                    _TinyFlag(icon: visible ? Icons.visibility_outlined : Icons.visibility_off_outlined, label: visible ? 'VISIBLE' : 'HIDDEN', active: visible),
                    _TinyFlag(icon: included ? Icons.inventory_2_outlined : Icons.add_card_outlined, label: included ? 'BASE' : 'EXTRA', active: included),
                  ],
                ),
                const SizedBox(height: 10),
                Row(children: [
                  const LText('Activation fee', style: TextStyle(color: brandTextSoft, fontSize: 9.5)),
                  const Spacer(),
                  LText(money(m['partner_activation_fee']), style: const TextStyle(color: brandNavy, fontSize: 11, fontWeight: FontWeight.w700)),
                ]),
                const SizedBox(height: 9),
                Row(children: [
                  const LText('30-day price', style: TextStyle(color: brandTextSoft, fontSize: 9.5)),
                  const Spacer(),
                  LText(included ? 'Included' : money(m['partner_price']), style: const TextStyle(color: brandNavy, fontSize: 16, fontWeight: FontWeight.w600)),
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
      LText(label, style: TextStyle(fontSize: 8.5, color: active ? brandSuccess : brandSteel, fontWeight: FontWeight.w700)),
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
      _DefinitionRow(label: 'Category', value: _localizedPartnerCategory(partner)),
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
    action: IconButton(onPressed: onEdit, tooltip: uiLiteral('Edit commercial terms'), icon: const Icon(Icons.edit_outlined, size: 18)),
    children: [
      _DefinitionRow(label: 'Billing mode', value: _humanize('${terms['billing_mode'] ?? 'PAID'}'), emphasis: true),
      _DefinitionRow(label: 'Charity status', value: _humanize('${terms['charity_status'] ?? 'NOT_REQUESTED'}')),
      _DefinitionRow(label: 'Activation fee', value: terms['activation_fee_waived'] == true ? 'Waived' : money(terms['activation_fee'])),
      _DefinitionRow(label: 'License status', value: _humanize('${license['status'] ?? 'NOT_PAID'}')),
      _DefinitionRow(label: 'License paid', value: '${money(license['paid_amount'])} / ${money(license['required_amount'])}'),
      _DefinitionRow(label: 'Individual base fee', value: money(billing['effective_base_fee'])),
      _DefinitionRow(label: 'Minimum monthly commitment', value: money(terms['minimum_monthly_commitment'])),
      _DefinitionRow(label: 'Quote / offer', value: '${terms['quote_reference'] ?? '—'}'),
      _DefinitionRow(label: 'Terms version', value: '${terms['terms_version'] ?? 1}'),
      _DefinitionRow(label: 'Extra modules', value: money(billing['extra_module_fee'])),
      _DefinitionRow(label: 'Current total', value: money(billing['current_total']), emphasis: true),
      _DefinitionRow(label: 'Annual increase', value: '${terms['annual_increase_percent'] ?? 5}% · January 1'),
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
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              final titleBlock = Row(
                children: [
                  Container(width: 36, height: 36, decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(9)), child: Icon(icon, color: brandGold, size: 19)),
                  const SizedBox(width: 10),
                  Expanded(child: LText(title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
                ],
              );
              if (action == null) return titleBlock;
              if (constraints.maxWidth < 460) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    titleBlock,
                    const SizedBox(height: 10),
                    Align(alignment: Alignment.centerLeft, child: action!),
                  ],
                );
              }
              return Row(
                children: [
                  Expanded(child: titleBlock),
                  const SizedBox(width: 10),
                  Flexible(child: Align(alignment: Alignment.centerRight, child: action!)),
                ],
              );
            },
          ),
          const SizedBox(height: 14),
          ...children,
        ],
      ),
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
    child: LayoutBuilder(
      builder: (context, constraints) {
        final valueStyle = TextStyle(
          color: brandNavy,
          fontSize: emphasis ? 13 : 11,
          fontWeight: emphasis ? FontWeight.w800 : FontWeight.w600,
        );
        if (constraints.maxWidth < 360) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              LText(label, style: const TextStyle(color: brandTextSoft, fontSize: 10.5)),
              const SizedBox(height: 3),
              LText(value, maxLines: 4, overflow: TextOverflow.ellipsis, style: valueStyle),
            ],
          );
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: LText(label, style: const TextStyle(color: brandTextSoft, fontSize: 10.5))),
            const SizedBox(width: 12),
            Flexible(child: LText(value, textAlign: TextAlign.right, maxLines: 3, overflow: TextOverflow.ellipsis, style: valueStyle)),
          ],
        );
      },
    ),
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
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final narrow = constraints.maxWidth < 520;
      return Wrap(
        spacing: 8,
        runSpacing: 8,
        children: [
          for (final item in items)
            SizedBox(
              width: narrow ? constraints.maxWidth : null,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
                decoration: BoxDecoration(color: brandNavy.withOpacity(.04), borderRadius: BorderRadius.circular(9), border: Border.all(color: brandMist)),
                child: Row(
                  mainAxisSize: narrow ? MainAxisSize.max : MainAxisSize.min,
                  children: [
                    Icon(item.icon, color: brandGold, size: 15),
                    const SizedBox(width: 7),
                    if (narrow)
                      Expanded(
                        child: Wrap(
                          children: [
                            LText('${item.label}: ', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
                            LText(item.value, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w700)),
                          ],
                        ),
                      )
                    else
                      ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 300),
                        child: Wrap(
                          children: [
                            LText('${item.label}: ', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
                            LText(item.value, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w700)),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
            ),
        ],
      );
    },
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
          const Expanded(child: LText('Documents', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
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
        LText('${document['name']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 11.5, fontWeight: FontWeight.w600)),
        const SizedBox(height: 2),
        LText(_humanize('${document['kind']}'), style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
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
          const Expanded(child: LText('Invoices', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
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
        LText('${invoice['id']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontSize: 11.5, fontWeight: FontWeight.w600)),
        const SizedBox(height: 2),
        LText('${invoice['invoice_date'] ?? ''}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
      ])),
      Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
        LText(money(invoice['total']), style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w600, fontSize: 14)),
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
      LText(title, style: const TextStyle(color: brandNavy, fontSize: 11.5, fontWeight: FontWeight.w600)),
      if (actionLabel != null && onTap != null) ...[
        const SizedBox(height: 8),
        TextButton(onPressed: onTap, child: LText(actionLabel!)),
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
    child: LText(label, style: const TextStyle(color: brandNavy, fontSize: 9.5, fontWeight: FontWeight.w800, letterSpacing: 1.6)),
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
    action: IconButton(onPressed: onEdit, tooltip: uiLiteral('Edit billing profile'), icon: const Icon(Icons.edit_outlined, size: 18)),
    children: [
      _DefinitionRow(label: 'Legal name', value: clean(profile['legal_name'])),
      _DefinitionRow(label: 'Billing email', value: clean(profile['email'])),
      _DefinitionRow(label: 'Tax ID', value: clean(profile['tax_id'])),
      _DefinitionRow(label: 'VAT rate', value: '${profile['vat_rate_percent'] ?? 0}%'),
      _DefinitionRow(label: 'VAT jurisdiction', value: clean(profile['vat_jurisdiction'])),
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
    title: 'Central-6 Finance Rules',
    icon: Icons.rule_folder_outlined,
    children: [
      _DefinitionRow(label: 'Partner activation', value: 'Final HIMATE approval required'),
      _DefinitionRow(label: 'Invoice lifecycle', value: 'Draft → Approved → Sent → Paid / Cancelled'),
      _DefinitionRow(label: 'Payment collection', value: 'Blocked until Sent'),
      _DefinitionRow(label: 'Partner visibility', value: 'Sent / Paid / Cancelled only'),
      _DefinitionRow(label: 'Zero-dollar support', value: 'Documented waiver · no invoice'),
      _DefinitionRow(label: 'Package price basis', value: 'Net + configured VAT'),
      _DefinitionRow(label: 'VAT authority', value: 'Admin billing profile'),
      _DefinitionRow(label: 'PDF delivery', value: 'Generated from approved ledger'),
      _DefinitionRow(label: 'Email delivery', value: 'Queued in delivery outbox'),
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
            LText('${module['label']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 13)),
            const SizedBox(height: 4),
            LText('${module['group_label']}', style: const TextStyle(color: brandSteel, fontSize: 10, fontWeight: FontWeight.w600)),
            const SizedBox(height: 3),
            LText('${module['key']}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.2)),
            const SizedBox(height: 12),
            Row(children: [
              LText('v${module['version'] ?? '1.0.0'} → ${module['latest_version'] ?? '1.0.0'}', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
              const Spacer(),
              LText(money(module['default_monthly_price']), style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w600, fontSize: 16)),
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
              LText(healthy ? 'Platform operational' : 'Platform requires attention', style: const TextStyle(color: brandWhite, fontSize: 24, fontWeight: FontWeight.w600)),
              const SizedBox(height: 5),
              LText('Environment: $environment · Version: $version', style: const TextStyle(color: Color(0xFFB8C6D6), fontSize: 11)),
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
      _DefinitionRow(label: 'Backups / restore', value: 'Encrypted · restore verified'),
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
        final stackActions = shouldStackContentActions(constraints.maxWidth, actions.length);
        final padding = constraints.maxWidth < 520 ? 16.0 : constraints.maxWidth < 1050 ? 22.0 : 28.0;
        final header = Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (eyebrow != null) ...[
              LText(eyebrow!, style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: 17, fontWeight: FontWeight.w600)),
              const SizedBox(height: 2),
            ],
            LText(title, style: GoogleFonts.cormorantGaramond(color: brandNavy, fontSize: narrow ? 36 : 42, fontWeight: FontWeight.w600, height: .98)),
            const SizedBox(height: 6),
            ConstrainedBox(constraints: const BoxConstraints(maxWidth: 760), child: LText(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 12.5, height: 1.45))),
          ],
        );

        return TweenAnimationBuilder<double>(
          tween: Tween(begin: 0, end: 1),
          duration: const Duration(milliseconds: 360),
          curve: Curves.easeOutCubic,
          builder: (context, value, animatedChild) => Opacity(
            opacity: value,
            child: Transform.translate(offset: Offset(0, 14 * (1 - value)), child: animatedChild),
          ),
          child: Scrollbar(
            child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(padding, 24, padding, 40),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (narrow || stackActions)
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      header,
                      if (actions.isNotEmpty) ...[
                        const SizedBox(height: 16),
                        Wrap(spacing: 9, runSpacing: 9, children: actions),
                      ],
                    ],
                  )
                else
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(child: header),
                      if (actions.isNotEmpty) ...[
                        const SizedBox(width: 20),
                        Flexible(
                          child: Align(
                            alignment: Alignment.topRight,
                            child: Wrap(alignment: WrapAlignment.end, spacing: 9, runSpacing: 9, children: actions),
                          ),
                        ),
                      ],
                    ],
                  ),
                const SizedBox(height: 22),
                child,
              ],
            ),
            ),
          ),
        );
      },
    );
  }
}

class ResponsiveKpiGrid extends StatelessWidget {
  const ResponsiveKpiGrid({required this.children, this.gap = 12, super.key});
  final List<Widget> children;
  final double gap;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = responsiveGridColumnsForWidth(constraints.maxWidth);
        final width = (constraints.maxWidth - gap * (columns - 1)) / columns;
        return Wrap(
          spacing: gap,
          runSpacing: gap,
          children: [for (final child in children) SizedBox(width: width, child: child)],
        );
      },
    );
  }
}

class Kpi extends StatefulWidget {
  const Kpi({
    required this.label,
    required this.value,
    required this.note,
    this.icon = Icons.auto_graph_outlined,
    this.accent = brandNavy,
    this.onTap,
    super.key,
  });
  final String label, value, note;
  final IconData icon;
  final Color accent;
  final VoidCallback? onTap;
  @override
  State<Kpi> createState() => _KpiState();
}

class _KpiState extends State<Kpi> {
  bool hover = false;
  @override
  Widget build(BuildContext context) => MouseRegion(
    cursor:widget.onTap==null?MouseCursor.defer:SystemMouseCursors.click,
    onEnter: (_) => setState(() => hover = true),
    onExit: (_) => setState(() => hover = false),
    child:Semantics(
      button:widget.onTap!=null,
      label:widget.label,
      child:GestureDetector(
        behavior:HitTestBehavior.opaque,
        onTap:widget.onTap,
        child:AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOut,
          width: double.infinity,
          height: 132,
          transform: Matrix4.translationValues(0, hover ? -3 : 0, 0),
          decoration: BoxDecoration(
            color: brandSurfaceRaised,
            borderRadius: BorderRadius.circular(14),
            border: Border.all(color: hover ? widget.accent.withOpacity(.72) : brandIonBlue.withOpacity(.22)),
            boxShadow: [BoxShadow(color: brandNavy.withOpacity(hover ? .085 : .035), blurRadius: hover ? 22 : 12, offset: Offset(0, hover ? 9 : 5))],
          ),
          child: Padding(
            padding: const EdgeInsets.all(15),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Row(children: [
                Icon(widget.icon, color: widget.accent, size: 22),
                const Spacer(),
                if(widget.onTap!=null)Icon(Icons.arrow_forward_rounded,color:widget.accent,size:16)
                else Container(width:5,height:5,decoration:BoxDecoration(color:widget.accent,shape:BoxShape.circle)),
              ]),
              const Spacer(),
              LText(widget.label, style: const TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w600)),
              const SizedBox(height: 2),
              FittedBox(fit: BoxFit.scaleDown, alignment: Alignment.centerLeft, child: LText(widget.value, style: const TextStyle(color: brandNavy, fontSize: 25, fontWeight: FontWeight.w600))),
              LText(widget.note, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.3)),
            ]),
          ),
        ),
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
              Container(padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5), decoration: BoxDecoration(color: tone.withOpacity(.08), borderRadius: BorderRadius.circular(99)), child: LText(status.toUpperCase(), style: TextStyle(color: tone, fontSize: 9, fontWeight: FontWeight.w700))),
            ]),
            const Spacer(),
            LText(name, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14)),
            const SizedBox(height: 3),
            LText(ok ? 'Service responding normally' : 'Awaiting healthy response', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
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
      child: LayoutBuilder(
        builder: (context, constraints) {
          final copy = Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              LText(title, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 16)),
              const SizedBox(height: 6),
              LText(message, style: const TextStyle(color: brandTextSoft, height: 1.45)),
            ],
          );
          final mark = Container(
            width: 46,
            height: 46,
            decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(11)),
            child: Icon(icon, color: brandGold, size: 22),
          );
          if (constraints.maxWidth < 360) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [mark, const SizedBox(height: 12), copy],
            );
          }
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [mark, const SizedBox(width: 15), Expanded(child: copy)],
          );
        },
      ),
    ),
  );
}