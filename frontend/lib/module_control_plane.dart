part of 'main.dart';

class ModuleControlPlanePage extends StatefulWidget {
  const ModuleControlPlanePage({required this.api, super.key});
  final Api api;

  @override
  State<ModuleControlPlanePage> createState() => _ModuleControlPlanePageState();
}

class _ModuleControlPlanePageState extends State<ModuleControlPlanePage> {
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> groups = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> partners = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> commercialRows = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> subscriptionRows = <Map<String, dynamic>>[];
  bool loading = false;
  String? error;
  String query = '';
  String groupFilter = 'ALL';
  String typeFilter = 'ALL';
  String commercialQuery = '';
  String commercialPartnerFilter = 'ALL';
  String commercialModuleFilter = 'ALL';
  String commercialStatusFilter = 'ALL';
  String commercialPerspective = 'PARTNER';
  int commercialShown = 120;

  static const moduleTypes = <String>[
    'CORE','FEATURE','INTEGRATION','REPORTING','WEBSITE','FINANCE','INFRASTRUCTURE',
  ];
  static const relationshipTypes = <String>[
    'REQUIRES','OPTIONAL_DEPENDENCY','INTEGRATES_WITH','EXTENDS','CONFLICTS_WITH','REPLACES',
  ];

  String s(dynamic value) => value == null ? '' : value.toString();

  String commercialMoney(dynamic value, String currency) {
    final code = currency.trim().isEmpty ? 'USD' : currency.trim().toUpperCase();
    return code + ' ' + number(value).toStringAsFixed(2);
  }

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final responses = await Future.wait([
        widget.api.get('/api/v1/modules', force: true),
        widget.api.get('/api/v1/module-groups', force: true),
        widget.api.get('/api/v1/partners?limit=200&offset=0&core_only=true', force: true),
      ]);
      final partnerItems = items(responses[2]);
      final partnerIDs = partnerItems.map((p) => s(p['id'])).where((id) => id.isNotEmpty).toList();
      Map<String, dynamic> matrix = <String, dynamic>{'items': <Map<String, dynamic>>[]};
      Map<String, dynamic> subscriptions = <String, dynamic>{'items': <Map<String, dynamic>>[]};
      if (partnerIDs.isNotEmpty) {
        final encoded = Uri.encodeQueryComponent(partnerIDs.join(','));
        final commercial = await Future.wait([
          widget.api.get('/api/v1/module-commercial-matrix?partner_ids=$encoded', force: true),
          widget.api.get('/api/v1/billing/subscription-matrix?partner_ids=$encoded', force: true),
        ]);
        matrix = commercial[0];
        subscriptions = commercial[1];
      }
      if (!mounted) return;
      setState(() {
        modules = items(responses[0]);
        groups = items(responses[1]);
        partners = partnerItems;
        commercialRows = items(matrix);
        subscriptionRows = items(subscriptions);
        commercialShown = 120;
        loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { error = e.toString(); loading = false; });
    }
  }

  void notify(String message, {bool failure = false}) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: LText(message),
      behavior: SnackBarBehavior.floating,
      backgroundColor: failure ? brandDanger : brandSuccess,
    ));
  }

  List<Map<String, dynamic>> get filtered {
    final q = query.trim().toLowerCase();
    return modules.where((module) {
      final text = [
        s(module['label']), s(module['key']), s(module['group_label']),
        s(module['source_repository']), s(module['source_path']), s(module['owner_team']),
      ].join(' ').toLowerCase();
      return (q.isEmpty || text.contains(q)) &&
          (groupFilter == 'ALL' || s(module['group_key']) == groupFilter) &&
          (typeFilter == 'ALL' || s(module['module_type']) == typeFilter);
    }).toList();
  }

  String partnerName(String partnerID) {
    for (final partner in partners) {
      if (s(partner['id']) == partnerID) {
        final display = s(partner['display_name']).trim();
        return display.isEmpty ? partnerID : display;
      }
    }
    return partnerID;
  }

  Map<String, dynamic>? commercialSubscription(String partnerID, String moduleKey) {
    for (final item in subscriptionRows) {
      if (s(item['partner_id']) == partnerID && s(item['module_key']) == moduleKey) return item;
    }
    return null;
  }

  List<Map<String, dynamic>> get filteredCommercialRows {
    final q = commercialQuery.trim().toLowerCase();
    final rows = commercialRows.where((row) {
      final partnerID = s(row['partner_id']);
      final moduleKey = s(row['key']);
      final text = [
        partnerName(partnerID), partnerID, s(row['label']), moduleKey, s(row['group_label']),
      ].join(' ').toLowerCase();
      return (q.isEmpty || text.contains(q)) &&
          (commercialPartnerFilter == 'ALL' || partnerID == commercialPartnerFilter) &&
          (commercialModuleFilter == 'ALL' || moduleKey == commercialModuleFilter) &&
          (commercialStatusFilter == 'ALL' || s(row['status']) == commercialStatusFilter);
    }).toList();
    rows.sort((a, b) {
      final aPartner = partnerName(s(a['partner_id'])).toLowerCase();
      final bPartner = partnerName(s(b['partner_id'])).toLowerCase();
      final aModule = s(a['label']).toLowerCase();
      final bModule = s(b['label']).toLowerCase();
      if (commercialPerspective == 'MODULE') {
        final moduleCompare = aModule.compareTo(bModule);
        return moduleCompare != 0 ? moduleCompare : aPartner.compareTo(bPartner);
      }
      final partnerCompare = aPartner.compareTo(bPartner);
      return partnerCompare != 0 ? partnerCompare : aModule.compareTo(bModule);
    });
    return rows;
  }

  Future<void> showCommercialHistory(Map<String, dynamic> row) async {
    final partnerID = s(row['partner_id']);
    final moduleKey = s(row['key']);
    try {
      final response = await widget.api.get(
        '/api/v1/partners/$partnerID/modules/$moduleKey/commercial-history',
        force: true,
      );
      final history = items(response);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => BrandDialog(
          title: partnerName(partnerID) + ' · ' + s(row['label']),
          subtitle: 'Commercial and entitlement changes are retained with actor, effective time and reason.',
          icon: Icons.history_rounded,
          width: 760,
          primaryLabel: 'Close',
          onPrimary: () => Navigator.pop(dialogContext),
          child: history.isEmpty
              ? const _MessageCard(
                  icon: Icons.history_toggle_off_outlined,
                  title: 'No commercial history yet',
                  message: 'The first partner-specific change will appear here.',
                )
              : Column(
                  children: [
                    for (final event in history) ...[
                      _InfoCard(
                        title: _humanize(s(event['field'])),
                        icon: Icons.history_rounded,
                        children: [
                          _DefinitionRow(label: 'Previous value', value: s(event['old_value']).isEmpty ? '—' : s(event['old_value'])),
                          _DefinitionRow(label: 'New value', value: s(event['new_value']).isEmpty ? '—' : s(event['new_value'])),
                          _DefinitionRow(label: 'Effective at', value: s(event['effective_at'])),
                          _DefinitionRow(label: 'Actor', value: s(event['actor']).isEmpty ? '—' : s(event['actor'])),
                          _DefinitionRow(label: 'Reason', value: s(event['reason']).isEmpty ? '—' : s(event['reason'])),
                        ],
                      ),
                      const SizedBox(height: 10),
                    ],
                  ],
                ),
        ),
      );
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> editCommercialAssignment(Map<String, dynamic> row) async {
    final partnerID = s(row['partner_id']);
    final moduleKey = s(row['key']);
    bool visible = row['visible'] == true;
    bool included = row['included_in_base'] == true;
    final recurring = TextEditingController(text: number(row['partner_price']).toStringAsFixed(2));
    final activation = TextEditingController(text: number(row['partner_activation_fee']).toStringAsFixed(2));
    final recurringEffective = TextEditingController();
    final activationEffective = TextEditingController();
    final quoteReference = TextEditingController(text: s(row['quote_reference']));
    final reason = TextEditingController();
    String contractCurrency = s(row['contract_currency']).isEmpty ? (s(row['currency']).isEmpty ? 'USD' : s(row['currency'])) : s(row['contract_currency']);
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: partnerName(partnerID) + ' · ' + s(row['label']),
          subtitle: 'Partner-specific commercial pricing. Current paid-period snapshots remain immutable.',
          icon: Icons.price_change_outlined,
          width: 760,
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            _RuleStrip(items: [
              _RuleItem(Icons.toggle_on_outlined, 'Entitlement', _humanize(s(row['entitlement_state']).isEmpty ? s(row['status']) : s(row['entitlement_state']))),
              _RuleItem(Icons.sell_outlined, 'Reference module price', money(row['reference_monthly_price'] ?? row['default_monthly_price'])),
              _RuleItem(Icons.bolt_outlined, 'Reference activation fee', money(row['reference_activation_fee'] ?? row['default_activation_fee'])),
            ]),
            const SizedBox(height: 14),
            SwitchListTile.adaptive(
              contentPadding: EdgeInsets.zero,
              value: visible,
              onChanged: (value) => setLocal(() => visible = value),
              title: const LText('Visible to partner'),
            ),
            SwitchListTile.adaptive(
              contentPadding: EdgeInsets.zero,
              value: included,
              onChanged: (value) => setLocal(() => included = value),
              title: const LText('Included in base service'),
              subtitle: const LText('Included modules remain visible in the commercial matrix but add no recurring module fee.'),
            ),
            const SizedBox(height: 8),
            ResponsiveFieldPair(
              first: DropdownButtonFormField<String>(
                value: contractCurrency,
                decoration: InputDecoration(labelText: uiLiteral('Contract currency')),
                items: const [
                  DropdownMenuItem(value: 'USD', child: LText('USD')),
                  DropdownMenuItem(value: 'EUR', child: LText('EUR')),
                  DropdownMenuItem(value: 'GBP', child: LText('GBP')),
                ],
                onChanged: (value) { if (value != null) setLocal(() => contractCurrency = value); },
              ),
              second: TextField(
                controller: quoteReference,
                decoration: InputDecoration(labelText: uiLiteral('Quote / offer reference'), hintText: uiLiteral('Partner-specific commercial offer')),
              ),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(
                controller: recurring,
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                decoration: InputDecoration(labelText: uiLiteral('Partner monthly price')),
              ),
              second: TextField(
                controller: recurringEffective,
                decoration: InputDecoration(labelText: uiLiteral('Price effective at'), hintText: uiLiteral('Optional RFC3339 timestamp')),
              ),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(
                controller: activation,
                keyboardType: const TextInputType.numberWithOptions(decimal: true),
                decoration: InputDecoration(labelText: uiLiteral('Partner activation fee')),
              ),
              second: TextField(
                controller: activationEffective,
                decoration: InputDecoration(labelText: uiLiteral('Activation fee effective at'), hintText: uiLiteral('Optional RFC3339 timestamp')),
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: reason,
              decoration: InputDecoration(labelText: uiLiteral('Change reason'), hintText: uiLiteral('Recorded in partner-module commercial history')),
            ),
          ]),
          primaryLabel: 'Save commercial terms',
          onPrimary: () => Navigator.pop(dialogContext, true),
        ),
      ),
    );
    if (ok == true) {
      final recurringValue = double.tryParse(recurring.text);
      final activationValue = double.tryParse(activation.text);
      if (recurringValue == null || recurringValue < 0 || activationValue == null || activationValue < 0) {
        notify('Commercial prices must be zero or greater.', failure: true);
      } else {
        try {
          await widget.api.patch('/api/v1/partners/$partnerID/modules/$moduleKey', {
            'visible': visible,
            'included_in_base': included,
            'contract_currency': contractCurrency,
            'quote_reference': quoteReference.text.trim(),
            'partner_price': recurringValue,
            'price_effective_at': recurringEffective.text.trim(),
            'partner_activation_fee': activationValue,
            'activation_fee_effective_at': activationEffective.text.trim(),
            'reason': reason.text.trim(),
          });
          await load();
          if (mounted) notify('Partner-module commercial terms updated.');
        } catch (e) {
          if (mounted) notify(e.toString(), failure: true);
        }
      }
    }
    for (final controller in [recurring, activation, recurringEffective, activationEffective, quoteReference, reason]) {
      controller.dispose();
    }
  }

  Widget commercialCard(Map<String, dynamic> row) {
    final partnerID = s(row['partner_id']);
    final moduleKey = s(row['key']);
    final subscription = commercialSubscription(partnerID, moduleKey);
    final configuredNext = row['next_partner_price'] ?? row['partner_price'];
    final nextAt = s(row['next_price_effective_at']).trim();
    final nextAtLabel = nextAt.isEmpty ? '' : (nextAt.length >= 10 ? nextAt.substring(0, 10) : nextAt);
    final currency = s(row['currency']);
    final currentIncluded = subscription?['current_period_included_in_base'] == true;
    final nextIncluded = subscription?['next_period_included_in_base'] == true;
    final currentPeriod = subscription == null
        ? 'Not started'
        : s(subscription['period_start']) + ' → ' + s(subscription['period_end_exclusive']);
    final renewalState = subscription == null
        ? 'No subscription'
        : s(subscription['lifecycle_state']).isNotEmpty
            ? _humanize(s(subscription['lifecycle_state']))
            : subscription['cancel_at_period_end'] == true
                ? 'Cancels at period end'
                : subscription['auto_renew'] == true
                    ? 'Auto-renew'
                    : 'No renewal';
    final primaryTitle = commercialPerspective == 'MODULE' ? s(row['label']) : partnerName(partnerID);
    final secondaryTitle = commercialPerspective == 'MODULE'
        ? partnerName(partnerID) + ' · ' + partnerID
        : s(row['label']) + ' · ' + moduleKey;
    final currentPeriodPrice = subscription == null
        ? '—'
        : currentIncluded
            ? 'Included'
            : commercialMoney(subscription['price'], s(subscription['currency']));
    final nextBillingPrice = subscription == null || subscription['next_period_price'] == null
        ? '—'
        : nextIncluded
            ? 'Included'
            : commercialMoney(subscription['next_period_price'], s(subscription['next_period_currency']));
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              LText(primaryTitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 13)),
              const SizedBox(height: 3),
              LText(secondaryTitle, maxLines: 2, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
            ])),
            _StatusPill(label: s(row['status'])),
          ]),
          const SizedBox(height: 12),
          _DefinitionRow(label: 'Activation date', value: s(row['activated_at']).isEmpty ? '—' : s(row['activated_at'])),
          _DefinitionRow(label: 'Current period', value: currentPeriod),
          _DefinitionRow(label: 'Current period price', value: currentPeriodPrice),
          _DefinitionRow(label: 'Next billing date', value: subscription == null ? '—' : s(subscription['next_billing_date'])),
          _DefinitionRow(label: 'Next billing price', value: nextBillingPrice),
          _DefinitionRow(label: 'Contract price', value: row['included_in_base'] == true ? 'Included in base service' : commercialMoney(row['partner_price'], currency)),
          _DefinitionRow(label: 'Pricing authority', value: _humanize(s(row['pricing_authority']).isEmpty ? 'PARTNER_CONTRACT' : s(row['pricing_authority']))),
          _DefinitionRow(label: 'Quote / offer', value: s(row['quote_reference']).isEmpty ? 'Not recorded' : s(row['quote_reference'])),
          _DefinitionRow(label: 'Price source', value: _humanize(s(row['price_source']))),
          _DefinitionRow(label: 'Next configured price', value: commercialMoney(configuredNext, currency) + (nextAtLabel.isEmpty ? '' : ' · ' + nextAtLabel)),
          _DefinitionRow(label: 'Activation fee', value: commercialMoney(row['partner_activation_fee'], currency)),
          _DefinitionRow(label: 'Activation fee source', value: _humanize(s(row['activation_fee_source']))),
          _DefinitionRow(label: 'Subscription lifecycle', value: renewalState),
          if (subscription?['cancellation_effective_at'] != null)
            _DefinitionRow(label: 'Cancellation effective', value: s(subscription?['cancellation_effective_at'])),
          _DefinitionRow(label: 'Partner visibility', value: row['visible'] == true ? 'Visible' : 'Hidden'),
          const SizedBox(height: 10),
          ResponsiveActionBar(
            breakpoint: 520,
            actions: [
              OutlinedButton.icon(
                onPressed: () => showCommercialHistory(row),
                icon: const Icon(Icons.history_rounded, size: 17),
                label: const LText('Commercial history'),
              ),
              FilledButton.icon(
                onPressed: () => editCommercialAssignment(row),
                icon: const Icon(Icons.price_change_outlined, size: 17),
                label: const LText('Edit commercial pricing'),
              ),
            ],
          ),
        ]),
      ),
    );
  }

  Future<void> addGroup() async {
    final key = TextEditingController();
    final labelEN = TextEditingController();
    final labelHU = TextEditingController();
    final order = TextEditingController(text: (groups.length + 1).toString());
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'Create module group',
        subtitle: 'Create a stable registry classification with independent English and Hungarian labels.',
        icon: Icons.category_outlined,
        width: 680,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          ResponsiveFieldPair(
            first: TextField(controller: labelEN, decoration: InputDecoration(labelText: uiLiteral('English group name *'))),
            second: TextField(controller: labelHU, decoration: InputDecoration(labelText: uiLiteral('Hungarian group name *'))),
          ),
          const SizedBox(height: 12),
          ResponsiveFieldPair(
            first: TextField(controller: key, decoration: InputDecoration(labelText: uiLiteral('Stable group key *'))),
            second: TextField(controller: order, keyboardType: TextInputType.number, decoration: InputDecoration(labelText: uiLiteral('Sort order'))),
          ),
        ]),
        primaryLabel: 'Create group',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );
    if (ok == true && key.text.trim().isNotEmpty && labelEN.text.trim().isNotEmpty && labelHU.text.trim().isNotEmpty) {
      try {
        await widget.api.post('/api/v1/module-groups', {
          'group_key': key.text.trim().toLowerCase(),
          'label_en': labelEN.text.trim(),
          'label_hu': labelHU.text.trim(),
          'sort_order': int.tryParse(order.text) ?? groups.length + 1,
        });
        await load();
        if (mounted) notify('Module group created.');
      } catch (e) {
        if (mounted) notify(e.toString(), failure: true);
      }
    }
    key.dispose(); labelEN.dispose(); labelHU.dispose(); order.dispose();
  }

  Future<Map<String, dynamic>?> moduleDialog({Map<String, dynamic>? module}) async {
    if (groups.isEmpty) return null;
    final editing = module != null;
    final labelEN = TextEditingController(text: s(module?['label_en']).isEmpty ? s(module?['label']) : s(module?['label_en']));
    final labelHU = TextEditingController(text: s(module?['label_hu']).isEmpty ? s(module?['label']) : s(module?['label_hu']));
    final key = TextEditingController(text: s(module?['key']));
    final descriptionEN = TextEditingController(text: s(module?['description_en']).isEmpty ? s(module?['description']) : s(module?['description_en']));
    final descriptionHU = TextEditingController(text: s(module?['description_hu']).isEmpty ? s(module?['description']) : s(module?['description_hu']));
    final price = TextEditingController(text: number(module?['default_monthly_price']).toStringAsFixed(2));
    final activationFee = TextEditingController(text: number(module?['default_activation_fee']).toStringAsFixed(2));
    final owner = TextEditingController(text: s(module?['owner_team']));
    final repo = TextEditingController(text: s(module?['source_repository']));
    final path = TextEditingController(text: s(module?['source_path']));
    final sourceRef = TextEditingController(text: s(module?['source_ref']));
    final commit = TextEditingController(text: s(module?['source_commit']));
    final artifactType = TextEditingController(text: s(module?['artifact_type']));
    final artifactReference = TextEditingController(text: s(module?['artifact_reference']));
    final latestVersion = TextEditingController(text: s(module?['latest_version']).isEmpty ? '1.0.0' : s(module?['latest_version']));
    final minPlatform = TextEditingController(text: s(module?['min_platform_version']));
    String group = s(module?['group_key']).isEmpty ? s(groups.first['group_key']) : s(module?['group_key']);
    String type = s(module?['module_type']).isEmpty ? 'FEATURE' : s(module?['module_type']);
    String availability = s(module?['availability']).isEmpty ? 'ACTIVE' : s(module?['availability']);
    String publicationStatus = s(module?['publication_status']).isEmpty ? 'UNPUBLISHED' : s(module?['publication_status']);
    String implementationState = s(module?['implementation_state']).isEmpty ? 'IN_DEVELOPMENT' : s(module?['implementation_state']);
    final legacyReference = TextEditingController(text: s(module?['legacy_reference']));
    String? dialogError;

    final result = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: editing ? 'Edit module registry' : 'Create module',
          subtitle: 'Bilingual business metadata, source identity, artifact reference and compatibility.',
          icon: Icons.hub_outlined,
          width: 900,
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            const _DialogSectionLabel('BILINGUAL IDENTITY'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: TextField(controller: labelEN, decoration: InputDecoration(labelText: uiLiteral('English module name *'))),
              second: TextField(controller: labelHU, decoration: InputDecoration(labelText: uiLiteral('Hungarian module name *'))),
            ),
            const SizedBox(height: 12),
            TextField(controller: key, readOnly: editing, decoration: InputDecoration(labelText: uiLiteral('Stable module key *'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: descriptionEN, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('English description'))),
              second: TextField(controller: descriptionHU, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('Hungarian description'))),
            ),
            const SizedBox(height: 18),
            const _DialogSectionLabel('COMMERCIAL & CLASSIFICATION'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: DropdownButtonFormField<String>(
                value: group,
                decoration: InputDecoration(labelText: uiLiteral('Module group')),
                items: [for (final item in groups) DropdownMenuItem(value: s(item['group_key']), child: LText(s(item['label'])))],
                onChanged: (value) { if (value != null) setLocal(() => group = value); },
              ),
              second: DropdownButtonFormField<String>(
                value: type,
                decoration: InputDecoration(labelText: uiLiteral('Module type')),
                items: [for (final value in moduleTypes) DropdownMenuItem(value: value, child: LText(_humanize(value)))],
                onChanged: (value) { if (value != null) setLocal(() => type = value); },
              ),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: price, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: InputDecoration(labelText: uiLiteral('Default monthly reference price'))),
              second: TextField(controller: activationFee, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: InputDecoration(labelText: uiLiteral('Default activation fee'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: DropdownButtonFormField<String>(
                value: publicationStatus,
                decoration: InputDecoration(labelText: uiLiteral('Publication status')),
                items: const [
                  DropdownMenuItem(value: 'UNPUBLISHED', child: LText('UNPUBLISHED')),
                  DropdownMenuItem(value: 'PUBLISHED', child: LText('PUBLISHED')),
                ],
                onChanged: (value) { if (value != null) setLocal(() => publicationStatus = value); },
              ),
              second: DropdownButtonFormField<String>(
                value: implementationState,
                decoration: InputDecoration(labelText: uiLiteral('Implementation state')),
                items: const [
                  DropdownMenuItem(value: 'LEGACY_REFERENCE', child: LText('LEGACY REFERENCE')),
                  DropdownMenuItem(value: 'IN_DEVELOPMENT', child: LText('IN DEVELOPMENT')),
                  DropdownMenuItem(value: 'READY', child: LText('READY')),
                ],
                onChanged: (value) { if (value != null) setLocal(() => implementationState = value); },
              ),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: DropdownButtonFormField<String>(
                value: availability,
                decoration: InputDecoration(labelText: uiLiteral('Operational availability')),
                items: const [
                  DropdownMenuItem(value: 'ACTIVE', child: LText('ACTIVE')),
                  DropdownMenuItem(value: 'UNAVAILABLE', child: LText('UNAVAILABLE')),
                  DropdownMenuItem(value: 'DEPRECATED', child: LText('DEPRECATED')),
                ],
                onChanged: (value) { if (value != null) setLocal(() => availability = value); },
              ),
              second: TextField(controller: legacyReference, decoration: InputDecoration(labelText: uiLiteral('Legacy/reference implementation'))),
            ),
            const SizedBox(height: 18),
            const _DialogSectionLabel('SOURCE & RELEASE'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: TextField(controller: owner, decoration: InputDecoration(labelText: uiLiteral('Owner / team'))),
              second: TextField(controller: latestVersion, decoration: InputDecoration(labelText: uiLiteral('Latest version'))),
            ),
            const SizedBox(height: 12),
            TextField(controller: repo, decoration: InputDecoration(labelText: uiLiteral('Git repository'), hintText: uiLiteral('himatecorp2025/himate-system'))),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: path, decoration: InputDecoration(labelText: uiLiteral('Source path'))),
              second: TextField(controller: sourceRef, decoration: InputDecoration(labelText: uiLiteral('Branch / tag'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: commit, decoration: InputDecoration(labelText: uiLiteral('Commit SHA'))),
              second: TextField(controller: minPlatform, decoration: InputDecoration(labelText: uiLiteral('Minimum platform version'))),
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextField(controller: artifactType, decoration: InputDecoration(labelText: uiLiteral('Artifact type'))),
              second: TextField(controller: artifactReference, decoration: InputDecoration(labelText: uiLiteral('Artifact reference / image digest'))),
            ),
            if (dialogError != null) ...[
              const SizedBox(height: 10),
              LText(dialogError!, style: const TextStyle(color: brandDanger, fontSize: 10.5, fontWeight: FontWeight.w600)),
            ],
          ]),
          primaryLabel: editing ? 'Save module' : 'Create module',
          onPrimary: () {
            if (labelEN.text.trim().isEmpty || labelHU.text.trim().isEmpty || key.text.trim().isEmpty) {
              setLocal(() => dialogError = 'English name, Hungarian name and stable key are required.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              if (!editing) 'key': key.text.trim().toLowerCase(),
              'label_en': labelEN.text.trim(),
              'label_hu': labelHU.text.trim(),
              'group_key': group,
              'description_en': descriptionEN.text.trim(),
              'description_hu': descriptionHU.text.trim(),
              'currency': s(module?['currency']).isEmpty ? 'USD' : s(module?['currency']),
              if (!editing) 'version': latestVersion.text.trim().isEmpty ? '1.0.0' : latestVersion.text.trim(),
              'latest_version': latestVersion.text.trim().isEmpty ? '1.0.0' : latestVersion.text.trim(),
              'default_monthly_price': double.tryParse(price.text) ?? 0,
              'default_activation_fee': double.tryParse(activationFee.text) ?? 0,
              'availability': availability,
              'publication_status': publicationStatus,
              'implementation_state': implementationState,
              'legacy_reference': legacyReference.text.trim(),
              'module_type': type,
              'owner_team': owner.text.trim(),
              'source_repository': repo.text.trim(),
              'source_path': path.text.trim(),
              'source_ref': sourceRef.text.trim(),
              'source_commit': commit.text.trim(),
              'artifact_type': artifactType.text.trim(),
              'artifact_reference': artifactReference.text.trim(),
              'min_platform_version': minPlatform.text.trim(),
              if (!editing) 'manifest': <String, dynamic>{'schema_version': 1},
            });
          },
        ),
      ),
    );
    for (final controller in [labelEN,labelHU,key,descriptionEN,descriptionHU,price,activationFee,owner,repo,path,sourceRef,commit,artifactType,artifactReference,latestVersion,minPlatform,legacyReference]) {
      controller.dispose();
    }
    return result;
  }

  Future<void> addModule() async {
    final payload = await moduleDialog();
    if (payload == null) return;
    try {
      await widget.api.post('/api/v1/modules', payload);
      await load();
      if (mounted) notify('Module registered.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> editModule(Map<String, dynamic> module) async {
    final payload = await moduleDialog(module: module);
    if (payload == null) return;
    try {
      await widget.api.patch('/api/v1/modules/' + s(module['key']), payload);
      await load();
      if (mounted) notify('Module registry updated.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> manageModule(Map<String, dynamic> module) async {
    final key = s(module['key']);
    var relationships = <Map<String, dynamic>>[];
    var metrics = <Map<String, dynamic>>[];
    var usage = <Map<String, dynamic>>[];
    var detailLoading = true;
    String? detailError;
    var requested = false;

    Future<void> refresh(StateSetter setLocal) async {
      setLocal(() { detailLoading = true; detailError = null; });
      try {
        final responses = await Future.wait([
          widget.api.get('/api/v1/modules/' + key + '/relationships', force: true),
          widget.api.get('/api/v1/modules/' + key + '/impact-metrics', force: true),
          widget.api.get('/api/v1/modules/' + key + '/usage', force: true),
        ]);
        relationships = items(responses[0]);
        metrics = items(responses[1]);
        usage = items(responses[2]);
        setLocal(() => detailLoading = false);
      } catch (e) {
        setLocal(() { detailLoading = false; detailError = e.toString(); });
      }
    }

    await showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) {
          if (!requested) {
            requested = true;
            WidgetsBinding.instance.addPostFrameCallback((_) => refresh(setLocal));
          }

          Future<void> addRelationship() async {
            final available = modules.where((m) => s(m['key']) != key).toList();
            if (available.isEmpty) return;
            String target = s(available.first['key']);
            String relation = 'REQUIRES';
            final note = TextEditingController();
            final ok = await showDialog<bool>(
              context: context,
              builder: (subContext) => StatefulBuilder(
                builder: (context, setSub) => BrandDialog(
                  title: 'Add module relationship',
                  subtitle: 'Register a dependency or integration relation.',
                  icon: Icons.account_tree_outlined,
                  width: 620,
                  child: Column(mainAxisSize: MainAxisSize.min, children: [
                    DropdownButtonFormField<String>(
                      value: target,
                      decoration: InputDecoration(labelText: uiLiteral('Target module')),
                      items: [for (final item in available) DropdownMenuItem(value: s(item['key']), child: LText(s(item['label']) + ' · ' + s(item['key'])))],
                      onChanged: (value) { if (value != null) setSub(() => target = value); },
                    ),
                    const SizedBox(height: 12),
                    DropdownButtonFormField<String>(
                      value: relation,
                      decoration: InputDecoration(labelText: uiLiteral('Relationship')),
                      items: [for (final value in relationshipTypes) DropdownMenuItem(value: value, child: LText(_humanize(value)))],
                      onChanged: (value) { if (value != null) setSub(() => relation = value); },
                    ),
                    const SizedBox(height: 12),
                    TextField(controller: note, decoration: InputDecoration(labelText: uiLiteral('Relationship note'))),
                  ]),
                  primaryLabel: 'Save relationship',
                  onPrimary: () => Navigator.pop(subContext, true),
                ),
              ),
            );
            if (ok == true) {
              try {
                await widget.api.post('/api/v1/modules/' + key + '/relationships', {
                  'target_module_key': target,
                  'relation_type': relation,
                  'note': note.text.trim(),
                });
                await refresh(setLocal);
              } catch (e) {
                if (mounted) notify(e.toString(), failure: true);
              }
            }
            note.dispose();
          }

          Future<void> removeRelationship(Map<String, dynamic> item) async {
            try {
              final uri = Uri(
                path: '/api/v1/modules/' + key + '/relationships/' + s(item['target_module_key']),
                queryParameters: {'type': s(item['relation_type'])},
              );
              await widget.api.delete(uri.toString());
              await refresh(setLocal);
            } catch (e) {
              if (mounted) notify(e.toString(), failure: true);
            }
          }

          Future<void> editMetrics() async {
            final lines = <String>[];
            for (final item in metrics) {
              final metric = s(item['metric_key']);
              final label = s(item['label']).trim();
              lines.add(label.isEmpty ? metric : metric + ' | ' + label);
            }
            final controller = TextEditingController(text: lines.join('\n'));
            final ok = await showDialog<bool>(
              context: context,
              builder: (subContext) => BrandDialog(
                title: 'Impact metric mapping',
                subtitle: 'One metric per line. Optional label can follow a | separator.',
                icon: Icons.analytics_outlined,
                width: 620,
                child: TextField(
                  controller: controller,
                  minLines: 8,
                  maxLines: 14,
                  decoration: InputDecoration(labelText: uiLiteral('Metric keys'), hintText: uiLiteral('campaigns.leads | Leads generated'), alignLabelWithHint: true),
                ),
                primaryLabel: 'Save mapping',
                onPrimary: () => Navigator.pop(subContext, true),
              ),
            );
            if (ok == true) {
              final mapped = <Map<String, dynamic>>[];
              for (final raw in controller.text.split('\n')) {
                final line = raw.trim();
                if (line.isEmpty) continue;
                final parts = line.split('|');
                mapped.add({
                  'metric_key': parts.first.trim(),
                  'label': parts.length > 1 ? parts.sublist(1).join('|').trim() : '',
                });
              }
              try {
                await widget.api.put('/api/v1/modules/' + key + '/impact-metrics', {'items': mapped});
                await refresh(setLocal);
              } catch (e) {
                if (mounted) notify(e.toString(), failure: true);
              }
            }
            controller.dispose();
          }

          return BrandDialog(
            title: s(module['label']),
            subtitle: 'Relationships, impact mapping and partner usage for ' + key + '.',
            icon: Icons.hub_outlined,
            width: 920,
            primaryLabel: 'Close',
            onPrimary: () => Navigator.pop(dialogContext),
            child: detailLoading
                ? const SizedBox(height: 220, child: Center(child: CircularProgressIndicator()))
                : detailError != null
                    ? _MessageCard(icon: Icons.error_outline_rounded, title: 'Module detail unavailable', message: detailError!)
                    : Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        _RuleStrip(items: [
                          _RuleItem(Icons.account_tree_outlined, 'Relationships', relationships.length.toString()),
                          _RuleItem(Icons.analytics_outlined, 'Impact metrics', metrics.length.toString()),
                          _RuleItem(Icons.business_outlined, 'Partner records', usage.length.toString()),
                          _RuleItem(Icons.code_outlined, 'Source', s(module['source_repository']).isEmpty ? 'Not linked' : s(module['source_repository'])),
                        ]),
                        const SizedBox(height: 18),
                        _SectionHeader(
                          title: 'Relationship Graph',
                          subtitle: 'The selected module is the source node; each card shows a directed relation.',
                          trailing: FilledButton.icon(onPressed: addRelationship, icon: const Icon(Icons.add_link_rounded), label: const LText('Add relation')),
                        ),
                        const SizedBox(height: 10),
                        if (relationships.isEmpty)
                          const _MessageCard(icon: Icons.account_tree_outlined, title: 'No relationships yet', message: 'Add dependency, integration, extension, conflict or replacement relations.')
                        else
                          Wrap(spacing: 10, runSpacing: 10, children: [
                            for (final relation in relationships)
                              Container(
                                width: 250,
                                padding: const EdgeInsets.all(13),
                                decoration: BoxDecoration(color: brandWhite, borderRadius: BorderRadius.circular(12), border: Border.all(color: brandMist)),
                                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                                  LText(s(module['label']), style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 11)),
                                  const SizedBox(height: 5),
                                  Row(children: [
                                    const Icon(Icons.arrow_forward_rounded, size: 15, color: brandGold),
                                    const SizedBox(width: 6),
                                    Expanded(child: LText(_humanize(s(relation['relation_type'])), style: const TextStyle(color: brandSteel, fontSize: 9, fontWeight: FontWeight.w700))),
                                  ]),
                                  const SizedBox(height: 5),
                                  LText(s(relation['target_label']), style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 11)),
                                  if (s(relation['note']).trim().isNotEmpty) ...[
                                    const SizedBox(height: 5),
                                    LText(s(relation['note']), style: const TextStyle(color: brandTextSoft, fontSize: 9.2)),
                                  ],
                                  Align(alignment: Alignment.centerRight, child: IconButton(onPressed: () => removeRelationship(relation), icon: const Icon(Icons.link_off_rounded, size: 18))),
                                ]),
                              ),
                          ]),
                        const SizedBox(height: 20),
                        _SectionHeader(
                          title: 'Impact Mapping',
                          subtitle: 'Metric keys attached here will power partner-facing module results.',
                          trailing: OutlinedButton.icon(onPressed: editMetrics, icon: const Icon(Icons.edit_outlined), label: const LText('Edit metrics')),
                        ),
                        const SizedBox(height: 10),
                        if (metrics.isEmpty)
                          const _MessageCard(icon: Icons.analytics_outlined, title: 'No impact metrics mapped', message: 'Map existing Impact metric keys to this module.')
                        else
                          Wrap(spacing: 8, runSpacing: 8, children: [
                            for (final metric in metrics)
                              _MiniCounter(label: s(metric['label']).trim().isEmpty ? s(metric['metric_key']) : s(metric['label']) + ' · ' + s(metric['metric_key'])),
                          ]),
                        const SizedBox(height: 20),
                        _SectionHeader(
                          title: 'Partner Usage',
                          subtitle: 'Current entitlement state across partners.',
                          trailing: _MiniCounter(label: usage.where((u) => u['status'] == 'ACTIVE').length.toString() + ' active'),
                        ),
                        const SizedBox(height: 10),
                        if (usage.isEmpty)
                          const _MessageCard(icon: Icons.business_outlined, title: 'No partner usage yet', message: 'The module has not been initialized for a partner.')
                        else
                          LayoutBuilder(builder: (context, constraints) {
                            final width = constraints.maxWidth < 700 ? constraints.maxWidth : (constraints.maxWidth - 10) / 2;
                            return Wrap(spacing: 10, runSpacing: 10, children: [
                              for (final item in usage)
                                SizedBox(
                                  width: width,
                                  child: _InfoCard(
                                    title: partnerName(s(item['partner_id'])),
                                    icon: Icons.business_outlined,
                                    children: [
                                      _DefinitionRow(label: 'Partner ID', value: s(item['partner_id'])),
                                      _DefinitionRow(label: 'Status', value: _humanize(s(item['status']))),
                                      _DefinitionRow(label: 'Included in base', value: item['included_in_base'] == true ? 'Yes' : 'No'),
                                      _DefinitionRow(label: 'Configured monthly price', value: commercialMoney(item['partner_price'], s(item['currency']))),
                                      _DefinitionRow(label: 'Activation fee', value: commercialMoney(item['partner_activation_fee'], s(item['currency']))),
                                    ],
                                  ),
                                ),
                            ]);
                          }),
                      ]),
          );
        },
      ),
    );
  }

  Widget moduleCard(Map<String, dynamic> module) {
    final sourceRepo = s(module['source_repository']).trim();
    final sourcePath = s(module['source_path']).trim();
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Container(width: 40, height: 40, decoration: BoxDecoration(color: brandNavy.withOpacity(.07), borderRadius: BorderRadius.circular(11)), child: const Icon(Icons.extension_outlined, color: brandNavy)),
            const SizedBox(width: 11),
            Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              LText(s(module['label']), maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 13)),
              const SizedBox(height: 3),
              LText(s(module['key']), maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
            ])),
            Column(crossAxisAlignment: CrossAxisAlignment.end, children: [
              _StatusPill(label: s(module['publication_status']).isEmpty ? 'UNPUBLISHED' : s(module['publication_status'])),
              const SizedBox(height: 4),
              _StatusPill(label: s(module['implementation_state']).isEmpty ? 'IN_DEVELOPMENT' : s(module['implementation_state'])),
            ]),
          ]),
          const SizedBox(height: 14),
          _DefinitionRow(label: 'Group', value: s(module['group_label']).isEmpty ? s(module['group_key']) : s(module['group_label'])),
          _DefinitionRow(label: 'Type', value: _humanize(s(module['module_type']).isEmpty ? 'FEATURE' : s(module['module_type']))),
          _DefinitionRow(label: 'Reference module price', value: (s(module['currency']).isEmpty ? 'USD' : s(module['currency'])) + ' ' + number(module['reference_monthly_price'] ?? module['default_monthly_price']).toStringAsFixed(2)),
          _DefinitionRow(label: 'Reference activation fee', value: (s(module['currency']).isEmpty ? 'USD' : s(module['currency'])) + ' ' + number(module['reference_activation_fee'] ?? module['default_activation_fee']).toStringAsFixed(2)),
          const _DefinitionRow(label: 'Billing authority', value: 'Partner-specific contract / quote'),
          _DefinitionRow(label: 'Latest version', value: s(module['latest_version']).isEmpty ? '—' : s(module['latest_version'])),
          _DefinitionRow(label: 'Owner', value: s(module['owner_team']).isEmpty ? '—' : s(module['owner_team'])),
          const SizedBox(height: 8),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(color: brandIvory, borderRadius: BorderRadius.circular(9), border: Border.all(color: brandMist)),
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const LText('SOURCE', style: TextStyle(color: brandSteel, fontWeight: FontWeight.w700, fontSize: 8.5, letterSpacing: .8)),
              const SizedBox(height: 5),
              LText(sourceRepo.isEmpty ? 'Source not linked' : sourceRepo + (sourcePath.isEmpty ? '' : ' · ' + sourcePath), maxLines: 2, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
            ]),
          ),
          const SizedBox(height: 18),
          Row(children: [
            Expanded(child: OutlinedButton.icon(onPressed: () => editModule(module), icon: const Icon(Icons.edit_outlined, size: 17), label: const LText('Edit'))),
            const SizedBox(width: 8),
            Expanded(child: FilledButton.icon(onPressed: () => manageModule(module), icon: const Icon(Icons.account_tree_outlined, size: 17), label: const LText('Manage'))),
          ]),
        ]),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final active = modules.where((m) => m['availability'] == 'ACTIVE').length;
    final linked = modules.where((m) => s(m['source_repository']).trim().isNotEmpty).length;
    final relations = modules.fold<int>(0, (sum, m) => sum + ((m['relationship_count'] as num?)?.toInt() ?? 0));
    final partnerUsage = modules.fold<int>(0, (sum, m) => sum + ((m['active_partner_count'] as num?)?.toInt() ?? 0));

    return Content(
      eyebrow: 'MODULE CONTROL PLANE',
      title: 'Modules',
      subtitle: 'Authoritative registry plus partner-by-partner commercial pricing, activation fees, subscription periods, dependencies and usage.',
      actions: [
        OutlinedButton.icon(onPressed: loading ? null : addGroup, icon: const Icon(Icons.category_outlined), label: const LText('Add group')),
        FilledButton.icon(onPressed: loading || groups.isEmpty ? null : addModule, icon: const Icon(Icons.add_box_outlined), label: const LText('Add module')),
      ],
      child: error != null && modules.isEmpty
          ? _MessageCard(icon: Icons.cloud_off_outlined, title: 'Module Control Plane unavailable', message: error!)
          : Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              ResponsiveKpiGrid(children: [
                Kpi(label: 'Module registry', value: modules.length.toString(), note: 'Canonical + custom modules', icon: Icons.hub_outlined, accent: brandNavy),
                Kpi(label: 'Active modules', value: active.toString(), note: 'Available for assignment', icon: Icons.check_circle_outline_rounded, accent: brandSuccess),
                Kpi(label: 'Source linked', value: linked.toString(), note: 'Git/source identity configured', icon: Icons.code_outlined, accent: brandSteel),
                Kpi(label: 'Relationships', value: relations.toString(), note: partnerUsage.toString() + ' active partner assignments', icon: Icons.account_tree_outlined, accent: brandGold),
              ]),
              const SizedBox(height: 22),
              _SectionHeader(
                title: 'Partner × Module Commercial Matrix',
                subtitle: 'Authoritative partner assignment, recurring price, activation fee and current subscription period in one control plane.',
                trailing: _MiniCounter(label: filteredCommercialRows.length.toString() + ' assignments'),
              ),
              const SizedBox(height: 12),
              _FilterSurface(
                child: LayoutBuilder(builder: (context, constraints) {
                  final search = TextField(
                    onChanged: (value) => setState(() { commercialQuery = value; commercialShown = 120; }),
                    decoration: InputDecoration(hintText: uiLiteral('Search partner or module...'), prefixIcon: Icon(Icons.search_rounded)),
                  );
                  final partner = DropdownButtonFormField<String>(
                    value: commercialPartnerFilter,
                    decoration: InputDecoration(labelText: uiLiteral('Partner')),
                    items: [
                      const DropdownMenuItem(value: 'ALL', child: LText('All partners')),
                      for (final item in partners)
                        DropdownMenuItem(value: s(item['id']), child: LText(partnerName(s(item['id'])))),
                    ],
                    onChanged: (value) => setState(() { commercialPartnerFilter = value ?? 'ALL'; commercialShown = 120; }),
                  );
                  final module = DropdownButtonFormField<String>(
                    value: commercialModuleFilter,
                    decoration: InputDecoration(labelText: uiLiteral('Module')),
                    items: [
                      const DropdownMenuItem(value: 'ALL', child: LText('All modules')),
                      for (final item in modules)
                        DropdownMenuItem(value: s(item['key']), child: LText(s(item['label']))),
                    ],
                    onChanged: (value) => setState(() { commercialModuleFilter = value ?? 'ALL'; commercialShown = 120; }),
                  );
                  final status = DropdownButtonFormField<String>(
                    value: commercialStatusFilter,
                    decoration: InputDecoration(labelText: uiLiteral('State')),
                    items: const [
                      DropdownMenuItem(value: 'ALL', child: LText('All states')),
                      DropdownMenuItem(value: 'ACTIVE', child: LText('Active')),
                      DropdownMenuItem(value: 'NOT_LICENSED', child: LText('Not licensed')),
                      DropdownMenuItem(value: 'MAINTENANCE', child: LText('Maintenance')),
                    ],
                    onChanged: (value) => setState(() { commercialStatusFilter = value ?? 'ALL'; commercialShown = 120; }),
                  );
                  final perspective = Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      ChoiceChip(
                        selected: commercialPerspective == 'PARTNER',
                        label: const LText('View by partner'),
                        onSelected: (_) => setState(() => commercialPerspective = 'PARTNER'),
                      ),
                      ChoiceChip(
                        selected: commercialPerspective == 'MODULE',
                        label: const LText('View by module'),
                        onSelected: (_) => setState(() => commercialPerspective = 'MODULE'),
                      ),
                    ],
                  );
                  if (constraints.maxWidth < 760) {
                    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      search,
                      const SizedBox(height: 10),
                      partner,
                      const SizedBox(height: 10),
                      module,
                      const SizedBox(height: 10),
                      status,
                      const SizedBox(height: 10),
                      perspective,
                    ]);
                  }
                  return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: partner)]),
                    const SizedBox(height: 10),
                    Row(children: [Expanded(child: module), const SizedBox(width: 10), Expanded(child: status)]),
                    const SizedBox(height: 10),
                    perspective,
                  ]);
                }),
              ),
              const SizedBox(height: 12),
              Builder(builder: (context) {
                final rows = filteredCommercialRows;
                if (rows.isEmpty) {
                  return const _MessageCard(
                    icon: Icons.price_change_outlined,
                    title: 'No partner-module assignments found',
                    message: 'Adjust the filters or create partners/modules to populate the commercial matrix.',
                  );
                }
                final visibleRows = rows.take(commercialShown).toList();
                return Column(children: [
                  LayoutBuilder(builder: (context, constraints) {
                    final width = constraints.maxWidth < 680
                        ? constraints.maxWidth
                        : constraints.maxWidth < 1120
                            ? (constraints.maxWidth - 12) / 2
                            : (constraints.maxWidth - 24) / 3;
                    return Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [for (final row in visibleRows) SizedBox(width: width, child: commercialCard(row))],
                    );
                  }),
                  if (visibleRows.length < rows.length) ...[
                    const SizedBox(height: 12),
                    OutlinedButton.icon(
                      onPressed: () => setState(() => commercialShown += 120),
                      icon: const Icon(Icons.expand_more_rounded),
                      label: LText('Show more · ' + (rows.length - visibleRows.length).toString() + ' remaining'),
                    ),
                  ],
                ]);
              }),
              const SizedBox(height: 26),
              _SectionHeader(
                title: 'Module Registry',
                subtitle: 'Source code remains versioned in Git; HIMATE stores the authoritative identity, source pointer, release and commercial metadata.',
                trailing: _MiniCounter(label: filtered.length.toString() + ' shown'),
              ),
              const SizedBox(height: 12),
              _FilterSurface(
                child: LayoutBuilder(builder: (context, constraints) {
                  final search = TextField(
                    onChanged: (value) => setState(() => query = value),
                    decoration: InputDecoration(hintText: uiLiteral('Search modules, source or owner...'), prefixIcon: Icon(Icons.search_rounded)),
                  );
                  final group = DropdownButtonFormField<String>(
                    value: groupFilter,
                    decoration: InputDecoration(labelText: uiLiteral('Group')),
                    items: [
                      const DropdownMenuItem(value: 'ALL', child: LText('All groups')),
                      for (final item in groups) DropdownMenuItem(value: s(item['group_key']), child: LText(s(item['label']))),
                    ],
                    onChanged: (value) => setState(() => groupFilter = value ?? 'ALL'),
                  );
                  final type = DropdownButtonFormField<String>(
                    value: typeFilter,
                    decoration: InputDecoration(labelText: uiLiteral('Type')),
                    items: [
                      const DropdownMenuItem(value: 'ALL', child: LText('All types')),
                      for (final value in moduleTypes) DropdownMenuItem(value: value, child: LText(_humanize(value))),
                    ],
                    onChanged: (value) => setState(() => typeFilter = value ?? 'ALL'),
                  );
                  if (constraints.maxWidth < 760) return Column(children: [search, const SizedBox(height: 10), group, const SizedBox(height: 10), type]);
                  return Row(children: [Expanded(flex: 2, child: search), const SizedBox(width: 10), Expanded(child: group), const SizedBox(width: 10), Expanded(child: type)]);
                }),
              ),
              const SizedBox(height: 12),
              if (filtered.isEmpty)
                const _MessageCard(icon: Icons.inventory_2_outlined, title: 'No modules found', message: 'No modules match the current filters.')
              else
                LayoutBuilder(builder: (context, constraints) {
                  final width = constraints.maxWidth < 650 ? constraints.maxWidth : constraints.maxWidth < 1050 ? (constraints.maxWidth - 12) / 2 : (constraints.maxWidth - 24) / 3;
                  return Wrap(spacing: 12, runSpacing: 12, children: [
                    for (final module in filtered) SizedBox(width: width, child: moduleCard(module)),
                  ]);
                }),
              if (loading) ...[
                const SizedBox(height: 12),
                const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
              ],
            ]),
    );
  }
}
