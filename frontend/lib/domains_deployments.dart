part of 'main.dart';

class DomainsDeploymentsPanel extends StatefulWidget {
  const DomainsDeploymentsPanel({
    required this.api,
    required this.initialEnvironments,
    super.key,
  });

  final Api api;
  final List<Map<String, dynamic>> initialEnvironments;

  @override
  State<DomainsDeploymentsPanel> createState() => _DomainsDeploymentsPanelState();
}

class _DomainsDeploymentsPanelState extends State<DomainsDeploymentsPanel> {
  late List<Map<String, dynamic>> environments;
  final Set<String> busy = <String>{};

  @override
  void initState() {
    super.initState();
    environments = [for (final e in widget.initialEnvironments) Map<String, dynamic>.from(e)];
  }

  @override
  void didUpdateWidget(covariant DomainsDeploymentsPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (busy.isEmpty && oldWidget.initialEnvironments != widget.initialEnvironments) {
      environments = [for (final e in widget.initialEnvironments) Map<String, dynamic>.from(e)];
    }
  }

  void _notify(String message, {bool failure = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: LText(message),
        behavior: SnackBarBehavior.floating,
        backgroundColor: failure ? brandDanger : brandSuccess,
      ),
    );
  }

  void _replace(Map<String, dynamic> updated) {
    final id = '${updated['id'] ?? ''}';
    setState(() {
      final index = environments.indexWhere((e) => '${e['id'] ?? ''}' == id);
      if (index >= 0) {
        environments[index] = Map<String, dynamic>.from(updated);
      } else {
        environments.add(Map<String, dynamic>.from(updated));
      }
      environments.sort((a, b) {
        final partner = '${a['partner_id']}'.compareTo('${b['partner_id']}');
        if (partner != 0) return partner;
        return '${a['kind']}'.compareTo('${b['kind']}');
      });
    });
  }

  Future<void> _action(
    Map<String, dynamic> environment,
    String suffix,
    String successMessage, {
    Map<String, dynamic> body = const <String, dynamic>{},
  }) async {
    final id = '${environment['id'] ?? ''}';
    if (id.isEmpty || busy.contains(id)) return;
    setState(() => busy.add(id));
    try {
      final updated = await widget.api.post('/api/v1/environments/$id/$suffix', body);
      if (!mounted) return;
      _replace(updated);
      _notify(successMessage);
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(id));
    }
  }

  List<String> _blockers(Map<String, dynamic> environment) {
    final raw = environment['launch_blockers'];
    if (raw is! List) return const <String>[];
    return raw.map((e) => e.toString()).where((e) => e.isNotEmpty).toList();
  }

  String _value(dynamic value, {String fallback = '—'}) {
    final v = value?.toString().trim() ?? '';
    return v.isEmpty ? fallback : v;
  }

  String _date(dynamic value) {
    final raw = value?.toString() ?? '';
    final parsed = DateTime.tryParse(raw)?.toLocal();
    if (parsed == null) return raw.isEmpty ? '—' : raw;
    String two(int n) => n.toString().padLeft(2, '0');
    return '${parsed.year}-${two(parsed.month)}-${two(parsed.day)} ${two(parsed.hour)}:${two(parsed.minute)}';
  }

  Future<void> _createProduction() async {
    final staging = environments.where((e) => '${e['kind']}' == 'STAGING').toList();
    final partnerIds = staging.map((e) => '${e['partner_id']}').where((e) => e.isNotEmpty).toSet().toList()..sort();
    if (partnerIds.isEmpty) {
      _notify('Create or provision a staging environment before production.', failure: true);
      return;
    }

    String partnerId = partnerIds.first;
    final hostname = TextEditingController();
    final release = TextEditingController();
    final version = TextEditingController();

    void hydrate(String id) {
      final source = staging.firstWhere(
        (e) => '${e['partner_id']}' == id,
        orElse: () => <String, dynamic>{},
      );
      release.text = _value(source['active_release'], fallback: _value(source['desired_release'], fallback: ''));
      version.text = _value(source['platform_version'], fallback: '');
    }
    hydrate(partnerId);

    String? dialogError;
    final payload = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: 'Create production environment',
          subtitle: 'Production requires an explicit public hostname. DNS and TLS must verify before launch.',
          icon: Icons.public_rounded,
          primaryLabel: 'Create production',
          onPrimary: () {
            final host = hostname.text.trim().toLowerCase();
            if (host.isEmpty || !host.contains('.') || host.contains('://')) {
              setDialogState(() => dialogError = 'Enter a public DNS hostname, for example partner.example.com.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              'partner_id': partnerId,
              'kind': 'PRODUCTION',
              'hostname': host,
              'platform_version': version.text.trim(),
              'desired_release': release.text.trim(),
              'config': <String, dynamic>{},
            });
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                value: partnerId,
                decoration: InputDecoration(labelText: uiLiteral('Partner')),
                items: [
                  for (final id in partnerIds) DropdownMenuItem(value: id, child: LText(id)),
                ],
                onChanged: (value) {
                  if (value == null) return;
                  setDialogState(() {
                    partnerId = value;
                    hydrate(value);
                    dialogError = null;
                  });
                },
              ),
              const SizedBox(height: 12),
              TextField(
                controller: hostname,
                decoration: InputDecoration(
                  labelText: uiLiteral('Production hostname *'),
                  hintText: uiLiteral('partner.example.com'),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(
                  controller: version,
                  decoration: InputDecoration(labelText: uiLiteral('Platform version')),
                ),
                second: TextField(
                  controller: release,
                  decoration: InputDecoration(labelText: uiLiteral('Desired release')),
                ),
              ),
              if (dialogError != null) ...[
                const SizedBox(height: 10),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
        ),
      ),
    );

    hostname.dispose();
    release.dispose();
    version.dispose();
    if (payload == null) return;

    try {
      final created = await widget.api.post('/api/v1/environments', payload);
      _replace(created);
      _notify('Production environment created.');
    } catch (e) {
      _notify(e.toString(), failure: true);
    }
  }

  Future<void> _editEnvironment(Map<String, dynamic> environment) async {
    final hostname = TextEditingController(text: _value(environment['hostname'], fallback: ''));
    final release = TextEditingController(text: _value(environment['desired_release'], fallback: ''));
    final version = TextEditingController(text: _value(environment['platform_version'], fallback: ''));
    String? dialogError;

    final payload = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: 'Environment configuration',
          subtitle: 'Changing the production hostname resets DNS/TLS verification and launch readiness.',
          icon: Icons.tune_rounded,
          primaryLabel: 'Save configuration',
          onPrimary: () {
            final host = hostname.text.trim().toLowerCase();
            if (host.isEmpty) {
              setDialogState(() => dialogError = 'Hostname is required.');
              return;
            }
            if ('${environment['kind']}' == 'PRODUCTION' && (!host.contains('.') || host.contains('://'))) {
              setDialogState(() => dialogError = 'Production requires a public DNS hostname.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              'hostname': host,
              'platform_version': version.text.trim(),
              'desired_release': release.text.trim(),
            });
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              TextField(controller: hostname, decoration: InputDecoration(labelText: uiLiteral('Hostname'))),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: version, decoration: InputDecoration(labelText: uiLiteral('Platform version'))),
                second: TextField(controller: release, decoration: InputDecoration(labelText: uiLiteral('Desired release'))),
              ),
              if (dialogError != null) ...[
                const SizedBox(height: 10),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
        ),
      ),
    );

    hostname.dispose();
    release.dispose();
    version.dispose();
    if (payload == null) return;

    final id = '${environment['id'] ?? ''}';
    setState(() => busy.add(id));
    try {
      final updated = await widget.api.patch('/api/v1/environments/$id', payload);
      _replace(updated);
      _notify('Environment configuration updated.');
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(id));
    }
  }

  Future<void> _launch(Map<String, dynamic> environment) async {
    final blockers = _blockers(environment);
    if (blockers.isNotEmpty) {
      _notify('Launch blocked: ${blockers.join('; ')}', failure: true);
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'Launch production',
        subtitle: 'This changes the environment from READY FOR LAUNCH to LIVE and records the launch actor.',
        icon: Icons.rocket_launch_outlined,
        primaryLabel: 'Go LIVE',
        onPrimary: () => Navigator.pop(dialogContext, true),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _DefinitionRow(label: 'Partner', value: _value(environment['partner_id'])),
            _DefinitionRow(label: 'Hostname', value: _value(environment['hostname'])),
            _DefinitionRow(label: 'Release', value: _value(environment['active_release'])),
            const SizedBox(height: 10),
            const LText(
              'The backend will re-check runtime health and all launch gates before committing LIVE.',
              style: TextStyle(color: brandTextSoft, fontSize: 11, height: 1.45),
            ),
          ],
        ),
      ),
    );
    if (confirmed == true) {
      await _action(environment, 'launch', 'Production is LIVE.');
    }
  }

  Widget _environmentCard(Map<String, dynamic> e) {
    final id = '${e['id'] ?? ''}';
    final kind = '${e['kind'] ?? 'UNKNOWN'}';
    final production = kind == 'PRODUCTION';
    final isBusy = busy.contains(id);
    final isLive = '${e['environment_status']}' == 'LIVE';
    final launchReady = e['launch_ready'] == true;
    final blockers = _blockers(e);

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
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: production ? brandGold.withOpacity(.12) : brandSteel.withOpacity(.10),
                    borderRadius: BorderRadius.circular(11),
                  ),
                  child: Icon(production ? Icons.public_rounded : Icons.science_outlined, color: production ? brandGold : brandSteel),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      LText(
                        '${e['partner_id'] ?? 'Partner'} · $kind',
                        style: const TextStyle(color: brandNavy, fontSize: 14, fontWeight: FontWeight.w700),
                      ),
                      const SizedBox(height: 3),
                      SelectableText(_value(e['hostname']), style: const TextStyle(color: brandTextSoft, fontSize: 10.5)),
                    ],
                  ),
                ),
                _StatusPill(label: '${e['environment_status'] ?? 'UNKNOWN'}'),
              ],
            ),
            const SizedBox(height: 14),
            const Divider(height: 1),
            const SizedBox(height: 7),
            _DefinitionRow(label: 'Deployment', value: _value(e['deployment_status'], fallback: 'UNKNOWN')),
            _DefinitionRow(label: 'Runtime', value: '${_value(e['runtime_status'], fallback: 'UNKNOWN')} · ${e['runtime_latency_ms'] ?? 0} ms'),
            _DefinitionRow(label: 'Desired release', value: _value(e['desired_release'])),
            _DefinitionRow(label: 'Active release', value: _value(e['active_release'])),
            _DefinitionRow(label: 'Platform version', value: _value(e['platform_version'])),
            if (production) ...[
              const SizedBox(height: 5),
              _DefinitionRow(label: 'DNS', value: _value(e['dns_status'], fallback: 'UNKNOWN')),
              _DefinitionRow(label: 'TLS', value: _value(e['tls_status'], fallback: 'UNKNOWN')),
              _DefinitionRow(label: 'Domain', value: _value(e['domain_status'], fallback: 'UNVERIFIED')),
              _DefinitionRow(label: 'Domain checked', value: _date(e['last_domain_check'])),
              if (_value(e['domain_error'], fallback: '').isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 5),
                  child: LText(
                    _value(e['domain_error'], fallback: ''),
                    style: const TextStyle(color: brandDanger, fontSize: 9.5, height: 1.35),
                  ),
                ),
              if (blockers.isNotEmpty && !isLive) ...[
                const SizedBox(height: 8),
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: brandWarning.withOpacity(.06),
                    border: Border.all(color: brandWarning.withOpacity(.18)),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: LText(
                    'Launch blockers: ${blockers.join(' · ')}',
                    style: const TextStyle(color: brandWarning, fontSize: 9.4, height: 1.4, fontWeight: FontWeight.w600),
                  ),
                ),
              ],
              if (isLive) ...[
                _DefinitionRow(label: 'Launched by', value: _value(e['launch_actor'])),
                _DefinitionRow(label: 'Launched at', value: _date(e['launched_at'])),
              ],
            ],
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: isBusy ? null : () => _editEnvironment(e),
                  icon: const Icon(Icons.tune_rounded, size: 17),
                  label: const LText('Configure'),
                ),
                if (production)
                  OutlinedButton.icon(
                    onPressed: isBusy ? null : () => _action(e, 'verify-domain', 'Domain verification completed.'),
                    icon: const Icon(Icons.verified_outlined, size: 17),
                    label: const LText('Verify DNS/TLS'),
                  ),
                FilledButton.icon(
                  onPressed: isBusy ? null : () => _action(e, 'deploy', production ? 'Production deployment completed.' : 'Staging deployment completed.'),
                  icon: const Icon(Icons.cloud_upload_outlined, size: 17),
                  label: LText('${e['deployment_status']}' == 'DEPLOYED' ? 'Redeploy' : 'Deploy'),
                ),
                if (production && !isLive)
                  FilledButton.icon(
                    onPressed: isBusy || !launchReady ? null : () => _launch(e),
                    icon: const Icon(Icons.rocket_launch_outlined, size: 17),
                    label: const LText('Go LIVE'),
                  ),
              ],
            ),
            if (isBusy) ...[
              const SizedBox(height: 10),
              const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
            ],
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final production = environments.where((e) => '${e['kind']}' == 'PRODUCTION').length;
    final live = environments.where((e) => '${e['environment_status']}' == 'LIVE').length;
    final ready = environments.where((e) => e['launch_ready'] == true && '${e['environment_status']}' != 'LIVE').length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Domains & Deployments',
          subtitle: 'Staging and production release state, DNS/TLS verification and controlled READY FOR LAUNCH → LIVE transition.',
          trailing: FilledButton.icon(
            onPressed: _createProduction,
            icon: const Icon(Icons.add_rounded),
            label: const LText('Add production'),
          ),
        ),
        const SizedBox(height: 12),
        _RuleStrip(items: [
          _RuleItem(Icons.layers_outlined, 'Environments', '${environments.length} total'),
          _RuleItem(Icons.public_outlined, 'Production', '$production configured'),
          _RuleItem(Icons.rocket_launch_outlined, 'Launch ready', '$ready ready'),
          _RuleItem(Icons.language_outlined, 'Live', '$live live'),
        ]),
        const SizedBox(height: 14),
        if (environments.isEmpty)
          const _MessageCard(
            icon: Icons.dns_outlined,
            title: 'No partner environments',
            message: 'Provision a staging environment first. Production can then be configured here with an explicit public hostname.',
          )
        else
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 760
                  ? constraints.maxWidth
                  : constraints.maxWidth < 1180
                      ? (constraints.maxWidth - 12) / 2
                      : (constraints.maxWidth - 24) / 3;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final e in environments)
                    SizedBox(width: width, child: _environmentCard(e)),
                ],
              );
            },
          ),
      ],
    );
  }
}
