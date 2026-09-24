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
  final passwordFocus = FocusNode();
  bool busy = false;
  bool obscure = true;
  bool remember = true;
  String? error;

  @override
  void dispose() {
    email.dispose();
    password.dispose();
    passwordFocus.dispose();
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
                                  focusNode: passwordFocus,
                                  obscureText: obscure,
                                  enableSuggestions: false,
                                  autocorrect: false,
                                  autofillHints: const [AutofillHints.password],
                                  onSubmitted: (_) => submit(),
                                  decoration: InputDecoration(
                                    labelText: uiLiteral('Password'),
                                    prefixIcon: const Icon(Icons.lock_outline_rounded),
                                    suffixIcon: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        IconButton(
                                          tooltip: uiLiteral('Clear password'),
                                          onPressed: () {
                                            TextInput.finishAutofillContext(shouldSave: false);
                                            password.clear();
                                            passwordFocus.requestFocus();
                                            if (mounted) setState(() => error = null);
                                          },
                                          icon: const Icon(Icons.close_rounded),
                                        ),
                                        IconButton(
                                          tooltip: uiLiteral(obscure ? 'Show password' : 'Hide password'),
                                          onPressed: () => setState(() => obscure = !obscure),
                                          icon: Icon(obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined),
                                        ),
                                      ],
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
  bool _defaultWorkspaceApplied = false;
  bool loading = true;
  String? error;
  Map<String, String> secondaryLoadErrors = <String, String>{};
  Map<String, dynamic> company = <String, dynamic>{};
  Map<String, dynamic> billing = <String, dynamic>{};
  Map<String, dynamic> plan = <String, dynamic>{};
  List<Map<String, dynamic>> plans = <Map<String, dynamic>>[];
  Map<String, dynamic> charity = <String, dynamic>{};
  Map<String, dynamic> charityModules = <String, dynamic>{};
  String planBillingFrequency = 'MONTHLY';
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

  Future<Map<String, dynamic>?> safeGet(
    String label,
    String path,
    Map<String, String> failures,
  ) async {
    try {
      return await widget.api.get(path, force: true);
    } catch (e) {
      failures[label] = e.toString();
      return null;
    }
  }

  Future<void> load() async {
    if (mounted) {
      setState(() {
        loading = true;
        error = null;
        secondaryLoadErrors = <String, String>{};
      });
    }

    final secondaryFailures = <String, String>{};
    final extrasFuture = Future.wait<Map<String, dynamic>?>([
      can('billing.read') ? safeGet('Billing subscriptions', '/partner/api/v1/billing/subscriptions', secondaryFailures) : Future.value(null),
      can('billing.read') ? safeGet('Billing invoices', '/partner/api/v1/billing/invoices', secondaryFailures) : Future.value(null),
      can('users.read') ? safeGet('Portal users', '/partner/api/v1/users', secondaryFailures) : Future.value(null),
      can('design.read') ? safeGet('Design settings', '/partner/api/v1/design', secondaryFailures) : Future.value(null),
      can('design.read') ? safeGet('Design media', '/partner/api/v1/design/media', secondaryFailures) : Future.value(null),
      can('billing.read') ? safeGet('Subscription plans', '/partner/api/v1/plans', secondaryFailures) : Future.value(null),
      can('billing.read') ? safeGet('Current plan', '/partner/api/v1/plan', secondaryFailures) : Future.value(null),
      can('billing.read') ? safeGet('Charity status', '/partner/api/v1/charity', secondaryFailures) : Future.value(null),
      can('modules.read') ? safeGet('Charity modules', '/partner/api/v1/charity/modules', secondaryFailures) : Future.value(null),
    ]);

    try {
      final dashboard = await widget.api.get('/partner/api/v1/dashboard', force: true);
      final companyRaw = dashboard['company'];
      final moduleRaw = dashboard['modules'];
      final billingRaw = dashboard['billing'];
      final impactRaw = dashboard['impact'];
      final degradedRaw = dashboard['degraded_sections'];
      final degradedSections = degradedRaw is List
          ? degradedRaw.map((item) => item.toString()).where((item) => item.isNotEmpty).toList()
          : <String>[];

      if (!mounted) return;
      setState(() {
        company = companyRaw is Map ? Map<String, dynamic>.from(companyRaw) : <String, dynamic>{};
        final moduleMap = moduleRaw is Map ? Map<String, dynamic>.from(moduleRaw) : <String, dynamic>{};
        modules = items(moduleMap);
        billing = billingRaw is Map ? Map<String, dynamic>.from(billingRaw) : <String, dynamic>{};
        final impactMap = impactRaw is Map ? Map<String, dynamic>.from(impactRaw) : <String, dynamic>{};
        impact = items(impactMap);
        loading = false;
      });

      final extras = await extrasFuture;
      if (!mounted) return;
      setState(() {
        if (extras[0] != null) subscriptions = items(extras[0]!);
        if (extras[1] != null) invoices = items(extras[1]!);
        if (extras[2] != null) users = items(extras[2]!);
        if (extras[3] != null) {
          designState = Map<String, dynamic>.from(extras[3]!);
          final rawProfiles = designState['profiles'];
          designProfiles = rawProfiles is List
              ? rawProfiles.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList()
              : <Map<String, dynamic>>[];
        }
        if (extras[4] != null) designMedia = items(extras[4]!);
        secondaryLoadErrors = <String, String>{
          ...secondaryFailures,
          for (final section in degradedSections)
            'Dashboard ${_humanize(section)}': 'The backend reported this section as temporarily degraded.',
        };
        final workspaceRaw = designState['workspace'];
        if (workspaceRaw is Map) {
          final defaultKey = (workspaceRaw['default_module_key'] ?? '').toString().trim();
          if (defaultKey.isNotEmpty) {
            final moduleIndex = modules.indexWhere((module) =>
              (module['key'] ?? '').toString() == defaultKey &&
              (module['access_state'] ?? '').toString() == 'ACTIVE' &&
              module['executable'] == true &&
              module['user_executable'] == true
            );
            if (moduleIndex >= 0) {
              final defaultModule = modules.removeAt(moduleIndex);
              modules.insert(0, defaultModule);
              if (!_defaultWorkspaceApplied) {
                final modulesPageIndex = visibleNav.indexWhere((item) => item.label == 'Modules');
                if (modulesPageIndex >= 0) selected = modulesPageIndex;
                _defaultWorkspaceApplied = true;
              }
            }
          }
        }
        if (extras[5] != null) plans = items(extras[5]!);
        if (extras[6] != null) plan = Map<String, dynamic>.from(extras[6]!);
        if (extras[7] != null) charity = Map<String, dynamic>.from(extras[7]!);
        if (extras[8] != null) charityModules = Map<String, dynamic>.from(extras[8]!);
        final loadedFrequency = '${plan['billing_frequency'] ?? ''}'.toUpperCase();
        if (loadedFrequency == 'MONTHLY' || loadedFrequency == 'ANNUAL') {
          planBillingFrequency = loadedFrequency;
        }
      });
    } catch (e) {
      if (mounted) setState(() { error = e.toString(); loading = false; });
    }
  }

  void toast(String message, {bool failure = false, bool warning = false}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: LText(message),
        behavior: SnackBarBehavior.floating,
        backgroundColor: failure
            ? brandDanger
            : warning
                ? brandWarning
                : brandSuccess,
      ),
    );
  }

  Widget portalPageWithLoadStatus(Widget page) {
    if (secondaryLoadErrors.isEmpty) return page;
    final labels = secondaryLoadErrors.keys.join(' · ');
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(18, 14, 18, 0),
          child: _MessageCard(
            icon: Icons.warning_amber_rounded,
            title: 'Some live data is temporarily unavailable',
            message:
                'HIMATE kept the last successfully loaded values instead of showing missing data as empty. Affected sections: $labels. Refresh after the service recovers.',
          ),
        ),
        const SizedBox(height: 8),
        Expanded(child: page),
      ],
    );
  }

  bool get hasManagedPlan => plan['configured'] == true;
  String get currentPlanKey => '${plan['plan_key'] ?? ''}'.toUpperCase();
  bool get charityApproved =>
      '${charity['billing_mode'] ?? ''}' == 'CHARITY' &&
      '${charity['charity_status'] ?? ''}' == 'APPROVED';
  String get charityStatus => '${charity['charity_status'] ?? 'NOT_REQUESTED'}'.toUpperCase();
  String planMoney(dynamic value) => intl.NumberFormat.currency(symbol: r'$', decimalDigits: 0).format(number(value));

  Future<List<String>?> chooseFlexModules({List<String>? initial}) async {
    final selected = <String>{...(initial ?? const <String>[])};
    final candidates = modules.where((m) =>
      m['publication_status'] == 'PUBLISHED' &&
      m['implementation_state'] == 'READY'
    ).toList();

    return showDialog<List<String>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => AlertDialog(
          title: const LText('Choose Flex modules'),
          content: SizedBox(
            width: 600,
            child: SingleChildScrollView(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                const LText(
                  'Choose up to 15 modules. Later Flex module changes take effect on the next calendar-month boundary.',
                  style: TextStyle(color: brandTextSoft, fontSize: 10.5),
                ),
                const SizedBox(height: 12),
                for (final module in candidates)
                  CheckboxListTile(
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    value: selected.contains('${module['key']}'),
                    title: LText('${module['label']}'),
                    subtitle: LText(
                      '${module['group_label'] ?? ''}',
                      style: const TextStyle(color: brandTextSoft, fontSize: 9),
                    ),
                    onChanged: (value) => setLocal(() {
                      final key = '${module['key']}';
                      if (value == true) {
                        if (selected.length < 15) selected.add(key);
                      } else {
                        selected.remove(key);
                      }
                    }),
                  ),
                const SizedBox(height: 8),
                LText(
                  '${selected.length} / 15 selected',
                  style: const TextStyle(color: brandSuccess, fontWeight: FontWeight.w700),
                ),
              ]),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const LText('Cancel')),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, selected.toList()..sort()),
              child: const LText('Confirm modules'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> requestCharityReview() async {
    final reason = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const LText('Request Charity review?'),
        content: SizedBox(
          width: 520,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const LText(
                'HIMATE reviews Charity eligibility before zero-dollar Charity access can be enabled. Approval is never automatic.',
                style: TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: reason,
                maxLines: 3,
                decoration: InputDecoration(
                  labelText: uiLiteral('Reason / organization context'),
                  hintText: uiLiteral('Optional information for the HIMATE review'),
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const LText('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const LText('Submit request')),
        ],
      ),
    );
    if (ok == true) {
      try {
        await widget.api.post('/partner/api/v1/charity/request', {
          'reason': reason.text.trim(),
        });
        await load();
        if (mounted) toast('Charity review request submitted to HIMATE.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }
    reason.dispose();
  }

  Future<List<String>?> chooseCharityModules({List<String>? initial}) async {
    final selected = <String>{...(initial ?? const <String>[])};
    final candidates = modules.where((m) =>
      m['publication_status'] == 'PUBLISHED' &&
      m['implementation_state'] == 'READY'
    ).toList();

    return showDialog<List<String>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => AlertDialog(
          title: const LText('Choose Charity modules'),
          content: SizedBox(
            width: 640,
            child: SingleChildScrollView(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                const LText(
                  'Your Charity access has been approved. Choose every published and ready module that is useful for your organization. There is no module-count limit.',
                  style: TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
                ),
                const SizedBox(height: 12),
                for (final module in candidates)
                  CheckboxListTile(
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    value: selected.contains('${module['key']}'),
                    title: LText('${module['label']}'),
                    subtitle: LText(
                      '${module['group_label'] ?? ''}',
                      style: const TextStyle(color: brandTextSoft, fontSize: 9),
                    ),
                    onChanged: (value) => setLocal(() {
                      final key = '${module['key']}';
                      if (value == true) {
                        selected.add(key);
                      } else {
                        selected.remove(key);
                      }
                    }),
                  ),
                const SizedBox(height: 8),
                LText(
                  '${selected.length} selected · no Charity limit',
                  style: const TextStyle(color: brandSuccess, fontWeight: FontWeight.w700),
                ),
              ]),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const LText('Cancel')),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, selected.toList()..sort()),
              child: const LText('Save Charity modules'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> manageCharityModules() async {
    if (!charityApproved) {
      toast('Charity module selection becomes available only after HIMATE approval.', failure: true);
      return;
    }
    final raw = charityModules['module_keys'];
    final current = raw is List ? raw.map((e) => e.toString()).toList() : <String>[];
    final chosen = await chooseCharityModules(initial: current);
    if (chosen == null) return;
    try {
      await widget.api.put('/partner/api/v1/charity/modules', {
        'module_keys': chosen,
        'reason': 'Partner Portal approved Charity module selection',
      });
      await load();
      if (mounted) toast('Charity module access updated.');
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
  }

  Future<void> selectSubscriptionPlan(Map<String, dynamic> target) async {
    final targetKey = '${target['plan_key']}';
    List<String> moduleKeys = <String>[];

    if (targetKey == 'FLEX') {
      final current = plan['active_module_keys'] is List
          ? (plan['active_module_keys'] as List).map((e) => e.toString()).toList()
          : <String>[];
      final chosen = await chooseFlexModules(initial: current);
      if (chosen == null) return;
      moduleKeys = chosen;
    }

    final amount = planBillingFrequency == 'ANNUAL'
        ? number(target['annual_price'])
        : number(target['monthly_price']);
    final periodLabel = planBillingFrequency == 'ANNUAL' ? 'year' : 'month';

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: LText('Choose ${target['display_name']}?'),
        content: LText(
          'Selected billing: ${planMoney(amount)} / $periodLabel. Upgrades take effect immediately and charge the full plan-price difference. Downgrades keep the current package until its billing boundary; no refund is issued.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const LText('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const LText('Confirm plan')),
        ],
      ),
    );
    if (confirmed != true) return;

    try {
      final response = await widget.api.patch('/partner/api/v1/plan', {
        'plan_key': targetKey,
        'billing_frequency': planBillingFrequency,
        'module_keys': moduleKeys,
        'reason': 'Partner Portal plan selection',
      });
      await load();
      if (!mounted) return;
      if (response['entitlement_sync_pending'] == true) {
        final warning = (response['warning'] ?? '').toString().trim();
        toast(
          warning.isEmpty
              ? 'The plan was saved, but module access is still synchronizing. HIMATE will retry automatically.'
              : warning,
          warning: true,
        );
      } else {
        toast('${target['display_name']} plan updated.');
      }
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
  }

  Future<void> manageFlexModules() async {
    final current = plan['active_module_keys'] is List
        ? (plan['active_module_keys'] as List).map((e) => e.toString()).toList()
        : <String>[];
    final chosen = await chooseFlexModules(initial: current);
    if (chosen == null) return;

    try {
      final response = await widget.api.put('/partner/api/v1/plan/modules', {
        'module_keys': chosen,
        'reason': 'Partner Portal Flex selection',
      });
      await load();
      if (mounted) toast('Flex module set scheduled for ${response['effective_at'] ?? 'next month'}.');
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
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
          subtitle: 'Roles apply only inside your organization and cannot grant HIMATE platform-administrator access. New users start with access to all modules currently owned by the partner; you can restrict that separately.',
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
          subtitle: 'Role and account status changes invalidate the user session immediately. Module access is managed separately and can never exceed the partner subscription.',
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

  Future<void> manageUserModuleAccess(Map<String, dynamic> target) async {
    if (!can('users.write')) return;
    final userId = '${target['id'] ?? ''}'.trim();
    if (userId.isEmpty) return;
    try {
      final state = await widget.api.get('/partner/api/v1/users/$userId/modules', force: true);
      String mode = '${state['access_mode'] ?? 'ALL_OWNED'}'.toUpperCase();
      final selected = <String>{
        if (state['selected_module_keys'] is List)
          ...(state['selected_module_keys'] as List).map((e) => e.toString()),
      };
      final rawOwned = state['owned_modules'];
      final owned = rawOwned is List
          ? rawOwned.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList()
          : <Map<String, dynamic>>[];

      final ok = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) => StatefulBuilder(
          builder: (context, setLocal) => BrandDialog(
            title: 'User module access',
            subtitle: 'Partner subscription first, user assignment second. A user can never receive a module that the organization does not currently own.',
            icon: Icons.admin_panel_settings_outlined,
            width: 760,
            primaryLabel: 'Save module access',
            onPrimary: () => Navigator.pop(dialogContext, true),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _DefinitionRow(label: 'User', value: '${target['name'] ?? target['email'] ?? userId}', emphasis: true),
                _DefinitionRow(label: 'Partner-owned modules', value: '${owned.length} ACTIVE + executable'),
                const SizedBox(height: 14),
                DropdownButtonFormField<String>(
                  value: mode,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Module access mode'),
                    helperText: uiLiteral('ALL_OWNED follows the partner entitlement automatically. SELECTED is an explicit subset.'),
                  ),
                  items: const [
                    DropdownMenuItem(value: 'ALL_OWNED', child: LText('All partner-owned modules')),
                    DropdownMenuItem(value: 'SELECTED', child: LText('Selected modules only')),
                  ],
                  onChanged: (value) => setLocal(() => mode = value ?? 'ALL_OWNED'),
                ),
                const SizedBox(height: 14),
                if (mode == 'ALL_OWNED')
                  const _MessageCard(
                    icon: Icons.verified_user_outlined,
                    title: 'Automatic entitlement intersection',
                    message: 'This user receives every module that is ACTIVE + executable for the partner. If the partner loses a module, user access disappears automatically.',
                  )
                else ...[
                  const LText(
                    'Choose from modules the partner currently owns. Locked, unavailable or coming-soon modules cannot be assigned.',
                    style: TextStyle(color: brandTextSoft, fontSize: 10, height: 1.4),
                  ),
                  const SizedBox(height: 8),
                  ConstrainedBox(
                    constraints: const BoxConstraints(maxHeight: 390),
                    child: SingleChildScrollView(
                      child: Column(
                        children: [
                          for (final module in owned)
                            CheckboxListTile(
                              dense: true,
                              contentPadding: EdgeInsets.zero,
                              value: selected.contains('${module['key']}'),
                              title: LText('${module['label'] ?? module['key']}'),
                              subtitle: LText(
                                '${module['group_label'] ?? module['group_key'] ?? ''}',
                                style: const TextStyle(color: brandTextSoft, fontSize: 9),
                              ),
                              onChanged: (value) => setLocal(() {
                                final key = '${module['key']}';
                                if (value == true) {
                                  selected.add(key);
                                } else {
                                  selected.remove(key);
                                }
                              }),
                            ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 8),
                  LText(
                    '${selected.length} selected',
                    style: const TextStyle(color: brandSuccess, fontWeight: FontWeight.w700),
                  ),
                ],
              ],
            ),
          ),
        ),
      );

      if (ok == true) {
        await widget.api.put('/partner/api/v1/users/$userId/modules', {
          'access_mode': mode,
          'module_keys': mode == 'SELECTED' ? (selected.toList()..sort()) : <String>[],
        });
        await load();
        if (mounted) toast('User module access updated.');
      }
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
  }

  Widget overview() {
    final activeModules = modules.where((m) => m['user_executable'] == true).length;
    final available = modules.length;
    return Content(
      eyebrow: 'PARTNER PORTAL',
      title: company['display_name']?.toString() ?? 'Your organization',
      subtitle: 'Your HIMATE services, measurable results and current commercial position in one tenant-isolated workspace.',
      actions: [OutlinedButton.icon(onPressed: load, icon: const Icon(Icons.refresh_rounded), label: const LText('Refresh'))],
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        ResponsiveKpiGrid(children: [
          Kpi(label: 'Active modules', value: activeModules.toString(), note: 'Currently enabled services', icon: Icons.extension_outlined, accent: brandNavy),
          Kpi(label: 'Catalog modules', value: available.toString(), note: 'Visible in Module Marketplace', icon: Icons.grid_view_rounded, accent: brandSteel),
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

  List<String> marketplaceStrings(dynamic raw) =>
      raw is List ? raw.map((e) => e.toString()).where((e) => e.trim().isNotEmpty).toList() : <String>[];

  void openBillingFromMarketplace() {
    final target = visibleNav.indexWhere((item) => item.label == 'Billing');
    if (target >= 0) setState(() => selected = target);
  }

  Widget moduleCard(Map<String, dynamic> module) {
    final access = '${module['access_state'] ?? 'LOCKED'}'.toUpperCase();
    final active = access == 'ACTIVE';
    final userExecutable = module['user_executable'] == true;
    final userAccessState = '${module['user_access_state'] ?? (active ? 'GRANTED' : 'ORGANIZATION_LOCKED')}';
    final locked = access == 'LOCKED';
    final comingSoon = access == 'COMING_SOON';
    final unavailable = access == 'UNAVAILABLE';
    final sub = subscriptionFor('${module['key']}');
    final cancelling = sub?['cancel_at_period_end'] == true;
    final blockers = marketplaceStrings(module['activation_blockers']);
    final availablePlans = marketplaceStrings(module['available_in_plan_names']);
    final upgradePlans = marketplaceStrings(module['upgrade_plan_names']);
    final currentPlan = charityApproved
        ? 'Charity access'
        : '${plan['display_name'] ?? plan['plan_key'] ?? ''}'.trim();
    final displayLabel = moduleDisplayName(module);
    final summary = moduleDisplayDescription(module);
    final presentationColor = modulePresentationCardColor(module);
    final isDefaultModule = workspaceDefaultModuleKey == (module['key'] ?? '').toString();
    final statusLabel = active
        ? (userExecutable ? 'INCLUDED' : 'NOT ASSIGNED')
        : locked
            ? 'LOCKED'
            : comingSoon
                ? 'COMING SOON'
                : unavailable
                    ? 'UNAVAILABLE'
                    : access;

    String planAccessText;
    if (active) {
      planAccessText = currentPlan.isEmpty ? 'Included in your subscription' : 'Included in $currentPlan';
    } else if (upgradePlans.isNotEmpty) {
      planAccessText = 'Available with ${upgradePlans.join(' / ')}';
    } else if (comingSoon) {
      planAccessText = 'Catalog preview · live access not released yet';
    } else if (availablePlans.isNotEmpty) {
      planAccessText = 'Available in ${availablePlans.join(' / ')}';
    } else {
      planAccessText = 'Not available with your current subscription';
    }

    Widget accessMessage() {
      if (active && !userExecutable) {
        return Container(
          width: double.infinity,
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: brandWarning.withOpacity(.07),
            borderRadius: BorderRadius.circular(9),
            border: Border.all(color: brandWarning.withOpacity(.18)),
          ),
          child: const LText(
            'Your organization owns this module, but it is not assigned to your user account. Ask an organization owner or admin for access.',
            style: TextStyle(color: brandWarning, fontSize: 9.5, fontWeight: FontWeight.w600, height: 1.4),
          ),
        );
      }
      if (active) {
        return Container(
          width: double.infinity,
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: brandSuccess.withOpacity(.07),
            borderRadius: BorderRadius.circular(9),
            border: Border.all(color: brandSuccess.withOpacity(.18)),
          ),
          child: const LText(
            'This module is included in your current subscription and is available to your organization.',
            style: TextStyle(color: brandSuccess, fontSize: 9.5, fontWeight: FontWeight.w600, height: 1.4),
          ),
        );
      }
      if (locked) {
        final extra = upgradePlans.isEmpty ? '' : ' Available with ${upgradePlans.join(' / ')}.';
        return Container(
          width: double.infinity,
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: brandWarning.withOpacity(.07),
            borderRadius: BorderRadius.circular(9),
            border: Border.all(color: brandWarning.withOpacity(.18)),
          ),
          child: LText(
            'This module is not available with your current subscription.$extra',
            style: const TextStyle(color: brandWarning, fontSize: 9.5, fontWeight: FontWeight.w600, height: 1.4),
          ),
        );
      }
      if (comingSoon) {
        return Container(
          width: double.infinity,
          padding: const EdgeInsets.all(10),
          decoration: BoxDecoration(
            color: brandSteel.withOpacity(.07),
            borderRadius: BorderRadius.circular(9),
            border: Border.all(color: brandSteel.withOpacity(.18)),
          ),
          child: const LText(
            'This canonical HIMATE module is visible for discovery, but live access is not available until its implementation is READY and PUBLISHED.',
            style: TextStyle(color: brandSteel, fontSize: 9.5, fontWeight: FontWeight.w600, height: 1.4),
          ),
        );
      }
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.all(10),
        decoration: BoxDecoration(
          color: brandTextSoft.withOpacity(.06),
          borderRadius: BorderRadius.circular(9),
          border: Border.all(color: brandTextSoft.withOpacity(.15)),
        ),
        child: const LText(
          'This module is currently unavailable for live use.',
          style: TextStyle(color: brandTextSoft, fontSize: 9.5, fontWeight: FontWeight.w600),
        ),
      );
    }

    return Card(
      color: presentationColor?.withOpacity(.10),
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            modulePresentationIcon(module),
            const SizedBox(width: 9),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  LText(
                    displayLabel,
                    style: TextStyle(color: workspacePrimaryColor, fontWeight: FontWeight.w700, fontSize: 14),
                  ),
                  if (displayLabel != (module['label'] ?? '').toString())
                    LText(
                      'HIMATE: ${module['label']}',
                      style: const TextStyle(color: brandTextSoft, fontSize: 8.5),
                    ),
                ],
              ),
            ),
            if (can('design.write'))
              IconButton(
                tooltip: uiLiteral('Customize module presentation'),
                onPressed: () => editModulePresentation(module),
                icon: const Icon(Icons.edit_outlined, size: 18),
              ),
            if (isDefaultModule) ...[
              const _StatusPill(label: 'DEFAULT'),
              const SizedBox(width: 6),
            ],
            _StatusPill(label: statusLabel),
          ]),
          const SizedBox(height: 7),
          LText(
            summary,
            maxLines: 4,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(color: brandTextSoft, fontSize: 10, height: 1.4),
          ),
          const SizedBox(height: 12),
          _DefinitionRow(label: 'Group', value: '${module['group_label'] ?? '—'}'),
          _DefinitionRow(label: 'Plan access', value: planAccessText),
          if (module['executable'] == true)
            _DefinitionRow(label: 'Live availability', value: 'READY + PUBLISHED')
          else
            _DefinitionRow(label: 'Live availability', value: 'Discovery only'),
          _DefinitionRow(
            label: 'Your access',
            value: userExecutable ? 'Assigned' : _humanize(userAccessState),
          ),
          const SizedBox(height: 8),
          accessMessage(),
          if (!hasManagedPlan && blockers.isNotEmpty) ...[
            const SizedBox(height: 8),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: brandWarning.withOpacity(.07),
                borderRadius: BorderRadius.circular(9),
                border: Border.all(color: brandWarning.withOpacity(.18)),
              ),
              child: LText(
                blockers.join(' · '),
                style: const TextStyle(color: brandWarning, fontSize: 9.5, fontWeight: FontWeight.w600),
              ),
            ),
          ],
          const SizedBox(height: 16),
          if (charityApproved && module['executable'] == true && can('modules.write'))
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: manageCharityModules,
                icon: const Icon(Icons.volunteer_activism_outlined),
                label: const LText('Manage Charity modules'),
              ),
            )
          else if (hasManagedPlan && locked && upgradePlans.isNotEmpty && can('billing.read'))
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: openBillingFromMarketplace,
                icon: const Icon(Icons.upgrade_rounded),
                label: const LText('View upgrade options'),
              ),
            )
          else if (!hasManagedPlan && !active && module['executable'] == true)
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                onPressed: can('modules.write') && module['can_activate'] == true ? () => activateModule(module) : null,
                icon: const Icon(Icons.add_circle_outline_rounded),
                label: const LText('Activate module'),
              ),
            )
          else if (!hasManagedPlan && active && sub != null && module['included_in_base'] != true && can('modules.write'))
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
    final included = modules.where((m) => '${m['access_state']}' == 'ACTIVE').toList();
    final locked = modules.where((m) => '${m['access_state']}' == 'LOCKED').toList();
    final comingSoon = modules.where((m) => '${m['access_state']}' == 'COMING_SOON').toList();
    final unavailable = modules.where((m) => '${m['access_state']}' == 'UNAVAILABLE').toList();

    Widget grid(List<Map<String, dynamic>> data) => LayoutBuilder(builder: (context, constraints) {
      final width = constraints.maxWidth < 650
          ? constraints.maxWidth
          : constraints.maxWidth < 1050
              ? (constraints.maxWidth - 12) / 2
              : (constraints.maxWidth - 24) / 3;
      return Wrap(
        spacing: 12,
        runSpacing: 12,
        children: [for (final module in data) SizedBox(width: width, child: moduleCard(module))],
      );
    });

    return Content(
      eyebrow: 'MODULE CATALOG',
      title: 'Module Marketplace',
      subtitle: charityApproved
          ? 'Your Charity access is approved. Choose the published and ready HIMATE modules that support your organization; Charity access has no module-count limit.'
          : hasManagedPlan
              ? 'Explore the complete HIMATE module catalog. Your current plan modules are available now; other modules remain visible so you can see what higher plans and future releases can add.'
              : 'Explore the HIMATE module catalog. Live activation remains subject to publication, readiness, commercial and dependency rules.',
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        if (charityApproved) ...[
          _MessageCard(
            icon: Icons.volunteer_activism_outlined,
            title: 'Charity access approved',
            message: 'Recurring service is complimentary. Choose any number of published and ready modules that fit your organization.',
          ),
          const SizedBox(height: 12),
          if (can('modules.write'))
            Align(
              alignment: Alignment.centerLeft,
              child: FilledButton.icon(
                onPressed: manageCharityModules,
                icon: const Icon(Icons.checklist_rounded),
                label: const LText('Choose Charity modules'),
              ),
            ),
          const SizedBox(height: 20),
        ],
        ResponsiveKpiGrid(children: [
          Kpi(label: 'Catalog modules', value: modules.length.toString(), note: 'Visible across HIMATE', icon: Icons.grid_view_rounded, accent: brandNavy),
          Kpi(label: 'Included', value: included.length.toString(), note: 'Available in your current access', icon: Icons.check_circle_outline_rounded, accent: brandSuccess),
          Kpi(label: 'Locked', value: locked.length.toString(), note: 'Visible for plan discovery', icon: Icons.lock_outline_rounded, accent: brandGold),
          Kpi(label: 'Coming soon', value: comingSoon.length.toString(), note: 'Canonical modules not live yet', icon: Icons.hourglass_top_rounded, accent: brandSteel),
        ]),
        const SizedBox(height: 26),
        _SectionHeader(
          title: 'Included in your plan',
          subtitle: 'Modules currently available to your organization.',
          trailing: _MiniCounter(label: '${included.length} included'),
        ),
        const SizedBox(height: 12),
        if (included.isEmpty)
          const _MessageCard(
            icon: Icons.extension_off_outlined,
            title: 'No live modules in the current plan',
            message: 'The complete module catalog remains visible below.',
          )
        else
          grid(included),
        const SizedBox(height: 28),
        _SectionHeader(
          title: 'Explore more modules',
          subtitle: 'Locked modules stay visible so you can understand what another subscription plan can add.',
          trailing: _MiniCounter(label: '${locked.length} locked'),
        ),
        const SizedBox(height: 12),
        if (locked.isEmpty)
          const _MessageCard(
            icon: Icons.check_circle_outline_rounded,
            title: 'No additional live modules are locked',
            message: 'Your current subscription already covers every live module available to you.',
          )
        else
          grid(locked),
        if (comingSoon.isNotEmpty) ...[
          const SizedBox(height: 28),
          _SectionHeader(
            title: 'Coming soon',
            subtitle: 'Canonical HIMATE modules that are discoverable now but are not yet released for live execution.',
            trailing: _MiniCounter(label: '${comingSoon.length} listed'),
          ),
          const SizedBox(height: 12),
          grid(comingSoon),
        ],
        if (unavailable.isNotEmpty) ...[
          const SizedBox(height: 28),
          _SectionHeader(
            title: 'Temporarily unavailable',
            subtitle: 'Modules currently restricted by operational availability.',
            trailing: _MiniCounter(label: '${unavailable.length} listed'),
          ),
          const SizedBox(height: 12),
          grid(unavailable),
        ],
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
      title: 'Billing & Subscription',
      subtitle: charityApproved
          ? 'Charity access is approved. Recurring service is complimentary and module access is managed through your approved Charity selection.'
          : 'Choose monthly or annual billing, manage your subscription plan and review provider-backed invoice history.',
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        _InfoCard(
          title: 'Charity status',
          icon: Icons.volunteer_activism_outlined,
          children: [
            _DefinitionRow(label: 'Review status', value: _humanize(charityStatus), emphasis: charityApproved),
            _DefinitionRow(label: 'Billing mode', value: _humanize('${charity['billing_mode'] ?? 'PAID'}')),
            if (charityApproved)
              const _DefinitionRow(label: 'Recurring charge', value: '\$0 · no Charity invoice'),
          ],
          action: charityApproved
              ? (can('modules.write')
                    ? OutlinedButton.icon(
                        onPressed: manageCharityModules,
                        icon: const Icon(Icons.checklist_rounded),
                        label: const LText('Manage modules'),
                      )
                    : null)
              : ((charityStatus == 'PENDING' || !can('modules.write'))
                    ? null
                    : OutlinedButton.icon(
                        onPressed: requestCharityReview,
                        icon: const Icon(Icons.volunteer_activism_outlined),
                        label: const LText('Request Charity review'),
                      )),
        ),
        const SizedBox(height: 20),
        if (charityStatus == 'PENDING') ...[
          const _MessageCard(
            icon: Icons.hourglass_top_rounded,
            title: 'Charity review pending',
            message: 'HIMATE must approve Charity eligibility before zero-dollar Charity access and unrestricted module selection become available.',
          ),
          const SizedBox(height: 20),
        ],
        if (plans.isNotEmpty && !charityApproved) ...[
          LayoutBuilder(builder: (context, constraints) {
            if (constraints.maxWidth < 760) {
              return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                const _SectionHeader(
                  title: 'Subscription Plans',
                  subtitle: 'Annual pricing shows the full list price and your discounted annual charge.',
                ),
                const SizedBox(height: 10),
                SegmentedButton<String>(
                  segments: const [
                    ButtonSegment(value: 'MONTHLY', label: LText('Monthly')),
                    ButtonSegment(value: 'ANNUAL', label: LText('Annual')),
                  ],
                  selected: {planBillingFrequency},
                  onSelectionChanged: (value) => setState(() => planBillingFrequency = value.first),
                ),
              ]);
            }
            return Row(children: [
              const Expanded(
                child: _SectionHeader(
                  title: 'Subscription Plans',
                  subtitle: 'Annual pricing shows the full list price and your discounted annual charge.',
                ),
              ),
              const SizedBox(width: 12),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'MONTHLY', label: LText('Monthly')),
                  ButtonSegment(value: 'ANNUAL', label: LText('Annual')),
                ],
                selected: {planBillingFrequency},
                onSelectionChanged: (value) => setState(() => planBillingFrequency = value.first),
              ),
            ]);
          }),
          const SizedBox(height: 12),
          LayoutBuilder(builder: (context, constraints) {
            final width = constraints.maxWidth < 680
                ? constraints.maxWidth
                : constraints.maxWidth < 1120
                    ? (constraints.maxWidth - 12) / 2
                    : (constraints.maxWidth - 24) / 3;
            return Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                for (final item in plans)
                  SizedBox(
                    width: width,
                    child: Card(
                      child: Padding(
                        padding: const EdgeInsets.all(18),
                        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Row(children: [
                            Expanded(
                              child: LText(
                                '${item['display_name']}',
                                style: const TextStyle(color: brandNavy, fontSize: 17, fontWeight: FontWeight.w800),
                              ),
                            ),
                            if (currentPlanKey == '${item['plan_key']}') const _StatusPill(label: 'CURRENT'),
                          ]),
                          const SizedBox(height: 10),
                          if (planBillingFrequency == 'MONTHLY')
                            LText(
                              '${planMoney(item['monthly_price'])} / month',
                              style: const TextStyle(color: brandGold, fontSize: 18, fontWeight: FontWeight.w800),
                            )
                          else ...[
                            if (number(item['annual_list_price']) > number(item['annual_price']))
                              Text(
                                planMoney(item['annual_list_price']),
                                style: const TextStyle(
                                  color: brandTextSoft,
                                  decoration: TextDecoration.lineThrough,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            LText(
                              '${planMoney(item['annual_price'])} / year',
                              style: const TextStyle(color: brandGold, fontSize: 18, fontWeight: FontWeight.w800),
                            ),
                            if (number(item['annual_savings']) > 0)
                              LText(
                                'Save ${planMoney(item['annual_savings'])}',
                                style: const TextStyle(color: brandSuccess, fontSize: 10.5, fontWeight: FontWeight.w700),
                              ),
                          ],
                          const SizedBox(height: 12),
                          _DefinitionRow(label: 'Modules', value: '${item['module_limit']}'),
                          _DefinitionRow(
                            label: 'Choice',
                            value: item['selection_mode'] == 'SELECTABLE'
                                ? 'Choose your modules'
                                : 'Fixed HIMATE package',
                          ),
                          const SizedBox(height: 14),
                          SizedBox(
                            width: double.infinity,
                            child: currentPlanKey == '${item['plan_key']}'
                                ? OutlinedButton(
                                    onPressed: currentPlanKey == 'FLEX' && can('modules.write')
                                        ? manageFlexModules
                                        : null,
                                    child: LText(currentPlanKey == 'FLEX' ? 'Manage Flex modules' : 'Current plan'),
                                  )
                                : FilledButton(
                                    onPressed: can('modules.write') && item['ready'] == true
                                        ? () => selectSubscriptionPlan(item)
                                        : null,
                                    child: const LText('Choose plan'),
                                  ),
                          ),
                        ]),
                      ),
                    ),
                  ),
              ],
            );
          }),
          const SizedBox(height: 24),
        ],
        ResponsiveKpiGrid(children: [
          Kpi(
            label: 'Current plan',
            value: hasManagedPlan ? '${plan['display_name'] ?? plan['plan_key']}' : 'Legacy',
            note: '${plan['billing_frequency'] ?? '—'}',
            icon: Icons.workspace_premium_outlined,
            accent: brandNavy,
          ),
          Kpi(
            label: 'Current charge',
            value: money(billing['current_total']),
            note: hasManagedPlan ? 'Plan price' : 'Legacy commercial total',
            icon: Icons.payments_outlined,
            accent: brandGold,
          ),
          Kpi(
            label: 'Module access',
            value: hasManagedPlan
                ? '${plan['module_limit'] ?? 0}'
                : '${modules.where((m) => m['status'] == 'ACTIVE').length}',
            note: hasManagedPlan ? '${plan['selection_mode'] ?? ''}' : 'Active modules',
            icon: Icons.extension_outlined,
            accent: brandSteel,
          ),
          Kpi(
            label: 'Next billing date',
            value: '${billing['next_billing_date'] ?? '—'}',
            note: hasManagedPlan ? '${plan['billing_frequency'] ?? ''}' : 'Legacy cycle',
            icon: Icons.calendar_month_outlined,
            accent: brandSuccess,
          ),
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
      subtitle: 'Manage organization roles and per-user module access. Effective module access is always limited by the partner subscription.',
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
                      action: can('users.write')
                          ? Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                IconButton(
                                  tooltip: uiLiteral('Manage module access'),
                                  onPressed: () => manageUserModuleAccess(user),
                                  icon: const Icon(Icons.extension_outlined),
                                ),
                                IconButton(
                                  tooltip: uiLiteral('Edit user'),
                                  onPressed: () => editUser(user),
                                  icon: const Icon(Icons.edit_outlined),
                                ),
                              ],
                            )
                          : null,
                      children: [
                        _DefinitionRow(label: 'Email', value: '${user['email']}'),
                        _DefinitionRow(label: 'Role', value: _humanize('${user['role'] ?? 'viewer'}')),
                        _DefinitionRow(label: 'Status', value: user['active'] == true ? 'Active' : 'Inactive'),
                        _DefinitionRow(
                          label: 'Module access',
                          value: '${user['module_access_mode']}' == 'SELECTED'
                              ? '${user['selected_module_count'] ?? 0} selected'
                              : 'All partner-owned modules',
                        ),
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
    final sidebar = workspaceSidebarColor;
    final foreground = workspaceReadableForeground(sidebar);
    return Container(
      width: 238,
      color: sidebar,
      padding: const EdgeInsets.fromLTRB(16, 22, 16, 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: workspaceLogo(height: 42),
          ),
          const SizedBox(height: 10),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: LText(
              workspaceDisplayName,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: foreground, fontSize: 12.5, fontWeight: FontWeight.w800),
            ),
          ),
          const SizedBox(height: 4),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: LText(
              'POWERED BY HIMATE',
              style: TextStyle(color: workspaceAccentColor, fontSize: 8, letterSpacing: 1.2, fontWeight: FontWeight.w800),
            ),
          ),
          const SizedBox(height: 22),
          for (var i = 0; i < items.length; i++)
            Padding(
              padding: const EdgeInsets.only(bottom: 5),
              child: ListTile(
                selected: selected == i,
                selectedColor: foreground,
                textColor: foreground.withOpacity(.78),
                iconColor: foreground.withOpacity(.78),
                selectedTileColor: foreground.withOpacity(.10),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(9)),
                leading: Icon(items[i].icon, size: 19),
                title: LText(items[i].label, style: const TextStyle(fontSize: 11.5, fontWeight: FontWeight.w700)),
                onTap: () => setState(() => selected = i),
              ),
            ),
          const Spacer(),
          Divider(color: foreground.withOpacity(.18)),
          ListTile(
            textColor: foreground.withOpacity(.82),
            iconColor: workspaceAccentColor,
            leading: const Icon(Icons.person_outline_rounded, size: 19),
            title: LText(
              '${widget.user['name'] ?? ''}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 10.5, fontWeight: FontWeight.w700),
            ),
            subtitle: LText(
              _humanize('${widget.user['role'] ?? 'viewer'}'),
              style: TextStyle(color: foreground.withOpacity(.58), fontSize: 8.5),
            ),
          ),
          TextButton.icon(
            onPressed: widget.onLogout,
            icon: Icon(Icons.logout_rounded, size: 18, color: workspaceAccentColor),
            label: LText('Sign out', style: TextStyle(color: foreground)),
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
    final background = workspaceBackgroundColor;
    final primary = workspacePrimaryColor;
    final sidebar = workspaceSidebarColor;
    final sidebarForeground = workspaceReadableForeground(sidebar);

    if (loading && company.isEmpty) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (error != null && company.isEmpty) {
      return Scaffold(
        backgroundColor: background,
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
            backgroundColor: background,
            body: Row(children: [
              navPanel(items),
              Expanded(
                child: Column(children: [
                  Container(
                    height: 64,
                    padding: const EdgeInsets.symmetric(horizontal: 24),
                    decoration: BoxDecoration(
                      color: brandWhite,
                      border: Border(bottom: BorderSide(color: primary.withOpacity(.12))),
                    ),
                    child: Row(children: [
                      Expanded(
                        child: LText(
                          workspaceDisplayName,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(color: primary, fontWeight: FontWeight.w800, fontSize: 13),
                        ),
                      ),
                      const SizedBox(width: 12),
                      if (can('notifications.read'))
                        NotificationCenterButton(
                          api: widget.api,
                          endpointPrefix: '/partner/api/v1/notifications',
                          panelSubtitle: 'Organization events that match your role and module access.',
                          iconColor: primary,
                        ),
                      IconButton(onPressed: load, tooltip: 'Refresh', icon: Icon(Icons.refresh_rounded, color: primary)),
                    ]),
                  ),
                  Expanded(
                    child: ColoredBox(
                      color: background,
                      child: portalPageWithLoadStatus(page),
                    ),
                  ),
                ]),
              ),
            ]),
          );
        }

        return Scaffold(
          backgroundColor: background,
          appBar: AppBar(
            backgroundColor: brandWhite,
            foregroundColor: primary,
            title: LText(workspaceDisplayName),
            actions: [
              if (can('notifications.read'))
                NotificationCenterButton(
                  api: widget.api,
                  endpointPrefix: '/partner/api/v1/notifications',
                  panelSubtitle: 'Organization events that match your role and module access.',
                  iconColor: primary,
                ),
              IconButton(onPressed: load, icon: const Icon(Icons.refresh_rounded)),
              IconButton(onPressed: widget.onLogout, icon: const Icon(Icons.logout_rounded)),
            ],
          ),
          drawer: Drawer(
            backgroundColor: sidebar,
            child: SafeArea(
              child: Column(children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(22, 22, 22, 8),
                  child: Align(alignment: Alignment.centerLeft, child: workspaceLogo(height: 42)),
                ),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 22),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: LText(
                      workspaceDisplayName,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: sidebarForeground, fontWeight: FontWeight.w800),
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.fromLTRB(22, 4, 22, 14),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: LText(
                      'POWERED BY HIMATE',
                      style: TextStyle(color: workspaceAccentColor, fontSize: 8, letterSpacing: 1.2, fontWeight: FontWeight.w800),
                    ),
                  ),
                ),
                Expanded(
                  child: ListView.builder(
                    itemCount: items.length,
                    itemBuilder: (context, i) => ListTile(
                      selected: selected == i,
                      selectedTileColor: sidebarForeground.withOpacity(.10),
                      textColor: sidebarForeground.withOpacity(.82),
                      iconColor: workspaceAccentColor,
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
          body: ColoredBox(color: background, child: portalPageWithLoadStatus(page)),
        );
      },
    );
  }
}
