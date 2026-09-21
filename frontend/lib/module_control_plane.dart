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
  bool loading = false;
  String? error;
  String query = '';
  String groupFilter = 'ALL';
  String typeFilter = 'ALL';

  static const moduleTypes = <String>[
    'CORE','FEATURE','INTEGRATION','REPORTING','WEBSITE','FINANCE','INFRASTRUCTURE',
  ];
  static const relationshipTypes = <String>[
    'REQUIRES','OPTIONAL_DEPENDENCY','INTEGRATES_WITH','EXTENDS','CONFLICTS_WITH','REPLACES',
  ];

  String s(dynamic value) => value == null ? '' : value.toString();

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
      ]);
      if (!mounted) return;
      setState(() {
        modules = items(responses[0]);
        groups = items(responses[1]);
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

  Future<void> addGroup() async {
    final key = TextEditingController();
    final label = TextEditingController();
    final order = TextEditingController(text: (groups.length + 1).toString());
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => BrandDialog(
        title: 'Create module group',
        subtitle: 'Create a stable classification used by the HIMATE module registry.',
        icon: Icons.category_outlined,
        width: 560,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          ResponsiveFieldPair(
            first: TextField(controller: label, decoration: InputDecoration(labelText: uiLiteral('Group name *'))),
            second: TextField(controller: key, decoration: InputDecoration(labelText: uiLiteral('Stable group key *'))),
          ),
          const SizedBox(height: 12),
          TextField(controller: order, keyboardType: TextInputType.number, decoration: InputDecoration(labelText: uiLiteral('Sort order'))),
        ]),
        primaryLabel: 'Create group',
        onPrimary: () => Navigator.pop(context, true),
      ),
    );
    if (ok == true && key.text.trim().isNotEmpty && label.text.trim().isNotEmpty) {
      try {
        await widget.api.post('/api/v1/module-groups', {
          'group_key': key.text.trim().toLowerCase(),
          'label': label.text.trim(),
          'sort_order': int.tryParse(order.text) ?? groups.length + 1,
        });
        await load();
        if (mounted) notify('Module group created.');
      } catch (e) {
        if (mounted) notify(e.toString(), failure: true);
      }
    }
    key.dispose(); label.dispose(); order.dispose();
  }

  Future<Map<String, dynamic>?> moduleDialog({Map<String, dynamic>? module}) async {
    if (groups.isEmpty) return null;
    final editing = module != null;
    final label = TextEditingController(text: s(module?['label']));
    final key = TextEditingController(text: s(module?['key']));
    final description = TextEditingController(text: s(module?['description']));
    final price = TextEditingController(text: number(module?['default_monthly_price']).toStringAsFixed(2));
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
    String? dialogError;

    final result = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: editing ? 'Edit module registry' : 'Create module',
          subtitle: 'Business metadata, source identity, artifact reference and compatibility.',
          icon: Icons.hub_outlined,
          width: 860,
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            const _DialogSectionLabel('IDENTITY & COMMERCIAL'),
            const SizedBox(height: 10),
            ResponsiveFieldPair(
              first: TextField(controller: label, decoration: InputDecoration(labelText: uiLiteral('Module name *'))),
              second: TextField(controller: key, readOnly: editing, decoration: InputDecoration(labelText: uiLiteral('Stable module key *'))),
            ),
            const SizedBox(height: 12),
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
              second: DropdownButtonFormField<String>(
                value: availability,
                decoration: InputDecoration(labelText: uiLiteral('Availability')),
                items: const [
                  DropdownMenuItem(value: 'ACTIVE', child: LText('ACTIVE')),
                  DropdownMenuItem(value: 'UNAVAILABLE', child: LText('UNAVAILABLE')),
                  DropdownMenuItem(value: 'DEPRECATED', child: LText('DEPRECATED')),
                ],
                onChanged: (value) { if (value != null) setLocal(() => availability = value); },
              ),
            ),
            const SizedBox(height: 12),
            TextField(controller: description, maxLines: 3, decoration: InputDecoration(labelText: uiLiteral('Description'))),
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
            if (label.text.trim().isEmpty || key.text.trim().isEmpty) {
              setLocal(() => dialogError = 'Module name and stable key are required.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              if (!editing) 'key': key.text.trim().toLowerCase(),
              'label': label.text.trim(),
              'group_key': group,
              'description': description.text.trim(),
              'currency': s(module?['currency']).isEmpty ? 'USD' : s(module?['currency']),
              if (!editing) 'version': latestVersion.text.trim().isEmpty ? '1.0.0' : latestVersion.text.trim(),
              'latest_version': latestVersion.text.trim().isEmpty ? '1.0.0' : latestVersion.text.trim(),
              'default_monthly_price': double.tryParse(price.text) ?? 0,
              'availability': availability,
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
    for (final controller in [label,key,description,price,owner,repo,path,sourceRef,commit,artifactType,artifactReference,latestVersion,minPlatform]) {
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
                                    title: s(item['partner_id']),
                                    icon: Icons.business_outlined,
                                    children: [
                                      _DefinitionRow(label: 'Status', value: s(item['status'])),
                                      _DefinitionRow(label: 'Included in base', value: item['included_in_base'] == true ? 'Yes' : 'No'),
                                      _DefinitionRow(label: 'Price override', value: money(item['price_override'])),
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
            _StatusPill(label: s(module['availability']).isEmpty ? 'ACTIVE' : s(module['availability'])),
          ]),
          const SizedBox(height: 14),
          _DefinitionRow(label: 'Group', value: s(module['group_label']).isEmpty ? s(module['group_key']) : s(module['group_label'])),
          _DefinitionRow(label: 'Type', value: _humanize(s(module['module_type']).isEmpty ? 'FEATURE' : s(module['module_type']))),
          _DefinitionRow(label: 'Default 30-day price', value: (s(module['currency']).isEmpty ? 'USD' : s(module['currency'])) + ' ' + number(module['default_monthly_price']).toStringAsFixed(2)),
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
          const Spacer(),
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
      subtitle: 'Authoritative registry, source identity, releases, dependencies, impact mapping and partner usage.',
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
                    for (final module in filtered) SizedBox(width: width, height: 390, child: moduleCard(module)),
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
