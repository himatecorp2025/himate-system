part of 'main.dart';

class Start22ConnectorPanel extends StatefulWidget {
  const Start22ConnectorPanel({required this.api, this.partnerId = 'ptr_000001', super.key});
  final Api api;
  final String partnerId;

  @override
  State<Start22ConnectorPanel> createState() => _Start22ConnectorPanelState();
}

class _Start22ConnectorPanelState extends State<Start22ConnectorPanel> {
  bool loading = true;
  String? error;
  Map<String, dynamic> mapping = <String, dynamic>{};
  Map<String, dynamic> summary = <String, dynamic>{};
  Map<String, dynamic> retention = <String, dynamic>{};

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final results = await Future.wait<Map<String, dynamic>>([
        widget.api.get('/api/v1/connectors/start22/mapping', force: true),
        widget.api.get('/api/v1/connectors/start22/summary?partner_id=${Uri.encodeQueryComponent(widget.partnerId)}', force: true),
        widget.api.get('/api/v1/connectors/start22/retention?partner_id=${Uri.encodeQueryComponent(widget.partnerId)}', force: true),
      ]);
      if (!mounted) return;
      setState(() {
        mapping = results[0];
        summary = results[1];
        retention = results[2];
      });
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  String timestamp(dynamic value) {
    final raw = (value ?? '').toString().trim();
    if (raw.isEmpty || raw == 'null') return '—';
    final parsed = DateTime.tryParse(raw);
    return parsed == null ? raw : HimateI18n.dateTime(HimateI18n.activeLocale, parsed);
  }

  @override
  Widget build(BuildContext context) {
    final states = items({'items': summary['states']});
    final state = states.firstWhere(
      (item) => '${item['environment'] ?? ''}' == 'PRODUCTION',
      orElse: () => states.isNotEmpty ? states.first : <String, dynamic>{},
    );
    final registered = (mapping['module_count'] as num?)?.toInt() ?? 0;
    final covered = (summary['module_coverage'] as num?)?.toInt() ?? 0;
    final batches = (summary['batches'] as num?)?.toInt() ?? 0;
    final records = (summary['records'] as num?)?.toInt() ?? 0;
    final routeErrors = (summary['route_errors'] as num?)?.toInt() ?? 0;
    final holds = (retention['legal_holds'] as num?)?.toInt() ?? 0;
    final privacyDeletes = (retention['privacy_delete_requests'] as num?)?.toInt() ?? 0;
    final syncStatus = '${state['sync_status'] ?? (records > 0 ? 'RECEIVED' : 'NEVER')}';
    final health = '${state['health'] ?? 'UNKNOWN'}';
    final protocol = '${state['protocol_version'] ?? mapping['protocol_version'] ?? '1.0'}';
    final sourceVersion = '${state['reported_version'] ?? '—'}';
    final dataEncryption = '${summary['data_encryption'] ?? mapping['data_encryption'] ?? 'AES-256-GCM'}';
    final dataKeyVersion = '${summary['data_key_version'] ?? mapping['data_key_version'] ?? '—'}';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Klavierhaus Data Connector',
          subtitle: 'One-way, allowlisted Klavierhaus → HIMATE data contract with signed batches, reconciliation and seven-year retention.',
          trailing: OutlinedButton.icon(
            onPressed: loading ? null : load,
            icon: const Icon(Icons.refresh_rounded),
            label: const LText('Refresh connector'),
          ),
        ),
        const SizedBox(height: 12),
        if (loading && mapping.isEmpty)
          const _BrandLoading()
        else if (error != null && mapping.isEmpty)
          _MessageCard(
            icon: Icons.cloud_off_outlined,
            title: 'Klavierhaus connector unavailable',
            message: error!,
          )
        else ...[
          const _RuleStrip(items: [
            _RuleItem(Icons.arrow_forward_rounded, 'Data direction', 'Klavierhaus → HIMATE'),
            _RuleItem(Icons.shield_outlined, 'Security', 'HMAC-SHA-512 + AES-256-GCM'),
            _RuleItem(Icons.inventory_2_outlined, 'Data contract', '38 allowlisted modules'),
            _RuleItem(Icons.schedule_rounded, 'Retention', 'HIMATE 7 years'),
          ]),
          const SizedBox(height: 14),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              _StatusPill(label: syncStatus),
              _MiniCounter(label: '$covered / $registered modules'),
              _MiniCounter(label: '$batches batches'),
              _MiniCounter(label: '$records records'),
              _MiniCounter(label: '$routeErrors route errors'),
            ],
          ),
          const SizedBox(height: 14),
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 760
                  ? constraints.maxWidth
                  : (constraints.maxWidth - 12) / 2;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  SizedBox(
                    width: width,
                    child: _InfoCard(
                      title: 'Connection & Sync',
                      icon: Icons.sync_alt_rounded,
                      children: [
                        _DefinitionRow(label: 'Partner', value: widget.partnerId),
                        _DefinitionRow(label: 'Environment', value: '${state['environment'] ?? '—'}'),
                        _DefinitionRow(label: 'Health', value: health),
                        _DefinitionRow(label: 'Sync status', value: syncStatus),
                        _DefinitionRow(label: 'Protocol version', value: protocol),
                        _DefinitionRow(label: 'Klavierhaus version', value: sourceVersion),
                        _DefinitionRow(label: 'Last heartbeat', value: timestamp(state['last_seen_at'])),
                        _DefinitionRow(label: 'Last data sync', value: timestamp(state['last_data_sync_at'])),
                        _DefinitionRow(label: 'Last reconciliation', value: timestamp(state['last_reconciliation_at'])),
                      ],
                    ),
                  ),
                  SizedBox(
                    width: width,
                    child: _InfoCard(
                      title: 'Retention & Protection',
                      icon: Icons.policy_outlined,
                      children: [
                        _DefinitionRow(label: 'Retention policy', value: '${retention['retention_policy'] ?? 'HIMATE_7Y'}'),
                        _DefinitionRow(label: 'Retention period', value: '${retention['retention_years'] ?? 7} years'),
                        _DefinitionRow(label: 'Oldest retained record', value: timestamp(retention['oldest_record_at'])),
                        _DefinitionRow(label: 'Next retention expiry', value: timestamp(retention['next_retention_expiry'])),
                        _DefinitionRow(label: 'Legal holds', value: '$holds'),
                        _DefinitionRow(label: 'Privacy delete requests', value: '$privacyDeletes'),
                        _DefinitionRow(label: 'Data at rest', value: '$dataEncryption envelope encrypted'),
                        _DefinitionRow(label: 'Encryption key version', value: dataKeyVersion),
                        const _DefinitionRow(label: 'Secrets in exported datasets', value: 'Prohibited'),
                        const _DefinitionRow(label: 'Raw customer message bodies', value: 'Prohibited'),
                      ],
                    ),
                  ),
                ],
              );
            },
          ),
          if (error != null) ...[
            const SizedBox(height: 12),
            _MessageCard(
              icon: Icons.warning_amber_rounded,
              title: 'Connector refresh warning',
              message: error!,
            ),
          ],
        ],
      ],
    );
  }
}
