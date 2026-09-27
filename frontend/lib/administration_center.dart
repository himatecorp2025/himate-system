part of 'main.dart';

String _central14Date(dynamic value) {
  final raw = (value ?? '').toString().trim();
  if (raw.isEmpty || raw == 'null') return '—';
  final parsed = DateTime.tryParse(raw)?.toLocal();
  if (parsed == null) return raw;
  String two(int value) => value.toString().padLeft(2, '0');
  return parsed.year.toString() + '-' + two(parsed.month) + '-' + two(parsed.day) + ' ' + two(parsed.hour) + ':' + two(parsed.minute);
}

class AdministrationCenterPage extends StatefulWidget {
  const AdministrationCenterPage({
    required this.api,
    required this.user,
    required this.canPartnersRead,
    required this.canBillingRead,
    required this.canBillingWrite,
    required this.canBackupsRead,
    required this.canBackupsApprove,
    required this.canAuditRead,
    super.key,
  });

  final Api api;
  final Map<String, dynamic> user;
  final bool canPartnersRead;
  final bool canBillingRead;
  final bool canBillingWrite;
  final bool canBackupsRead;
  final bool canBackupsApprove;
  final bool canAuditRead;

  @override
  State<AdministrationCenterPage> createState() => _AdministrationCenterPageState();
}

class _AdministrationCenterPageState extends State<AdministrationCenterPage> {
  final TextEditingController partnerSearch = TextEditingController();
  Timer? searchTimer;
  String section = 'root';
  String lifecycle = 'ALL';
  bool loading = true;
  String? error;
  Map<String, dynamic> company = <String, dynamic>{};
  Map<String, dynamic> kpis = <String, dynamic>{};
  List<Map<String, dynamic>> partners = <Map<String, dynamic>>[];
  Map<String, dynamic>? selectedPartner;

  @override
  void initState() {
    super.initState();
    load();
  }

  @override
  void dispose() {
    searchTimer?.cancel();
    partnerSearch.dispose();
    super.dispose();
  }

  String administrationPath() {
    final params = <String, String>{'limit': '200', 'offset': '0'};
    final q = partnerSearch.text.trim();
    if (q.isNotEmpty) params['q'] = q;
    if (lifecycle != 'ALL') params['lifecycle'] = lifecycle;
    return Uri(path: '/api/v1/central/administration', queryParameters: params).toString();
  }

  Future<void> load({bool quiet = false, bool force = false}) async {
    if (!quiet && mounted) setState(() { loading = true; error = null; });
    try {
      final model = await widget.api.get(administrationPath(), force: force, maxAge: const Duration(seconds: 20));
      if (!mounted) return;
      setState(() {
        company = model['company'] is Map ? Map<String, dynamic>.from(model['company'] as Map) : <String, dynamic>{};
        kpis = model['kpis'] is Map ? Map<String, dynamic>.from(model['kpis'] as Map) : <String, dynamic>{};
        partners = items(model);
        if (selectedPartner != null) {
          final id = (selectedPartner!['partner_id'] ?? '').toString();
          final match = partners.where((item) => (item['partner_id'] ?? '').toString() == id);
          if (match.isNotEmpty) selectedPartner = match.first;
        }
        error = null;
      });
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (!quiet && mounted) setState(() => loading = false);
    }
  }

  void searchChanged(String _) {
    searchTimer?.cancel();
    searchTimer = Timer(const Duration(milliseconds: 280), () => load(quiet: true));
  }

  void go(String next, {Map<String, dynamic>? partner}) {
    setState(() {
      section = next;
      if (partner != null) selectedPartner = partner;
    });
  }

  void backToRoot() => setState(() {
        section = 'root';
        selectedPartner = null;
      });

  Widget centerCard({
    required String title,
    required String subtitle,
    required IconData icon,
    required String metric,
    required VoidCallback? onTap,
  }) {
    final enabled = onTap != null;
    return Card(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(18),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Opacity(
            opacity: enabled ? 1 : .55,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: 52,
                      height: 52,
                      decoration: BoxDecoration(
                        color: brandGold.withOpacity(.12),
                        borderRadius: BorderRadius.circular(15),
                      ),
                      child: Icon(icon, color: brandGold, size: 25),
                    ),
                    const Spacer(),
                    _MiniCounter(label: metric),
                  ],
                ),
                const SizedBox(height: 22),
                LText(title, style: const TextStyle(fontSize: 19, fontWeight: FontWeight.w800, color: brandNavy)),
                const SizedBox(height: 8),
                LText(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 11.5, height: 1.5)),
                const SizedBox(height: 18),
                Row(
                  children: [
                    LText(enabled ? 'Open administration center' : 'Permission required',
                        style: TextStyle(
                          color: enabled ? brandGold : brandTextSoft,
                          fontWeight: FontWeight.w700,
                          fontSize: 10.5,
                        )),
                    const SizedBox(width: 5),
                    Icon(enabled ? Icons.arrow_forward_rounded : Icons.lock_outline_rounded,
                        color: enabled ? brandGold : brandTextSoft, size: 16),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget workspaceCard({
    required String title,
    required String subtitle,
    required IconData icon,
    required String metric,
    required VoidCallback onTap,
    Color accent = brandGold,
  }) =>
      Card(
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(16),
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
                      decoration: BoxDecoration(color: accent.withOpacity(.11), borderRadius: BorderRadius.circular(12)),
                      child: Icon(icon, color: accent, size: 20),
                    ),
                    const Spacer(),
                    _MiniCounter(label: metric),
                  ],
                ),
                const SizedBox(height: 15),
                LText(title, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 14)),
                const SizedBox(height: 6),
                LText(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45)),
                const SizedBox(height: 12),
                const Row(
                  children: [
                    LText('Open workspace', style: TextStyle(color: brandGold, fontWeight: FontWeight.w700, fontSize: 10.5)),
                    SizedBox(width: 5),
                    Icon(Icons.arrow_forward_rounded, size: 15, color: brandGold),
                  ],
                ),
              ],
            ),
          ),
        ),
      );

  Widget rootView() {
    final partnerCount = (kpis['partners'] as num?)?.toInt() ?? partners.length;
    final admins = (company['active_administrators'] as num?)?.toInt() ?? 0;
    final documents = (company['document_count'] as num?)?.toInt() ?? 0;
    final auditEvents = (company['audit_event_count'] as num?)?.toInt() ?? 0;
    final recoverability = (company['recoverability_status'] ?? 'UNVERIFIED').toString();

    return Content(
      showHeader: false,
      title: 'Administration',
      subtitle: 'Central management of HIMATE, partner administration, documents, access and recovery.',
      actions: [
        OutlinedButton.icon(
          onPressed: loading ? null : () => load(force: true),
          icon: const Icon(Icons.refresh_rounded),
          label: const LText('Refresh'),
        ),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ResponsiveKpiGrid(children: [
            Kpi(label: 'Administrators', value: '$admins', note: 'Active HIMATE administrators', icon: Icons.groups_2_outlined, accent: brandSteel),
            Kpi(label: 'Documents', value: '$documents', note: 'Corporate document records', icon: Icons.folder_outlined, accent: brandGold),
            Kpi(label: 'Audit events', value: '$auditEvents', note: 'Immutable central audit trail', icon: Icons.shield_outlined, accent: brandSuccess),
            Kpi(label: 'Recovery', value: recoverability, note: 'Platform backup & restore verification', icon: Icons.restore_rounded, accent: recoverability == 'VERIFIED' ? brandSteel : brandWarning),
          ]),
          const SizedBox(height: 18),
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 760 ? constraints.maxWidth : (constraints.maxWidth - 14) / 2;
              return Wrap(
                spacing: 14,
                runSpacing: 14,
                children: [
                  SizedBox(
                    width: width,
                    child: _AdministrationCenterHeroCard(
                      title: 'HIMATE Administration Center',
                      subtitle: 'Corporate-level administration, settings, administrator access, documents, finance and platform recovery.',
                      icon: Icons.settings_outlined,
                      accent: brandSteel,
                      bullets: const [
                        'System configuration and governance',
                        'Users, administrators and access',
                        'Corporate documents and finance',
                        'Platform backup and recovery',
                      ],
                      actionLabel: 'Open',
                      onTap: () => go('company'),
                    ),
                  ),
                  SizedBox(
                    width: width,
                    child: _AdministrationCenterHeroCard(
                      title: 'Partner Administration Center',
                      subtitle: 'Partner-by-partner administration with tenant-scoped finance, documents, audit and recovery.',
                      icon: Icons.groups_2_outlined,
                      accent: brandGold,
                      bullets: [
                        uiBilingual(
                          '$partnerCount partner administration records',
                          '$partnerCount partner adminisztrációs rekord',
                        ),
                        'Partner users and lifecycle context',
                        'Tenant documents and audit history',
                        'Verified partner backup and recovery',
                      ],
                      actionLabel: 'Open',
                      onTap: widget.canPartnersRead ? () => go('partners') : null,
                    ),
                  ),
                ],
              );
            },
          ),
          const SizedBox(height: 16),
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 640
                  ? constraints.maxWidth
                  : constraints.maxWidth < 1120
                      ? (constraints.maxWidth - 12) / 2
                      : (constraints.maxWidth - 36) / 4;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  if (widget.canBillingRead)
                    SizedBox(width: width, child: _AdministrationQuickCard(title: 'Finance', subtitle: 'Billing identity and finance administration.', icon: Icons.paid_outlined, accent: brandSuccess, onTap: () => go('company_finance'))),
                  if (widget.canBillingRead)
                    SizedBox(width: width, child: _AdministrationQuickCard(title: 'Documents', subtitle: 'Corporate document registry and search.', icon: Icons.description_outlined, accent: brandSteel, onTap: () => go('company_documents'))),
                  SizedBox(width: width, child: _AdministrationQuickCard(title: 'Permissions & Settings', subtitle: 'Roles, administrators, audit and settings.', icon: Icons.shield_outlined, accent: const Color(0xFF7557E8), onTap: () => go('company_governance'))),
                  if (widget.canBackupsRead)
                    SizedBox(width: width, child: _AdministrationQuickCard(title: 'System Backup & Recovery', subtitle: 'Encrypted restore points and verification.', icon: Icons.restore_rounded, accent: brandSteel, onTap: () => go('company_recovery'))),
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  Widget companyView() {
    final profile = company['profile'] is Map ? Map<String, dynamic>.from(company['profile'] as Map) : <String, dynamic>{};
    final legalName = (profile['legal_name'] ?? 'HIMATE').toString();
    final recoverability = (company['recoverability_status'] ?? 'UNVERIFIED').toString();
    return Content(
      eyebrow: 'CENTRAL-14 · HIMATE CENTER',
      title: legalName.trim().isEmpty ? 'HIMATE Administration Center' : legalName + ' · Administration',
      subtitle: 'Corporate administration stays separated from partner tenant data while reusing the authoritative Billing, Identity/Audit and Backups domains.',
      actions: [
        OutlinedButton.icon(onPressed: backToRoot, icon: const Icon(Icons.arrow_back_rounded), label: const LText('Back')),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ResponsiveKpiGrid(
            children: [
              Kpi(label: 'Administrators', value: ((company['active_administrators'] as num?)?.toInt() ?? 0).toString(), note: 'Active HIMATE administrators', icon: Icons.admin_panel_settings_outlined, accent: brandNavy),
              Kpi(label: 'Documents', value: ((company['document_count'] as num?)?.toInt() ?? 0).toString(), note: 'Corporate document registry', icon: Icons.folder_copy_outlined, accent: brandGold),
              Kpi(label: 'Audit Events', value: ((company['audit_event_count'] as num?)?.toInt() ?? 0).toString(), note: 'Immutable central audit trail', icon: Icons.fact_check_outlined, accent: brandSteel),
              Kpi(label: 'Platform Recovery', value: recoverability, note: 'Encrypted backup + restore test', icon: Icons.restore_rounded, accent: recoverability == 'VERIFIED' ? brandSuccess : brandWarning),
            ],
          ),
          const SizedBox(height: 20),
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
                children: [
                  if (widget.canBillingRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'Financial Administration',
                      subtitle: 'Company billing identity, bank/tax details and the operational invoice register.',
                      icon: Icons.account_balance_outlined,
                      metric: 'FINANCE',
                      onTap: () => go('company_finance'),
                    )),
                  if (widget.canBillingRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'Corporate Documents',
                      subtitle: 'Searchable company document registry for policies, legal, governance and internal finance references.',
                      icon: Icons.folder_copy_outlined,
                      metric: ((company['document_count'] as num?)?.toInt() ?? 0).toString() + ' DOCS',
                      onTap: () => go('company_documents'),
                    )),
                  SizedBox(width: width, child: workspaceCard(
                    title: 'Governance, Settings & Access',
                    subtitle: 'Central audit search, platform secrets, company settings, roles and administrator access management.',
                    icon: Icons.admin_panel_settings_outlined,
                    metric: widget.canAuditRead ? 'AUDIT + RBAC' : 'RBAC',
                    onTap: () => go('company_governance'),
                  )),
                  if (widget.canBackupsRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'System Backup & Recovery',
                      subtitle: 'Scheduled encrypted HIMATE platform restore points and real scratch-database recovery verification.',
                      icon: Icons.settings_backup_restore_rounded,
                      metric: recoverability,
                      accent: recoverability == 'VERIFIED' ? brandSuccess : brandWarning,
                      onTap: () => go('company_recovery'),
                    )),
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  Widget partnerListView() {
    final verified = (kpis['verified_recovery'] as num?)?.toInt() ?? 0;
    final needs = (kpis['needs_recovery_verification'] as num?)?.toInt() ?? 0;
    return Content(
      eyebrow: 'CENTRAL-14 · PARTNER CENTER',
      title: 'Partner Administration Center',
      subtitle: 'Every card opens one tenant-scoped administration workspace. Financial, document, audit and recovery data never cross partner boundaries.',
      actions: [
        OutlinedButton.icon(onPressed: backToRoot, icon: const Icon(Icons.arrow_back_rounded), label: const LText('Back')),
        OutlinedButton.icon(onPressed: () => load(), icon: const Icon(Icons.refresh_rounded), label: const LText('Refresh')),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ResponsiveKpiGrid(children: [
            Kpi(label: 'Partners', value: ((kpis['partners'] as num?)?.toInt() ?? partners.length).toString(), note: 'Authoritative partner registry', icon: Icons.business_outlined, accent: brandNavy),
            Kpi(label: 'Verified Recovery', value: verified.toString(), note: 'Latest restore point passed restore test', icon: Icons.verified_user_outlined, accent: brandSuccess),
            Kpi(label: 'Needs Verification', value: needs.toString(), note: 'Backup/recovery proof not current', icon: Icons.warning_amber_rounded, accent: brandWarning),
            Kpi(label: 'Documents', value: ((kpis['documents'] as num?)?.toInt() ?? 0).toString(), note: 'Partner administration document records', icon: Icons.folder_copy_outlined, accent: brandGold),
          ]),
          const SizedBox(height: 16),
          ResponsiveFieldPair(
            first: TextField(
              controller: partnerSearch,
              onChanged: searchChanged,
              decoration: InputDecoration(labelText: uiLiteral('Search partners'), prefixIcon: const Icon(Icons.search_rounded)),
            ),
            second: DropdownButtonFormField<String>(
              value: lifecycle,
              decoration: InputDecoration(labelText: uiLiteral('Lifecycle')),
              items: const [
                DropdownMenuItem(value: 'ALL', child: LText('All lifecycles')),
                DropdownMenuItem(value: 'PROSPECT', child: LText('PROSPECT')),
                DropdownMenuItem(value: 'ONBOARDING', child: LText('ONBOARDING')),
                DropdownMenuItem(value: 'TESTING', child: LText('TESTING')),
                DropdownMenuItem(value: 'READY_FOR_LAUNCH', child: LText('READY FOR LAUNCH')),
                DropdownMenuItem(value: 'LIVE', child: LText('LIVE')),
                DropdownMenuItem(value: 'SUSPENDED', child: LText('SUSPENDED')),
                DropdownMenuItem(value: 'ARCHIVED', child: LText('ARCHIVED')),
              ],
              onChanged: (value) {
                if (value == null) return;
                setState(() => lifecycle = value);
                load(quiet: true);
              },
            ),
          ),
          const SizedBox(height: 16),
          if (partners.isEmpty)
            const _MessageCard(
              icon: Icons.business_outlined,
              title: 'No partner administration records match',
              message: 'Adjust the search or lifecycle filter.',
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
                  children: [
                    for (final partner in partners)
                      SizedBox(
                        width: width,
                        child: Card(
                          child: InkWell(
                            borderRadius: BorderRadius.circular(16),
                            onTap: () => go('partner', partner: partner),
                            child: Padding(
                              padding: const EdgeInsets.all(18),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Row(
                                    children: [
                                      Container(
                                        width: 40,
                                        height: 40,
                                        decoration: BoxDecoration(color: brandGold.withOpacity(.11), borderRadius: BorderRadius.circular(11)),
                                        child: const Icon(Icons.business_center_outlined, color: brandGold, size: 19),
                                      ),
                                      const SizedBox(width: 10),
                                      Expanded(
                                        child: Column(
                                          crossAxisAlignment: CrossAxisAlignment.start,
                                          children: [
                                            LText((partner['partner_name'] ?? partner['partner_id']).toString(), style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 13.5)),
                                            const SizedBox(height: 2),
                                            LText((partner['partner_id'] ?? '').toString(), style: const TextStyle(color: brandTextSoft, fontSize: 10)),
                                          ],
                                        ),
                                      ),
                                      _StatusPill(label: (partner['lifecycle'] ?? 'UNKNOWN').toString()),
                                    ],
                                  ),
                                  const SizedBox(height: 13),
                                  _DefinitionRow(label: 'Documents', value: ((partner['document_count'] as num?)?.toInt() ?? 0).toString()),
                                  _DefinitionRow(label: 'Invoices', value: ((partner['invoice_count'] as num?)?.toInt() ?? 0).toString()),
                                  _DefinitionRow(label: 'Audit events', value: ((partner['audit_event_count'] as num?)?.toInt() ?? 0).toString()),
                                  _DefinitionRow(label: 'Recoverability', value: (partner['recoverability_status'] ?? 'UNVERIFIED').toString()),
                                  _DefinitionRow(label: 'Latest backup', value: _central14Date(partner['latest_backup_at'])),
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
        ],
      ),
    );
  }

  Widget partnerHubView() {
    final partner = selectedPartner;
    if (partner == null) return partnerListView();
    final partnerId = (partner['partner_id'] ?? '').toString();
    final name = (partner['partner_name'] ?? partnerId).toString();
    final recovery = (partner['recoverability_status'] ?? 'UNVERIFIED').toString();
    return Content(
      eyebrow: 'CENTRAL-14 · PARTNER ADMINISTRATION',
      title: name,
      subtitle: 'Tenant-scoped administration for ' + partnerId + '. All records remain owned by their authoritative domain services.',
      actions: [
        OutlinedButton.icon(onPressed: () => go('partners'), icon: const Icon(Icons.arrow_back_rounded), label: const LText('Back')),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _SectionHeader(
            title: 'Administration workspaces',
            subtitle: 'Open only the domain you need. Partner ID stays fixed across every workspace.',
          ),
          const SizedBox(height: 12),
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
                children: [
                  if (widget.canBillingRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'Financial Administration',
                      subtitle: 'Partner invoice queue and current commercial administration summary.',
                      icon: Icons.receipt_long_outlined,
                      metric: ((partner['invoice_count'] as num?)?.toInt() ?? 0).toString() + ' INVOICES',
                      onTap: () => go('partner_finance'),
                    )),
                  if (widget.canBillingRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'Documents',
                      subtitle: 'Search and register tenant-scoped administrative document references.',
                      icon: Icons.folder_copy_outlined,
                      metric: ((partner['document_count'] as num?)?.toInt() ?? 0).toString() + ' DOCS',
                      onTap: () => go('partner_documents'),
                    )),
                  if (widget.canAuditRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'Audit & Logs',
                      subtitle: 'Search the immutable audit trail already constrained to this partner.',
                      icon: Icons.fact_check_outlined,
                      metric: ((partner['audit_event_count'] as num?)?.toInt() ?? 0).toString() + ' EVENTS',
                      onTap: () => go('partner_audit'),
                    )),
                  if (widget.canBackupsRead)
                    SizedBox(width: width, child: workspaceCard(
                      title: 'Backup & Recovery',
                      subtitle: 'Backup policy, verified restore point, restore test and gated production recovery.',
                      icon: Icons.restore_rounded,
                      metric: recovery,
                      accent: recovery == 'VERIFIED' ? brandSuccess : brandWarning,
                      onTap: () => go('partner_recovery'),
                    )),
                ],
              );
            },
          ),
          const SizedBox(height: 18),
          _InfoCard(
            title: 'Partner administration scope',
            icon: Icons.business_outlined,
            children: [
              _DefinitionRow(label: 'Partner ID', value: partnerId),
              _DefinitionRow(label: 'Legal Name', value: (partner['legal_name'] ?? '—').toString()),
              _DefinitionRow(label: 'Brand', value: (partner['brand_name'] ?? '—').toString()),
              _DefinitionRow(label: 'Lifecycle', value: (partner['lifecycle'] ?? '—').toString()),
              _DefinitionRow(label: 'Latest Document', value: _central14Date(partner['last_document_at'])),
              _DefinitionRow(label: 'Latest Audit', value: _central14Date(partner['last_audit_at'])),
            ],
          ),
        ],
      ),
    );
  }

  Widget backContent({
    required String eyebrow,
    required String title,
    required String subtitle,
    required Widget child,
    String backSection = 'company',
  }) =>
      Content(
        eyebrow: eyebrow,
        title: title,
        subtitle: subtitle,
        actions: [
          OutlinedButton.icon(
            onPressed: () => go(backSection),
            icon: const Icon(Icons.arrow_back_rounded),
            label: const LText('Back'),
          ),
        ],
        child: child,
      );

  @override
  Widget build(BuildContext context) {
    if (loading && company.isEmpty && partners.isEmpty) {
      return const Content(
        showHeader: false,
        title: 'Administration',
        subtitle: 'Central management of HIMATE, partner administration, documents, access and recovery.',
        child: _BrandLoading(),
      );
    }
    if (error != null && company.isEmpty && partners.isEmpty) {
      return Content(
        showHeader: false,
        eyebrow: 'CENTRAL-14 · ADMINISTRATION',
        title: 'Administration Center',
        subtitle: 'Corporate and partner administration read model.',
        child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Administration unavailable', message: error!),
      );
    }

    switch (section) {
      case 'company':
        return companyView();
      case 'partners':
        return partnerListView();
      case 'partner':
        return partnerHubView();
      case 'company_governance':
        return AdministrationPage(api: widget.api, user: widget.user, onBack: () => go('company'));
      case 'company_documents':
        return backContent(
          eyebrow: 'CENTRAL-14 · HIMATE DOCUMENTS',
          title: 'Corporate Documents',
          subtitle: 'Searchable HIMATE corporate document registry.',
          child: AdministrationDocumentsPanel(
            api: widget.api,
            path: '/api/v1/billing/company/documents',
            canWrite: widget.canBillingWrite,
            title: 'HIMATE corporate documents',
          ),
        );
      case 'company_finance':
        return backContent(
          eyebrow: 'CENTRAL-14 · HIMATE FINANCE ADMIN',
          title: 'Financial Administration',
          subtitle: 'Company billing identity and operational invoice register.',
          child: AdministrationFinancePanel(api: widget.api, companyProfile: company['profile'] is Map ? Map<String, dynamic>.from(company['profile'] as Map) : <String, dynamic>{}),
        );
      case 'company_recovery':
        return backContent(
          eyebrow: 'CENTRAL-14 · PLATFORM RECOVERY',
          title: 'System Backup & Recovery',
          subtitle: 'Encrypted platform backup and restore verification. Live platform replacement is maintenance-only.',
          child: AdministrationRecoveryPanel(
            api: widget.api,
            partnerRows: partners,
            ids: const <String>['_platform'],
            canMutate: widget.canBackupsApprove,
          ),
        );
      case 'partner_documents':
        final partner = selectedPartner!;
        final id = (partner['partner_id'] ?? '').toString();
        return backContent(
          eyebrow: 'CENTRAL-14 · PARTNER DOCUMENTS',
          title: (partner['partner_name'] ?? id).toString() + ' · Documents',
          subtitle: 'Tenant-scoped document administration and search.',
          backSection: 'partner',
          child: AdministrationDocumentsPanel(
            api: widget.api,
            path: '/api/v1/billing/partners/' + id + '/documents',
            canWrite: widget.canBillingWrite,
            title: 'Partner documents',
          ),
        );
      case 'partner_finance':
        final partner = selectedPartner!;
        final id = (partner['partner_id'] ?? '').toString();
        return backContent(
          eyebrow: 'CENTRAL-14 · PARTNER FINANCE ADMIN',
          title: (partner['partner_name'] ?? id).toString() + ' · Financial Administration',
          subtitle: 'Invoice and finance records constrained to this partner.',
          backSection: 'partner',
          child: AdministrationFinancePanel(api: widget.api, partnerId: id),
        );
      case 'partner_audit':
        final partner = selectedPartner!;
        final id = (partner['partner_id'] ?? '').toString();
        return backContent(
          eyebrow: 'CENTRAL-14 · PARTNER AUDIT',
          title: (partner['partner_name'] ?? id).toString() + ' · Audit & Logs',
          subtitle: 'Immutable audit events constrained to partner ' + id + '.',
          backSection: 'partner',
          child: PartnerAdministrationAuditPanel(api: widget.api, partnerId: id),
        );
      case 'partner_recovery':
        final partner = selectedPartner!;
        final id = (partner['partner_id'] ?? '').toString();
        return backContent(
          eyebrow: 'CENTRAL-14 · PARTNER RECOVERY',
          title: (partner['partner_name'] ?? id).toString() + ' · Backup & Recovery',
          subtitle: 'Verified tenant recovery with mandatory suspension gate and automatic safety backup.',
          backSection: 'partner',
          child: AdministrationRecoveryPanel(
            api: widget.api,
            partnerRows: <Map<String, dynamic>>[partner],
            ids: <String>[id],
            canMutate: widget.canBackupsApprove,
          ),
        );
      default:
        return rootView();
    }
  }
}

class _AdministrationCenterHeroCard extends StatefulWidget {
  const _AdministrationCenterHeroCard({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.accent,
    required this.bullets,
    required this.actionLabel,
    this.onTap,
  });
  final String title, subtitle, actionLabel;
  final IconData icon;
  final Color accent;
  final List<String> bullets;
  final VoidCallback? onTap;

  @override
  State<_AdministrationCenterHeroCard> createState() => _AdministrationCenterHeroCardState();
}

class _AdministrationCenterHeroCardState extends State<_AdministrationCenterHeroCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) {
    final enabled = widget.onTap != null;
    return MouseRegion(
      onEnter: enabled ? (_) => setState(() => hover = true) : null,
      onExit: enabled ? (_) => setState(() => hover = false) : null,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        transform: Matrix4.translationValues(0, hover ? -4 : 0, 0),
        constraints: const BoxConstraints(minHeight: 330),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(18),
          border: Border.all(
            color: hover ? widget.accent.withOpacity(.65) : brandMist,
            width: hover ? 1.4 : 1,
          ),
          gradient: LinearGradient(
            colors: [brandWhite, widget.accent.withOpacity(hover ? .07 : .035)],
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
          ),
          boxShadow: [
            BoxShadow(
              color: brandNavy.withOpacity(hover ? .10 : .045),
              blurRadius: hover ? 26 : 14,
              offset: Offset(0, hover ? 10 : 5),
            ),
          ],
        ),
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: widget.onTap,
            borderRadius: BorderRadius.circular(18),
            child: Stack(
              children: [
                Positioned(
                  right: 14,
                  top: 12,
                  child: Icon(widget.icon, size: 104, color: widget.accent.withOpacity(.055)),
                ),
                Padding(
                  padding: const EdgeInsets.all(24),
                  child: Opacity(
                    opacity: enabled ? 1 : .55,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(children: [
                          Container(
                            width: 58,
                            height: 58,
                            decoration: BoxDecoration(
                              color: widget.accent.withOpacity(.11),
                              borderRadius: BorderRadius.circular(15),
                            ),
                            child: Icon(widget.icon, color: widget.accent, size: 29),
                          ),
                          const SizedBox(width:14),
                          Expanded(
                            child: Column(
                              crossAxisAlignment:CrossAxisAlignment.start,
                              children:[
                                LText(
                                  widget.title,
                                  style:GoogleFonts.cormorantGaramond(
                                    color:brandNavy,
                                    fontSize:26,
                                    fontWeight:FontWeight.w700,
                                    height:1.02,
                                  ),
                                ),
                                const SizedBox(height:4),
                                LText(widget.subtitle,style:const TextStyle(color:brandTextSoft,fontSize:10.5,height:1.4)),
                              ],
                            ),
                          ),
                        ]),
                        const SizedBox(height:20),
                        for (final item in widget.bullets)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 10),
                            child: Row(children:[
                              Container(
                                width:20,
                                height:20,
                                decoration:BoxDecoration(color:widget.accent,borderRadius:BorderRadius.circular(99)),
                                child:const Icon(Icons.check_rounded,color:Colors.white,size:13),
                              ),
                              const SizedBox(width:9),
                              Expanded(child:LText(item,style:const TextStyle(color:brandCharcoal,fontSize:10.5))),
                            ]),
                          ),
                        const Spacer(),
                        SizedBox(
                          width: 210,
                          child: FilledButton.icon(
                            onPressed: widget.onTap,
                            style: FilledButton.styleFrom(backgroundColor: widget.accent),
                            icon: Icon(hover ? Icons.arrow_forward_rounded : Icons.open_in_new_rounded,size:17),
                            label: LText(widget.actionLabel),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _AdministrationQuickCard extends StatefulWidget {
  const _AdministrationQuickCard({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.accent,
    this.onTap,
  });
  final String title,subtitle;
  final IconData icon;
  final Color accent;
  final VoidCallback? onTap;

  @override
  State<_AdministrationQuickCard> createState() => _AdministrationQuickCardState();
}

class _AdministrationQuickCardState extends State<_AdministrationQuickCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) => MouseRegion(
    onEnter: widget.onTap == null ? null : (_) => setState(() => hover = true),
    onExit: widget.onTap == null ? null : (_) => setState(() => hover = false),
    child: AnimatedContainer(
      duration: const Duration(milliseconds: 170),
      transform: Matrix4.translationValues(0, hover ? -3 : 0, 0),
      decoration: BoxDecoration(
        color: brandWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: hover ? widget.accent.withOpacity(.55) : brandMist),
        boxShadow: [
          BoxShadow(
            color: brandNavy.withOpacity(hover ? .08 : .035),
            blurRadius: hover ? 18 : 10,
            offset: Offset(0, hover ? 7 : 3),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap:widget.onTap,
          borderRadius:BorderRadius.circular(16),
          child:Padding(
            padding:const EdgeInsets.all(17),
            child:Column(crossAxisAlignment:CrossAxisAlignment.start,children:[
              Row(children:[
                Container(
                  width:42,
                  height:42,
                  decoration:BoxDecoration(color:widget.accent.withOpacity(.09),borderRadius:BorderRadius.circular(12)),
                  child:Icon(widget.icon,color:widget.accent,size:21),
                ),
                const Spacer(),
                Icon(Icons.arrow_forward_rounded,color:hover?widget.accent:brandTextSoft,size:16),
              ]),
              const SizedBox(height:13),
              LText(widget.title,style:const TextStyle(color:brandNavy,fontSize:12,fontWeight:FontWeight.w800)),
              const SizedBox(height:6),
              LText(widget.subtitle,style:const TextStyle(color:brandTextSoft,fontSize:9.5,height:1.4)),
              const SizedBox(height:12),
              LText(
                uiLiteral('Open workspace'),
                style:TextStyle(color:hover?widget.accent:brandSteel,fontSize:9.5,fontWeight:FontWeight.w700),
              ),
            ]),
          ),
        ),
      ),
    ),
  );
}

class AdministrationDocumentsPanel extends StatefulWidget {
  const AdministrationDocumentsPanel({
    required this.api,
    required this.path,
    required this.canWrite,
    required this.title,
    super.key,
  });
  final Api api;
  final String path;
  final bool canWrite;
  final String title;

  @override
  State<AdministrationDocumentsPanel> createState() => _AdministrationDocumentsPanelState();
}

class _AdministrationDocumentsPanelState extends State<AdministrationDocumentsPanel> {
  final TextEditingController search = TextEditingController();
  Timer? debounce;
  List<Map<String, dynamic>> docs = <Map<String, dynamic>>[];
  bool loading = true;
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

  String queryPath() {
    final q = search.text.trim();
    if (q.isEmpty) return widget.path;
    final separator = widget.path.contains('?') ? '&' : '?';
    return widget.path + separator + 'q=' + Uri.encodeQueryComponent(q);
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final response = await widget.api.get(queryPath(), force: true, maxAge: Duration.zero);
      if (!mounted) return;
      setState(() => docs = items(response));
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

  Future<void> registerDocument() async {
    final name = TextEditingController();
    final reference = TextEditingController();
    final note = TextEditingController();
    String kind = 'ADMINISTRATION';
    String? dialogError;
    final payload = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: 'Register administrative document',
          subtitle: 'Register the durable document reference and metadata in the authoritative Billing document registry.',
          icon: Icons.note_add_outlined,
          primaryLabel: 'Register document',
          onPrimary: () {
            if (name.text.trim().isEmpty) {
              setDialogState(() => dialogError = 'Document name is required.');
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              'kind': kind,
              'name': name.text.trim(),
              'storage_url': reference.text.trim(),
              'note': note.text.trim(),
            });
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                value: kind,
                decoration: InputDecoration(labelText: uiLiteral('Document type')),
                items: const [
                  DropdownMenuItem(value: 'ADMINISTRATION', child: LText('Administration')),
                  DropdownMenuItem(value: 'CORPORATE', child: LText('Corporate')),
                  DropdownMenuItem(value: 'LEGAL', child: LText('Legal')),
                  DropdownMenuItem(value: 'POLICY', child: LText('Policy')),
                  DropdownMenuItem(value: 'GOVERNANCE', child: LText('Governance')),
                  DropdownMenuItem(value: 'FINANCE_INTERNAL', child: LText('Internal Finance')),
                  DropdownMenuItem(value: 'OTHER', child: LText('Other')),
                ],
                onChanged: (value) {
                  if (value != null) setDialogState(() => kind = value);
                },
              ),
              const SizedBox(height: 12),
              TextField(controller: name, decoration: InputDecoration(labelText: uiLiteral('Document name'))),
              const SizedBox(height: 12),
              TextField(controller: reference, decoration: InputDecoration(labelText: uiLiteral('Storage / evidence reference'))),
              const SizedBox(height: 12),
              TextField(controller: note, minLines: 2, maxLines: 4, decoration: InputDecoration(labelText: uiLiteral('Note'))),
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
        ),
      ),
    );
    name.dispose();
    reference.dispose();
    note.dispose();
    if (payload == null) return;
    try {
      await widget.api.post(widget.path, payload);
      await load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: LText('Document registered.'), behavior: SnackBarBehavior.floating));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: LText(e.toString()), backgroundColor: brandDanger, behavior: SnackBarBehavior.floating));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: widget.title,
          subtitle: 'Search by name, type, note or durable reference.',
          trailing: widget.canWrite
              ? FilledButton.icon(onPressed: registerDocument, icon: const Icon(Icons.note_add_outlined), label: const LText('Register document'))
              : null,
        ),
        const SizedBox(height: 12),
        TextField(
          controller: search,
          onChanged: searchChanged,
          decoration: InputDecoration(labelText: uiLiteral('Search documents'), prefixIcon: const Icon(Icons.search_rounded)),
        ),
        const SizedBox(height: 14),
        if (loading && docs.isEmpty)
          const _BrandLoading()
        else if (error != null && docs.isEmpty)
          _MessageCard(icon: Icons.cloud_off_outlined, title: 'Documents unavailable', message: error!)
        else if (docs.isEmpty)
          const _MessageCard(icon: Icons.folder_off_outlined, title: 'No documents found', message: 'No document records match the current search.')
        else
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 720 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final doc in docs)
                    SizedBox(
                      width: width,
                      child: _InfoCard(
                        title: (doc['name'] ?? 'Document').toString(),
                        icon: Icons.description_outlined,
                        children: [
                          _DefinitionRow(label: 'Type', value: (doc['kind'] ?? '—').toString()),
                          _DefinitionRow(label: 'Reference', value: (doc['storage_url'] ?? '').toString().trim().isEmpty ? '—' : (doc['storage_url'] ?? '').toString()),
                          _DefinitionRow(label: 'Uploaded By', value: (doc['uploaded_by'] ?? '—').toString()),
                          _DefinitionRow(label: 'Created', value: _central14Date(doc['created_at'])),
                          if ((doc['note'] ?? '').toString().trim().isNotEmpty)
                            _DefinitionRow(label: 'Note', value: (doc['note'] ?? '').toString()),
                        ],
                      ),
                    ),
                ],
              );
            },
          ),
      ],
    );
  }
}

class AdministrationFinancePanel extends StatefulWidget {
  const AdministrationFinancePanel({required this.api, this.partnerId, this.companyProfile = const <String, dynamic>{}, super.key});
  final Api api;
  final String? partnerId;
  final Map<String, dynamic> companyProfile;

  @override
  State<AdministrationFinancePanel> createState() => _AdministrationFinancePanelState();
}

class _AdministrationFinancePanelState extends State<AdministrationFinancePanel> {
  bool loading = true;
  String? error;
  List<Map<String, dynamic>> invoices = <Map<String, dynamic>>[];

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    try {
      final params = <String, String>{};
      if ((widget.partnerId ?? '').isNotEmpty) params['partner_id'] = widget.partnerId!;
      final path = Uri(path: '/api/v1/billing/invoices', queryParameters: params.isEmpty ? null : params).toString();
      final response = await widget.api.get(path, force: true);
      if (!mounted) return;
      setState(() => invoices = items(response));
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  double number(dynamic value) => value is num ? value.toDouble() : double.tryParse((value ?? '').toString()) ?? 0;

  @override
  Widget build(BuildContext context) {
    if (loading && invoices.isEmpty) return const _BrandLoading();
    if (error != null && invoices.isEmpty) return _MessageCard(icon: Icons.cloud_off_outlined, title: 'Finance data unavailable', message: error!);
    final paid = invoices.where((item) => (item['workflow_status'] ?? item['status'] ?? '').toString() == 'PAID').length;
    final draft = invoices.where((item) => (item['workflow_status'] ?? '').toString() == 'DRAFT').length;
    final gross = invoices.fold<double>(0, (sum, item) => sum + number(item['gross_total'] ?? item['total']));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ResponsiveKpiGrid(children: [
          Kpi(label: 'Invoices', value: invoices.length.toString(), note: 'Operational invoice register', icon: Icons.receipt_long_outlined, accent: brandNavy),
          Kpi(label: 'Paid', value: paid.toString(), note: 'Paid workflow records', icon: Icons.task_alt_rounded, accent: brandSuccess),
          Kpi(label: 'Draft', value: draft.toString(), note: 'Draft invoices', icon: Icons.edit_note_outlined, accent: brandWarning),
          Kpi(label: 'Gross Registered', value: gross.toStringAsFixed(2), note: 'Aggregate invoice gross value', icon: Icons.account_balance_wallet_outlined, accent: brandGold),
        ]),
        if (widget.companyProfile.isNotEmpty) ...[
          const SizedBox(height: 18),
          _InfoCard(
            title: 'Company billing identity',
            icon: Icons.account_balance_outlined,
            children: [
              _DefinitionRow(label: 'Legal Name', value: (widget.companyProfile['legal_name'] ?? '—').toString()),
              _DefinitionRow(label: 'Registration Number', value: (widget.companyProfile['registration_number'] ?? '—').toString()),
              _DefinitionRow(label: 'Tax ID', value: (widget.companyProfile['tax_id'] ?? '—').toString()),
              _DefinitionRow(label: 'Email', value: (widget.companyProfile['email'] ?? '—').toString()),
              _DefinitionRow(label: 'Bank', value: (widget.companyProfile['bank_name'] ?? '—').toString()),
              _DefinitionRow(label: 'IBAN', value: (widget.companyProfile['iban'] ?? '—').toString()),
              _DefinitionRow(label: 'SWIFT', value: (widget.companyProfile['swift'] ?? '—').toString()),
            ],
          ),
        ],
        const SizedBox(height: 18),
        _SectionHeader(title: 'Recent invoice administration', subtitle: 'Most recent invoice workflow records.'),
        const SizedBox(height: 10),
        if (invoices.isEmpty)
          const _MessageCard(icon: Icons.receipt_long_outlined, title: 'No invoices', message: 'No invoice records exist in this administration scope.')
        else
          ...[
            for (final invoice in invoices.take(20))
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(14),
                  child: Row(
                    children: [
                      const Icon(Icons.receipt_long_outlined, color: brandGold, size: 19),
                      const SizedBox(width: 10),
                      Expanded(child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          LText((invoice['id'] ?? 'Invoice').toString(), style: const TextStyle(fontWeight: FontWeight.w700)),
                          const SizedBox(height: 3),
                          LText((invoice['partner_id'] ?? '').toString() + ' · ' + _central14Date(invoice['created_at']), style: const TextStyle(color: brandTextSoft, fontSize: 10.5)),
                        ],
                      )),
                      _StatusPill(label: (invoice['workflow_status'] ?? invoice['status'] ?? 'UNKNOWN').toString()),
                    ],
                  ),
                ),
              ),
          ],
      ],
    );
  }
}

class PartnerAdministrationAuditPanel extends StatefulWidget {
  const PartnerAdministrationAuditPanel({required this.api, required this.partnerId, super.key});
  final Api api;
  final String partnerId;

  @override
  State<PartnerAdministrationAuditPanel> createState() => _PartnerAdministrationAuditPanelState();
}

class _PartnerAdministrationAuditPanelState extends State<PartnerAdministrationAuditPanel> {
  final TextEditingController search = TextEditingController();
  Timer? debounce;
  bool loading = true;
  String? error;
  List<Map<String, dynamic>> events = <Map<String, dynamic>>[];

  @override
  void initState() { super.initState(); load(); }

  @override
  void dispose() { debounce?.cancel(); search.dispose(); super.dispose(); }

  Future<void> load() async {
    final params = <String, String>{'partner_id': widget.partnerId, 'limit': '100', 'offset': '0'};
    if (search.text.trim().isNotEmpty) params['q'] = search.text.trim();
    try {
      final response = await widget.api.get(Uri(path: '/api/v1/audit/events', queryParameters: params).toString(), force: true);
      if (!mounted) return;
      setState(() => events = items(response));
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            controller: search,
            onChanged: (_) {
              debounce?.cancel();
              debounce = Timer(const Duration(milliseconds: 280), load);
            },
            decoration: InputDecoration(labelText: uiLiteral('Search audit log'), prefixIcon: const Icon(Icons.search_rounded)),
          ),
          const SizedBox(height: 14),
          if (loading && events.isEmpty)
            const _BrandLoading()
          else if (error != null && events.isEmpty)
            _MessageCard(icon: Icons.cloud_off_outlined, title: 'Audit log unavailable', message: error!)
          else if (events.isEmpty)
            const _MessageCard(icon: Icons.fact_check_outlined, title: 'No audit events', message: 'No events match the current partner search.')
          else
            ...[
              for (final event in events)
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(14),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(children: [
                          Expanded(child: LText((event['action'] ?? 'AUDIT').toString(), style: const TextStyle(fontWeight: FontWeight.w700))),
                          _StatusPill(label: (event['outcome'] ?? 'UNKNOWN').toString()),
                        ]),
                        const SizedBox(height: 7),
                        _DefinitionRow(label: 'Actor', value: (event['actor_name'] ?? event['actor_id'] ?? '—').toString()),
                        _DefinitionRow(label: 'Method', value: (event['method'] ?? '—').toString()),
                        _DefinitionRow(label: 'Path', value: (event['path'] ?? '—').toString()),
                        _DefinitionRow(label: 'Time', value: _central14Date(event['created_at'])),
                      ],
                    ),
                  ),
                ),
            ],
        ],
      );
}

class AdministrationRecoveryPanel extends StatefulWidget {
  const AdministrationRecoveryPanel({
    required this.api,
    required this.partnerRows,
    required this.ids,
    required this.canMutate,
    super.key,
  });
  final Api api;
  final List<Map<String, dynamic>> partnerRows;
  final List<String> ids;
  final bool canMutate;

  @override
  State<AdministrationRecoveryPanel> createState() => _AdministrationRecoveryPanelState();
}

class _AdministrationRecoveryPanelState extends State<AdministrationRecoveryPanel> {
  bool loading = true;
  String? error;
  List<Map<String, dynamic>> summary = <Map<String, dynamic>>[];
  String provider = 'UNKNOWN';

  @override
  void initState() { super.initState(); load(); }

  Future<void> load() async {
    try {
      final response = await widget.api.get('/api/v1/backups/summary', force: true);
      if (!mounted) return;
      final allowed = widget.ids.toSet();
      setState(() {
        summary = [for (final item in items(response)) if (allowed.contains((item['partner_id'] ?? '').toString())) item];
        provider = (response['provider'] ?? 'UNKNOWN').toString();
      });
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (loading && summary.isEmpty) return const _BrandLoading();
    if (error != null && summary.isEmpty) return _MessageCard(icon: Icons.cloud_off_outlined, title: 'Recovery data unavailable', message: error!);
    final labels = <String, String>{'_platform': 'HIMATE Platform'};
    final eligible = <String, bool>{};
    for (final row in widget.partnerRows) {
      final id = (row['partner_id'] ?? '').toString();
      if (id.isEmpty) continue;
      labels[id] = (row['partner_name'] ?? id).toString();
      final lifecycle = (row['lifecycle'] ?? '').toString();
      eligible[id] = lifecycle == 'SUSPENDED' || row['test_partner'] == true;
    }
    return BackupsPanel(
      api: widget.api,
      initialSummary: summary,
      partnerIds: widget.ids,
      initialProvider: provider,
      partnerLabels: labels,
      productionRestoreEligible: eligible,
      canMutate: widget.canMutate,
      scopeToPartnerIds: true,
    );
  }
}
