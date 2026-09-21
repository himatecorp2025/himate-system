part of 'main.dart';

extension Start223CommercialAutomationUI on _PartnerWorkspaceState {
  List<Map<String, dynamic>> _start223WorkflowItems() {
    final raw = commercialStatus?['workflow'];
    if (raw is! List) return <Map<String, dynamic>>[];
    return raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  List<String> _start223StringList(dynamic value) {
    if (value is! List) return <String>[];
    return value.map((e) => e.toString()).where((e) => e.trim().isNotEmpty).toList();
  }

  Future<void> start223EditAgreement() async {
    String status = '${agreement?['status'] ?? 'DRAFT'}'.toUpperCase();
    if (status != 'DRAFT' && status != 'AGREED') status = 'DRAFT';
    final reference = TextEditingController(text: '${agreement?['agreement_reference'] ?? ''}');
    final note = TextEditingController(text: '${agreement?['note'] ?? ''}');

    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Commercial Agreement',
          subtitle: 'Provisioning for a new partner requires an explicitly confirmed commercial agreement before payment verification can open the gate.',
          icon: Icons.handshake_outlined,
          width: 680,
          primaryLabel: 'Save agreement',
          onPrimary: () {
            if (status == 'AGREED' && reference.text.trim().isEmpty) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: LText('An AGREED commercial agreement requires a persistent agreement reference.'),
                  behavior: SnackBarBehavior.floating,
                  backgroundColor: brandWarning,
                ),
              );
              return;
            }
            Navigator.pop(dialogContext, true);
          },
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                value: status,
                decoration: InputDecoration(labelText: uiLiteral('Agreement status')),
                items: const [
                  DropdownMenuItem(value: 'DRAFT', child: LText('Draft')),
                  DropdownMenuItem(value: 'AGREED', child: LText('Agreed')),
                ],
                onChanged: (value) {
                  if (value != null) setLocal(() => status = value);
                },
              ),
              const SizedBox(height: 12),
              TextField(
                controller: reference,
                decoration: InputDecoration(
                  labelText: uiLiteral('Agreement reference'),
                  hintText: uiLiteral('Signed contract / persistent document reference'),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: note,
                maxLines: 3,
                decoration: InputDecoration(labelText: uiLiteral('Agreement note')),
              ),
              const SizedBox(height: 12),
              const _RuleStrip(
                items: [
                  _RuleItem(Icons.lock_outline_rounded, 'Provisioning gate', 'AGREED required for new partners'),
                  _RuleItem(Icons.history_rounded, 'Audit', 'Agreement transition is recorded as a billing event'),
                ],
              ),
            ],
          ),
        ),
      ),
    );

    if (ok == true) {
      try {
        await widget.api.put('/api/v1/billing/partners/${partner['id']}/agreement', {
          'status': status,
          'agreement_reference': reference.text.trim(),
          'note': note.text.trim(),
        });
        await _loadSupplementary();
        if (mounted) success('Commercial agreement updated.');
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: LText('Agreement could not be updated: $e'),
              behavior: SnackBarBehavior.floating,
              backgroundColor: brandDanger,
            ),
          );
        }
      }
    }

    reference.dispose();
    note.dispose();
  }

  Future<void> start223EditWebsiteAdapter() async {
    final current = websiteAdapter ?? <String, dynamic>{};
    String adapterType = '${current['adapter_type'] ?? 'GENERIC_HTTP'}'.toUpperCase();
    if (!const {'GENERIC_HTTP', 'WORDPRESS', 'CUSTOM_API'}.contains(adapterType)) {
      adapterType = 'GENERIC_HTTP';
    }
    final primaryDomain = '${partner['primary_domain'] ?? ''}'.trim();
    final currentBase = '${current['site_base_url'] ?? ''}'.trim();
    final baseUrl = TextEditingController(
      text: currentBase.isNotEmpty
          ? currentBase
          : primaryDomain.isEmpty
              ? ''
              : 'https://$primaryDomain',
    );
    final currentDomains = _start223StringList(current['allowed_domains']);
    final domains = TextEditingController(
      text: currentDomains.isNotEmpty ? currentDomains.join(', ') : primaryDomain,
    );
    bool enabled = current['enabled'] == true || current['configured'] != true;

    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: 'Partner Website Adapter',
          subtitle: 'Configure how the existing partner website connects to HIMATE. This does not replace or reprogram the partner website.',
          icon: Icons.hub_outlined,
          width: 760,
          primaryLabel: 'Save adapter',
          onPrimary: () {
            if (baseUrl.text.trim().isEmpty || domains.text.trim().isEmpty) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: LText('Website base URL and at least one allowed domain are required.'),
                  behavior: SnackBarBehavior.floating,
                  backgroundColor: brandWarning,
                ),
              );
              return;
            }
            Navigator.pop(dialogContext, true);
          },
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: adapterType,
                  decoration: InputDecoration(labelText: uiLiteral('Adapter type')),
                  items: const [
                    DropdownMenuItem(value: 'GENERIC_HTTP', child: LText('Generic HTTP / REST')),
                    DropdownMenuItem(value: 'WORDPRESS', child: LText('WordPress adapter')),
                    DropdownMenuItem(value: 'CUSTOM_API', child: LText('Custom API adapter')),
                  ],
                  onChanged: (value) {
                    if (value != null) setLocal(() => adapterType = value);
                  },
                ),
                second: SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  value: enabled,
                  title: const LText('Enabled'),
                  subtitle: const LText('Commercial-state access fails closed when disabled.'),
                  onChanged: (value) => setLocal(() => enabled = value),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: baseUrl,
                decoration: InputDecoration(
                  labelText: uiLiteral('Partner website base URL'),
                  hintText: uiLiteral('https://example.com'),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: domains,
                decoration: InputDecoration(
                  labelText: uiLiteral('Allowed domains'),
                  hintText: uiLiteral('example.com, www.example.com'),
                ),
              ),
              const SizedBox(height: 14),
              const _RuleStrip(
                items: [
                  _RuleItem(Icons.domain_verification_outlined, 'Domain binding', 'Primary partner domain must be allowed'),
                  _RuleItem(Icons.privacy_tip_outlined, 'Privacy', 'Aggregated/minimized data only'),
                  _RuleItem(Icons.key_outlined, 'Tenant scope', 'Connector credential binds the partner identity'),
                ],
              ),
              const SizedBox(height: 12),
              const _MessageCard(
                icon: Icons.code_off_outlined,
                title: 'No website rewrite',
                message: 'The partner website remains its own application. HIMATE exposes connector endpoints for entitlements, configuration, metrics, aggregated data and reconciliation.',
              ),
            ],
          ),
        ),
      ),
    );

    if (ok == true) {
      try {
        final domainValues = domains.text
            .split(',')
            .map((value) => value.trim())
            .where((value) => value.isNotEmpty)
            .toList();
        await widget.api.put('/api/v1/connectors/${partner['id']}/website-adapter', {
          'environment': 'PRODUCTION',
          'adapter_type': adapterType,
          'site_base_url': baseUrl.text.trim(),
          'allowed_domains': domainValues,
          'capabilities': const [
            'ENTITLEMENTS',
            'HEARTBEAT',
            'METRICS',
            'AGGREGATED_DATA',
            'RECONCILIATION',
          ],
          'enabled': enabled,
          'config': const {
            'partner_portal': '/partner/login',
            'commercial_state': '/connector/v1/commercial-state',
            'data_batches': '/connector/v1/data/batches',
            'metrics': '/connector/v1/metrics',
            'reconciliation': '/connector/v1/reconcile',
          },
        });
        await _loadSupplementary();
        if (mounted) success('Partner Website Adapter updated.');
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: LText('Website adapter could not be updated: $e'),
              behavior: SnackBarBehavior.floating,
              backgroundColor: brandDanger,
            ),
          );
        }
      }
    }

    baseUrl.dispose();
    domains.dispose();
  }

  Widget start223CommercialWorkflowPanel() {
    final workflow = _start223WorkflowItems();
    final status = commercialStatus ?? <String, dynamic>{};
    final activationFee = status['activation_fee'] is Map
        ? Map<String, dynamic>.from(status['activation_fee'] as Map)
        : <String, dynamic>{};
    final payment = status['payment'] is Map
        ? Map<String, dynamic>.from(status['payment'] as Map)
        : <String, dynamic>{};
    final evidence = status['evidence'] is Map
        ? Map<String, dynamic>.from(status['evidence'] as Map)
        : <String, dynamic>{};

    Widget workflowCard() => _InfoCard(
          title: 'Commercial Activation Workflow',
          icon: Icons.account_tree_outlined,
          action: OutlinedButton.icon(
            onPressed: start223EditAgreement,
            icon: const Icon(Icons.handshake_outlined, size: 17),
            label: const LText('Agreement'),
          ),
          children: [
            _DefinitionRow(label: 'Agreement', value: '${agreement?['status'] ?? 'DRAFT'}'),
            _DefinitionRow(label: 'Agreement reference', value: '${agreement?['agreement_reference'] ?? '—'}'),
            _DefinitionRow(
              label: 'Activation fee',
              value: '${activationFee['currency'] ?? terms?['currency'] ?? 'USD'} ${number(activationFee['required_amount'] ?? terms?['activation_fee']).toStringAsFixed(2)}',
            ),
            _DefinitionRow(
              label: 'Paid',
              value: '${activationFee['currency'] ?? terms?['currency'] ?? 'USD'} ${number(activationFee['paid_amount'] ?? license?['paid_amount']).toStringAsFixed(2)}',
            ),
            _DefinitionRow(label: 'Payment status', value: '${payment['status'] ?? license?['status'] ?? 'NOT_PAID'}'),
            _DefinitionRow(label: 'Commercial evidence', value: '${evidence['commercial_count'] ?? documents.length} record(s)'),
            _DefinitionRow(
              label: 'Provisioning gate',
              value: status['provisioning_allowed'] == true ? 'READY' : 'BLOCKED',
              emphasis: true,
            ),
            _DefinitionRow(label: 'Next action', value: _humanize('${status['next_action'] ?? 'CONFIRM_COMMERCIAL_AGREEMENT'}')),
            if (workflow.isNotEmpty) ...[
              const SizedBox(height: 10),
              Wrap(
                spacing: 7,
                runSpacing: 7,
                children: [
                  for (final step in workflow)
                    Chip(
                      avatar: Icon(
                        step['complete'] == true ? Icons.check_circle_rounded : Icons.radio_button_unchecked_rounded,
                        size: 16,
                        color: step['complete'] == true ? brandSuccess : brandTextSoft,
                      ),
                      label: LText(
                        _humanize('${step['key'] ?? ''}'),
                        style: TextStyle(
                          color: step['complete'] == true ? brandNavy : brandTextSoft,
                          fontSize: 9.2,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                ],
              ),
            ],
          ],
        );

    Widget eventsCard() => _InfoCard(
          title: 'Commercial Event Ledger',
          icon: Icons.history_toggle_off_rounded,
          children: billingEvents.isEmpty
              ? const [
                  LText(
                    'Commercial state changes will appear here as immutable billing events.',
                    style: TextStyle(color: brandTextSoft, fontSize: 10.2, height: 1.45),
                  ),
                ]
              : [
                  for (final event in billingEvents.take(7)) ...[
                    Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Icon(Icons.circle, size: 7, color: brandGold),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              LText(
                                _humanize('${event['event_type'] ?? ''}'),
                                style: const TextStyle(color: brandNavy, fontSize: 10.5, fontWeight: FontWeight.w700),
                              ),
                              const SizedBox(height: 2),
                              LText(
                                '${event['effective_at'] ?? ''}',
                                style: const TextStyle(color: brandTextSoft, fontSize: 8.8),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 9),
                  ],
                ],
        );

    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < 930) {
          return Column(children: [workflowCard(), const SizedBox(height: 14), eventsCard()]);
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: workflowCard()),
            const SizedBox(width: 14),
            Expanded(child: eventsCard()),
          ],
        );
      },
    );
  }

  Widget start223WebsiteAdapterPanel() {
    final adapter = websiteAdapter ?? <String, dynamic>{};
    final configured = adapter['configured'] == true;
    final domains = _start223StringList(adapter['allowed_domains']);
    final capabilities = _start223StringList(adapter['capabilities']);
    return _InfoCard(
      title: 'Partner Website Adapter',
      icon: Icons.language_outlined,
      action: FilledButton.icon(
        onPressed: start223EditWebsiteAdapter,
        icon: Icon(configured ? Icons.tune_rounded : Icons.add_link_rounded, size: 17),
        label: LText(configured ? 'Configure adapter' : 'Add adapter'),
      ),
      children: [
        _DefinitionRow(label: 'Configured', value: configured ? 'Yes' : 'No'),
        _DefinitionRow(label: 'Enabled', value: adapter['enabled'] == true ? 'Yes' : 'No'),
        _DefinitionRow(label: 'Environment', value: '${adapter['environment'] ?? 'PRODUCTION'}'),
        _DefinitionRow(label: 'Adapter type', value: _humanize('${adapter['adapter_type'] ?? 'GENERIC_HTTP'}')),
        _DefinitionRow(label: 'Website', value: '${adapter['site_base_url'] ?? '—'}'),
        _DefinitionRow(label: 'Allowed domains', value: domains.isEmpty ? '—' : domains.join(', ')),
        _DefinitionRow(label: 'Privacy mode', value: '${adapter['privacy_mode'] ?? 'AGGREGATED_ONLY'}'),
        _DefinitionRow(label: 'Capabilities', value: capabilities.isEmpty ? '—' : capabilities.join(' · ')),
        const SizedBox(height: 10),
        const _RuleStrip(
          items: [
            _RuleItem(Icons.code_off_outlined, 'Website', 'Existing site stays intact'),
            _RuleItem(Icons.sync_alt_rounded, 'Connector', 'Entitlements + aggregated results'),
            _RuleItem(Icons.security_outlined, 'Scope', 'Credential-bound tenant isolation'),
          ],
        ),
      ],
    );
  }
}
