part of 'main.dart';

class BackupsPanel extends StatefulWidget {
  const BackupsPanel({
    required this.api,
    required this.initialSummary,
    required this.partnerIds,
    required this.initialProvider,
    this.partnerLabels = const <String, String>{},
    this.productionRestoreEligible = const <String, bool>{},
    this.canMutate = true,
    this.canApproveRestore,
    this.scopeToPartnerIds = false,
    super.key,
  });

  final Api api;
  final List<Map<String, dynamic>> initialSummary;
  final List<String> partnerIds;
  final String initialProvider;
  final Map<String, String> partnerLabels;
  final Map<String, bool> productionRestoreEligible;
  final bool canMutate;
  final bool? canApproveRestore;
  final bool scopeToPartnerIds;

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
    final allowed = widget.partnerIds.toSet();
    final copied = [
      for (final item in value)
        if (!widget.scopeToPartnerIds || allowed.contains((item['partner_id'] ?? '').toString()))
          Map<String, dynamic>.from(item),
    ];
    copied.sort((a, b) => (a['partner_id'] ?? '').toString().compareTo((b['partner_id'] ?? '').toString()));
    return copied;
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

  String _value(dynamic value, {String fallback = '—'}) {
    final raw = value?.toString().trim() ?? '';
    return raw.isEmpty || raw == 'null' ? fallback : raw;
  }

  String _displayName(String partnerId) {
    if (partnerId == '_platform') return 'HIMATE Platform';
    return widget.partnerLabels[partnerId] ?? partnerId;
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
      _notify(uiBilingual('Provision a partner before creating a restore point.', 'Visszaállítási pont létrehozása előtt provisionálj egy partnert.'), failure: true);
      return null;
    }
    String selected = ids.first;
    return showDialog<String?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: uiLiteral('Create restore point'),
          subtitle: uiLiteral('Database, media and configuration are captured, encrypted and copied to the configured durable backup storage.'),
          icon: Icons.backup_outlined,
          primaryLabel: 'Start backup',
          onPrimary: () => Navigator.pop(dialogContext, selected),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                value: selected,
                decoration: InputDecoration(labelText: uiLiteral('Partner')),
                items: [
                  for (final id in ids) DropdownMenuItem(value: id, child: LText(_displayName(id))),
                ],
                onChanged: (value) {
                  if (value != null) setDialogState(() => selected = value);
                },
              ),
              const SizedBox(height: 12),
              const LText(
                'Every successful restore point automatically queues a real restore test from the durable stored copy.',
                style: TextStyle(color: brandTextSoft, fontSize: 11, height: 1.45),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _createRestorePoint([String? partnerId]) async {
    if (!widget.canMutate) return;
    final id = partnerId ?? await _choosePartner();
    if (id == null || id.isEmpty || busy.contains(id)) return;
    setState(() => busy.add(id));
    try {
      final point = await widget.api.post('/api/v1/backups', <String, dynamic>{'partner_id': id});
      final pointId = _value(point['id'], fallback: '');
      if (pointId.isEmpty) {
        throw StateError('Backup service did not return a restore-point identifier.');
      }
      _notify(uiBilingual('Restore point $pointId queued.', 'Visszaállítási pont sorba állítva: $pointId.'));
      await _pollRestorePoint(id, pointId);
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
      _notify(uiBilingual('A READY restore point is required before restore testing.', 'A visszaállítási teszthez READY állapotú visszaállítási pont szükséges.'), failure: true);
      return;
    }
    if (busy.contains(partnerId)) return;
    setState(() => busy.add(partnerId));
    try {
      final test = await widget.api.post('/api/v1/backups/restore-points/$pointId/restore-test');
      final testId = _value(test['id'], fallback: '');
      if (testId.isEmpty) {
        throw StateError('Backup service did not return a restore-test identifier.');
      }
      _notify(uiBilingual('Restore test $testId queued from the durable stored copy.', 'A(z) $testId visszaállítási teszt sorba állítva a tartós mentett példányból.'));
      await _pollRestoreTest(partnerId, testId);
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(partnerId));
    }
  }

  Future<void> _pollRestorePoint(String partnerId, String pointId) async {
    for (var attempt = 0; attempt < 90; attempt++) {
      if (!mounted) return;
      await Future<void>.delayed(const Duration(seconds: 2));
      try {
        final point = await widget.api.get('/api/v1/backups/restore-points/$pointId', force: true);
        final backupStatus = _value(point['status'], fallback: 'UNKNOWN');
        if (backupStatus == 'FAILED') {
          await _refresh(quiet: true);
          _notify(uiBilingual('Backup failed. Open the audit log for diagnostics.', 'A biztonsági mentés sikertelen. Nyisd meg az auditnaplót a diagnosztikához.'), failure: true);
          return;
        }
        if (backupStatus == 'READY') {
          final response = await widget.api.get(
            '/api/v1/backups/restore-tests?restore_point_id=$pointId&limit=1',
            force: true,
          );
          final tests = items(response);
          if (tests.isNotEmpty) {
            final testStatus = _value(tests.first['status'], fallback: 'UNKNOWN');
            if (testStatus == 'FAILED') {
              await _refresh(quiet: true);
              _notify(uiBilingual('Restore verification failed. Recoverability is not verified.', 'A visszaállítás ellenőrzése sikertelen. A helyreállíthatóság nincs igazolva.'), failure: true);
              return;
            }
            if (testStatus == 'PASSED') {
              await _refresh(quiet: true);
              _notify(uiBilingual('Backup and restore verification passed.', 'A mentés és visszaállítás ellenőrzése sikeres.'));
              return;
            }
          }
        }
        if (attempt % 3 == 0) await _refresh(quiet: true);
      } catch (_) {
        if (attempt % 3 == 0) await _refresh(quiet: true);
      }
    }
    await _refresh(quiet: true);
    _notify(uiBilingual('Backup/restore is still running. Refresh the panel to see the latest state.', 'A mentés/visszaállítás még fut. Frissítsd a panelt a legújabb állapotért.'));
  }

  Future<void> _pollRestoreTest(String partnerId, String testId) async {
    for (var attempt = 0; attempt < 90; attempt++) {
      if (!mounted) return;
      await Future<void>.delayed(const Duration(seconds: 2));
      try {
        final test = await widget.api.get('/api/v1/backups/restore-tests/$testId', force: true);
        final status = _value(test['status'], fallback: 'UNKNOWN');
        if (status == 'FAILED') {
          await _refresh(quiet: true);
          _notify(uiBilingual('Restore verification failed. Recoverability is not verified.', 'A visszaállítás ellenőrzése sikertelen. A helyreállíthatóság nincs igazolva.'), failure: true);
          return;
        }
        if (status == 'PASSED') {
          await _refresh(quiet: true);
          _notify(uiBilingual('Restore verification passed.', 'A visszaállítás ellenőrzése sikeres.'));
          return;
        }
        if (attempt % 3 == 0) await _refresh(quiet: true);
      } catch (_) {
        if (attempt % 3 == 0) await _refresh(quiet: true);
      }
    }
    await _refresh(quiet: true);
    _notify(uiBilingual('Restore verification is still running. Refresh the panel to see the latest state.', 'A visszaállítás ellenőrzése még fut. Frissítsd a panelt a legújabb állapotért.'));
  }

  Future<void> _restoreProduction(String partnerId) async {
    if ((widget.canApproveRestore ?? widget.canMutate) != true) {
      _notify(uiLiteral('Production restore approval permission is required.'), failure: true);
      return;
    }
    if (partnerId == '_platform') {
      _notify(uiBilingual('Platform production recovery is maintenance-only and cannot be executed from the live control plane.', 'A platform éles helyreállítása csak karbantartási módban végezhető, az élő vezérlősíkról nem indítható.'), failure: true);
      return;
    }
    final summary = _summaryFor(partnerId);
    final pointId = _value(summary['latest_restore_point_id'], fallback: '');
    final recoverability = _value(summary['recoverability_status'], fallback: 'UNVERIFIED');
    if (pointId.isEmpty || recoverability != 'VERIFIED') {
      _notify(uiBilingual('A VERIFIED restore point is required before production recovery.', 'Az éles helyreállításhoz VERIFIED állapotú visszaállítási pont szükséges.'), failure: true);
      return;
    }
    if (widget.productionRestoreEligible[partnerId] != true) {
      _notify(uiBilingual('Suspend the partner before production recovery. Test Partners are exempt from the suspension gate.', 'Éles helyreállítás előtt függeszd fel a partnert. A tesztpartnerek kivételt képeznek a felfüggesztési kapu alól.'), failure: true);
      return;
    }
    final reason = TextEditingController();
    final confirmation = TextEditingController();
    String? dialogError;
    final payload = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: uiLiteral('Restore verified partner backup'),
          subtitle: uiLiteral('This is a production recovery operation. A fresh safety backup is created automatically before the verified restore point replaces the partner database, media and captured configuration.'),
          icon: Icons.restore_rounded,
          primaryLabel: 'Start production restore',
          onPrimary: () {
            if (reason.text.trim().length < 5) {
              setDialogState(() => dialogError = 'Enter a recovery reason of at least 5 characters.');
              return;
            }
            final required = 'RESTORE $partnerId';
            if (confirmation.text.trim() != required) {
              setDialogState(() => dialogError = 'Type $required exactly to confirm.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              'reason': reason.text.trim(),
              'confirmation': confirmation.text.trim(),
            });
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _DefinitionRow(label: uiLiteral('Restore point'), value: pointId),
              _DefinitionRow(label: uiLiteral('Recoverability'), value: uiLiteral(_humanize(recoverability))),
              const SizedBox(height: 12),
              TextField(
                controller: reason,
                minLines: 2,
                maxLines: 4,
                decoration: InputDecoration(labelText: uiLiteral('Recovery reason')),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: confirmation,
                decoration: InputDecoration(
                  labelText: uiLiteral('Confirmation'),
                  hintText: 'RESTORE $partnerId',
                ),
              ),
              const SizedBox(height: 10),
              const LText(
                'The restore job is audit-visible and component status is persisted for database, media and configuration recovery.',
                style: TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
              ),
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
        ),
      ),
    );
    reason.dispose();
    confirmation.dispose();
    if (payload == null || busy.contains(partnerId)) return;

    setState(() => busy.add(partnerId));
    try {
      final job = await widget.api.post('/api/v1/backups/restore-points/$pointId/restore?partner_id=$partnerId', payload);
      final jobId = _value(job['id'], fallback: '');
      if (jobId.isEmpty) throw StateError('Backup service did not return a restore-job identifier.');
      _notify(uiBilingual('Production restore $jobId queued. A safety backup will be created first.', 'Az éles visszaállítás sorba állítva ($jobId). Először biztonsági mentés készül.'));
      await _pollProductionRestore(partnerId, jobId);
    } catch (e) {
      _notify(e.toString(), failure: true);
    } finally {
      if (mounted) setState(() => busy.remove(partnerId));
    }
  }

  Future<void> _pollProductionRestore(String partnerId, String jobId) async {
    for (var attempt = 0; attempt < 120; attempt++) {
      if (!mounted) return;
      await Future<void>.delayed(const Duration(seconds: 2));
      try {
        final job = await widget.api.get('/api/v1/backups/restores/$jobId', force: true);
        final status = _value(job['status'], fallback: 'UNKNOWN');
        if (status == 'FAILED') {
          await _refresh(quiet: true);
          _notify(uiBilingual('Production restore failed', 'Az éles visszaállítás sikertelen') + ': ' + _value(job['error'], fallback: uiBilingual('Open audit logs for diagnostics.', 'Nyisd meg az auditnaplót a diagnosztikához.')), failure: true);
          return;
        }
        if (status == 'COMPLETED') {
          await _refresh(quiet: true);
          _notify(uiBilingual('Production restore completed. Database, media and configuration recovery passed.', 'Az éles visszaállítás befejeződött. Az adatbázis, média és konfiguráció helyreállítása sikeres.'));
          return;
        }
      } catch (_) {}
    }
    await _refresh(quiet: true);
    _notify(uiBilingual('Production restore is still running. Refresh the recovery panel for the latest state.', 'Az éles visszaállítás még fut. Frissítsd a helyreállítási panelt a legújabb állapotért.'));
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
          title: uiLiteral('Backup policy'),
          subtitle: uiLiteral('Retention and scheduling are partner-scoped. Expired restore points are removed from durable backup storage.'),
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
                  decoration: InputDecoration(labelText: uiLiteral('Retention days')),
                ),
                second: TextField(
                  controller: maxPoints,
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(labelText: uiLiteral('Max restore points')),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: schedule,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(labelText: uiLiteral('Automatic backup interval (hours)')),
              ),
              const SizedBox(height: 8),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: enabled,
                title: LText(uiLiteral('Automatic backups enabled')),
                subtitle: LText(uiLiteral('The durable scheduler queues a restore point when the interval is due.')),
                onChanged: (value) => setDialogState(() => enabled = value),
              ),
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontWeight: FontWeight.w600)),
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
      _notify(uiBilingual('Backup policy updated.', 'A mentési szabályzat frissítve.'));
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
                      LText(_displayName(partnerId), style: const TextStyle(color: brandNavy, fontSize: 14, fontWeight: FontWeight.w700)),
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
            _DefinitionRow(label: uiLiteral('Backup'), value: uiLiteral(_humanize(backupStatus))),
            _DefinitionRow(label: uiLiteral('Backup completed'), value: _date(item['latest_backup_at'])),
            _DefinitionRow(label: uiLiteral('Restore test'), value: uiLiteral(_humanize(restoreStatus))),
            _DefinitionRow(label: uiLiteral('Restore tested'), value: _date(item['latest_restore_test_at'])),
            _DefinitionRow(label: uiLiteral('Backup storage'), value: _value(item['provider'], fallback: provider)),
            _DefinitionRow(label: uiLiteral('Retention'), value: uiBilingual('${item['retention_days'] ?? 30} days', '${item['retention_days'] ?? 30} nap')),
            _DefinitionRow(label: uiLiteral('Restore-point limit'), value: '${item['max_restore_points'] ?? 30}'),
            _DefinitionRow(label: uiLiteral('Automatic interval'), value: uiBilingual('${item['schedule_hours'] ?? 24} hours', '${item['schedule_hours'] ?? 24} óra')),
            _DefinitionRow(label: uiLiteral('Automatic backups'), value: uiLiteral(item['enabled'] == false ? 'Disabled' : 'Enabled')),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: isBusy || !widget.canMutate ? null : () => _editPolicy(partnerId),
                  icon: const Icon(Icons.policy_outlined, size: 17),
                  label: LText(uiLiteral('Policy')),
                ),
                FilledButton.icon(
                  onPressed: isBusy || !widget.canMutate ? null : () => _createRestorePoint(partnerId),
                  icon: const Icon(Icons.backup_outlined, size: 17),
                  label: LText(uiLiteral('Create restore point')),
                ),
                OutlinedButton.icon(
                  onPressed: isBusy || !widget.canMutate || backupStatus != 'READY' || pointId.isEmpty ? null : () => _runRestoreTest(partnerId),
                  icon: const Icon(Icons.restore_page_outlined, size: 17),
                  label: LText(uiLiteral('Run restore test')),
                ),
                if (partnerId != '_platform')
                  FilledButton.icon(
                    onPressed: isBusy ||
                            (widget.canApproveRestore ?? widget.canMutate) != true ||
                            recoverability != 'VERIFIED' ||
                            backupStatus != 'READY' ||
                            pointId.isEmpty ||
                            widget.productionRestoreEligible[partnerId] != true
                        ? null
                        : () => _restoreProduction(partnerId),
                    icon: const Icon(Icons.restore_rounded, size: 17),
                    label: LText(uiLiteral('Restore verified backup')),
                  ),
              ],
            ),
            if (partnerId == '_platform') ...[
              const SizedBox(height: 10),
              const LText(
                'HIMATE platform restore points are automatically scheduled and restore-tested. Production platform replacement is intentionally maintenance-only because the live control-plane database cannot safely replace itself.',
                style: TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
              ),
            ],
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
          title: uiLiteral('Backups & Recoverability'),
          subtitle: uiLiteral('Encrypted partner database, media and configuration restore points with durable storage, retention and mandatory restore verification.'),
          trailing: Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton.icon(
                onPressed: refreshing ? null : () => _refresh(),
                icon: refreshing
                    ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.refresh_rounded, size: 17),
                label: LText(uiLiteral('Refresh')),
              ),
              if (widget.canMutate)
                FilledButton.icon(
                  onPressed: partnerIds.isEmpty ? null : () => _createRestorePoint(),
                  icon: const Icon(Icons.add_rounded),
                  label: LText(uiLiteral('New restore point')),
                )
              else
                _MiniCounter(label: uiLiteral('Read only')),
            ],
          ),
        ),
        const SizedBox(height: 12),
        _RuleStrip(items: [
          _RuleItem(Icons.storage_outlined, uiLiteral('Partners'), uiBilingual('${partnerIds.length} tracked', '${partnerIds.length} követve')),
          _RuleItem(Icons.cloud_done_outlined, uiLiteral('Storage'), provider.isEmpty ? uiLiteral('Not configured') : provider.toUpperCase()),
          _RuleItem(Icons.verified_outlined, uiLiteral('Recoverable'), uiBilingual('$verified verified', '$verified ellenőrzött')),
          _RuleItem(failed > 0 ? Icons.error_outline_rounded : Icons.sync_rounded, uiLiteral(failed > 0 ? 'Failed' : 'In progress'), failed > 0 ? uiBilingual('$failed failed', '$failed sikertelen') : uiBilingual('$active active', '$active aktív')),
        ]),
        const SizedBox(height: 14),
        if (partnerIds.isEmpty)
          _MessageCard(
            icon: Icons.backup_outlined,
            title: uiLiteral('No provisioned partner available'),
            message: uiLiteral('Provision a partner first. The backup service will then capture its isolated database, media namespace and configuration state.'),
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
