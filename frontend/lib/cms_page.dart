part of 'main.dart';

class CMSSectionDraft {
  CMSSectionDraft({
    required this.id,
    required this.componentType,
    required this.heading,
    required this.body,
    required this.mediaAssetId,
    required this.ctaLabel,
    required this.ctaUrl,
    required this.visible,
    required this.sortOrder,
  });

  String id;
  String componentType;
  String heading;
  String body;
  String mediaAssetId;
  String ctaLabel;
  String ctaUrl;
  bool visible;
  int sortOrder;

  factory CMSSectionDraft.fromMap(Map<String, dynamic> value) => CMSSectionDraft(
        id: (value['id'] ?? '').toString(),
        componentType: (value['component_type'] ?? 'TEXT').toString(),
        heading: (value['heading'] ?? '').toString(),
        body: (value['body'] ?? '').toString(),
        mediaAssetId: (value['media_asset_id'] ?? '').toString(),
        ctaLabel: (value['cta_label'] ?? '').toString(),
        ctaUrl: (value['cta_url'] ?? '').toString(),
        visible: value['visible'] == true,
        sortOrder: (value['sort_order'] as num?)?.toInt() ?? 0,
      );

  Map<String, dynamic> toJson(int index) => <String, dynamic>{
        'id': id.trim(),
        'component_type': componentType,
        'heading': heading.trim(),
        'body': body.trim(),
        'media_asset_id': mediaAssetId.trim(),
        'cta_label': ctaLabel.trim(),
        'cta_url': ctaUrl.trim(),
        'visible': visible,
        'sort_order': index * 10,
        'settings': <String, dynamic>{},
      };
}

class WebsiteMarketingPage extends StatefulWidget {
  const WebsiteMarketingPage({
    required this.api,
    this.canCms = true,
    this.canContact = true,
    this.canConnections = true,
    this.canEnvironments = true,
    super.key,
  });
  final Api api;
  final bool canCms;
  final bool canContact;
  final bool canConnections;
  final bool canEnvironments;

  @override
  State<WebsiteMarketingPage> createState() => _WebsiteMarketingPageState();
}

class _WebsiteMarketingPageState extends State<WebsiteMarketingPage> {
  List<Map<String, dynamic>> pages = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> media = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> environments = <Map<String, dynamic>>[];
  Map<String, dynamic> websiteKpis = <String, dynamic>{};
  Map<String, dynamic> websiteMeta = <String, dynamic>{};
  Map<String, dynamic> websiteAccess = <String, dynamic>{};
  bool loading = true;
  String section = 'overview';
  String? error;

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load({bool force = false}) async {
    if (!mounted) return;
    setState(() {
      if (pages.isEmpty && media.isEmpty && environments.isEmpty) loading = true;
      error = null;
    });
    try {
      final model = await widget.api.get(
        centralWebsiteInitialPath(),
        force: force,
        maxAge: const Duration(seconds: 15),
      );
      if (!mounted) return;
      setState(() {
        pages = items(<String,dynamic>{'items': model['pages']});
        media = items(<String,dynamic>{'items': model['media']});
        environments = items(<String,dynamic>{'items': model['environments']});
        websiteKpis = model['kpis'] is Map
            ? Map<String,dynamic>.from(model['kpis'] as Map)
            : <String,dynamic>{};
        websiteMeta = model['meta'] is Map
            ? Map<String,dynamic>.from(model['meta'] as Map)
            : <String,dynamic>{};
        websiteAccess = model['access'] is Map
            ? Map<String,dynamic>.from(model['access'] as Map)
            : <String,dynamic>{};
        loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        loading = false;
        error = e.toString();
      });
    }
  }

  bool get canCmsWrite => websiteAccess['cms_write'] == true;
  bool get canCmsApprove => websiteAccess['cms_approve'] == true;
  bool get canContactWrite => websiteAccess['contact_write'] == true;
  bool get canEnvironmentWrite => websiteAccess['environments_write'] == true;
  bool get canEnvironmentApprove => websiteAccess['environments_approve'] == true;

  Future<void> createPage() async {
    if (!canCmsWrite) return;

    final key = TextEditingController();
    final name = TextEditingController();
    final slug = TextEditingController();
    final seoTitle = TextEditingController();
    final meta = TextEditingController();
    final canonical = TextEditingController();
    final keywords = TextEditingController();
    final localeChoice = ValueNotifier<String>(HimateI18n.activeLocale == 'hu_HU' ? 'hu_HU' : 'en_US');
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'New CMS page',
        subtitle: 'Create the first immutable draft. Existing public pages remain unchanged until START-17.',
        icon: Icons.web_outlined,
        width: 720,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ResponsiveFieldPair(
              first: TextField(
                controller: key,
                decoration: InputDecoration(labelText: uiLiteral('Stable page key *'), hintText: uiLiteral('landing')),
              ),
              second: TextField(
                controller: name,
                decoration: InputDecoration(labelText: uiLiteral('Admin page name *'), hintText: uiLiteral('HIMATE Landing')),
              ),
            ),
            const SizedBox(height: 12),
            TextField(controller: slug, decoration: InputDecoration(labelText: uiLiteral('Slug *'), hintText: uiLiteral('landing'))),
            const SizedBox(height: 12),
            ValueListenableBuilder<String>(
              valueListenable: localeChoice,
              builder: (context, value, _) => DropdownButtonFormField<String>(
                value: value,
                decoration: InputDecoration(labelText: uiLiteral('Language')),
                items: const [
                  DropdownMenuItem(value: 'en_US', child: LText('English (US)')),
                  DropdownMenuItem(value: 'hu_HU', child: LText('Magyar')),
                ],
                onChanged: (next) {
                  if (next != null) localeChoice.value = next;
                },
              ),
            ),
            const SizedBox(height: 12),
            TextField(controller: seoTitle, decoration: InputDecoration(labelText: uiLiteral('SEO title'))),
            const SizedBox(height: 12),
            TextField(controller: meta, maxLines: 2, decoration: InputDecoration(labelText: uiLiteral('Meta description'))),
            const SizedBox(height: 12),
            TextField(controller: canonical, decoration: InputDecoration(labelText: uiLiteral('Canonical HTTPS URL'))),
            const SizedBox(height: 12),
            TextField(
              controller: keywords,
              maxLines: 2,
              decoration: InputDecoration(
                labelText: uiLiteral('Page keywords'),
                hintText: uiLiteral('arts, culture, communities'),
                helperText: uiLiteral('Comma-separated · up to 24 unique page keywords'),
              ),
            ),
          ],
        ),
        primaryLabel: 'Create draft',
        onPrimary: () {
          if (key.text.trim().isEmpty || name.text.trim().isEmpty || slug.text.trim().isEmpty) {
            ScaffoldMessenger.of(dialogContext).showSnackBar(
              const SnackBar(content: LText('Page key, name and slug are required.'), behavior: SnackBarBehavior.floating),
            );
            return;
          }
          Navigator.pop(dialogContext, true);
        },
      ),
    );

    if (ok == true) {
      await widget.api.post('/api/v1/cms/pages', <String, dynamic>{
        'page_key': key.text.trim(),
        'name': name.text.trim(),
        'locale': localeChoice.value,
        'version': <String, dynamic>{
          'slug': slug.text.trim(),
          'seo': <String, dynamic>{
            'title': seoTitle.text.trim(),
            'meta_description': meta.text.trim(),
            'keywords': _cmsKeywords(keywords.text),
            'canonical': canonical.text.trim(),
            'og_title': '',
            'og_description': '',
            'og_image_asset_id': '',
            'noindex': false,
          },
          'sections': <Map<String, dynamic>>[],
        },
      });
      await load(force: true);
    }

    localeChoice.dispose();
    for (final controller in <TextEditingController>[key, name, slug, seoTitle, meta, canonical, keywords]) {
      controller.dispose();
    }
  }

  Future<void> uploadMedia() async {
    if (!canCmsWrite) return;

    final file = await pickBrowserFile('image/png,image/jpeg,image/webp,video/mp4,video/webm');
    if (file == null) return;

    final alt = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'Upload CMS media',
        subtitle: 'PNG, JPEG, WebP, MP4 or WebM · max 64 MiB · content-sniffed · SHA-256 verified.',
        icon: Icons.perm_media_outlined,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _DefinitionRow(label: 'File', value: file.name),
            const SizedBox(height: 12),
            TextField(
              controller: alt,
              decoration: InputDecoration(labelText: uiLiteral('Alt / media description'), hintText: uiLiteral('Describe the image or video for accessibility')),
            ),
          ],
        ),
        primaryLabel: 'Upload media',
        onPrimary: () => Navigator.pop(dialogContext, true),
      ),
    );

    if (ok == true) {
      final bytes = await readBrowserFile(file);
      await widget.api.multipart(
        '/api/v1/cms/media',
        <String, String>{'alt_text': alt.text.trim()},
        bytes,
        file.name,
      );
      await load(force: true);
    }
    alt.dispose();
  }

  Future<void> editDraft(Map<String, dynamic> page) async {
    if (!canCmsWrite) return;

    final id = (page['id'] ?? '').toString();
    final detail = await widget.api.get('/api/v1/cms/pages/' + id, force: true);
    final draft = detail['draft'] is Map
        ? Map<String, dynamic>.from(detail['draft'] as Map)
        : <String, dynamic>{};

    final payload = await showDialog<Map<String, dynamic>>(
      context: context,
      barrierDismissible: false,
      builder: (_) => CMSDraftEditorDialog(page: detail, draft: draft, media: media),
    );

    if (payload != null) {
      await widget.api.put('/api/v1/cms/pages/' + id + '/draft', payload);
      await load(force: true);
    }
  }

  Future<void> createPreview(Map<String, dynamic> page) async {
    if (!canCmsWrite) return;

    final id = (page['id'] ?? '').toString();
    try {
      final result = await widget.api.post('/api/v1/cms/pages/' + id + '/preview');
      final path = (result['preview_html_path'] ?? result['preview_path'] ?? '').toString();
      if (path.isNotEmpty) openBrowserDownload(path);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: LText('Full-page CMS preview created and opened in a private noindex tab.'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
      await load(force: true);
    } catch (e) {
      _showError('Preview could not be created', e);
    }
  }

  Future<void> publish(Map<String, dynamic> page) async {
    if (!canCmsApprove) return;

    final id = (page['id'] ?? '').toString();
    try {
      await widget.api.post('/api/v1/cms/pages/' + id + '/publish');
      await load(force: true);
    } catch (e) {
      _showError('Publish blocked', e);
    }
  }

  void _showError(String prefix, Object errorValue) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: LText(prefix + ': ' + errorValue.toString()),
        behavior: SnackBarBehavior.floating,
        backgroundColor: brandDanger,
      ),
    );
  }

  Future<void> showVersions(Map<String, dynamic> page) async {
    final id = (page['id'] ?? '').toString();
    final response = await widget.api.get('/api/v1/cms/pages/' + id + '/versions', force: true);
    final versions = items(response);
    if (!mounted) return;

    await showDialog<void>(
      context: context,
      builder: (dialogContext) => Dialog(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: 820, maxHeight: MediaQuery.of(dialogContext).size.height * .82),
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 18, 14, 14),
                child: Row(
                  children: [
                    const Icon(Icons.history_rounded, color: brandGold),
                    const SizedBox(width: 10),
                    Expanded(
                      child: LText(
                        'Version history · ' + (page['name'] ?? '').toString(),
                        style: Theme.of(dialogContext).textTheme.titleLarge,
                      ),
                    ),
                    IconButton(onPressed: () => Navigator.pop(dialogContext), icon: const Icon(Icons.close_rounded)),
                  ],
                ),
              ),
              const Divider(height: 1),
              Expanded(
                child: ListView.separated(
                  padding: const EdgeInsets.all(18),
                  itemCount: versions.length,
                  separatorBuilder: (_, __) => const Divider(),
                  itemBuilder: (_, index) {
                    final version = versions[index];
                    final state = (version['state'] ?? '').toString();
                    final rollback = (version['rollback_of_version_id'] ?? '').toString();
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: _StatusPill(label: state),
                      title: LText(
                        'Version ' + (version['version_no'] ?? '').toString() + ' · /' + (version['slug'] ?? '').toString(),
                        style: const TextStyle(fontWeight: FontWeight.w700),
                      ),
                      subtitle: LText(
                        rollback.isEmpty
                            ? 'Created ' + (version['created_at'] ?? '').toString() + ' · ' + (version['created_by'] ?? '').toString()
                            : 'Rollback activation of ' + rollback + ' · ' + (version['created_at'] ?? '').toString(),
                      ),
                      trailing: state == 'PUBLISHED' && canCmsApprove
                          ? OutlinedButton(
                              onPressed: () async {
                                await widget.api.post(
                                  '/api/v1/cms/pages/' + id + '/rollback',
                                  <String, dynamic>{'version_id': (version['id'] ?? '').toString()},
                                );
                                if (dialogContext.mounted) Navigator.pop(dialogContext);
                                await load(force: true);
                              },
                              child: const LText('Restore'),
                            )
                          : null,
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

  Future<void> showAudit(Map<String, dynamic> page) async {
    final id = (page['id'] ?? '').toString();
    final response = await widget.api.get('/api/v1/cms/pages/' + id + '/audit', force: true);
    final events = items(response);
    if (!mounted) return;

    await showDialog<void>(
      context: context,
      builder: (dialogContext) => Dialog(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: 820, maxHeight: MediaQuery.of(dialogContext).size.height * .82),
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 18, 14, 14),
                child: Row(
                  children: [
                    const Icon(Icons.receipt_long_outlined, color: brandGold),
                    const SizedBox(width: 10),
                    Expanded(
                      child: LText(
                        'CMS audit · ' + (page['name'] ?? '').toString(),
                        style: Theme.of(dialogContext).textTheme.titleLarge,
                      ),
                    ),
                    IconButton(onPressed: () => Navigator.pop(dialogContext), icon: const Icon(Icons.close_rounded)),
                  ],
                ),
              ),
              const Divider(height: 1),
              Expanded(
                child: ListView.separated(
                  padding: const EdgeInsets.all(18),
                  itemCount: events.length,
                  separatorBuilder: (_, __) => const Divider(),
                  itemBuilder: (_, index) {
                    final event = events[index];
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: const Icon(Icons.fiber_manual_record_rounded, color: brandGold, size: 12),
                      title: LText((event['action'] ?? '').toString(), style: const TextStyle(fontWeight: FontWeight.w700)),
                      subtitle: LText(
                        (event['created_at'] ?? '').toString() +
                            ' · actor ' +
                            (event['actor'] ?? 'system').toString() +
                            ' · ' +
                            (event['correlation_id'] ?? '').toString(),
                      ),
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

  Widget pageCard(Map<String, dynamic> page, double width) {
    final state = (page['workflow_state'] ?? 'DRAFT').toString();
    final draftNo = (page['draft_version_no'] ?? 0).toString();
    final previewNo = (page['preview_version_no'] ?? 0).toString();
    final publishedNo = (page['published_version_no'] ?? 0).toString();

    return SizedBox(
      width: width,
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(18),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 38,
                    height: 38,
                    decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(10)),
                    child: const Icon(Icons.web_outlined, color: brandGold, size: 19),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: LText(
                      (page['name'] ?? '').toString(),
                      style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700, fontSize: 14),
                    ),
                  ),
                  _StatusPill(label: state),
                ],
              ),
              const SizedBox(height: 14),
              _DefinitionRow(label: 'Page key', value: (page['page_key'] ?? '').toString()),
              _DefinitionRow(label: 'Language', value: (page['locale'] ?? 'en_US').toString() == 'hu_HU' ? 'Magyar' : 'English (US)'),
              _DefinitionRow(label: 'Draft', value: 'v' + draftNo),
              _DefinitionRow(label: 'Preview', value: previewNo == '0' ? '—' : 'v' + previewNo),
              _DefinitionRow(label: 'Published', value: publishedNo == '0' ? '—' : 'v' + publishedNo),
              _DefinitionRow(label: 'Preview token', value: page['preview_token_active'] == true ? 'Active' : 'Not issued'),
              const SizedBox(height: 12),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  if (canCmsWrite)
                    FilledButton.icon(
                      onPressed: () => editDraft(page),
                      icon: const Icon(Icons.edit_outlined),
                      label: const LText('Edit draft'),
                    ),
                  if (canCmsWrite)
                    OutlinedButton.icon(
                      onPressed: () => createPreview(page),
                      icon: const Icon(Icons.visibility_outlined),
                      label: const LText('Preview'),
                    ),
                  if (canCmsApprove)
                    OutlinedButton.icon(
                      onPressed: () => publish(page),
                      icon: const Icon(Icons.publish_outlined),
                      label: const LText('Publish'),
                    ),
                  TextButton.icon(
                    onPressed: () => showVersions(page),
                    icon: const Icon(Icons.history_rounded),
                    label: const LText('Versions'),
                  ),
                  TextButton.icon(
                    onPressed: () => showAudit(page),
                    icon: const Icon(Icons.receipt_long_outlined),
                    label: const LText('Audit'),
                  ),
                  if (!canCmsWrite && !canCmsApprove)
                    _MiniCounter(label: uiLiteral('Read only')),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget hubCard({
    required String key,
    required String title,
    required String subtitle,
    required IconData icon,
    required String metric,
  }) {
    return Card(
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () => setState(() => section = key),
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
                    decoration: BoxDecoration(color: brandGold.withOpacity(.10), borderRadius: BorderRadius.circular(12)),
                    child: Icon(icon, color: brandGold, size: 20),
                  ),
                  const Spacer(),
                  _MiniCounter(label: metric),
                ],
              ),
              const SizedBox(height: 16),
              LText(title, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 15)),
              const SizedBox(height: 6),
              LText(subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 11, height: 1.45)),
              const SizedBox(height: 14),
              const Row(
                children: [
                  LText('Open workspace', style: TextStyle(color: brandGold, fontWeight: FontWeight.w700, fontSize: 10.5)),
                  SizedBox(width: 5),
                  Icon(Icons.arrow_forward_rounded, color: brandGold, size: 15),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget hubOverview() {
    final pageCount = (websiteKpis['pages'] as num?)?.toInt() ?? pages.length;
    final publishedPages = (websiteKpis['published_pages'] as num?)?.toInt() ?? 0;
    final mediaCount = (websiteKpis['media_assets'] as num?)?.toInt() ?? media.length;
    final imageAssets = (websiteKpis['image_assets'] as num?)?.toInt() ?? 0;
    final environmentCount = (websiteKpis['environments'] as num?)?.toInt() ?? environments.length;
    final liveEnvironments = (websiteKpis['live_environments'] as num?)?.toInt() ?? 0;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ResponsiveKpiGrid(children: [
          Kpi(label: 'Pages', value: widget.canCms ? '$pageCount' : '—', note: widget.canCms ? 'CMS page records' : 'Permission required', icon: Icons.description_outlined, accent: brandSteel),
          Kpi(label: 'Media assets', value: widget.canCms ? '$mediaCount' : '—', note: widget.canCms ? uiBilingual('$imageAssets image assets', '$imageAssets képfájl') : 'Permission required', icon: Icons.perm_media_outlined, accent: brandGold),
          Kpi(label: 'Published pages', value: widget.canCms ? '$publishedPages' : '—', note: widget.canCms ? 'Published or active content' : 'Permission required', icon: Icons.public_outlined, accent: brandSuccess),
          Kpi(
            label: 'Live environments',
            value: widget.canEnvironments ? '$liveEnvironments' : '—',
            note: widget.canEnvironments
                ? uiBilingual('$environmentCount deployment environments', '$environmentCount telepítési környezet')
                : 'Permission required',
            icon: Icons.cloud_done_outlined,
            accent: const Color(0xFF6C63D9),
          ),
        ]),
        const SizedBox(height: 18),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth < 700
                ? constraints.maxWidth
                : constraints.maxWidth < 1120
                    ? (constraints.maxWidth - 12) / 2
                    : (constraints.maxWidth - 24) / 3;
            final cards = <Widget>[
              if (widget.canCms)
                SizedBox(width: width, child: _WebsiteHubCard(title: 'Design Guide', subtitle: 'Brand assets, colors, typography, layout and preview/publish workflow.', icon: Icons.brush_outlined, accent: brandSteel, onTap: () => setState(() => section = 'design'))),
              if (widget.canCms)
                SizedBox(width: width, child: _WebsiteHubCard(title: 'CMS', subtitle: 'Website pages and content editing with immutable draft, preview and publish history.', icon: Icons.description_outlined, accent: const Color(0xFF7557E8), onTap: () => setState(() => section = 'pages'))),
              if (widget.canCms)
                SizedBox(width: width, child: _WebsiteHubCard(title: 'SEO', subtitle: 'Metadata, page keywords, Open Graph and technical discovery controls.', icon: Icons.search_rounded, accent: brandGold, onTap: () => setState(() => section = 'seo'))),
              if (widget.canEnvironments)
                SizedBox(width: width, child: _WebsiteHubCard(title: 'Domain & Deployment', subtitle: 'Production domains, TLS and deployment environment status.', icon: Icons.public_outlined, accent: brandSuccess, onTap: () => setState(() => section = 'domains'))),
              if (widget.canCms)
                SizedBox(width: width, child: _WebsiteHubCard(title: 'Analytics', subtitle: 'Website measurement readiness and analytics integration status without invented traffic data.', icon: Icons.bar_chart_rounded, accent: brandSteel, onTap: () => setState(() => section = 'analytics'))),
              if (widget.canConnections)
                SizedBox(width: width, child: _WebsiteHubCard(title: 'Partner Connections', subtitle: 'Partner website adapters, connector state and last successful synchronization.', icon: Icons.groups_2_outlined, accent: const Color(0xFFD84965), onTap: () => setState(() => section = 'connections'))),
            ];
            return Wrap(spacing: 12, runSpacing: 12, children: cards);
          },
        ),
        if (widget.canContact) ...[
          const SizedBox(height: 18),
          _CentralActionCard(
            title: 'Customer Inbox',
            subtitle: 'Website inquiries and customer follow-up remain available as a focused marketing operations workspace.',
            icon: Icons.mark_email_unread_outlined,
            accent: brandGold,
            onTap: () => setState(() => section = 'leads'),
          ),
        ],
      ],
    );
  }

  Widget cmsPagesSection() => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _RuleStrip(items: [
            _RuleItem(Icons.edit_note_outlined, 'Workflow', 'DRAFT → PREVIEW → PUBLISHED'),
            _RuleItem(Icons.security_outlined, 'Public boundary', 'Published content only'),
            _RuleItem(Icons.history_rounded, 'Versioning', 'Every edit creates a new version'),
            _RuleItem(Icons.restart_alt_rounded, 'Rollback', 'Published history remains restorable'),
          ]),
          const SizedBox(height: 22),
          _SectionHeader(
            title: 'CMS Pages',
            subtitle: 'Each page keeps independent draft, preview and published snapshots.',
            trailing: _MiniCounter(label: pages.length.toString() + ' pages'),
          ),
          const SizedBox(height: 12),
          if (pages.isEmpty)
            const _MessageCard(
              icon: Icons.web_outlined,
              title: 'No CMS pages yet',
              message: 'Create a page draft, preview it, then publish it to activate CMS content on the public website.',
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 720
                    ? constraints.maxWidth
                    : constraints.maxWidth < 1180
                        ? (constraints.maxWidth - 12) / 2
                        : (constraints.maxWidth - 24) / 3;
                return Wrap(spacing: 12, runSpacing: 12, children: [for (final page in pages) pageCard(page, width)]);
              },
            ),
        ],
      );

  Widget mediaSection() => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _SectionHeader(
            title: 'Media Assets',
            subtitle: 'CMS images and story videos are content-sniffed, checksum-backed and referenced by stable asset ID.',
            trailing: _MiniCounter(label: media.length.toString() + ' assets'),
          ),
          const SizedBox(height: 12),
          if (media.isEmpty)
            const _MessageCard(
              icon: Icons.perm_media_outlined,
              title: 'No CMS media yet',
              message: 'Upload an image or MP4/WebM story video before linking media to page sections.',
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 650 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final asset in media)
                      SizedBox(
                        width: width,
                        child: _InfoCard(
                          title: (asset['original_filename'] ?? '').toString(),
                          icon: Icons.image_outlined,
                          action: IconButton(
                            tooltip: uiLiteral('Preview media'),
                            onPressed: () => openBrowserDownload('/api/v1/cms/media/' + (asset['id'] ?? '').toString() + '/preview'),
                            icon: const Icon(Icons.visibility_outlined),
                          ),
                          children: [
                            _DefinitionRow(label: 'Asset ID', value: (asset['id'] ?? '').toString()),
                            _DefinitionRow(label: 'Type', value: (asset['mime_type'] ?? '').toString()),
                            _DefinitionRow(label: 'Alt text', value: (asset['alt_text'] ?? '').toString().isEmpty ? '—' : (asset['alt_text'] ?? '').toString()),
                            _DefinitionRow(label: 'SHA-256', value: _cmsShort((asset['sha256'] ?? '').toString())),
                          ],
                        ),
                      ),
                  ],
                );
              },
            ),
        ],
      );

  Widget activeSection() {
    switch (section) {
      case 'design':
        return DesignGuidePanel(
          api: widget.api,
          media: media,
          canWrite: canCmsWrite,
          canApprove: canCmsApprove,
        );
      case 'pages':
        return cmsPagesSection();
      case 'media':
        return mediaSection();
      case 'seo':
        return SEOKeywordsPanel(
          api: widget.api,
          media: media,
          canWrite: canCmsWrite,
          canApprove: canCmsApprove,
        );
      case 'leads':
        return ContactLeadsPanel(api: widget.api, canWrite: canContactWrite);
      case 'connections':
        return PartnerConnectionsPanel(api: widget.api);
      case 'domains':
        return WebsiteDomainsPanel(
          api: widget.api,
          initialEnvironments: environments,
          canWrite: canEnvironmentWrite,
          canApprove: canEnvironmentApprove,
        );
      case 'analytics':
        return WebsiteAnalyticsPanel(pages: pages, media: media, kpis: websiteKpis);
      default:
        return hubOverview();
    }
  }

  @override
  Widget build(BuildContext context) {
    if (loading && pages.isEmpty && media.isEmpty && environments.isEmpty) {
      return const Content(
        showHeader: false,
        title: 'Website & Marketing',
        subtitle: 'Website management, online presence, marketing tools and analytics in one place.',
        child: _BrandLoading(),
      );
    }
    if (error != null && pages.isEmpty && media.isEmpty && environments.isEmpty) {
      return Content(
        showHeader: false,
        eyebrow: 'WEBSITE · MARKETING · PARTNER OPERATIONS',
        title: 'Website & Marketing',
        subtitle: 'Content, brand, discovery and partner operations workspaces.',
        child: _MessageCard(icon: Icons.error_outline_rounded, title: 'Website & Marketing unavailable', message: error!),
      );
    }

    final status = '${websiteMeta['status'] ?? 'healthy'}'.toLowerCase();
    final unavailable = websiteMeta['unavailable'] is List
        ? (websiteMeta['unavailable'] as List).map((e) => '$e').where((e) => e.isNotEmpty).toList()
        : <String>[];

    return Content(
      showHeader: section != 'overview',
      title: section == 'overview' ? 'Website & Marketing' : ({
        'design': 'Design Guide',
        'pages': 'CMS Pages',
        'media': 'Media Library',
        'seo': 'SEO & Keywords',
        'leads': 'Customer Inbox',
        'connections': 'Partner Data Connections',
        'domains': 'Domain & Deployment',
        'analytics': 'Analytics',
      }[section] ?? 'Website & Marketing'),
      subtitle: section == 'overview'
          ? 'Website management, online presence, marketing tools and analytics in one place.'
          : 'Focused workspace · use Back to return to Website & Marketing.',
      actions: [
        if (section != 'overview')
          OutlinedButton.icon(
            onPressed: () => setState(() => section = 'overview'),
            icon: const Icon(Icons.arrow_back_rounded),
            label: const LText('Back'),
          ),
        if (section == 'pages' && canCmsWrite)
          FilledButton.icon(onPressed: createPage, icon: const Icon(Icons.add_rounded), label: const LText('New CMS page')),
        if (section == 'media' && canCmsWrite)
          FilledButton.icon(onPressed: uploadMedia, icon: const Icon(Icons.perm_media_outlined), label: const LText('Upload media')),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if ((status == 'partial' || status == 'unavailable' || status == 'stale') && unavailable.isNotEmpty) ...[
            _MessageCard(
              icon: status == 'stale' ? Icons.history_rounded : Icons.warning_amber_rounded,
              title: uiLiteral(status == 'stale' ? 'Website data is temporarily stale' : 'Website data is partially available'),
              message: '${uiLiteral('Unavailable services')}: ${unavailable.join(', ')}',
            ),
            const SizedBox(height: 14),
          ],
          activeSection(),
        ],
      ),
    );
  }
}

class _WebsiteHubCard extends StatefulWidget {
  const _WebsiteHubCard({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.accent,
    required this.onTap,
  });
  final String title, subtitle;
  final IconData icon;
  final Color accent;
  final VoidCallback onTap;

  @override
  State<_WebsiteHubCard> createState() => _WebsiteHubCardState();
}

class _WebsiteHubCardState extends State<_WebsiteHubCard> {
  bool hover = false;

  @override
  Widget build(BuildContext context) => MouseRegion(
        onEnter: (_) => setState(() => hover = true),
        onExit: (_) => setState(() => hover = false),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 170),
          transform: Matrix4.translationValues(0, hover ? -3 : 0, 0),
          decoration: BoxDecoration(
            color: brandWhite,
            borderRadius: BorderRadius.circular(17),
            border: Border.all(color: hover ? brandGold.withOpacity(.72) : brandMist, width: hover ? 1.3 : 1),
            boxShadow: [
              BoxShadow(
                color: brandNavy.withOpacity(hover ? .09 : .045),
                blurRadius: hover ? 22 : 12,
                offset: Offset(0, hover ? 8 : 4),
              ),
            ],
          ),
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(17),
              onTap: widget.onTap,
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(children: [
                      Container(
                        width: 54,
                        height: 54,
                        decoration: BoxDecoration(color: widget.accent.withOpacity(.09), borderRadius: BorderRadius.circular(14)),
                        child: Icon(widget.icon, color: widget.accent, size: 27),
                      ),
                      const SizedBox(width: 14),
                      Expanded(
                        child: LText(
                          widget.title,
                          style: GoogleFonts.cormorantGaramond(
                            color: brandNavy,
                            fontSize: 22,
                            fontWeight: FontWeight.w700,
                            height: 1.05,
                          ),
                        ),
                      ),
                      AnimatedRotation(
                        turns: hover ? 0 : -.02,
                        duration: const Duration(milliseconds: 170),
                        child: Icon(Icons.arrow_forward_rounded, color: hover ? brandGold : brandSteel, size: 19),
                      ),
                    ]),
                    const SizedBox(height: 14),
                    LText(widget.subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45)),
                    const SizedBox(height: 18),
                    AnimatedContainer(
                      duration: const Duration(milliseconds: 170),
                      width: double.infinity,
                      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                      decoration: BoxDecoration(
                        color: hover ? brandGold.withOpacity(.09) : const Color(0xFFF4F7FB),
                        borderRadius: BorderRadius.circular(10),
                        border: Border.all(color: hover ? brandGold.withOpacity(.45) : brandMist),
                      ),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          LText(
                            uiLiteral('Open workspace'),
                            style: TextStyle(
                              color: hover ? const Color(0xFF8B6508) : brandSteel,
                              fontSize: 10.5,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Icon(Icons.arrow_forward_rounded, color: hover ? brandGold : brandSteel, size: 17),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );
}

class WebsiteDomainsPanel extends StatelessWidget {
  const WebsiteDomainsPanel({
    required this.api,
    required this.initialEnvironments,
    required this.canWrite,
    required this.canApprove,
    super.key,
  });
  final Api api;
  final List<Map<String,dynamic>> initialEnvironments;
  final bool canWrite;
  final bool canApprove;

  @override
  Widget build(BuildContext context) => DomainsDeploymentsPanel(
        api: api,
        initialEnvironments: initialEnvironments,
        canWrite: canWrite,
        canApprove: canApprove,
      );
}

class WebsiteAnalyticsPanel extends StatelessWidget {
  const WebsiteAnalyticsPanel({
    required this.pages,
    required this.media,
    required this.kpis,
    super.key,
  });
  final List<Map<String, dynamic>> pages;
  final List<Map<String, dynamic>> media;
  final Map<String,dynamic> kpis;

  @override
  Widget build(BuildContext context) {
    final pageCount = (kpis['pages'] as num?)?.toInt() ?? pages.length;
    final published = (kpis['published_pages'] as num?)?.toInt() ?? 0;
    final mediaCount = (kpis['media_assets'] as num?)?.toInt() ?? media.length;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ResponsiveKpiGrid(children: [
          Kpi(label: 'CMS pages', value: '$pageCount', note: 'Tracked content records', icon: Icons.description_outlined, accent: brandSteel),
          Kpi(label: 'Published', value: '$published', note: 'Published or active pages', icon: Icons.public_outlined, accent: brandSuccess),
          Kpi(label: 'Media assets', value: '$mediaCount', note: 'CMS media records', icon: Icons.perm_media_outlined, accent: brandGold),
          const Kpi(label: 'Traffic analytics', value: '—', note: 'No authoritative traffic dataset connected', icon: Icons.query_stats_outlined, accent: Color(0xFF6C63D9)),
        ]),
        const SizedBox(height: 16),
        const _MessageCard(
          icon: Icons.analytics_outlined,
          title: 'Traffic analytics connector is not reporting data yet',
          message: 'HIMATE does not invent visits, behavior or conversion figures. When an authoritative analytics/heatmap provider is connected, this workspace can render those real measurements.',
        ),
      ],
    );
  }
}

String _cmsShort(String value) => value.length > 14 ? value.substring(0, 14) + '…' : value;

List<String> _cmsKeywords(String value) => value
    .split(RegExp(r'[,;\n]'))
    .map((item) => item.trim())
    .where((item) => item.isNotEmpty)
    .toSet()
    .take(24)
    .toList();

String _cmsKeywordText(dynamic value) {
  if (value is! List) return '';
  return value
      .map((item) => item.toString().trim())
      .where((item) => item.isNotEmpty)
      .join(', ');
}

class CMSDraftEditorDialog extends StatefulWidget {
  const CMSDraftEditorDialog({
    required this.page,
    required this.draft,
    required this.media,
    super.key,
  });

  final Map<String, dynamic> page;
  final Map<String, dynamic> draft;
  final List<Map<String, dynamic>> media;

  @override
  State<CMSDraftEditorDialog> createState() => _CMSDraftEditorDialogState();
}

class _CMSDraftEditorDialogState extends State<CMSDraftEditorDialog> {
  late final TextEditingController slug;
  late final TextEditingController seoTitle;
  late final TextEditingController meta;
  late final TextEditingController keywords;
  late final TextEditingController canonical;
  late final TextEditingController ogTitle;
  late final TextEditingController ogDescription;
  bool noindex = false;
  String ogImage = '';
  final List<CMSSectionDraft> sections = <CMSSectionDraft>[];

  static const List<String> componentTypes = <String>[
    'HERO',
    'MISSION',
    'INDUSTRIES',
    'TECHNOLOGY',
    'PARTNER_NETWORK',
    'IMPACT',
    'CASE_STUDIES',
    'CONTACT',
    'FEATURE',
    'CTA',
    'STORY_VIDEO',
    'TEXT',
  ];

  @override
  void initState() {
    super.initState();
    final seo = widget.draft['seo'] is Map
        ? Map<String, dynamic>.from(widget.draft['seo'] as Map)
        : <String, dynamic>{};
    slug = TextEditingController(
      text: (widget.draft['slug'] ?? widget.page['page_key'] ?? '').toString(),
    );
    seoTitle = TextEditingController(text: (seo['title'] ?? '').toString());
    meta = TextEditingController(text: (seo['meta_description'] ?? '').toString());
    keywords = TextEditingController(text: _cmsKeywordText(seo['keywords']));
    canonical = TextEditingController(text: (seo['canonical'] ?? '').toString());
    ogTitle = TextEditingController(text: (seo['og_title'] ?? '').toString());
    ogDescription = TextEditingController(text: (seo['og_description'] ?? '').toString());
    noindex = seo['noindex'] == true;
    ogImage = (seo['og_image_asset_id'] ?? '').toString();

    final raw = widget.draft['sections'];
    if (raw is List) {
      for (final item in raw.whereType<Map>()) {
        sections.add(CMSSectionDraft.fromMap(Map<String, dynamic>.from(item)));
      }
    }
    sections.sort((a, b) => a.sortOrder.compareTo(b.sortOrder));
  }

  @override
  void dispose() {
    for (final controller in <TextEditingController>[
      slug,
      seoTitle,
      meta,
      keywords,
      canonical,
      ogTitle,
      ogDescription,
    ]) {
      controller.dispose();
    }
    super.dispose();
  }

  void addSection() {
    setState(() {
      sections.add(
        CMSSectionDraft(
          id: 'section-' + (sections.length + 1).toString(),
          componentType: sections.isEmpty ? 'HERO' : 'TEXT',
          heading: '',
          body: '',
          mediaAssetId: '',
          ctaLabel: '',
          ctaUrl: '',
          visible: true,
          sortOrder: sections.length * 10,
        ),
      );
    });
  }

  void move(int index, int delta) {
    final target = index + delta;
    if (target < 0 || target >= sections.length) return;
    setState(() {
      final item = sections.removeAt(index);
      sections.insert(target, item);
    });
  }

  Map<String, dynamic> payload() => <String, dynamic>{
        'slug': slug.text.trim(),
        'seo': <String, dynamic>{
          'title': seoTitle.text.trim(),
          'meta_description': meta.text.trim(),
          'keywords': _cmsKeywords(keywords.text),
          'canonical': canonical.text.trim(),
          'og_title': ogTitle.text.trim(),
          'og_description': ogDescription.text.trim(),
          'og_image_asset_id': ogImage,
          'noindex': noindex,
        },
        'sections': <Map<String, dynamic>>[
          for (var i = 0; i < sections.length; i++) sections[i].toJson(i),
        ],
      };

  Widget mediaDropdown(
    String value,
    ValueChanged<String> onChanged, {
    String label = 'Media asset',
  }) {
    final ids = widget.media.map((e) => (e['id'] ?? '').toString()).toSet();
    final safeValue = value.isEmpty || ids.contains(value) ? value : '';

    return DropdownButtonFormField<String>(
      value: safeValue,
      decoration: InputDecoration(labelText: label),
      items: <DropdownMenuItem<String>>[
        const DropdownMenuItem<String>(value: '', child: LText('No media')),
        for (final asset in widget.media)
          DropdownMenuItem<String>(
            value: (asset['id'] ?? '').toString(),
            child: LText(
              (asset['original_filename'] ?? '').toString() +
                  ' · ' +
                  (asset['id'] ?? '').toString(),
              overflow: TextOverflow.ellipsis,
            ),
          ),
      ],
      onChanged: (value) => onChanged(value ?? ''),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 18),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: 980,
          maxHeight: MediaQuery.of(context).size.height * .92,
        ),
        child: Container(
          decoration: BoxDecoration(
            color: brandWhite,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: brandMist),
          ),
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(22, 18, 14, 14),
                child: Row(
                  children: [
                    const Icon(Icons.edit_note_outlined, color: brandGold),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          LText(
                            'Edit CMS draft · ' + (widget.page['name'] ?? '').toString(),
                            style: Theme.of(context).textTheme.titleLarge,
                          ),
                          const SizedBox(height: 3),
                          const LText(
                            'Saving creates a new immutable DRAFT version. Publishing requires a fresh PREVIEW version.',
                            style: TextStyle(color: brandTextSoft, fontSize: 11.5),
                          ),
                        ],
                      ),
                    ),
                    IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close_rounded)),
                  ],
                ),
              ),
              const Divider(height: 1),
              Expanded(
                child: ListView(
                  padding: const EdgeInsets.all(20),
                  children: [
                    LText('SEO & routing', style: Theme.of(context).textTheme.titleMedium),
                    const SizedBox(height: 12),
                    TextField(controller: slug, decoration: InputDecoration(labelText: uiLiteral('Slug *'))),
                    const SizedBox(height: 12),
                    ResponsiveFieldPair(
                      first: TextField(
                        controller: seoTitle,
                        decoration: InputDecoration(labelText: uiLiteral('SEO title * for publish')),
                      ),
                      second: TextField(
                        controller: canonical,
                        decoration: InputDecoration(labelText: uiLiteral('Canonical HTTPS URL * for publish')),
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: meta,
                      maxLines: 2,
                      decoration: InputDecoration(labelText: uiLiteral('Meta description * for publish')),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: keywords,
                      maxLines: 2,
                      decoration: InputDecoration(
                        labelText: uiLiteral('Page keywords'),
                        hintText: uiLiteral('arts, culture, communities'),
                        helperText: uiLiteral('Comma-separated · up to 24 unique page keywords'),
                      ),
                    ),
                    const SizedBox(height: 12),
                    ResponsiveFieldPair(
                      first: TextField(
                        controller: ogTitle,
                        decoration: InputDecoration(labelText: uiLiteral('Open Graph title')),
                      ),
                      second: mediaDropdown(
                        ogImage,
                        (value) => setState(() => ogImage = value),
                        label: 'Open Graph image',
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: ogDescription,
                      maxLines: 2,
                      decoration: InputDecoration(labelText: uiLiteral('Open Graph description')),
                    ),
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      value: noindex,
                      title: const LText('Noindex'),
                      subtitle: const LText('Published content stays available but is marked not to be indexed.'),
                      onChanged: (value) => setState(() => noindex = value),
                    ),
                    const Divider(height: 28),
                    ResponsiveActionBar(
                      breakpoint: 520,
                      leading: LText('Content sections', style: Theme.of(context).textTheme.titleMedium),
                      actions: [
                        OutlinedButton.icon(
                          onPressed: addSection,
                          icon: const Icon(Icons.add_rounded),
                          label: const LText('Add section'),
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    if (sections.isEmpty)
                      const _MessageCard(
                        icon: Icons.view_agenda_outlined,
                        title: 'No sections in this draft',
                        message: 'Add at least one visible section before publishing.',
                      )
                    else
                      for (var index = 0; index < sections.length; index++)
                        _CMSSectionEditor(
                          key: ValueKey<String>(sections[index].id + '-' + index.toString()),
                          section: sections[index],
                          index: index,
                          total: sections.length,
                          componentTypes: componentTypes,
                          media: widget.media,
                          onMoveUp: index > 0 ? () => move(index, -1) : null,
                          onMoveDown: index < sections.length - 1 ? () => move(index, 1) : null,
                          onRemove: () => setState(() => sections.removeAt(index)),
                          onChanged: () => setState(() {}),
                        ),
                  ],
                ),
              ),
              const Divider(height: 1),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 14, 20, 18),
                child: ResponsiveActionBar(
                  breakpoint: 520,
                  actions: [
                    TextButton(onPressed: () => Navigator.pop(context), child: const LText('Cancel')),
                    FilledButton.icon(
                      onPressed: () => Navigator.pop(context, payload()),
                      icon: const Icon(Icons.save_outlined),
                      label: const LText('Save new draft version'),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _CMSSectionEditor extends StatelessWidget {
  const _CMSSectionEditor({
    required this.section,
    required this.index,
    required this.total,
    required this.componentTypes,
    required this.media,
    required this.onMoveUp,
    required this.onMoveDown,
    required this.onRemove,
    required this.onChanged,
    super.key,
  });

  final CMSSectionDraft section;
  final int index;
  final int total;
  final List<String> componentTypes;
  final List<Map<String, dynamic>> media;
  final VoidCallback? onMoveUp;
  final VoidCallback? onMoveDown;
  final VoidCallback onRemove;
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context) {
    final mediaIds = media.map((e) => (e['id'] ?? '').toString()).toSet();
    final mediaValue = section.mediaAssetId.isEmpty || mediaIds.contains(section.mediaAssetId)
        ? section.mediaAssetId
        : '';

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(
              children: [
                Expanded(
                  child: LText(
                    'Section ' + (index + 1).toString(),
                    style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700),
                  ),
                ),
                IconButton(onPressed: onMoveUp, icon: const Icon(Icons.arrow_upward_rounded), tooltip: uiLiteral('Move up')),
                IconButton(onPressed: onMoveDown, icon: const Icon(Icons.arrow_downward_rounded), tooltip: uiLiteral('Move down')),
                IconButton(
                  onPressed: onRemove,
                  icon: const Icon(Icons.delete_outline_rounded),
                  color: brandDanger,
                  tooltip: uiLiteral('Remove from draft'),
                ),
              ],
            ),
            const SizedBox(height: 8),
            ResponsiveFieldPair(
              first: TextFormField(
                initialValue: section.id,
                decoration: InputDecoration(labelText: uiLiteral('Section ID *'), hintText: uiLiteral('hero, primary, secondary, modules, programs, impact, contact, story-video')),
                onChanged: (value) => section.id = value,
              ),
              second: DropdownButtonFormField<String>(
                value: componentTypes.contains(section.componentType) ? section.componentType : 'TEXT',
                decoration: InputDecoration(labelText: uiLiteral('Component type')),
                items: <DropdownMenuItem<String>>[
                  for (final type in componentTypes)
                    DropdownMenuItem<String>(
                      value: type,
                      child: LText(type.replaceAll('_', ' ')),
                    ),
                ],
                onChanged: (value) {
                  if (value != null) {
                    section.componentType = value;
                    onChanged();
                  }
                },
              ),
            ),
            const SizedBox(height: 12),
            TextFormField(
              initialValue: section.heading,
              decoration: InputDecoration(labelText: uiLiteral('Heading * when visible')),
              onChanged: (value) => section.heading = value,
            ),
            const SizedBox(height: 12),
            TextFormField(
              initialValue: section.body,
              maxLines: 4,
              decoration: InputDecoration(labelText: uiLiteral('Body')),
              onChanged: (value) => section.body = value,
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              value: mediaValue,
              decoration: InputDecoration(labelText: uiLiteral('Media asset')),
              items: <DropdownMenuItem<String>>[
                const DropdownMenuItem<String>(value: '', child: LText('No media')),
                for (final asset in media)
                  DropdownMenuItem<String>(
                    value: (asset['id'] ?? '').toString(),
                    child: LText(
                      (asset['original_filename'] ?? '').toString() +
                          ' · ' +
                          (asset['id'] ?? '').toString(),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
              ],
              onChanged: (value) {
                section.mediaAssetId = value ?? '';
                onChanged();
              },
            ),
            const SizedBox(height: 12),
            ResponsiveFieldPair(
              first: TextFormField(
                initialValue: section.ctaLabel,
                decoration: InputDecoration(labelText: uiLiteral('CTA label')),
                onChanged: (value) => section.ctaLabel = value,
              ),
              second: TextFormField(
                initialValue: section.ctaUrl,
                decoration: InputDecoration(labelText: uiLiteral('CTA URL')),
                onChanged: (value) => section.ctaUrl = value,
              ),
            ),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              value: section.visible,
              title: const LText('Visible'),
              subtitle: const LText('Turning this off preserves the section content but removes it from public/preview output.'),
              onChanged: (value) {
                section.visible = value;
                onChanged();
              },
            ),
          ],
        ),
      ),
    );
  }
}
