import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/browser_client.dart';
import 'package:http/http.dart' as http;

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const HimateApp());
}

const navy = Color(0xFF071B33);
const gold = Color(0xFFD5A23F);
const canvas = Color(0xFFF5F7FA);
const muted = Color(0xFF667085);
const success = Color(0xFF18794E);

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
  void initState() {
    super.initState();
    restore();
  }

  Future<void> restore() async {
    try {
      user = await api.get('/api/v1/auth/me');
    } on ApiError catch (e) {
      if (e.status != 401) rethrow;
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> login(String email, String password) async {
    user = await api.post('/api/v1/auth/login', {'email': email, 'password': password});
    if (mounted) setState(() {});
  }

  Future<void> logout() async {
    await api.post('/api/v1/auth/logout');
    user = null;
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final theme = ThemeData(
      useMaterial3: true,
      scaffoldBackgroundColor: canvas,
      colorScheme: ColorScheme.fromSeed(seedColor: navy, primary: navy, secondary: gold),
      inputDecorationTheme: const InputDecorationTheme(border: OutlineInputBorder(), filled: true, fillColor: Colors.white),
      cardTheme: const CardThemeData(color: Colors.white, elevation: 0, shape: RoundedRectangleBorder(borderRadius: BorderRadius.all(Radius.circular(18)), side: BorderSide(color: Color(0xFFE2E8F0)))),
    );
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'HIMATE System',
      theme: theme,
      home: loading
          ? const Scaffold(body: Center(child: CircularProgressIndicator()))
          : user == null
              ? LoginPage(onLogin: login)
              : Shell(api: api, user: user!, onLogout: logout),
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
  String? error;

  @override
  void dispose() {
    email.dispose();
    password.dispose();
    super.dispose();
  }

  Future<void> submit() async {
    if (email.text.trim().isEmpty || password.text.isEmpty) return;
    setState(() { busy = true; error = null; });
    try {
      await widget.onLogin(email.text.trim(), password.text);
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          if (MediaQuery.sizeOf(context).width >= 900)
            Expanded(
              child: Container(
                color: navy,
                padding: const EdgeInsets.all(56),
                child: const Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('HIMATE', style: TextStyle(color: Colors.white, fontSize: 30, fontWeight: FontWeight.w800, letterSpacing: 3)),
                    Text('SYSTEM', style: TextStyle(color: gold, letterSpacing: 5, fontWeight: FontWeight.w700)),
                    Spacer(),
                    Text('Art Drives a Better Tomorrow', style: TextStyle(color: Colors.white, fontSize: 42, height: 1.08, fontWeight: FontWeight.w700)),
                    SizedBox(height: 20),
                    Text('Central partner, module and commercial control for the HIMATE platform.', style: TextStyle(color: Color(0xFFD5DEEA), fontSize: 17, height: 1.5)),
                    SizedBox(height: 36),
                    Text('START 04–08 · MICROSERVICE CONTROL PLANE', style: TextStyle(color: gold, letterSpacing: 1.8, fontWeight: FontWeight.w700)),
                  ],
                ),
              ),
            ),
          Expanded(
            child: Center(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(28),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 430),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text('Administrator sign in', style: Theme.of(context).textTheme.headlineMedium?.copyWith(fontWeight: FontWeight.w700)),
                      const SizedBox(height: 8),
                      const Text('Use the HIMATE administrator credentials configured in Render.', style: TextStyle(color: muted)),
                      const SizedBox(height: 28),
                      TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Email', prefixIcon: Icon(Icons.alternate_email))),
                      const SizedBox(height: 14),
                      TextField(controller: password, obscureText: obscure, onSubmitted: (_) => submit(), decoration: InputDecoration(labelText: 'Password', prefixIcon: const Icon(Icons.lock_outline), suffixIcon: IconButton(onPressed: () => setState(() => obscure = !obscure), icon: Icon(obscure ? Icons.visibility : Icons.visibility_off)))),
                      if (error != null) ...[const SizedBox(height: 12), Text(error!, style: const TextStyle(color: Colors.red))],
                      const SizedBox(height: 20),
                      FilledButton(onPressed: busy ? null : submit, style: FilledButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 18)), child: busy ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)) : const Text('Sign in')),
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
  static const labels = ['Dashboard', 'Partners', 'Licensing & Finance', 'System & Operations'];
  static const icons = [Icons.dashboard_outlined, Icons.business_outlined, Icons.account_balance_wallet_outlined, Icons.settings_suggest_outlined];

  Widget page() {
    if (selected == 0) return DashboardPage(api: widget.api);
    if (selected == 1) return PartnersPage(api: widget.api);
    if (selected == 2) return FinancePage(api: widget.api);
    return SystemPage(api: widget.api);
  }

  @override
  Widget build(BuildContext context) {
    final wide = MediaQuery.sizeOf(context).width >= 950;
    if (!wide) {
      return Scaffold(
        appBar: AppBar(title: Text(labels[selected]), actions: [IconButton(onPressed: widget.onLogout, icon: const Icon(Icons.logout))]),
        drawer: Drawer(
          child: ListView(
            padding: EdgeInsets.zero,
            children: [
              const DrawerHeader(decoration: BoxDecoration(color: navy), child: Align(alignment: Alignment.bottomLeft, child: Text('HIMATE SYSTEM', style: TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 20)))),
              for (var i = 0; i < labels.length; i++) ListTile(leading: Icon(icons[i]), title: Text(labels[i]), selected: i == selected, onTap: () { setState(() => selected = i); Navigator.pop(context); }),
            ],
          ),
        ),
        body: page(),
      );
    }
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            extended: MediaQuery.sizeOf(context).width >= 1220,
            minExtendedWidth: 260,
            backgroundColor: navy,
            selectedIndex: selected,
            onDestinationSelected: (i) => setState(() => selected = i),
            selectedIconTheme: const IconThemeData(color: gold),
            unselectedIconTheme: const IconThemeData(color: Colors.white60),
            selectedLabelTextStyle: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700),
            unselectedLabelTextStyle: const TextStyle(color: Colors.white70),
            leading: const Padding(padding: EdgeInsets.symmetric(vertical: 24), child: Text('HIMATE', style: TextStyle(color: Colors.white, fontWeight: FontWeight.w800, letterSpacing: 2))),
            trailing: Expanded(child: Align(alignment: Alignment.bottomCenter, child: Padding(padding: const EdgeInsets.only(bottom: 20), child: IconButton(onPressed: widget.onLogout, icon: const Icon(Icons.logout), color: Colors.white70)))),
            destinations: [for (var i = 0; i < labels.length; i++) NavigationRailDestination(icon: Icon(icons[i]), label: Text(labels[i]))],
          ),
          Expanded(child: page()),
        ],
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
