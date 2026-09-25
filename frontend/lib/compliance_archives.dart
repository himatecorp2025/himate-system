part of 'main.dart';

class ComplianceArchivesPage extends StatefulWidget {
  const ComplianceArchivesPage({required this.api, super.key});
  final Api api;

  @override
  State<ComplianceArchivesPage> createState() => _ComplianceArchivesPageState();
}

class _ComplianceArchivesPageState extends State<ComplianceArchivesPage> {
  final searchController = TextEditingController();
  Timer? _searchTimer;
  List<Map<String, dynamic>> archives = <Map<String, dynamic>>[];
  bool loading = false;
  String? error;
  int total = 0;
  int offset = 0;
  final int pageSize = 50;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    load(reset: true);
  }

  @override
  void dispose() {
    _searchTimer?.cancel();
    searchController.dispose();
    super.dispose();
  }

  Uri _uri() {
    final params = <String, String>{'limit': '${pageSize}', 'offset': '${offset}'};
    final q = searchController.text.trim();
    if (q.isNotEmpty) params['q'] = q;
    return Uri(path: '/api/v1/archives', queryParameters: params);
  }

  Future<void> load({bool reset = false}) async {
    if (reset) offset = 0;
    final generation = ++_generation;
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final response = await widget.api.get(_uri().toString(), force: true);
      if (!mounted || generation != _generation) return;
      setState(() {
        archives = items(response);
        total = (response['total'] as num?)?.toInt() ?? archives.length;
        loading = false;
      });
    } catch (e) {
      if (!mounted || generation != _generation) return;
      setState(() { loading = false; error = e.toString(); });
    }
  }

  void searchChanged(String _) {
    _searchTimer?.cancel();
    _searchTimer = Timer(const Duration(milliseconds: 280), () => load(reset: true));
  }

  String _formatTime(dynamic value) {
    final raw = '${value ?? ''}';
    final parsed = DateTime.tryParse(raw)?.toLocal();
    if (parsed == null) return raw.isEmpty ? '—' : raw;
    String two(int v) => v.toString().padLeft(2, '0');
    return '${parsed.year}-${two(parsed.month)}-${two(parsed.day)} ${two(parsed.hour)}:${two(parsed.minute)}';
  }

  Future<void> openArchive(Map<String, dynamic> summary) async {
    final partnerId = '${summary['partner_id'] ?? ''}'.trim();
    if (partnerId.isEmpty) return;
    Map<String, dynamic> detail;
    try {
      detail = await widget.api.get('/api/v1/archives/${Uri.encodeComponent(partnerId)}', force: true);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: LText('Compliance Archive could not be opened: ${e}')),
      );
      return;
    }
    if (!mounted) return;
    final payload = detail['payload'];
    final payloadMap = payload is Map ? Map<String, dynamic>.from(payload) : <String, dynamic>{};
    final sourceRows = <MapEntry<String, int>>[];
    for (final entry in payloadMap.entries) {
      if (entry.value is List) sourceRows.add(MapEntry(entry.key, (entry.value as List).length));
    }
    sourceRows.sort((a, b) => a.key.compareTo(b.key));
    await showDialog<void>(
      context: context,
      builder: (context) => BrandDialog(
        title: '${detail['display_name'] ?? partnerId} — Compliance Archive',
        subtitle: 'Read-only seven-year legal and audit evidence snapshot.',
        icon: Icons.inventory_2_outlined,
        width: 760,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _RuleStrip(items: [
              _RuleItem(Icons.lock_outline_rounded, 'State', '${detail['read_only'] == true ? 'READ ONLY' : 'UNKNOWN'}'),
              _RuleItem(Icons.verified_outlined, 'Integrity', '${detail['integrity_status'] ?? 'UNKNOWN'}'),
              _RuleItem(Icons.schedule_outlined, 'Retention', '${detail['retention_years'] ?? 7} years'),
            ]),
            const SizedBox(height: 16),
            _DefinitionRow(label: 'Partner ID', value: partnerId),
            _DefinitionRow(label: 'Legal name', value: '${detail['legal_name'] ?? '—'}'),
            _DefinitionRow(label: 'Archived', value: _formatTime(detail['archived_at'])),
            _DefinitionRow(label: 'Retain until', value: _formatTime(detail['retain_until'])),
            _DefinitionRow(label: 'Archived by', value: '${detail['created_by'] ?? '—'}'),
            _DefinitionRow(label: 'Reason', value: '${detail['archive_reason'] ?? '—'}'),
            const SizedBox(height: 12),
            const _DialogSectionLabel('INTEGRITY'),
            const SizedBox(height: 8),
            SelectableText(
              '${detail['payload_sha256'] ?? '—'}',
              style: const TextStyle(fontFamily: 'monospace', fontSize: 11, color: brandTextSoft),
            ),
            const SizedBox(height: 16),
            const _DialogSectionLabel('ARCHIVED RECORD GROUPS'),
            const SizedBox(height: 8),
            if (sourceRows.isEmpty)
              const LText('No archived record groups were returned.')
            else
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final source in sourceRows)
                    Chip(
                      avatar: const Icon(Icons.article_outlined, size: 17),
                      label: LText('${_humanize(source.key)} · ${source.value}'),
                    ),
                ],
              ),
            const SizedBox(height: 14),
            const LText(
              'Authentication secrets, sessions, MFA secrets and provider payment credentials are intentionally excluded from the seven-year archive.',
              style: TextStyle(color: brandTextSoft, fontSize: 11.5, height: 1.4),
            ),
          ],
        ),
        primaryLabel: 'Close',
        onPrimary: () => Navigator.pop(context),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final currentPage = offset ~/ pageSize + 1;
    final pageCount = total == 0 ? 1 : (total + pageSize - 1) ~/ pageSize;
    return Content(
      eyebrow: 'COMPLIANCE & RETENTION',
      title: 'Archives',
      subtitle: 'Immutable seven-year legal, financial and audit evidence for archived partner organizations.',
      actions: [
        OutlinedButton.icon(
          onPressed: loading ? null : () => load(),
          icon: const Icon(Icons.refresh_rounded),
          label: const LText('Refresh'),
        ),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _RuleStrip(items: [
            _RuleItem(Icons.lock_outline_rounded, 'Mutation', 'Read only'),
            _RuleItem(Icons.calendar_month_outlined, 'Retention', '7 years minimum'),
            _RuleItem(Icons.fingerprint_rounded, 'Integrity', 'SHA-256 verified'),
            _RuleItem(Icons.security_outlined, 'Access', 'Audit permission'),
          ]),
          const SizedBox(height: 22),
          _FilterSurface(
            child: TextField(
              controller: searchController,
              onChanged: searchChanged,
              decoration: InputDecoration(
                labelText: uiLiteral('Search Archives'),
                hintText: uiLiteral('Partner ID, display name or legal name'),
                prefixIcon: const Icon(Icons.search_rounded),
              ),
            ),
          ),
          const SizedBox(height: 16),
          if (error != null)
            _MessageCard(icon: Icons.error_outline_rounded, title: 'Archives could not be loaded', message: error!)
          else if (loading && archives.isEmpty)
            const _MessageCard(icon: Icons.sync_rounded, title: 'Loading Compliance Archives', message: 'Reading immutable archive metadata.')
          else if (archives.isEmpty)
            const _MessageCard(icon: Icons.inventory_2_outlined, title: 'No archived companies', message: 'A seven-year Compliance Archive is created automatically when a partner reaches ARCHIVED lifecycle state.')
          else
            LayoutBuilder(
              builder: (context, c) {
                final width = c.maxWidth < 720 ? c.maxWidth : c.maxWidth < 1100 ? (c.maxWidth - 12) / 2 : (c.maxWidth - 24) / 3;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final archive in archives)
                      SizedBox(
                        width: width,
                        child: Card(
                          child: InkWell(
                            borderRadius: BorderRadius.circular(12),
                            onTap: () => openArchive(archive),
                            child: Padding(
                              padding: const EdgeInsets.all(17),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Row(
                                    children: [
                                      const Icon(Icons.inventory_2_outlined, color: brandNavy),
                                      const SizedBox(width: 9),
                                      Expanded(child: LText('${archive['display_name'] ?? archive['partner_id'] ?? 'Archived partner'}', style: const TextStyle(fontWeight: FontWeight.w800, color: brandNavy))),
                                      _StatusPill(label: '${archive['integrity_status'] ?? 'UNKNOWN'}'),
                                    ],
                                  ),
                                  const SizedBox(height: 12),
                                  _DefinitionRow(label: 'Partner ID', value: '${archive['partner_id'] ?? '—'}'),
                                  _DefinitionRow(label: 'Legal name', value: '${archive['legal_name'] ?? '—'}'),
                                  _DefinitionRow(label: 'Archived', value: _formatTime(archive['archived_at'])),
                                  _DefinitionRow(label: 'Retain until', value: _formatTime(archive['retain_until'])),
                                  const SizedBox(height: 8),
                                  const Row(
                                    children: [
                                      Icon(Icons.lock_outline_rounded, size: 15, color: brandTextSoft),
                                      SizedBox(width: 6),
                                      Expanded(child: LText('Immutable · open for evidence summary', style: TextStyle(fontSize: 10.5, color: brandTextSoft))),
                                    ],
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                      ),
                  ],
                );
              },
            ),
          const SizedBox(height: 18),
          Row(
            children: [
              LText('Page ${currentPage} of ${pageCount} · ${total} archived companies', style: const TextStyle(color: brandTextSoft, fontSize: 11)),
              const Spacer(),
              IconButton(
                tooltip: 'Previous page',
                onPressed: offset > 0 && !loading ? () { offset = (offset - pageSize).clamp(0, 1 << 30); load(); } : null,
                icon: const Icon(Icons.chevron_left_rounded),
              ),
              IconButton(
                tooltip: 'Next page',
                onPressed: offset + pageSize < total && !loading ? () { offset += pageSize; load(); } : null,
                icon: const Icon(Icons.chevron_right_rounded),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
