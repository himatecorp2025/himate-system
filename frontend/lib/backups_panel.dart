part of 'main.dart';

class BackupsPanel extends StatefulWidget {
  const BackupsPanel({
    required this.api,
    required this.initialSummary,
    required this.partnerIds,
    required this.initialProvider,
    super.key,
  });

  final Api api;
  final List<Map<String, dynamic>> initialSummary;
  final List<String> partnerIds;
  final String initialProvider;

  @override
  State<BackupsPanel> createState() => _BackupsPanelState();
}

class _BackupsPanelState extends State<BackupsPanel> {
  late List<Map<String, dynamic>> summaries;
  late String provider;
  final Set<String> busy = <String>{};
  bool refreshing = false;

  @override
  void initState() {
    super.initState();
    summaries = _copySummary(widget.initialSummary);
    provider = widget.initialProvider;
  }

  @override
  void didUpdateWidget(covariant BackupsPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (busy.isEmpty && !refreshing) {
      summaries = _copySummary(widget.initialSummary);
      provider = widget.initialProvider;
    }
  }

  List<Map<String, dynamic>> _copySummary(List<Map<String, dynamic>> value) {
    final copied = [for (final item in value) Map<String, dynamic>.from(item)];
    copied.sort((a, b) => '${a['partner_id']}'.compareTo('${b['partner_id']}'));
    return copied;
  }

  void _notify(String message, {bool failure = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        behavior: SnackBarBehavior.floating,
        backgroundColor: failure ? brandDanger : brandSuccess,
      ),
    );
  }

  String _value(dynamic value, {String fallback = '—'}) {
    final raw = value?.toString().trim() ?? '';
    return raw.isEmpty || raw == 'null' ? fallback : raw;
  }

  String _date(dynamic value) {
    final raw = value?.toString() ?? '';
    final parsed = DateTime.tryParse(raw)?.toLocal();
    if (parsed == null) return raw.isEmpty || raw == 'null' ? '—' : raw;
    String two(int n) => n.toString().padLeft(2, '0');
    return '${parsed.year}-${two(parsed.month)}-${two(parsed.day)} ${two(parsed.hour)}:${two(parsed.minute)}';
  }

  List<String> get _allPartnerIds {
    final ids = <String>{...widget.partnerIds};
    for (final item in summaries) {
      final id = '${item['partner_id'] ?? ''}'.trim();
      if (id.isNotEmpty) ids.add(id);
    }
    final sorted = ids.toList()..sort();
    return sorted;
  }

  Map<String, dynamic> _summaryFor(String partnerId) {
    return summaries.firstWhere(
      (item) => '${item['partner_id'] ?? ''}' == partnerId,
      orElse: () => <String, dynamic>{
        'partner_id': partnerId,
        'retention_days': 30,
        'max_restore_points': 30,
        'schedule_hours': 24,
        'enabled': true,
        'latest_restore_point_id': '',
        'latest_backup_status': 'NEVER',
        'latest_restore_test_status': 'NEVER',
        'recoverability_status': 'UNVERIFIED',
        'provider': provider,
      },
    );
  }

  Future<Map<String, dynamic>?> _refresh({bool quiet = false}) async {
    if (!quiet && mounted) setState(() => refreshing = true);
    try {
      final response = await widget.api.get('/api/v1/backups/summary', force: true);
      if (!mounted) return response;
      setState(() {
        summaries = _copySummary(items(response));
        provider = _value(response['provider'], fallback: provider);
      });
      return response;
    } catch (e) {
      if (!quiet) _notify(e.toString(), failure: true);
      return null;
    } finally {
      if (!quiet && mounted) setState(() => refreshing = false);
    }
  }

  Future<String?> _choosePartner() async {
    final ids = _allPartnerIds;
    if (ids.isEmpty) {
      _notify('Provision a partner before creating a restore point.', failure: true);
      return null;
    }
    String selected = ids.first;
    return showDialog<String?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: 'Create restore point',
          subtitle: 'Database, media and configuration are captured, encrypted and copied to the configured offsite provider.',
          icon: Icons.backup_outlined,
          primaryLabel: 'Start backup',
          onPrimary: () => Navigator.pop(dialogContext, selected),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                value: selected,
                decoration: const InputDecoration(labelText: 'Partner'),
                items: [
                  for (final id in ids) DropdownMenuItem(value: id, child: Text(id)),
                ],
                onChanged: (value) {
                  if (value != null) setDialogState(() => selected = value);
                },
              ),
              const SizedBox(height: 12),
              const Text(
                'Every successful restore point automatically queues a real restore test from the offsite copy.',
                style: TextStyle(color: brandTextSoft, fontSize: 11, height: 1.45),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _createRestorePoint([String? partnerId]) async {
    final id = partnerId ?? await _choosePartner();
    if (id == null || id.isEmpty || busy.contains(id)) return;
    setState(() => busy.add(id));
    try {
      final point = await widget.api.post('/api/v1/backups', <String, dynamic>{'partner_id': id});
      final pointId = _value(point['id'], fallback: '');
      _notify(pointId.isEmpty ? 'Restore point queued.' : 'Restore point $pointId queued.');
      await _pollPartner(id);
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(id));
    }
  }

  Future<void> _runRestoreTest(String partnerId) async {
    final summary = _summaryFor(partnerId);
    final pointId = _value(summary['latest_restore_point_id'], fallback: '');
    final backupStatus = _value(summary['latest_backup_status'], fallback: 'NEVER');
    if (pointId.isEmpty || backupStatus != 'READY') {
      _notify('A READY restore point is required before restore testing.', failure: true);
      return;
    }
    if (busy.contains(partnerId)) return;
    setState(() => busy.add(partnerId));
    try {
      await widget.api.post('/api/v1/backups/restore-points/$pointId/restore-test');
      _notify('Restore test queued from the offsite copy.');
      await _pollPartner(partnerId, waitForNewTest: true);
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(partnerId));
    }
  }

  Future<void> _pollPartner(String partnerId, {bool waitForNewTest = false}) async {
    String? initialTestStatus;
    if (waitForNewTest) {
      initialTestStatus = _value(_summaryFor(partnerId)['latest_restore_test_status'], fallback: 'NEVER');
    }
    for (var attempt = 0; attempt < 90; attempt++) {
      if (!mounted) return;
      await Future<void>.delayed(const Duration(seconds: 2));
      await _refresh(quiet: true);
      final summary = _summaryFor(partnerId);
      final backup = _value(summary['latest_backup_status'], fallback: 'NEVER');
      final test = _value(summary['latest_restore_test_status'], fallback: 'NEVER');
      if (backup == 'FAILED') {
        _notify('Backup failed. Open the restore-point details or audit log for diagnostics.', failure: true);
        return;
      }
      if (test == 'FAILED') {
        _notify('Restore verification failed. Recoverability is not verified.', failure: true);
        return;
      }
      if (backup == 'READY' && test == 'PASSED') {
        if (!waitForNewTest || initialTestStatus != 'PASSED' || attempt > 0) {
          _notify('Backup and restore verification passed.');
          return;
        }
      }
    }
    _notify('Backup/restore is still running. Refresh the panel to see the latest state.');
  }

  Future<void> _editPolicy(String partnerId) async {
    final current = _summaryFor(partnerId);
    final retention = TextEditingController(text: '${current['retention_days'] ?? 30}');
    final maxPoints = TextEditingController(text: '${current['max_restore_points'] ?? 30}');
    final schedule = TextEditingController(text: '${current['schedule_hours'] ?? 24}');
    bool enabled = current['enabled'] != false;
    String? dialogError;

    final payload = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: 'Backup policy',
          subtitle: 'Retention and scheduling are partner-scoped. Expired restore points are removed from the offsite provider.',
          icon: Icons.policy_outlined,
          primaryLabel: 'Save policy',
          onPrimary: () {
            final retentionDays = int.tryParse(retention.text.trim());
            final restorePoints = int.tryParse(maxPoints.text.trim());
            final scheduleHours = int.tryParse(schedule.text.trim());
            if (retentionDays == null || retentionDays < 1 || retentionDays > 3650) {
              setDialogState(() => dialogError = 'Retention must be between 1 and 3650 days.');
              return;
            }
            if (restorePoints == null || restorePoints < 1 || restorePoints > 365) {
              setDialogState(() => dialogError = 'Restore-point limit must be between 1 and 365.');
              return;
            }
            if (scheduleHours == null || scheduleHours < 1 || scheduleHours > 720) {
              setDialogState(() => dialogError = 'Schedule must be between 1 and 720 hours.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              'retention_days': retentionDays,
              'max_restore_points': restorePoints,
              'schedule_hours': scheduleHours,
              'enabled': enabled,
            });
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ResponsiveFieldPair(
                first: TextField(
                  controller: retention,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(labelText: 'Retention days'),
                ),
                second: TextField(
                  controller: maxPoints,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(labelText: 'Max restore points'),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: schedule,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(labelText: 'Automatic backup interval (hours)'),
              ),
              const SizedBox(height: 8),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: enabled,
                title: const Text('Automatic backups enabled'),
                subtitle: const Text('The durable scheduler queues a restore point when the interval is due.'),
                onChanged: (value) => setDialogState(() => enabled = value),
              ),
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                Text(dialogError!, style: const TextStyle(color: brandDanger, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
        ),
      ),
    );

    retention.dispose();
    maxPoints.dispose();
    schedule.dispose();
    if (payload == null || busy.contains(partnerId)) return;

    setState(() => busy.add(partnerId));
    try {
      await widget.api.put('/api/v1/backups/policies/$partnerId', payload);
      await _refresh(quiet: true);
      _notify('Backup policy updated.');
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(partnerId));
    }
  }

  Widget _partnerCard(String partnerId) {
    final item = _summaryFor(partnerId);
    final backupStatus = _value(item['latest_backup_status'], fallback: 'NEVER');
    final restoreStatus = _value(item['latest_restore_test_status'], fallback: 'NEVER');
    final recoverability = _value(item['recoverability_status'], fallback: 'UNVERIFIED');
    final pointId = _value(item['latest_restore_point_id'], fallback: '');
    final isBusy = busy.contains(partnerId);

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
                    color: recoverability == 'VERIFIED' ? brandSuccess.withOpacity(.10) : brandGold.withOpacity(.12),
                    borderRadius: BorderRadius.circular(11),
                  ),
                  child: Icon(
                    recoverability == 'VERIFIED' ? Icons.verified_user_outlined : Icons.backup_outlined,
                    color: recoverability == 'VERIFIED' ? brandSuccess : brandGold,
                  ),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(partnerId, style: const TextStyle(color: brandNavy, fontSize: 14, fontWeight: FontWeight.w700)),
                      const SizedBox(height: 3),
                      SelectableText(
                        pointId.isEmpty ? 'No restore point yet' : pointId,
                        style: const TextStyle(color: brandTextSoft, fontSize: 10.5),
                      ),
                    ],
                  ),
                ),
                _StatusPill(label: recoverability),
              ],
            ),
            const SizedBox(height: 14),
            const Divider(height: 1),
            const SizedBox(height: 7),
            _DefinitionRow(label: 'Backup', value: backupStatus),
            _DefinitionRow(label: 'Backup completed', value: _date(item['latest_backup_at'])),
            _DefinitionRow(label: 'Restore test', value: restoreStatus),
            _DefinitionRow(label: 'Restore tested', value: _date(item['latest_restore_test_at'])),
            _DefinitionRow(label: 'Offsite provider', value: _value(item['provider'], fallback: provider)),
            _DefinitionRow(label: 'Retention', value: '${item['retention_days'] ?? 30} days'),
            _DefinitionRow(label: 'Restore-point limit', value: '${item['max_restore_points'] ?? 30}'),
            _DefinitionRow(label: 'Automatic interval', value: '${item['schedule_hours'] ?? 24} hours'),
            _DefinitionRow(label: 'Automatic backups', value: item['enabled'] == false ? 'Disabled' : 'Enabled'),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: isBusy ? null : () => _editPolicy(partnerId),
                  icon: const Icon(Icons.policy_outlined, size: 17),
                  label: const Text('Policy'),
                ),
                FilledButton.icon(
                  onPressed: isBusy ? null : () => _createRestorePoint(partnerId),
                  icon: const Icon(Icons.backup_outlined, size: 17),
                  label: const Text('Create restore point'),
                ),
                OutlinedButton.icon(
                  onPressed: isBusy || backupStatus != 'READY' || pointId.isEmpty ? null : () => _runRestoreTest(partnerId),
                  icon: const Icon(Icons.restore_page_outlined, size: 17),
                  label: const Text('Run restore test'),
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
    final partnerIds = _allPartnerIds;
    final verified = partnerIds.where((id) => _value(_summaryFor(id)['recoverability_status'], fallback: 'UNVERIFIED') == 'VERIFIED').length;
    final failed = partnerIds.where((id) {
      final summary = _summaryFor(id);
      return _value(summary['latest_backup_status'], fallback: 'NEVER') == 'FAILED' ||
          _value(summary['latest_restore_test_status'], fallback: 'NEVER') == 'FAILED';
    }).length;
    final active = partnerIds.where((id) {
      final summary = _summaryFor(id);
      final backup = _value(summary['latest_backup_status'], fallback: 'NEVER');
      final test = _value(summary['latest_restore_test_status'], fallback: 'NEVER');
      return backup == 'QUEUED' || backup == 'RUNNING' || test == 'QUEUED' || test == 'RUNNING';
    }).length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Backups & Recoverability',
          subtitle: 'Encrypted partner database, media and configuration restore points with offsite replication, retention and mandatory restore verification.',
          trailing: Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton.icon(
                onPressed: refreshing ? null : () => _refresh(),
                icon: refreshing
                    ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.refresh_rounded, size: 17),
                label: const Text('Refresh'),
              ),
              FilledButton.icon(
                onPressed: partnerIds.isEmpty ? null : () => _createRestorePoint(),
                icon: const Icon(Icons.add_rounded),
                label: const Text('New restore point'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        _RuleStrip(items: [
          _RuleItem(Icons.storage_outlined, 'Partners', '${partnerIds.length} tracked'),
          _RuleItem(Icons.cloud_done_outlined, 'Offsite', provider.isEmpty ? 'not configured' : provider.toUpperCase()),
          _RuleItem(Icons.verified_outlined, 'Recoverable', '$verified verified'),
          _RuleItem(failed > 0 ? Icons.error_outline_rounded : Icons.sync_rounded, failed > 0 ? 'Failed' : 'In progress', failed > 0 ? '$failed failed' : '$active active'),
        ]),
        const SizedBox(height: 14),
        if (partnerIds.isEmpty)
          const _MessageCard(
            icon: Icons.backup_outlined,
            title: 'No provisioned partner available',
            message: 'Provision a partner first. The backup service will then capture its isolated database, media namespace and configuration state.',
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
                  for (final id in partnerIds) SizedBox(width: width, child: _partnerCard(id)),
                ],
              );
            },
          ),
      ],
    );
  }
}
