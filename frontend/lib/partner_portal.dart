part of 'main.dart';

class PartnerPortalApp extends StatefulWidget {
  const PartnerPortalApp({super.key});

  @override
  State<PartnerPortalApp> createState() => _PartnerPortalAppState();
}

class _PartnerPortalAppState extends State<PartnerPortalApp> {
  final Api api = Api();
  final GlobalKey<NavigatorState> navigatorKey = GlobalKey<NavigatorState>();
  Map<String, dynamic>? user;
  bool loading = true;
  String localeCode = 'en_US';

  @override
  void initState() {
    super.initState();
    final stored = html.window.localStorage['himate_partner_locale'];
    if (stored == 'hu_HU' || stored == 'en_US') localeCode = stored!;
    if (Uri.base.path == '/partner/app' || Uri.base.path.startsWith('/partner/app/')) {
      restore();
    } else {
      loading = false;
      unawaited(restore(silent: true));
    }
  }

  Future<void> restore({bool silent = false}) async {
    try {
      final next = await api.get('/partner/api/v1/auth/me', force: true).timeout(const Duration(seconds: 3));
      if (!mounted) return;
      user = next;
      final preferred = next['preferred_locale']?.toString();
      if (preferred == 'hu_HU' || preferred == 'en_US') localeCode = preferred!;
      setState(() => loading = false);
      if (silent && Uri.base.path == '/partner/login') {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          navigatorKey.currentState?.pushNamedAndRemoveUntil('/partner/app', (_) => false);
        });
      }
    } catch (_) {
      if (!mounted) return;
      user = null;
      setState(() => loading = false);
      if (!silent && Uri.base.path.startsWith('/partner/app')) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          navigatorKey.currentState?.pushNamedAndRemoveUntil('/partner/login', (_) => false);
        });
      }
    }
  }

  void setLocale(String value) {
    final normalized = value == 'hu_HU' ? 'hu_HU' : 'en_US';
    html.window.localStorage['himate_partner_locale'] = normalized;
    setState(() => localeCode = normalized);
  }

  Future<void> login(String email, String password, bool remember) async {
    user = await api.post('/partner/api/v1/auth/login', {
      'email': email,
      'password': password,
      'remember': remember,
    });
    if (!mounted) return;
    setState(() {});
    navigatorKey.currentState?.pushNamedAndRemoveUntil('/partner/app', (_) => false);
  }

  Future<void> logout() async {
    try {
      await api.post('/partner/api/v1/auth/logout');
    } catch (_) {}
    api.clearCache();
    user = null;
    if (!mounted) return;
    setState(() {});
    navigatorKey.currentState?.pushNamedAndRemoveUntil('/partner/login', (_) => false);
  }

  Widget loginPage() => PartnerPortalLoginPage(
        onLogin: login,
        localeCode: localeCode,
        onLocaleChanged: setLocale,
      );

  @override
  Widget build(BuildContext context) {
    HimateI18n.activeLocale = localeCode;
    final initial = Uri.base.path.startsWith('/partner/app') ? '/partner/app' : '/partner/login';
    return MaterialApp(
      navigatorKey: navigatorKey,
      debugShowCheckedModeBanner: false,
      title: 'HIMATE Partner Portal',
      theme: buildBrandTheme(),
      locale: himateLocaleFromCode(localeCode),
      supportedLocales: himateSupportedLocales,
      localizationsDelegates: const [
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      initialRoute: initial,
      routes: {
        '/partner/login': (_) => loading
            ? const _PartnerPortalLoading()
            : user == null
                ? loginPage()
                : _SignedInRedirect(onContinue: () {
                    navigatorKey.currentState?.pushNamedAndRemoveUntil('/partner/app', (_) => false);
                  }),
        '/partner/app': (_) => loading
            ? const _PartnerPortalLoading()
            : user == null
                ? loginPage()
                : PartnerPortalShell(api: api, user: user!, onLogout: logout),
      },
      onUnknownRoute: (_) => MaterialPageRoute(
        settings: const RouteSettings(name: '/partner/login'),
        builder: (_) => user == null
            ? loginPage()
            : PartnerPortalShell(api: api, user: user!, onLogout: logout),
      ),
    );
  }
}

class _PartnerPortalLoading extends StatelessWidget {
  const _PartnerPortalLoading();

  @override
  Widget build(BuildContext context) => const Scaffold(
        backgroundColor: brandNavyDeep,
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              HimateLogo(onDark: true, width: 190),
              SizedBox(height: 20),
              SizedBox(width: 28, height: 28, child: CircularProgressIndicator(strokeWidth: 2.2, color: brandGold)),
            ],
          ),
        ),
      );
}

class PartnerPortalLoginPage extends StatefulWidget {
  const PartnerPortalLoginPage({
    required this.onLogin,
    required this.localeCode,
    required this.onLocaleChanged,
    super.key,
  });

  final Future<void> Function(String email, String password, bool remember) onLogin;
  final String localeCode;
  final ValueChanged<String> onLocaleChanged;

  @override
  State<PartnerPortalLoginPage> createState() => _PartnerPortalLoginPageState();
}

class _PartnerPortalLoginPageState extends State<PartnerPortalLoginPage> {
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
      setState(() => error = 'Enter your email and password.');
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: brandNavyDeep,
      body: Stack(
        children: [
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [brandNavyDeep, brandNavy, brandNavySoft],
                ),
              ),
            ),
          ),
          SafeArea(
            child: SingleChildScrollView(
              padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 28),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 470),
                  child: Column(
                    children: [
                      Row(
                        children: [
                          const HimateLogo(onDark: true, width: 190),
                          const Spacer(),
                          DropdownButtonHideUnderline(
                            child: DropdownButton<String>(
                              value: widget.localeCode,
                              dropdownColor: brandWhite,
                              iconEnabledColor: brandGold,
                              style: const TextStyle(color: brandWhite, fontWeight: FontWeight.w700),
                              items: const [
                                DropdownMenuItem(value: 'en_US', child: LText('EN', style: TextStyle(color: brandNavy))),
                                DropdownMenuItem(value: 'hu_HU', child: LText('HU', style: TextStyle(color: brandNavy))),
                              ],
                              onChanged: (value) { if (value != null) widget.onLocaleChanged(value); },
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 64),
                      Card(
                        child: Padding(
                          padding: const EdgeInsets.all(34),
                          child: AutofillGroup(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.stretch,
                              children: [
                                LText(
                                  'Partner Portal',
                                  textAlign: TextAlign.center,
                                  style: GoogleFonts.cormorantGaramond(
                                    color: brandNavy,
                                    fontSize: 40,
                                    fontWeight: FontWeight.w700,
                                  ),
                                ),
                                const SizedBox(height: 8),
                                const LText(
                                  'Your modules, results, billing and company workspace.',
                                  textAlign: TextAlign.center,
                                  style: TextStyle(color: brandTextSoft, fontSize: 12.5),
                                ),
                                const SizedBox(height: 28),
                                TextField(
                                  controller: email,
                                  keyboardType: TextInputType.emailAddress,
                                  autofillHints: const [AutofillHints.username, AutofillHints.email],
                                  decoration: InputDecoration(labelText: uiLiteral('Email'), prefixIcon: Icon(Icons.mail_outline_rounded)),
                                ),
                                const SizedBox(height: 12),
                                TextField(
                                  controller: password,
                                  obscureText: obscure,
                                  autofillHints: const [AutofillHints.password],
                                  onSubmitted: (_) => submit(),
                                  decoration: InputDecoration(
                                    labelText: uiLiteral('Password'),
                                    prefixIcon: const Icon(Icons.lock_outline_rounded),
                                    suffixIcon: IconButton(
                                      onPressed: () => setState(() => obscure = !obscure),
                                      icon: Icon(obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined),
                                    ),
                                  ),
                                ),
                                const SizedBox(height: 8),
                                Row(
                                  children: [
                                    Checkbox(value: remember, onChanged: (value) => setState(() => remember = value ?? false)),
                                    const LText('Remember me', style: TextStyle(color: brandTextSoft, fontSize: 11)),
                                  ],
                                ),
                                if (error != null) ...[
                                  const SizedBox(height: 8),
                                  LText(error!, style: const TextStyle(color: brandDanger, fontSize: 10.5, fontWeight: FontWeight.w600)),
                                ],
                                const SizedBox(height: 16),
                                FilledButton.icon(
                                  onPressed: busy ? null : submit,
                                  icon: busy
                                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2, color: brandWhite))
                                      : const Icon(Icons.login_rounded),
                                  label: LText(busy ? 'Signing in...' : 'Sign in securely'),
                                ),
                                const SizedBox(height: 16),
                                const Divider(),
                                const SizedBox(height: 8),
                                const Row(
                                  children: [
                                    Icon(Icons.lock_outline_rounded, size: 16, color: brandSteel),
                                    SizedBox(width: 7),
                                    Expanded(
                                      child: LText(
                                        'Access is restricted to your organization. Tenant scope is enforced by HIMATE.',
                                        style: TextStyle(color: brandTextSoft, fontSize: 9.5, height: 1.4),
                                      ),
                                    ),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _PortalNavSpec {
  const _PortalNavSpec(this.label, this.icon, this.permission);
  final String label;
  final IconData icon;
  final String permission;
}

class PartnerPortalShell extends StatefulWidget {
  const PartnerPortalShell({required this.api, required this.user, required this.onLogout, super.key});
  final Api api;
  final Map<String, dynamic> user;
  final Future<void> Function() onLogout;

  @override
  State<PartnerPortalShell> createState() => _PartnerPortalShellState();
}

class _PartnerPortalShellState extends State<PartnerPortalShell> {
  int selected = 0;
  bool loading = true;
  String? error;
  Map<String, dynamic> company = <String, dynamic>{};
  Map<String, dynamic> billing = <String, dynamic>{};
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> impact = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> subscriptions = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> invoices = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> users = <Map<String, dynamic>>[];
  Map<String, dynamic> designState = <String, dynamic>{};
  List<Map<String, dynamic>> designProfiles = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> designMedia = <Map<String, dynamic>>[];

  static const nav = <_PortalNavSpec>[
    _PortalNavSpec('Overview', Icons.dashboard_outlined, 'dashboard.read'),
    _PortalNavSpec('Modules', Icons.extension_outlined, 'modules.read'),
    _PortalNavSpec('Results', Icons.insights_outlined, 'impact.read'),
    _PortalNavSpec('Billing', Icons.receipt_long_outlined, 'billing.read'),
    _PortalNavSpec('Company', Icons.apartment_outlined, 'company.read'),
    _PortalNavSpec('Design', Icons.palette_outlined, 'design.read'),
    _PortalNavSpec('Users', Icons.group_outlined, 'users.read'),
  ];

  bool can(String permission) {
    final raw = widget.user['permissions'];
    if (raw is! List) return false;
    final values = raw.map((e) => e.toString()).toSet();
    return values.contains('*') || values.contains(permission);
  }

  List<_PortalNavSpec> get visibleNav => nav.where((item) => can(item.permission)).toList();

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<Map<String, dynamic>?> safeGet(String path) async {
    try {
      return await widget.api.get(path, force: true);
    } catch (_) {
      return null;
    }
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final dashboard = await widget.api.get('/partner/api/v1/dashboard', force: true);
      final companyRaw = dashboard['company'];
      final moduleRaw = dashboard['modules'];
      final billingRaw = dashboard['billing'];
      final impactRaw = dashboard['impact'];
      final extras = await Future.wait<Map<String, dynamic>?>([
        can('billing.read') ? safeGet('/partner/api/v1/billing/subscriptions') : Future.value(null),
        can('billing.read') ? safeGet('/partner/api/v1/billing/invoices') : Future.value(null),
        can('users.read') ? safeGet('/partner/api/v1/users') : Future.value(null),
        can('design.read') ? safeGet('/partner/api/v1/design') : Future.value(null),
        can('design.read') ? safeGet('/partner/api/v1/design/media') : Future.value(null),
      ]);
      if (!mounted) return;
      setState(() {
        company = companyRaw is Map ? Map<String, dynamic>.from(companyRaw) : <String, dynamic>{};
        final moduleMap = moduleRaw is Map ? Map<String, dynamic>.from(moduleRaw) : <String, dynamic>{};
        modules = items(moduleMap);
        billing = billingRaw is Map ? Map<String, dynamic>.from(billingRaw) : <String, dynamic>{};
        final impactMap = impactRaw is Map ? Map<String, dynamic>.from(impactRaw) : <String, dynamic>{};
        impact = items(impactMap);
        subscriptions = extras[0] == null ? <Map<String, dynamic>>[] : items(extras[0]!);
        invoices = extras[1] == null ? <Map<String, dynamic>>[] : items(extras[1]!);
        users = extras[2] == null ? <Map<String, dynamic>>[] : items(extras[2]!);
        designState = extras[3] == null ? <String, dynamic>{} : Map<String, dynamic>.from(extras[3]!);
        final rawProfiles = designState['profiles'];
        designProfiles = rawProfiles is List
            ? rawProfiles.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList()
            : <Map<String, dynamic>>[];
        designMedia = extras[4] == null ? <Map<String, dynamic>>[] : items(extras[4]!);
        loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { error = e.toString(); loading = false; });
    }
  }

  void toast(String message, {bool failure = false}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: LText(message),
        behavior: SnackBarBehavior.floating,
        backgroundColor: failure ? brandDanger : brandSuccess,
      ),
    );
  }

  Map<String, dynamic>? subscriptionFor(String key) {
    for (final item in subscriptions) {
      if ('${item['module_key']}' == key) return item;
    }
    return null;
  }

  Future<void> activateModule(Map<String, dynamic> module) async {
    final label = '${module['label']}';
    final price = number(module['partner_price']);
    final currency = '${module['currency'] ?? 'USD'}';
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: LText('Activate $label?'),
        content: LText(
          module['included_in_base'] == true
              ? 'This module is included in your base package. Activation takes effect immediately.'
              : 'This module will be added at $currency ${price.toStringAsFixed(2)} per 30-day cycle. Activation takes effect immediately.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const LText('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const LText('Activate module')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.post('/partner/api/v1/modules/${module['key']}/activate');
      await load();
      if (mounted) toast('$label activated.');
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
  }

  Future<void> setCancellation(Map<String, dynamic> module, bool cancelAtEnd) async {
    final label = '${module['label']}';
    final sub = subscriptionFor('${module['key']}');
    final end = '${sub?['period_end_exclusive'] ?? billing['next_billing_date'] ?? 'the current cycle end'}';
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: LText(cancelAtEnd ? 'Cancel $label?' : 'Keep $label active?'),
        content: LText(
          cancelAtEnd
              ? 'The module remains active through $end and will not renew after that period. Historical data is preserved.'
              : 'The scheduled cancellation will be withdrawn and the module will continue renewing.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const LText('Back')),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: LText(cancelAtEnd ? 'Cancel at period end' : 'Continue renewal'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.patch('/partner/api/v1/modules/${module['key']}/subscription', {
        'cancel_at_period_end': cancelAtEnd,
      });
      await load();
      if (mounted) toast(cancelAtEnd ? 'Cancellation scheduled for the current period end.' : 'Cancellation withdrawn.');
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
  }

  Future<void> editCompany() async {
    final fields = <String, TextEditingController>{
      'display_name': TextEditingController(text: '${company['display_name'] ?? ''}'),
      'legal_name': TextEditingController(text: '${company['legal_name'] ?? ''}'),
      'brand_name': TextEditingController(text: '${company['brand_name'] ?? ''}'),
      'registration_number': TextEditingController(text: '${company['registration_number'] ?? ''}'),
      'tax_id': TextEditingController(text: '${company['tax_id'] ?? ''}'),
      'contact_name': TextEditingController(text: '${company['contact_name'] ?? ''}'),
      'contact_email': TextEditingController(text: '${company['contact_email'] ?? ''}'),
      'finance_contact_name': TextEditingController(text: '${company['finance_contact_name'] ?? ''}'),
      'finance_contact_email': TextEditingController(text: '${company['finance_contact_email'] ?? ''}'),
      'technical_contact_name': TextEditingController(text: '${company['technical_contact_name'] ?? ''}'),
      'technical_contact_email': TextEditingController(text: '${company['technical_contact_email'] ?? ''}'),
      'marketing_contact_name': TextEditingController(text: '${company['marketing_contact_name'] ?? ''}'),
      'marketing_contact_email': TextEditingController(text: '${company['marketing_contact_email'] ?? ''}'),
      'country': TextEditingController(text: '${company['country'] ?? ''}'),
      'state_region': TextEditingController(text: '${company['state_region'] ?? ''}'),
      'city': TextEditingController(text: '${company['city'] ?? ''}'),
      'postal_code': TextEditingController(text: '${company['postal_code'] ?? ''}'),
      'address_line1': TextEditingController(text: '${company['address_line1'] ?? ''}'),
      'address_line2': TextEditingController(text: '${company['address_line2'] ?? ''}'),
      'website': TextEditingController(text: '${company['website'] ?? ''}'),
      'phone': TextEditingController(text: '${company['phone'] ?? ''}'),
      'logo_url': TextEditingController(text: '${company['logo_url'] ?? ''}'),
    };
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'Company profile',
        subtitle: 'Update your organization identity and contacts. Lifecycle and platform controls remain managed by HIMATE.',
        icon: Icons.apartment_outlined,
        width: 850,
        primaryLabel: 'Save company profile',
        onPrimary: () => Navigator.pop(dialogContext, true),
        child: Column(
          children: [
            ResponsiveFieldPair(
              first: TextField(controller: fields['display_name'], decoration: InputDecoration(labelText: uiLiteral('Display name'))),
              second: TextField(controller: fields['legal_name'], decoration: InputDecoration(labelText: uiLiteral('Legal name'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['brand_name'], decoration: InputDecoration(labelText: uiLiteral('Brand / DBA'))),
              second: TextField(controller: fields['registration_number'], decoration: InputDecoration(labelText: uiLiteral('Registration number'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['tax_id'], decoration: InputDecoration(labelText: uiLiteral('Tax / VAT ID'))),
              second: TextField(controller: fields['phone'], decoration: InputDecoration(labelText: uiLiteral('Phone'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['contact_name'], decoration: InputDecoration(labelText: uiLiteral('Primary contact'))),
              second: TextField(controller: fields['contact_email'], decoration: InputDecoration(labelText: uiLiteral('Primary email'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['finance_contact_name'], decoration: InputDecoration(labelText: uiLiteral('Finance contact'))),
              second: TextField(controller: fields['finance_contact_email'], decoration: InputDecoration(labelText: uiLiteral('Finance email'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['technical_contact_name'], decoration: InputDecoration(labelText: uiLiteral('Technical contact'))),
              second: TextField(controller: fields['technical_contact_email'], decoration: InputDecoration(labelText: uiLiteral('Technical email'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['marketing_contact_name'], decoration: InputDecoration(labelText: uiLiteral('Marketing contact'))),
              second: TextField(controller: fields['marketing_contact_email'], decoration: InputDecoration(labelText: uiLiteral('Marketing email'))),
            ),
            const SizedBox(height: 12),
            TextField(controller: fields['address_line1'], decoration: InputDecoration(labelText: uiLiteral('Address line 1'))),
            const SizedBox(height: 12),
            TextField(controller: fields['address_line2'], decoration: InputDecoration(labelText: uiLiteral('Address line 2'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['city'], decoration: InputDecoration(labelText: uiLiteral('City'))),
              second: TextField(controller: fields['postal_code'], decoration: InputDecoration(labelText: uiLiteral('Postal code'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['state_region'], decoration: InputDecoration(labelText: uiLiteral('State / region'))),
              second: TextField(controller: fields['country'], decoration: InputDecoration(labelText: uiLiteral('Country'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: fields['website'], decoration: InputDecoration(labelText: uiLiteral('Website'))),
              second: TextField(controller: fields['logo_url'], decoration: InputDecoration(labelText: uiLiteral('Logo URL'))),
            ),
          ],
        ),
      ),
    );
    if (ok == true) {
      try {
        await widget.api.patch('/partner/api/v1/company', {
          for (final entry in fields.entries) entry.key: entry.value.text.trim(),
        });
        await load();
        if (mounted) toast('Company profile updated.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }
    for (final controller in fields.values) {
      controller.dispose();
    }
  }

  Future<void> createUser() async {
    final name = TextEditingController();
    final email = TextEditingController();
    final password = TextEditingController();
    String role = 'viewer';
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Add Partner Portal user',
          subtitle: 'Roles apply only inside your organization and cannot grant HIMATE platform-administrator access.',
          icon: Icons.person_add_alt_1_rounded,
          width: 650,
          primaryLabel: 'Create user',
          onPrimary: () => Navigator.pop(dialogContext, true),
          child: Column(
            children: [
              TextField(controller: name, decoration: InputDecoration(labelText: uiLiteral('Name'))),
              const SizedBox(height: 12),
              TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Email'))),
              const SizedBox(height: 12),
              TextField(controller: password, obscureText: true, decoration: InputDecoration(labelText: uiLiteral('Temporary password'))),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: role,
                decoration: InputDecoration(labelText: uiLiteral('Portal role')),
                items: const [
                  DropdownMenuItem(value: 'owner', child: LText('Owner')),
                  DropdownMenuItem(value: 'admin', child: LText('Admin')),
                  DropdownMenuItem(value: 'billing', child: LText('Billing')),
                  DropdownMenuItem(value: 'viewer', child: LText('Viewer')),
                ],
                onChanged: (value) { if (value != null) setLocal(() => role = value); },
              ),
            ],
          ),
        ),
      ),
    );
    if (ok == true) {
      try {
        await widget.api.post('/partner/api/v1/users', {
          'name': name.text.trim(),
          'email': email.text.trim(),
          'password': password.text,
          'role': role,
        });
        await load();
        if (mounted) toast('Partner Portal user created.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }
    name.dispose(); email.dispose(); password.dispose();
  }

  Future<void> editUser(Map<String, dynamic> target) async {
    final name = TextEditingController(text: '${target['name'] ?? ''}');
    final email = TextEditingController(text: '${target['email'] ?? ''}');
    String role = '${target['role'] ?? 'viewer'}';
    bool active = target['active'] != false;
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Edit portal user',
          subtitle: 'Role and account status changes invalidate the user session immediately.',
          icon: Icons.manage_accounts_outlined,
          width: 650,
          primaryLabel: 'Save user',
          onPrimary: () => Navigator.pop(dialogContext, true),
          child: Column(
            children: [
              TextField(controller: name, decoration: InputDecoration(labelText: uiLiteral('Name'))),
              const SizedBox(height: 12),
              TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Email'))),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                value: role,
                decoration: InputDecoration(labelText: uiLiteral('Portal role')),
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
                title: const LText('Active account'),
                onChanged: (value) => setLocal(() => active = value),
              ),
            ],
          ),
        ),
      ),
    );
    if (ok == true) {
      try {
        await widget.api.patch('/partner/api/v1/users/${target['id']}', {
          'name': name.text.trim(),
          'email': email.text.trim(),
          'role': role,
          'active': active,
        });
        await load();
        if (mounted) toast('Partner Portal user updated.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }
    name.dispose(); email.dispose();
  }

  Widget overview() {
    final activeModules = modules.where((m) => m['status'] == 'ACTIVE').length;
    final available = modules.where((m) => m['can_activate'] == true).length;
    return Content(
      eyebrow: 'PARTNER PORTAL',
      title: company['display_name']?.toString() ?? 'Your organization',
      subtitle: 'Your HIMATE services, measurable results and current commercial position in one tenant-isolated workspace.',
      actions: [OutlinedButton.icon(onPressed: load, icon: const Icon(Icons.refresh_rounded), label: const LText('Refresh'))],
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        ResponsiveKpiGrid(children: [
          Kpi(label: 'Active modules', value: activeModules.toString(), note: 'Currently enabled services', icon: Icons.extension_outlined, accent: brandNavy),
          Kpi(label: 'Available modules', value: available.toString(), note: 'Eligible for activation', icon: Icons.add_circle_outline_rounded, accent: brandSteel),
          Kpi(label: 'Current 30-day total', value: money(billing['current_total']), note: 'Base + active extras', icon: Icons.payments_outlined, accent: brandGold),
          Kpi(label: 'Impact metrics', value: impact.length.toString(), note: 'Results currently reported', icon: Icons.insights_outlined, accent: brandSuccess),
        ]),
        const SizedBox(height: 22),
        LayoutBuilder(builder: (context, constraints) {
          final width = constraints.maxWidth < 760 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
          return Wrap(spacing: 12, runSpacing: 12, children: [
            SizedBox(width: width, child: _InfoCard(title: 'Service', icon: Icons.apartment_outlined, children: [
              _DefinitionRow(label: 'Lifecycle', value: '${company['lifecycle'] ?? '—'}'),
              _DefinitionRow(label: 'Platform version', value: '${company['platform_version'] ?? '—'}'),
              _DefinitionRow(label: 'Primary domain', value: '${company['primary_domain'] ?? '—'}'),
              _DefinitionRow(label: 'System health', value: '${company['system_health'] ?? '—'}'),
            ])),
            SizedBox(width: width, child: _InfoCard(title: 'Next billing cycle', icon: Icons.calendar_month_outlined, children: [
              _DefinitionRow(label: 'Next billing date', value: '${billing['next_billing_date'] ?? '—'}'),
              _DefinitionRow(label: 'Base service fee', value: money(billing['effective_base_fee'])),
              _DefinitionRow(label: 'Module fee', value: money(billing['extra_module_fee'])),
              _DefinitionRow(label: 'Current total', value: money(billing['current_total']), emphasis: true),
            ])),
          ]);
        }),
      ]),
    );
  }

  Widget moduleCard(Map<String, dynamic> module) {
    final active = module['status'] == 'ACTIVE';
    final sub = subscriptionFor('${module['key']}');
    final cancelling = sub?['cancel_at_period_end'] == true;
    final blockers = (module['activation_blockers'] is List)
        ? (module['activation_blockers'] as List).map((e) => e.toString()).toList()
        : <String>[];
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Expanded(child: LText('${module['label']}', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14))),
            _StatusPill(label: '${module['status'] ?? 'NOT_LICENSED'}'),
          ]),
          const SizedBox(height: 7),
          LText('${module['description'] ?? ''}', maxLines: 3, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 10, height: 1.4)),
          const SizedBox(height: 12),
          _DefinitionRow(label: 'Group', value: '${module['group_label'] ?? '—'}'),
          _DefinitionRow(label: 'Version', value: '${module['latest_version'] ?? '—'}'),
          _DefinitionRow(
            label: '30-day price',
            value: module['included_in_base'] == true ? 'Included in base' : '${module['currency'] ?? 'USD'} ${number(module['partner_price']).toStringAsFixed(2)}',
          ),
          if (active && sub != null) ...[
            _DefinitionRow(label: 'Current period ends', value: '${sub['period_end_exclusive'] ?? '—'}'),
            _DefinitionRow(label: 'Renewal', value: cancelling ? 'Stops at period end' : 'Automatic'),
          ],
          if (blockers.isNotEmpty) ...[
            const SizedBox(height: 8),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(color: brandWarning.withOpacity(.07), borderRadius: BorderRadius.circular(9), border: Border.all(color: brandWarning.withOpacity(.18))),
              child: LText(blockers.join(' · '), style: const TextStyle(color: brandWarning, fontSize: 9.5, fontWeight: FontWeight.w600)),
            ),
          ],
          const SizedBox(height: 18),
          if (!active)
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                onPressed: can('modules.write') && module['can_activate'] == true ? () => activateModule(module) : null,
                icon: const Icon(Icons.add_circle_outline_rounded),
                label: const LText('Activate module'),
              ),
            )
          else if (sub != null && module['included_in_base'] != true && can('modules.write'))
            SizedBox(
              width: double.infinity,
              child: cancelling
                  ? OutlinedButton.icon(
                      onPressed: () => setCancellation(module, false),
                      icon: const Icon(Icons.restart_alt_rounded),
                      label: const LText('Continue renewal'),
                    )
                  : OutlinedButton.icon(
                      onPressed: () => setCancellation(module, true),
                      icon: const Icon(Icons.event_busy_outlined),
                      label: const LText('Cancel at period end'),
                    ),
            ),
        ]),
      ),
    );
  }

  Widget modulesPage() {
    final active = modules.where((m) => m['status'] == 'ACTIVE').toList();
    final available = modules.where((m) => m['status'] != 'ACTIVE').toList();
    Widget grid(List<Map<String, dynamic>> data) => LayoutBuilder(builder: (context, constraints) {
      final width = constraints.maxWidth < 650 ? constraints.maxWidth : constraints.maxWidth < 1050 ? (constraints.maxWidth - 12) / 2 : (constraints.maxWidth - 24) / 3;
      return Wrap(spacing: 12, runSpacing: 12, children: [
        for (final module in data) SizedBox(width: width, child: moduleCard(module)),
      ]);
    });
    return Content(
      eyebrow: 'MY SERVICES',
      title: 'Modules',
      subtitle: 'Activate eligible HIMATE modules or schedule an active module to stop at the end of its current paid 30-day period.',
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        _SectionHeader(title: 'My Modules', subtitle: 'Currently active for your organization.', trailing: _MiniCounter(label: '${active.length} active')),
        const SizedBox(height: 12),
        if (active.isEmpty) const _MessageCard(icon: Icons.extension_off_outlined, title: 'No active modules', message: 'Available modules are listed below.') else grid(active),
        const SizedBox(height: 26),
        _SectionHeader(title: 'Available Modules', subtitle: 'Availability, pricing and dependency rules are controlled by HIMATE.', trailing: _MiniCounter(label: '${available.length} listed')),
        const SizedBox(height: 12),
        if (available.isEmpty) const _MessageCard(icon: Icons.check_circle_outline_rounded, title: 'Everything is active', message: 'There are no additional modules in your current catalog.') else grid(available),
      ]),
    );
  }

  Widget resultsPage() {
    return Content(
      eyebrow: 'MEASURABLE VALUE',
      title: 'Results & Impact',
      subtitle: 'Partner-scoped performance metrics received and calculated by the HIMATE Impact layer.',
      child: impact.isEmpty
          ? const _MessageCard(icon: Icons.insights_outlined, title: 'No reported metrics yet', message: 'Results will appear when verified or connector-sourced impact data becomes available.')
          : LayoutBuilder(builder: (context, constraints) {
              final width = constraints.maxWidth < 620 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
              return Wrap(spacing: 12, runSpacing: 12, children: [
                for (final item in impact)
                  SizedBox(
                    width: width,
                    child: _InfoCard(
                      title: '${item['label'] ?? item['metric_key']}',
                      icon: Icons.auto_graph_outlined,
                      children: [
                        _DefinitionRow(label: 'Current value', value: '${item['numeric_value'] ?? item['text_value'] ?? '—'} ${item['unit'] ?? ''}', emphasis: true),
                        _DefinitionRow(label: 'Latest period', value: '${item['latest_period_end'] ?? '—'}'),
                        _DefinitionRow(label: 'Observations', value: '${item['observations'] ?? 0}'),
                        if (item['delta_from_baseline'] != null)
                          _DefinitionRow(label: 'Delta from baseline', value: '${number(item['delta_from_baseline']).toStringAsFixed(2)}'),
                      ],
                    ),
                  ),
              ]);
            }),
    );
  }

  Widget billingPage() {
    return Content(
      eyebrow: 'COMMERCIAL',
      title: 'Billing',
      subtitle: 'Your current 30-day service value, renewal timing and invoice history.',
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        ResponsiveKpiGrid(children: [
          Kpi(label: 'Base fee', value: money(billing['effective_base_fee']), note: 'Current service base', icon: Icons.home_work_outlined, accent: brandNavy),
          Kpi(label: 'Module fee', value: money(billing['extra_module_fee']), note: 'Active non-base modules', icon: Icons.extension_outlined, accent: brandSteel),
          Kpi(label: 'Current total', value: money(billing['current_total']), note: 'Per 30-day cycle', icon: Icons.payments_outlined, accent: brandGold),
          Kpi(label: 'Next billing date', value: '${billing['next_billing_date'] ?? '—'}', note: 'Activation-anchored cycle', icon: Icons.calendar_month_outlined, accent: brandSuccess),
        ]),
        const SizedBox(height: 24),
        _SectionHeader(title: 'Invoices', subtitle: 'Commercial invoice history for your organization.', trailing: _MiniCounter(label: '${invoices.length} invoices')),
        const SizedBox(height: 10),
        if (invoices.isEmpty)
          const _MessageCard(icon: Icons.receipt_long_outlined, title: 'No invoices yet', message: 'Invoices will appear here when billing records are generated.')
        else
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(children: [
                for (var i = 0; i < invoices.length; i++) ...[
                  Row(children: [
                    const Icon(Icons.receipt_long_outlined, size: 18, color: brandGold),
                    const SizedBox(width: 10),
                    Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      LText('${invoices[i]['id'] ?? 'Invoice'}', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700)),
                      LText('${invoices[i]['service_period_start'] ?? ''} — ${invoices[i]['service_period_end_exclusive'] ?? ''}', style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
                    ])),
                    LText('${invoices[i]['currency'] ?? 'USD'} ${number(invoices[i]['total']).toStringAsFixed(2)}', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800)),
                    const SizedBox(width: 10),
                    _StatusPill(label: '${invoices[i]['status'] ?? 'DRAFT'}'),
                  ]),
                  if (i < invoices.length - 1) const Divider(height: 24),
                ],
              ]),
            ),
          ),
      ]),
    );
  }

  Widget companyPage() {
    return Content(
      eyebrow: 'ORGANIZATION',
      title: 'Company Profile',
      subtitle: 'Your own organization identity and operational contacts. Platform lifecycle remains controlled by HIMATE.',
      actions: [
        if (can('company.write')) FilledButton.icon(onPressed: editCompany, icon: const Icon(Icons.edit_outlined), label: const LText('Edit profile')),
      ],
      child: LayoutBuilder(builder: (context, constraints) {
        final width = constraints.maxWidth < 760 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
        return Wrap(spacing: 12, runSpacing: 12, children: [
          SizedBox(width: width, child: _InfoCard(title: 'Identity', icon: Icons.apartment_outlined, children: [
            _DefinitionRow(label: 'Display name', value: '${company['display_name'] ?? '—'}'),
            _DefinitionRow(label: 'Legal name', value: '${company['legal_name'] ?? '—'}'),
            _DefinitionRow(label: 'Brand', value: '${company['brand_name'] ?? '—'}'),
            _DefinitionRow(label: 'Registration', value: '${company['registration_number'] ?? '—'}'),
            _DefinitionRow(label: 'Tax / VAT', value: '${company['tax_id'] ?? '—'}'),
            _DefinitionRow(label: 'Website', value: '${company['website'] ?? '—'}'),
          ])),
          SizedBox(width: width, child: _InfoCard(title: 'Address & Contact', icon: Icons.contact_mail_outlined, children: [
            _DefinitionRow(label: 'Primary contact', value: '${company['contact_name'] ?? '—'}'),
            _DefinitionRow(label: 'Email', value: '${company['contact_email'] ?? '—'}'),
            _DefinitionRow(label: 'Phone', value: '${company['phone'] ?? '—'}'),
            _DefinitionRow(label: 'City', value: '${company['city'] ?? '—'}'),
            _DefinitionRow(label: 'Region', value: '${company['state_region'] ?? '—'}'),
            _DefinitionRow(label: 'Country', value: '${company['country'] ?? '—'}'),
          ])),
          SizedBox(width: width, child: _InfoCard(title: 'Finance Contact', icon: Icons.account_balance_wallet_outlined, children: [
            _DefinitionRow(label: 'Name', value: '${company['finance_contact_name'] ?? '—'}'),
            _DefinitionRow(label: 'Email', value: '${company['finance_contact_email'] ?? '—'}'),
          ])),
          SizedBox(width: width, child: _InfoCard(title: 'Technical & Marketing', icon: Icons.hub_outlined, children: [
            _DefinitionRow(label: 'Technical', value: '${company['technical_contact_name'] ?? '—'}'),
            _DefinitionRow(label: 'Technical email', value: '${company['technical_contact_email'] ?? '—'}'),
            _DefinitionRow(label: 'Marketing', value: '${company['marketing_contact_name'] ?? '—'}'),
            _DefinitionRow(label: 'Marketing email', value: '${company['marketing_contact_email'] ?? '—'}'),
          ])),
        ]);
      }),
    );
  }

  Widget usersPage() {
    return Content(
      eyebrow: 'ACCESS',
      title: 'Partner Users',
      subtitle: 'Manage access for your own organization only. Portal roles cannot grant HIMATE control-plane authority.',
      actions: [
        if (can('users.write')) FilledButton.icon(onPressed: createUser, icon: const Icon(Icons.person_add_alt_1_rounded), label: const LText('Add user')),
      ],
      child: users.isEmpty
          ? const _MessageCard(icon: Icons.group_outlined, title: 'No portal users', message: 'Create an organization user to provide Partner Portal access.')
          : LayoutBuilder(builder: (context, constraints) {
              final width = constraints.maxWidth < 680 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
              return Wrap(spacing: 12, runSpacing: 12, children: [
                for (final user in users)
                  SizedBox(
                    width: width,
                    child: _InfoCard(
                      title: '${user['name']}',
                      icon: Icons.person_outline_rounded,
                      action: can('users.write') ? IconButton(onPressed: () => editUser(user), icon: const Icon(Icons.edit_outlined)) : null,
                      children: [
                        _DefinitionRow(label: 'Email', value: '${user['email']}'),
                        _DefinitionRow(label: 'Role', value: _humanize('${user['role'] ?? 'viewer'}')),
                        _DefinitionRow(label: 'Status', value: user['active'] == true ? 'Active' : 'Inactive'),
                      ],
                    ),
                  ),
              ]);
            }),
    );
  }

  Widget pageFor(_PortalNavSpec item) {
    switch (item.label) {
      case 'Modules': return modulesPage();
      case 'Results': return resultsPage();
      case 'Billing': return billingPage();
      case 'Company': return companyPage();
      case 'Design': return designPage();
      case 'Users': return usersPage();
      default: return overview();
    }
  }

  Widget navPanel(List<_PortalNavSpec> items) {
    return Container(
      width: 238,
      color: brandNavyDeep,
      padding: const EdgeInsets.fromLTRB(16, 22, 16, 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Padding(padding: EdgeInsets.symmetric(horizontal: 8), child: HimateLogo(onDark: true, width: 174)),
          const SizedBox(height: 28),
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: 8),
            child: LText('PARTNER PORTAL', style: TextStyle(color: brandGold, fontSize: 9, letterSpacing: 1.4, fontWeight: FontWeight.w800)),
          ),
          const SizedBox(height: 10),
          for (var i = 0; i < items.length; i++)
            Padding(
              padding: const EdgeInsets.only(bottom: 5),
              child: ListTile(
                selected: selected == i,
                selectedColor: brandWhite,
                textColor: const Color(0xFFC7D1DD),
                iconColor: const Color(0xFFC7D1DD),
                selectedTileColor: brandWhite.withOpacity(.08),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(9)),
                leading: Icon(items[i].icon, size: 19),
                title: LText(items[i].label, style: const TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700)),
                onTap: () => setState(() => selected = i),
              ),
            ),
          const Spacer(),
          const Divider(color: Color(0xFF263950)),
          ListTile(
            textColor: const Color(0xFFC7D1DD),
            iconColor: brandGold,
            leading: const Icon(Icons.person_outline_rounded, size: 19),
            title: LText('${widget.user['name'] ?? ''}', maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 10.5, fontWeight: FontWeight.w700)),
            subtitle: LText(_humanize('${widget.user['role'] ?? 'viewer'}'), style: const TextStyle(color: Color(0xFF8FA1B5), fontSize: 8.5)),
          ),
          TextButton.icon(
            onPressed: widget.onLogout,
            icon: const Icon(Icons.logout_rounded, size: 18, color: brandGold),
            label: const LText('Sign out', style: TextStyle(color: brandWhite)),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final items = visibleNav;
    if (selected >= items.length) selected = 0;
    final page = items.isEmpty ? const SizedBox.shrink() : pageFor(items[selected]);

    if (loading && company.isEmpty) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (error != null && company.isEmpty) {
      return Scaffold(
        backgroundColor: brandIvory,
        body: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 560),
            child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Partner Portal unavailable', message: error!),
          ),
        ),
      );
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final wide = constraints.maxWidth >= 900;
        if (wide) {
          return Scaffold(
            backgroundColor: brandIvory,
            body: Row(children: [
              navPanel(items),
              Expanded(
                child: Column(children: [
                  Container(
                    height: 64,
                    padding: const EdgeInsets.symmetric(horizontal: 24),
                    decoration: const BoxDecoration(color: brandWhite, border: Border(bottom: BorderSide(color: brandMist))),
                    child: Row(children: [
                      Expanded(
                        child: LText(
                          '${company['display_name'] ?? 'Partner'}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 13),
                        ),
                      ),
                      const SizedBox(width: 12),
                      IconButton(onPressed: load, tooltip: 'Refresh', icon: const Icon(Icons.refresh_rounded)),
                    ]),
                  ),
                  Expanded(child: page),
                ]),
              ),
            ]),
          );
        }

        return Scaffold(
          backgroundColor: brandIvory,
          appBar: AppBar(
            title: LText('${company['display_name'] ?? 'Partner Portal'}'),
            actions: [
              IconButton(onPressed: load, icon: const Icon(Icons.refresh_rounded)),
              IconButton(onPressed: widget.onLogout, icon: const Icon(Icons.logout_rounded)),
            ],
          ),
          drawer: Drawer(
            backgroundColor: brandNavyDeep,
            child: SafeArea(
              child: Column(children: [
                const Padding(padding: EdgeInsets.all(22), child: HimateLogo(onDark: true, width: 176)),
                Expanded(
                  child: ListView.builder(
                    itemCount: items.length,
                    itemBuilder: (context, i) => ListTile(
                      selected: selected == i,
                      selectedTileColor: brandWhite.withOpacity(.08),
                      textColor: const Color(0xFFC7D1DD),
                      iconColor: brandGold,
                      leading: Icon(items[i].icon),
                      title: LText(items[i].label),
                      onTap: () {
                        setState(() => selected = i);
                        Navigator.pop(context);
                      },
                    ),
                  ),
                ),
              ]),
            ),
          ),
          body: page,
        );
      },
    );
  }
}
