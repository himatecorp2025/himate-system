part of 'main.dart';

class PartnerConnectionsPanel extends StatefulWidget {
  const PartnerConnectionsPanel({required this.api, super.key});
  final Api api;

  @override
  State<PartnerConnectionsPanel> createState() => _PartnerConnectionsPanelState();
}

class _PartnerConnectionsPanelState extends State<PartnerConnectionsPanel> {
  final TextEditingController search = TextEditingController();
  Timer? debounce;
  List<Map<String, dynamic>> partners = <Map<String, dynamic>>[];
  Map<String, dynamic> kpis = <String, dynamic>{};
  bool loading = true;
  String status = 'ALL';
  String? error;

  @override
  void initState() {
    super.initState();
    load();
  }

  @override
  void dispose() {
    debounce?.cancel();
    search.dispose();
    super.dispose();
  }

  String path() {
    if (search.text.trim().isEmpty && status == 'ALL') {
      return centralConnectionsInitialPath();
    }
    final params = <String, String>{'limit': '120', 'offset': '0'};
    if (search.text.trim().isNotEmpty) params['q'] = search.text.trim();
    if (status != 'ALL') params['status'] = status;
    return Uri(path: '/api/v1/central/connections', queryParameters: params).toString();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final model = await widget.api.get(path(), force: true, maxAge: const Duration(seconds: 20));
      if (!mounted) return;
      setState(() {
        partners = items(model);
        kpis = model['kpis'] is Map ? Map<String, dynamic>.from(model['kpis'] as Map) : <String, dynamic>{};
      });
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void searchChanged(String _) {
    debounce?.cancel();
    debounce = Timer(const Duration(milliseconds: 280), load);
  }

  String timestamp(dynamic value) {
    final raw = (value ?? '').toString().trim();
    if (raw.isEmpty || raw == 'null') return 'Never';
    final parsed = DateTime.tryParse(raw);
    return parsed == null ? raw : HimateI18n.dateTime(HimateI18n.activeLocale, parsed);
  }

  Color statusColor(String value) {
    switch (value) {
      case 'ACTIVE': return brandSuccess;
      case 'SUSPENDED': return brandWarning;
      case 'DELETED': return brandDanger;
      default: return brandSteel;
    }
  }

  Future<void> showIntegrations(Map<String, dynamic> partner) async {
    final integrations = items(<String, dynamic>{'items': partner['integrations']});
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => Dialog(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: 760, maxHeight: MediaQuery.sizeOf(dialogContext).height * .82),
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 18, 12, 14),
                child: Row(
                  children: [
                    const Icon(Icons.hub_outlined, color: brandGold),
                    const SizedBox(width: 10),
                    Expanded(
                      child: LText(
                        'Integrations · ${partner['partner_name'] ?? partner['partner_id']}',
                        style: Theme.of(dialogContext).textTheme.titleLarge,
                      ),
                    ),
                    IconButton(onPressed: () => Navigator.pop(dialogContext), icon: const Icon(Icons.close_rounded)),
                  ],
                ),
              ),
              const Divider(height: 1),
              Expanded(
                child: integrations.isEmpty
                    ? const Center(
                        child: Padding(
                          padding: EdgeInsets.all(24),
                          child: _MessageCard(
                            icon: Icons.link_off_rounded,
                            title: 'No configured integrations',
                            message: 'This partner currently has no Connector credential or Website Adapter configuration.',
                          ),
                        ),
                      )
                    : ListView.separated(
                        padding: const EdgeInsets.all(18),
                        itemCount: integrations.length,
                        separatorBuilder: (_, __) => const Divider(height: 24),
                        itemBuilder: (_, index) {
                          final integration = integrations[index];
                          final type = '${integration['type'] ?? 'UNKNOWN'}';
                          final label = '${integration['label'] ?? type}';
                          return Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                children: [
                                  Expanded(child: LText(label, style: const TextStyle(fontWeight: FontWeight.w700))),
                                  _StatusPill(label: integration['enabled'] == true ? 'ACTIVE' : 'INACTIVE'),
                                ],
                              ),
                              const SizedBox(height: 8),
                              _DefinitionRow(label: 'Type', value: type),
                              _DefinitionRow(label: 'Environment', value: '${integration['environment'] ?? '—'}'),
                              if ('${integration['health'] ?? ''}'.isNotEmpty)
                                _DefinitionRow(label: 'Health', value: '${integration['health']}'),
                              if ('${integration['sync_status'] ?? ''}'.isNotEmpty)
                                _DefinitionRow(label: 'Sync status', value: '${integration['sync_status']}'),
                              if ('${integration['site_base_url'] ?? ''}'.isNotEmpty)
                                _DefinitionRow(label: 'Website', value: '${integration['site_base_url']}'),
                              if ('${integration['last_successful_sync'] ?? ''}'.isNotEmpty)
                                _DefinitionRow(label: 'Last successful sync', value: timestamp(integration['last_successful_sync'])),
                              if ('${integration['last_error'] ?? ''}'.isNotEmpty)
                                _DefinitionRow(label: 'Last error', value: '${integration['last_error']}'),
                            ],
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

  Widget partnerCard(Map<String, dynamic> partner, double width) {
    final connectionStatus = '${partner['connection_status'] ?? 'INACTIVE'}';
    final count = (partner['integration_count'] as num?)?.toInt() ?? 0;
    final types = partner['connection_types'] is List
        ? (partner['connection_types'] as List).map((e) => '$e').join(', ')
        : '';
    final lastError = '${partner['last_error'] ?? ''}'.trim();
    return SizedBox(
      width: width,
      child: Card(
        child: InkWell(
          borderRadius: BorderRadius.circular(16),
          onTap: () => showIntegrations(partner),
          child: Padding(
            padding: const EdgeInsets.all(18),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: 42,
                      height: 42,
                      decoration: BoxDecoration(
                        color: statusColor(connectionStatus).withOpacity(.10),
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Icon(Icons.hub_outlined, color: statusColor(connectionStatus), size: 20),
                    ),
                    const SizedBox(width: 11),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          LText('${partner['partner_name'] ?? partner['partner_id']}', style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 14)),
                          const SizedBox(height: 2),
                          LText('${partner['partner_id'] ?? ''}', style: const TextStyle(color: brandTextSoft, fontSize: 10.5)),
                        ],
                      ),
                    ),
                    _StatusPill(label: connectionStatus),
                  ],
                ),
                const SizedBox(height: 15),
                _DefinitionRow(label: 'Last Successful Sync', value: timestamp(partner['last_successful_sync'])),
                _DefinitionRow(label: 'Last Error', value: lastError.isEmpty ? 'None' : lastError),
                _DefinitionRow(label: 'Connection Type', value: types.isEmpty ? 'Not configured' : types),
                _DefinitionRow(label: 'Integrations', value: '$count'),
                const SizedBox(height: 10),
                Row(
                  children: [
                    const Icon(Icons.open_in_new_rounded, size: 15, color: brandGold),
                    const SizedBox(width: 6),
                    Expanded(
                      child: LText(
                        count == 0 ? 'Open partner connection record' : 'Open $count integration${count == 1 ? '' : 's'}',
                        style: const TextStyle(color: brandTextSoft, fontSize: 10.5),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final active = (kpis['active'] as num?)?.toInt() ?? 0;
    final inactive = (kpis['inactive'] as num?)?.toInt() ?? 0;
    final suspended = (kpis['suspended'] as num?)?.toInt() ?? 0;
    final integrationCount = (kpis['integration_count'] as num?)?.toInt() ?? 0;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Partner Data Connections',
          subtitle: 'Partner-first runtime view of real Connector credentials, sync state and Website Adapters. Integration vendors appear only when configured for that partner.',
          trailing: OutlinedButton.icon(
            onPressed: loading ? null : load,
            icon: const Icon(Icons.refresh_rounded),
            label: const LText('Refresh'),
          ),
        ),
        const SizedBox(height: 12),
        ResponsiveKpiGrid(
          children: [
            Kpi(label: 'Active', value: '$active', note: 'Operational partner connections', icon: Icons.link_rounded, accent: brandSuccess),
            Kpi(label: 'Inactive', value: '$inactive', note: 'No active connection', icon: Icons.link_off_rounded, accent: brandSteel),
            Kpi(label: 'Suspended', value: '$suspended', note: 'Configured with runtime failure', icon: Icons.pause_circle_outline, accent: brandWarning),
            Kpi(label: 'Integrations', value: '$integrationCount', note: 'Real configured integration records', icon: Icons.hub_outlined, accent: brandGold),
          ],
        ),
        const SizedBox(height: 16),
        ResponsiveFieldPair(
          first: TextField(
            controller: search,
            onChanged: searchChanged,
            decoration: InputDecoration(
              labelText: uiLiteral('Search partners'),
              prefixIcon: const Icon(Icons.search_rounded),
            ),
          ),
          second: DropdownButtonFormField<String>(
            value: status,
            decoration: InputDecoration(labelText: uiLiteral('Connection status')),
            items: const [
              DropdownMenuItem(value: 'ALL', child: LText('All statuses')),
              DropdownMenuItem(value: 'ACTIVE', child: LText('ACTIVE')),
              DropdownMenuItem(value: 'INACTIVE', child: LText('INACTIVE')),
              DropdownMenuItem(value: 'SUSPENDED', child: LText('SUSPENDED')),
              DropdownMenuItem(value: 'DELETED', child: LText('DELETED')),
            ],
            onChanged: (next) {
              if (next == null) return;
              setState(() => status = next);
              load();
            },
          ),
        ),
        const SizedBox(height: 14),
        if (loading && partners.isEmpty)
          const _BrandLoading()
        else if (error != null && partners.isEmpty)
          _MessageCard(icon: Icons.cloud_off_outlined, title: 'Partner connections unavailable', message: error!)
        else if (partners.isEmpty)
          const _MessageCard(
            icon: Icons.hub_outlined,
            title: 'No partner connections match',
            message: 'Adjust the search or status filter. No synthetic integration records are generated.',
          )
        else
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 700
                  ? constraints.maxWidth
                  : constraints.maxWidth < 1120
                      ? (constraints.maxWidth - 12) / 2
                      : (constraints.maxWidth - 24) / 3;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [for (final partner in partners) partnerCard(partner, width)],
              );
            },
          ),
      ],
    );
  }
}
