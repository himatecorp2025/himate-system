part of 'main.dart';

class ModuleControlPlanePage extends StatefulWidget {
  const ModuleControlPlanePage({required this.api, super.key});
  final Api api;

  @override
  State<ModuleControlPlanePage> createState() => _ModuleControlPlanePageState();
}

class _ModuleControlPlanePageState extends State<ModuleControlPlanePage> {
  List<Map<String, dynamic>> modules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> registryModules = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> groups = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> topicRows = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> partners = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> commercialGroups = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> subscriptionPlans = <Map<String, dynamic>>[];
  Map<String, dynamic> registryKpis = <String, dynamic>{};
  bool showSubscriptionPlans = false;
  bool showCommercialMatrix = false;
  bool loading = false;
  String? error;
  String query = '';
  String? selectedGroupKey;
  String registryPreset = 'TOPICS';
  String groupFilter = 'ALL';
  String typeFilter = 'ALL';
  String commercialQuery = '';
  String commercialPartnerFilter = 'ALL';
  String commercialModuleFilter = 'ALL';
  String commercialStatusFilter = 'ALL';
  String commercialPerspective = 'PARTNER';
  int commercialShown = 120;
  int commercialGroupCount = 0;
  int commercialAssignmentCount = 0;
  Timer? _registryDebounce;
  Timer? _commercialDebounce;

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

  @override
  void dispose() {
    _registryDebounce?.cancel();
    _commercialDebounce?.cancel();
    super.dispose();
  }

  String _centralModulesPath() {
    final params = <String, String>{
      'perspective': commercialPerspective,
      'commercial_limit': '$commercialShown',
    };
    if (query.trim().isNotEmpty) params['registry_q'] = query.trim();
    final effectiveGroup = selectedGroupKey ?? (groupFilter == 'ALL' ? null : groupFilter);
    if (effectiveGroup != null && effectiveGroup.isNotEmpty) params['registry_group'] = effectiveGroup;
    if (typeFilter != 'ALL') params['registry_type'] = typeFilter;
    if (registryPreset != 'TOPICS' && registryPreset != 'ALL') {
      params['registry_preset'] = registryPreset;
    }
    if (commercialQuery.trim().isNotEmpty) params['commercial_q'] = commercialQuery.trim();
    if (commercialPartnerFilter != 'ALL') params['commercial_partner'] = commercialPartnerFilter;
    if (commercialModuleFilter != 'ALL') params['commercial_module'] = commercialModuleFilter;
    if (commercialStatusFilter != 'ALL') params['commercial_status'] = commercialStatusFilter;
    return Uri(path: '/api/v1/central/modules', queryParameters: params).toString();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final model = await widget.api.get(
        _centralModulesPath(),
        maxAge: const Duration(seconds: 5),
      );
      if (!mounted) return;
      final registry = model['registry'] is Map
          ? Map<String, dynamic>.from(model['registry'] as Map)
          : <String, dynamic>{};
      final commercial = model['commercial'] is Map
          ? Map<String, dynamic>.from(model['commercial'] as Map)
          : <String, dynamic>{};
      setState(() {
        modules = items(<String, dynamic>{'items': model['module_options']});
        registryModules = items(<String, dynamic>{'items': registry['modules']});
        groups = items(<String, dynamic>{'items': registry['groups']});
        topicRows = items(<String, dynamic>{'items': registry['topics']});
        registryKpis = registry['kpis'] is Map
            ? Map<String, dynamic>.from(registry['kpis'] as Map)
            : <String, dynamic>{};
        partners = items(<String, dynamic>{'items': model['partners']});
        commercialGroups = items(<String, dynamic>{'items': commercial['groups']});
        commercialGroupCount = (commercial['group_count'] as num?)?.toInt() ?? commercialGroups.length;
        commercialAssignmentCount = (commercial['assignment_count'] as num?)?.toInt() ?? 0;
        subscriptionPlans = items(<String, dynamic>{'items': model['plans']});
        loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { error = e.toString(); loading = false; });
    }
  }

  void _scheduleRegistryReload() {
    _registryDebounce?.cancel();
    _registryDebounce = Timer(const Duration(milliseconds: 220), () {
      if (mounted) unawaited(load());
    });
  }

  void _scheduleCommercialReload() {
    _commercialDebounce?.cancel();
    _commercialDebounce = Timer(const Duration(milliseconds: 220), () {
      if (mounted) unawaited(load());
    });
  }

  void notify(String message, {bool failure = false}) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: LText(message),
      behavior: SnackBarBehavior.floating,
      backgroundColor: failure ? brandDanger : brandSuccess,
    ));
  }

  List<Map<String, dynamic>> get filtered => registryModules;

  String groupLabel(Map<String, dynamic> group) {
    final key = HimateI18n.activeLocale == 'hu_HU' ? 'label_hu' : 'label_en';
    final localized = s(group[key]).trim();
    return localized.isEmpty ? s(group['label']) : localized;
  }

  String moduleLabelForLocale(Map<String, dynamic> module) {
    final key = HimateI18n.activeLocale == 'hu_HU' ? 'label_hu' : 'label_en';
    final localized = s(module[key]).trim();
    return localized.isEmpty ? s(module['label']) : localized;
  }

  List<Map<String, dynamic>> get primaryGroups =>
      groups.where((group) => group['is_primary_navigation'] == true).toList();

  Map<String, dynamic>? groupByKey(String key) {
    for (final group in groups) {
      if (s(group['group_key']) == key) return group;
    }
    return null;
  }

  Map<String, dynamic>? topicByKey(String key) {
    for (final topic in topicRows) {
      if (s(topic['group_key']) == key) return topic;
    }
    return null;
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

  void showTopicOverview() {
    setState(() {
      selectedGroupKey = null;
      registryPreset = 'TOPICS';
      groupFilter = 'ALL';
      typeFilter = 'ALL';
      query = '';
    });
    unawaited(load());
  }

  void applyRegistryPreset(String preset) {
    setState(() {
      selectedGroupKey = null;
      registryPreset = preset;
      groupFilter = 'ALL';
      typeFilter = 'ALL';
      query = '';
    });
    unawaited(load());
  }

  void openTopic(String groupKey) {
    setState(() {
      selectedGroupKey = groupKey;
      registryPreset = 'ALL';
      groupFilter = 'ALL';
      typeFilter = 'ALL';
      query = '';
    });
    unawaited(load());
  }

  Future<void> moveModuleToGroup(Map<String, dynamic> module, String targetGroupKey) async {
    final current = s(module['group_key']);
    if (targetGroupKey.isEmpty || targetGroupKey == current) return;
    try {
      await widget.api.patch('/api/v1/modules/' + s(module['key']), {'group_key': targetGroupKey});
      await load();
      if (mounted) notify('Module moved to ' + groupLabel(groupByKey(targetGroupKey) ?? <String, dynamic>{'label': targetGroupKey}) + '.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
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
                decoration: InputDecoration(labelText: uiLiteral('Partner 30-day price')),
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
    final subscription = row['subscription'] is Map
        ? Map<String, dynamic>.from(row['subscription'] as Map)
        : null;
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
    final resolvedPartnerName = s(row['partner_name']).isEmpty ? partnerName(partnerID) : s(row['partner_name']);
    final primaryTitle = commercialPerspective == 'MODULE' ? s(row['label']) : resolvedPartnerName;
    final secondaryTitle = commercialPerspective == 'MODULE'
        ? resolvedPartnerName + ' · ' + partnerID
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

  Widget commercialGroupCard(Map<String, dynamic> group) {
    final byModule = commercialPerspective == 'MODULE';
    final rows = items(<String, dynamic>{
      'items': byModule ? group['partners'] : group['modules'],
    });
    final title = byModule
        ? (s(group['module_label']).isEmpty ? s(group['module_key']) : s(group['module_label']))
        : (s(group['partner_name']).isEmpty ? s(group['partner_id']) : s(group['partner_name']));
    final subtitle = byModule
        ? '${s(group['module_key'])} · ${rows.length} ${uiLiteral('partners')}'
        : '${s(group['partner_id'])} · ${rows.length} ${uiLiteral('modules')}';
    return Card(
      clipBehavior: Clip.antiAlias,
      child: ExpansionTile(
        maintainState: true,
        tilePadding: const EdgeInsets.symmetric(horizontal: 18, vertical: 8),
        childrenPadding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
        leading: Icon(
          byModule ? Icons.extension_outlined : Icons.business_outlined,
          color: byModule ? brandSteel : brandGold,
        ),
        title: LText(
          title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 14),
        ),
        subtitle: LText(
          subtitle,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(color: brandTextSoft, fontSize: 9.5),
        ),
        children: [
          if (rows.isEmpty)
            const Padding(
              padding: EdgeInsets.all(12),
              child: _MessageCard(
                icon: Icons.inbox_outlined,
                title: 'No assignments',
                message: 'No partner-module assignments match this group.',
              ),
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 760
                    ? constraints.maxWidth
                    : (constraints.maxWidth - 10) / 2;
                return Wrap(
                  spacing: 10,
                  runSpacing: 10,
                  children: [
                    for (final row in rows)
                      SizedBox(width: width, child: commercialCard(row)),
                  ],
                );
              },
            ),
        ],
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
              first: TextField(controller: price, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: InputDecoration(labelText: uiLiteral('Default 30-day price'))),
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
    var usageSummary = <String, dynamic>{};
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
        final rawSummary = responses[2]['usage_summary'];
        usageSummary = rawSummary is Map ? Map<String, dynamic>.from(rawSummary) : <String, dynamic>{};
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
            subtitle: uiLiteral('Relationships, impact mapping and partner usage for') + ' ' + key + '.',
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
                          subtitle: 'Current entitlement plus real runtime frequency from successful Partner Portal module operations.',
                          trailing: _MiniCounter(label: usage.where((u) => u['status'] == 'ACTIVE').length.toString() + ' active'),
                        ),
                        const SizedBox(height: 10),
                        _RuleStrip(items: [
                          _RuleItem(Icons.today_outlined, 'Last 7 days', '${usageSummary['events_7d'] ?? 0}'),
                          _RuleItem(Icons.calendar_month_outlined, 'Last 30 days', '${usageSummary['events_30d'] ?? 0}'),
                          _RuleItem(Icons.query_stats_outlined, 'All runtime events', '${usageSummary['events_total'] ?? 0}'),
                          _RuleItem(Icons.business_center_outlined, 'Partners using it', '${usageSummary['partners_with_usage'] ?? 0}'),
                        ]),
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
                                      _DefinitionRow(label: 'Runtime uses · 7 days', value: '${item['usage_events_7d'] ?? 0}'),
                                      _DefinitionRow(label: 'Runtime uses · 30 days', value: '${item['usage_events_30d'] ?? 0}'),
                                      _DefinitionRow(label: 'Runtime uses · total', value: '${item['usage_events_total'] ?? 0}'),
                                      _DefinitionRow(label: 'Last used', value: s(item['last_used_at']).isEmpty ? '—' : s(item['last_used_at'])),
                                      _DefinitionRow(label: 'Configured 30-day price', value: commercialMoney(item['partner_price'], s(item['currency']))),
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

  IconData groupIcon(String key) {
    switch (key) {
      case 'finance_invoicing':
        return Icons.account_balance_wallet_outlined;
      case 'client_operations':
        return Icons.groups_2_outlined;
      case 'marketing':
        return Icons.campaign_outlined;
      case 'website_events':
        return Icons.language_outlined;
      case 'security_system':
        return Icons.security_outlined;
      default:
        return Icons.category_outlined;
    }
  }

  Widget topicGroupCard(Map<String, dynamic> group) {
    final key = s(group['group_key']);
    final meta = topicByKey(key) ?? group;
    final moduleCount = (meta['module_count'] as num?)?.toInt() ?? 0;
    final liveReady = (meta['live_ready'] as num?)?.toInt() ?? 0;
    final inDevelopment = (meta['in_development'] as num?)?.toInt() ?? 0;
    final assignments = (meta['active_partner_assignments'] as num?)?.toInt() ?? 0;
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => openTopic(key),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(children: [
              Container(
                width: 46,
                height: 46,
                decoration: BoxDecoration(
                  color: brandNavy.withOpacity(.07),
                  borderRadius: BorderRadius.circular(13),
                ),
                child: Icon(groupIcon(key), color: brandNavy, size: 23),
              ),
              const Spacer(),
              const Icon(Icons.arrow_forward_rounded, color: brandGold, size: 19),
            ]),
            const SizedBox(height: 18),
            LText(
              groupLabel(group),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: brandNavy, fontSize: 17, fontWeight: FontWeight.w800),
            ),
            const SizedBox(height: 6),
            LText(
              '$moduleCount ${uiLiteral('modules')}',
              style: const TextStyle(color: brandTextSoft, fontSize: 10.5, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 16),
            Wrap(spacing: 7, runSpacing: 7, children: [
              _StatusPill(label: '$liveReady ${uiLiteral('live ready')}'),
              if (inDevelopment > 0) _StatusPill(label: '$inDevelopment ${uiLiteral('in development')}'),
            ]),
            const SizedBox(height: 12),
            LText(
              '$assignments ${uiLiteral('active partner assignments')}',
              style: const TextStyle(color: brandTextSoft, fontSize: 9.5),
            ),
          ]),
        ),
      ),
    );
  }

  Widget moduleCard(Map<String, dynamic> module) {
    final activePartners = (module['active_partner_count'] as num?)?.toInt() ?? 0;
    final availability = s(module['availability']).isEmpty ? 'ACTIVE' : s(module['availability']);
    final publication = s(module['publication_status']).isEmpty ? 'UNPUBLISHED' : s(module['publication_status']);
    final implementation = s(module['implementation_state']).isEmpty ? 'IN_DEVELOPMENT' : s(module['implementation_state']);
    final ready = availability == 'ACTIVE' && publication == 'PUBLISHED' && implementation == 'READY';

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => manageModule(module),
        child: Padding(
          padding: const EdgeInsets.all(17),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: ready ? brandSuccess.withOpacity(.08) : brandNavy.withOpacity(.07),
                  borderRadius: BorderRadius.circular(11),
                ),
                child: Icon(Icons.extension_outlined, color: ready ? brandSuccess : brandNavy),
              ),
              const SizedBox(width: 11),
              Expanded(
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  LText(
                    moduleLabelForLocale(module),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 13.5),
                  ),
                  const SizedBox(height: 3),
                  LText(
                    s(module['key']),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: brandTextSoft, fontSize: 9),
                  ),
                ]),
              ),
              PopupMenuButton<String>(
                tooltip: uiLiteral('Module actions'),
                onSelected: (value) {
                  if (value == 'details') {
                    manageModule(module);
                  } else if (value == 'edit') {
                    editModule(module);
                  } else if (value.startsWith('move:')) {
                    moveModuleToGroup(module, value.substring(5));
                  }
                },
                itemBuilder: (_) => [
                  PopupMenuItem(value: 'details', child: LText(uiLiteral('Open details'))),
                  PopupMenuItem(value: 'edit', child: LText(uiLiteral('Edit module'))),
                  const PopupMenuDivider(),
                  for (final group in primaryGroups)
                    if (s(group['group_key']) != s(module['group_key']))
                      PopupMenuItem(
                        value: 'move:' + s(group['group_key']),
                        child: LText('${uiLiteral('Move to')} ${groupLabel(group)}'),
                      ),
                ],
              ),
            ]),
            const SizedBox(height: 14),
            Wrap(spacing: 7, runSpacing: 7, children: [
              _StatusPill(label: ready ? 'ACTIVE' : availability),
              _StatusPill(label: publication),
              if (implementation != 'READY') _StatusPill(label: implementation),
            ]),
            const SizedBox(height: 14),
            Row(children: [
              const Icon(Icons.business_outlined, size: 16, color: brandSteel),
              const SizedBox(width: 6),
              Expanded(
                child: LText(
                  '$activePartners ${uiLiteral('active partners')}',
                  style: const TextStyle(color: brandTextSoft, fontSize: 10, fontWeight: FontWeight.w600),
                ),
              ),
              const Icon(Icons.arrow_forward_rounded, size: 16, color: brandGold),
            ]),
          ]),
        ),
      ),
    );
  }

  String planMoney(dynamic value) => intl.NumberFormat.currency(symbol: '\$', decimalDigits: 0).format(number(value));

  String moduleLabel(String key) {
    for (final module in modules) {
      if (s(module['key']) == key) return s(module['label']);
    }
    return key;
  }

  Future<void> configureFixedPlan(Map<String, dynamic> plan) async {
    final limit = (plan['module_limit'] as num?)?.toInt() ?? 0;
    final selected = <String>{
      ...((plan['fixed_module_keys'] is List)
          ? (plan['fixed_module_keys'] as List).map((e) => e.toString())
          : const <String>[]),
    };
    final candidates = modules.where((m) =>
      s(m['publication_status']) == 'PUBLISHED' &&
      s(m['implementation_state']) == 'READY'
    ).toList();

    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => AlertDialog(
          title: LText('Configure ${s(plan['display_name'])} modules'),
          content: SizedBox(
            width: 580,
            child: SingleChildScrollView(
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                LText(
                  'Select exactly $limit PUBLISHED + READY modules. Existing package changes take effect from the next calendar month.',
                  style: const TextStyle(color: brandTextSoft, fontSize: 10.5),
                ),
                const SizedBox(height: 12),
                for (final module in candidates)
                  CheckboxListTile(
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    value: selected.contains(s(module['key'])),
                    title: LText(s(module['label'])),
                    subtitle: LText(s(module['group_label']), style: const TextStyle(color: brandTextSoft, fontSize: 9)),
                    onChanged: (value) => setLocal(() {
                      final key = s(module['key']);
                      if (value == true) {
                        if (selected.length < limit) selected.add(key);
                      } else {
                        selected.remove(key);
                      }
                    }),
                  ),
                const SizedBox(height: 8),
                LText(
                  '${selected.length} / $limit selected',
                  style: TextStyle(
                    color: selected.length == limit ? brandSuccess : brandWarning,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ]),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const LText('Cancel')),
            FilledButton(
              onPressed: selected.length == limit ? () => Navigator.pop(dialogContext, true) : null,
              child: const LText('Save package'),
            ),
          ],
        ),
      ),
    );
    if (ok != true) return;

    try {
      await widget.api.patch('/api/v1/billing/plans/${s(plan['plan_key'])}', {
        'fixed_module_keys': selected.toList()..sort(),
      });
      await load();
      if (mounted) notify('${s(plan['display_name'])} package updated.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> editPlanPrice(Map<String, dynamic> plan) async {
    final monthly = TextEditingController(text: number(plan['monthly_price']).toStringAsFixed(0));
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'Edit ${s(plan['display_name'])} pricing',
        subtitle: 'Package prices are net. VAT is added from the HIMATE billing profile at invoice and payment time.',
        icon: Icons.price_change_outlined,
        width: 520,
        child: TextField(
          controller: monthly,
          keyboardType: const TextInputType.numberWithOptions(decimal: true),
          decoration: InputDecoration(labelText: uiLiteral('Net monthly price (USD)')),
        ),
        primaryLabel: 'Save net price',
        onPrimary: () => Navigator.pop(dialogContext, true),
      ),
    );
    if (ok != true) {
      monthly.dispose();
      return;
    }
    final value = double.tryParse(monthly.text.trim().replaceAll(',', '.'));
    monthly.dispose();
    if (value == null || value < 0) {
      notify('Enter a valid non-negative package price.', failure: true);
      return;
    }
    try {
      await widget.api.patch('/api/v1/billing/plans/${s(plan['plan_key'])}', {
        'monthly_price': value,
        'reason': 'Central-5 administrator package price update',
      });
      await load();
      if (mounted) notify('${s(plan['display_name'])} net price updated.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> showPackageDetails(Map<String, dynamic> plan) async {
    final unlimited = plan['unlimited_modules'] == true || s(plan['selection_mode']) == 'UNLIMITED';
    final keys = plan['fixed_module_keys'] is List
        ? (plan['fixed_module_keys'] as List).map((e) => e.toString()).toList()
        : <String>[];
    final availableNow = modules.where((m) =>
      s(m['availability']) == 'ACTIVE' &&
      s(m['publication_status']) == 'PUBLISHED' &&
      s(m['implementation_state']) == 'READY'
    ).length;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: '${s(plan['display_name'])} package',
        subtitle: unlimited
            ? '$availableNow modules available today + every future eligible HIMATE module.'
            : 'HIMATE-managed package with an authoritative fixed module set.',
        icon: unlimited ? Icons.all_inclusive_rounded : Icons.inventory_2_outlined,
        width: 720,
        child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          ResponsiveFieldPair(
            first: _DefinitionRow(label: 'Net monthly price', value: '${planMoney(plan['monthly_net_price'] ?? plan['monthly_price'])} + ${s(plan['tax_label']).isEmpty ? 'VAT' : s(plan['tax_label'])}'),
            second: _DefinitionRow(label: 'Current VAT rate', value: '${number(plan['vat_rate_percent']).toStringAsFixed(2)}%'),
          ),
          const SizedBox(height: 10),
          ResponsiveFieldPair(
            first: _DefinitionRow(label: 'Active partners', value: '${plan['active_partner_count'] ?? 0}'),
            second: _DefinitionRow(label: 'Module entitlement', value: unlimited ? 'Unlimited' : '${plan['module_limit'] ?? 0} fixed modules'),
          ),
          if (unlimited) ...[
            const SizedBox(height: 14),
            _MessageCard(
              icon: Icons.auto_awesome_outlined,
              title: 'Unlimited by rule — not by a stored module count',
              message: '$availableNow modules are eligible today. Newly released modules enter Premium entitlement automatically without changing the package configuration.',
            ),
          ] else ...[
            const SizedBox(height: 14),
            LText('Included modules', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 12)),
            const SizedBox(height: 8),
            Wrap(spacing: 6, runSpacing: 6, children: [for (final key in keys) Chip(label: LText(moduleLabel(key)))]),
          ],
        ]),
        primaryLabel: 'Close',
        onPrimary: () => Navigator.pop(dialogContext),
      ),
    );
  }

  Widget subscriptionPlanCard(Map<String, dynamic> plan) {
    final annualList = number(plan['annual_list_price']);
    final annual = number(plan['annual_price']);
    final savings = number(plan['annual_savings']);
    final fixed = s(plan['selection_mode']) == 'FIXED';
    final unlimited = plan['unlimited_modules'] == true || s(plan['selection_mode']) == 'UNLIMITED';
    final ready = plan['ready'] == true;
    final keys = plan['fixed_module_keys'] is List
        ? (plan['fixed_module_keys'] as List).map((e) => e.toString()).toList()
        : <String>[];
    final availableNow = modules.where((m) =>
      s(m['availability']) == 'ACTIVE' &&
      s(m['publication_status']) == 'PUBLISHED' &&
      s(m['implementation_state']) == 'READY'
    ).length;
    final taxLabel = s(plan['tax_label']).isEmpty ? 'VAT' : s(plan['tax_label']);

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Expanded(
              child: LText(
                s(plan['display_name']),
                style: const TextStyle(color: brandNavy, fontSize: 18, fontWeight: FontWeight.w800),
              ),
            ),
            _StatusPill(label: unlimited ? 'UNLIMITED' : ready ? 'READY' : 'SETUP REQUIRED'),
          ]),
          const SizedBox(height: 8),
          LText(
            '${planMoney(plan['monthly_net_price'] ?? plan['monthly_price'])} / month + $taxLabel',
            style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 15),
          ),
          const SizedBox(height: 3),
          LText(
            'Current $taxLabel: ${number(plan['vat_rate_percent']).toStringAsFixed(2)}% · net price',
            style: const TextStyle(color: brandTextSoft, fontSize: 9.8),
          ),
          const SizedBox(height: 10),
          if (annual < annualList)
            Text(
              planMoney(annualList),
              style: const TextStyle(
                color: brandTextSoft,
                decoration: TextDecoration.lineThrough,
                fontWeight: FontWeight.w700,
              ),
            ),
          LText(
            '${planMoney(annual)} / year net',
            style: const TextStyle(color: brandGold, fontWeight: FontWeight.w800, fontSize: 16),
          ),
          if (savings > 0)
            LText(
              'Save ${planMoney(savings)} with annual prepayment',
              style: const TextStyle(color: brandSuccess, fontSize: 10, fontWeight: FontWeight.w700),
            ),
          const SizedBox(height: 14),
          _DefinitionRow(
            label: 'Module capacity',
            value: unlimited ? 'Unlimited' : '${plan['module_limit'] ?? 0}',
          ),
          _DefinitionRow(
            label: 'Entitlement model',
            value: unlimited ? 'All current + future eligible modules' : 'Fixed by HIMATE',
          ),
          _DefinitionRow(label: 'Active partners', value: '${plan['active_partner_count'] ?? 0}'),
          if (unlimited)
            _DefinitionRow(label: 'Available today', value: '$availableNow + all future modules')
          else
            _DefinitionRow(
              label: 'Configured modules',
              value: '${keys.length} / ${plan['module_limit'] ?? 0}',
            ),
          if (fixed && keys.isNotEmpty) ...[
            const SizedBox(height: 8),
            Wrap(
              spacing: 6,
              runSpacing: 6,
              children: [for (final key in keys) Chip(label: LText(moduleLabel(key)))],
            ),
          ],
          if (unlimited) ...[
            const SizedBox(height: 10),
            _MessageCard(
              icon: Icons.all_inclusive_rounded,
              title: 'Premium grows automatically',
              message: '$availableNow modules are available today. Every newly released eligible module is included automatically.',
            ),
          ],
          const SizedBox(height: 16),
          Wrap(spacing: 8, runSpacing: 8, children: [
            OutlinedButton.icon(
              onPressed: () => showPackageDetails(plan),
              icon: const Icon(Icons.open_in_new_rounded, size: 17),
              label: const LText('Package details'),
            ),
            OutlinedButton.icon(
              onPressed: () => editPlanPrice(plan),
              icon: const Icon(Icons.price_change_outlined, size: 17),
              label: const LText('Edit net price'),
            ),
            if (fixed)
              FilledButton.icon(
                onPressed: () => configureFixedPlan(plan),
                icon: const Icon(Icons.tune_rounded),
                label: LText(ready ? 'Change included modules' : 'Configure included modules'),
              ),
          ]),
        ]),
      ),
    );
  }

  Widget subscriptionPlansPage() {
    final visible = subscriptionPlans.where((p) => s(p['plan_key']) != 'CUSTOM').toList();
    return Content(
      eyebrow: 'CENTRAL-5 · COMMERCIAL PACKAGING',
      title: 'Packages',
      subtitle: 'Starter includes 10 HIMATE-defined modules, Business includes 20, and Premium provides unlimited access to every current and future eligible module.',
      actions: [
        OutlinedButton.icon(
          onPressed: () => setState(() => showSubscriptionPlans = false),
          icon: const Icon(Icons.hub_outlined),
          label: const LText('Back to Modules'),
        ),
        OutlinedButton.icon(
          onPressed: loading ? null : load,
          icon: const Icon(Icons.refresh_rounded),
          label: const LText('Refresh'),
        ),
      ],
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        _MessageCard(
          icon: Icons.payments_outlined,
          title: 'Central-5 pricing authority',
          message: 'Starter: USD 990/month + VAT · Business: USD 1,490/month + VAT · Premium: USD 2,490/month + VAT. VAT is controlled from the HIMATE billing profile and is currently ${subscriptionPlans.isEmpty ? '0' : number(subscriptionPlans.first['vat_rate_percent']).toStringAsFixed(2)}%.',
        ),
        const SizedBox(height: 18),
        LayoutBuilder(builder: (context, constraints) {
          final width = constraints.maxWidth < 680
              ? constraints.maxWidth
              : constraints.maxWidth < 1120
                  ? (constraints.maxWidth - 12) / 2
                  : (constraints.maxWidth - 24) / 3;
          return Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [for (final plan in visible) SizedBox(width: width, child: subscriptionPlanCard(plan))],
          );
        }),
        if (loading) ...[
          const SizedBox(height: 12),
          const LinearProgressIndicator(minHeight: 2),
        ],
      ]),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (showSubscriptionPlans) return subscriptionPlansPage();

    final registryTotal = (registryKpis['module_registry'] as num?)?.toInt() ?? modules.length;
    final liveReady = (registryKpis['active_modules'] as num?)?.toInt() ?? 0;
    final linked = (registryKpis['source_linked'] as num?)?.toInt() ?? 0;
    final relations = (registryKpis['relationships'] as num?)?.toInt() ?? 0;
    final partnerUsage = (registryKpis['active_partner_assignments'] as num?)?.toInt() ?? 0;
    final currentGroup = selectedGroupKey == null ? null : groupByKey(selectedGroupKey!);
    final topicOverview = selectedGroupKey == null && registryPreset == 'TOPICS';

    String registryTitle = uiLiteral('Module Topics');
    String registrySubtitle = uiLiteral('Open a topic to see its modules. Module cards can be moved to another topic from their action menu.');
    if (currentGroup != null) {
      registryTitle = groupLabel(currentGroup);
      registrySubtitle = uiLiteral('Topic modules. Open a card for usage, dependencies and impact details.');
    } else if (registryPreset == 'ACTIVE') {
      registryTitle = uiLiteral('Active Modules');
      registrySubtitle = uiLiteral('READY + PUBLISHED modules currently available for live assignment.');
    } else if (registryPreset == 'SOURCE_LINKED') {
      registryTitle = uiLiteral('Source Linked');
      registrySubtitle = uiLiteral('Modules with an authoritative Git/source identity configured.');
    } else if (registryPreset == 'RELATIONSHIPS') {
      registryTitle = uiLiteral('Module Relationships');
      registrySubtitle = uiLiteral('Modules participating in dependency, integration, extension, conflict or replacement relationships.');
    }

    return Content(
      eyebrow: uiLiteral('MODULE CONTROL PLANE'),
      title: uiLiteral('Modules'),
      subtitle: uiLiteral('Topic-driven module registry, partner usage and commercial control in one authoritative workspace.'),
      actions: [
        OutlinedButton.icon(
          onPressed: () => setState(() => showSubscriptionPlans = true),
          icon: const Icon(Icons.workspace_premium_outlined),
          label: LText(uiLiteral('Packages')),
        ),
        OutlinedButton.icon(
          onPressed: () => setState(() => showCommercialMatrix = !showCommercialMatrix),
          icon: Icon(showCommercialMatrix ? Icons.expand_less_rounded : Icons.price_change_outlined),
          label: LText(uiLiteral(showCommercialMatrix ? 'Hide Commercial Matrix' : 'Commercial Matrix')),
        ),
        OutlinedButton.icon(
          onPressed: loading ? null : addGroup,
          icon: const Icon(Icons.category_outlined),
          label: LText(uiLiteral('Add group')),
        ),
        FilledButton.icon(
          onPressed: loading || groups.isEmpty ? null : addModule,
          icon: const Icon(Icons.add_box_outlined),
          label: LText(uiLiteral('Add module')),
        ),
      ],
      child: error != null && modules.isEmpty
          ? _MessageCard(icon: Icons.cloud_off_outlined, title: uiLiteral('Module Control Plane unavailable'), message: error!)
          : Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              ResponsiveKpiGrid(children: [
                Kpi(
                  label: uiLiteral('Module registry'),
                  value: registryTotal.toString(),
                  note: uiLiteral('Canonical + custom modules'),
                  icon: Icons.hub_outlined,
                  accent: brandNavy,
                  onTap: showTopicOverview,
                ),
                Kpi(
                  label: uiLiteral('Active modules'),
                  value: liveReady.toString(),
                  note: uiLiteral('READY + PUBLISHED for live assignment'),
                  icon: Icons.check_circle_outline_rounded,
                  accent: brandSuccess,
                  onTap: () => applyRegistryPreset('ACTIVE'),
                ),
                Kpi(
                  label: uiLiteral('Source linked'),
                  value: linked.toString(),
                  note: uiLiteral('Git/source identity configured'),
                  icon: Icons.code_outlined,
                  accent: brandSteel,
                  onTap: () => applyRegistryPreset('SOURCE_LINKED'),
                ),
                Kpi(
                  label: uiLiteral('Relationships'),
                  value: relations.toString(),
                  note: '$partnerUsage ${uiLiteral('active partner assignments')}',
                  icon: Icons.account_tree_outlined,
                  accent: brandGold,
                  onTap: () => applyRegistryPreset('RELATIONSHIPS'),
                ),
              ]),
              const SizedBox(height: 24),
              Row(crossAxisAlignment: CrossAxisAlignment.center, children: [
                if (!topicOverview) ...[
                  IconButton(
                    tooltip: uiLiteral('Back to module topics'),
                    onPressed: showTopicOverview,
                    icon: const Icon(Icons.arrow_back_rounded),
                  ),
                  const SizedBox(width: 4),
                ],
                Expanded(
                  child: _SectionHeader(
                    title: registryTitle,
                    subtitle: registrySubtitle,
                    trailing: _MiniCounter(
                      label: topicOverview
                          ? '${topicRows.length} ${uiLiteral('topics')} · $registryTotal ${uiLiteral('modules')}'
                          : '${filtered.length} ${uiLiteral('modules')}',
                    ),
                  ),
                ),
              ]),
              const SizedBox(height: 12),
              if (topicOverview)
                LayoutBuilder(builder: (context, constraints) {
                  final width = constraints.maxWidth < 640
                      ? constraints.maxWidth
                      : constraints.maxWidth < 1060
                          ? (constraints.maxWidth - 12) / 2
                          : (constraints.maxWidth - 24) / 3;
                  return Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      for (final group in topicRows)
                        SizedBox(width: width, child: topicGroupCard(group)),
                    ],
                  );
                })
              else ...[
                _FilterSurface(
                  child: LayoutBuilder(builder: (context, constraints) {
                    final search = TextField(
                      onChanged: (value) {
                        setState(() => query = value);
                        _scheduleRegistryReload();
                      },
                      decoration: InputDecoration(
                        hintText: uiLiteral('Search modules...'),
                        prefixIcon: const Icon(Icons.search_rounded),
                      ),
                    );
                    final type = DropdownButtonFormField<String>(
                      value: typeFilter,
                      decoration: InputDecoration(labelText: uiLiteral('Type')),
                      items: [
                        DropdownMenuItem(value: 'ALL', child: LText(uiLiteral('All types'))),
                        for (final value in moduleTypes)
                          DropdownMenuItem(value: value, child: LText(uiLiteral(_humanize(value)))),
                      ],
                      onChanged: (value) {
                        setState(() => typeFilter = value ?? 'ALL');
                        unawaited(load());
                      },
                    );
                    if (constraints.maxWidth < 760) {
                      return Column(children: [search, const SizedBox(height: 10), type]);
                    }
                    return Row(children: [
                      Expanded(flex: 2, child: search),
                      const SizedBox(width: 10),
                      Expanded(child: type),
                    ]);
                  }),
                ),
                const SizedBox(height: 12),
                if (currentGroup != null && primaryGroups.length > 1) ...[
                  Wrap(
                    spacing: 7,
                    runSpacing: 7,
                    children: [
                      for (final group in primaryGroups)
                        if (s(group['group_key']) != selectedGroupKey)
                          ActionChip(
                            avatar: Icon(groupIcon(s(group['group_key'])), size: 15),
                            label: LText('${uiLiteral('Browse')} ${groupLabel(group)}'),
                            onPressed: () => openTopic(s(group['group_key'])),
                          ),
                    ],
                  ),
                  const SizedBox(height: 12),
                ],
                if (filtered.isEmpty)
                  _MessageCard(
                    icon: Icons.inventory_2_outlined,
                    title: uiLiteral('No modules found'),
                    message: uiLiteral('No modules match the current topic or filters.'),
                  )
                else
                  LayoutBuilder(builder: (context, constraints) {
                    final width = constraints.maxWidth < 650
                        ? constraints.maxWidth
                        : constraints.maxWidth < 1050
                            ? (constraints.maxWidth - 12) / 2
                            : (constraints.maxWidth - 24) / 3;
                    return Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [
                        for (final module in filtered)
                          SizedBox(width: width, child: moduleCard(module)),
                      ],
                    );
                  }),
              ],
              const SizedBox(height: 28),
              _SectionHeader(
                title: uiLiteral('Partner × Module Commercial Matrix'),
                subtitle: uiLiteral('Partner-specific assignment, recurring price, activation fee and subscription state remain available without dominating the registry view.'),
                trailing: OutlinedButton.icon(
                  onPressed: () => setState(() => showCommercialMatrix = !showCommercialMatrix),
                  icon: Icon(showCommercialMatrix ? Icons.expand_less_rounded : Icons.expand_more_rounded),
                  label: LText(uiLiteral(showCommercialMatrix ? 'Hide matrix' : 'Open matrix')),
                ),
              ),
              if (showCommercialMatrix) ...[
                const SizedBox(height: 12),
                _FilterSurface(
                  child: LayoutBuilder(builder: (context, constraints) {
                    final search = TextField(
                      onChanged: (value) {
                        setState(() { commercialQuery = value; commercialShown = 120; });
                        _scheduleCommercialReload();
                      },
                      decoration: InputDecoration(hintText: uiLiteral('Search partner or module...'), prefixIcon: const Icon(Icons.search_rounded)),
                    );
                    final partner = DropdownButtonFormField<String>(
                      value: commercialPartnerFilter,
                      decoration: InputDecoration(labelText: uiLiteral('Partner')),
                      items: [
                        DropdownMenuItem(value: 'ALL', child: LText(uiLiteral('All partners'))),
                        for (final item in partners)
                          DropdownMenuItem(value: s(item['id']), child: LText(partnerName(s(item['id'])))),
                      ],
                      onChanged: (value) {
                        setState(() { commercialPartnerFilter = value ?? 'ALL'; commercialShown = 120; });
                        unawaited(load());
                      },
                    );
                    final module = DropdownButtonFormField<String>(
                      value: commercialModuleFilter,
                      decoration: InputDecoration(labelText: uiLiteral('Module')),
                      items: [
                        DropdownMenuItem(value: 'ALL', child: LText(uiLiteral('All modules'))),
                        for (final item in modules)
                          DropdownMenuItem(value: s(item['key']), child: LText(moduleLabelForLocale(item))),
                      ],
                      onChanged: (value) {
                        setState(() { commercialModuleFilter = value ?? 'ALL'; commercialShown = 120; });
                        unawaited(load());
                      },
                    );
                    final status = DropdownButtonFormField<String>(
                      value: commercialStatusFilter,
                      decoration: InputDecoration(labelText: uiLiteral('State')),
                      items: [
                        DropdownMenuItem(value: 'ALL', child: LText(uiLiteral('All states'))),
                        DropdownMenuItem(value: 'ACTIVE', child: LText(uiLiteral('Active'))),
                        DropdownMenuItem(value: 'NOT_LICENSED', child: LText(uiLiteral('Not licensed'))),
                        DropdownMenuItem(value: 'MAINTENANCE', child: LText(uiLiteral('Maintenance'))),
                      ],
                      onChanged: (value) {
                        setState(() { commercialStatusFilter = value ?? 'ALL'; commercialShown = 120; });
                        unawaited(load());
                      },
                    );
                    final perspective = Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        ChoiceChip(
                          selected: commercialPerspective == 'PARTNER',
                          label: LText(uiLiteral('View by partner')),
                          onSelected: (_) {
                            setState(() { commercialPerspective = 'PARTNER'; commercialShown = 120; });
                            unawaited(load());
                          },
                        ),
                        ChoiceChip(
                          selected: commercialPerspective == 'MODULE',
                          label: LText(uiLiteral('View by module')),
                          onSelected: (_) {
                            setState(() { commercialPerspective = 'MODULE'; commercialShown = 120; });
                            unawaited(load());
                          },
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
                    return _MessageCard(
                      icon: Icons.price_change_outlined,
                      title: uiLiteral('No partner-module assignments found'),
                      message: uiLiteral('Adjust the filters or create partners/modules to populate the commercial matrix.'),
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
                        children: [
                          for (final row in visibleRows)
                            SizedBox(width: width, child: commercialCard(row)),
                        ],
                      );
                    }),
                    if (visibleRows.length < rows.length) ...[
                      const SizedBox(height: 12),
                      OutlinedButton.icon(
                        onPressed: () => setState(() => commercialShown += 120),
                        icon: const Icon(Icons.expand_more_rounded),
                        label: LText(
                          '${uiLiteral('Show more')} · ${rows.length - visibleRows.length} ${uiLiteral('remaining')}',
                        ),
                      ),
                    ],
                  ]);
                }),
              ],
              if (loading) ...[
                const SizedBox(height: 12),
                const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
              ],
            ]),
    );
  }
}
