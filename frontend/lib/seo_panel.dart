part of 'main.dart';

class SEOKeywordsPanel extends StatefulWidget {
  const SEOKeywordsPanel({required this.api, required this.media, super.key});

  final Api api;
  final List<Map<String, dynamic>> media;

  @override
  State<SEOKeywordsPanel> createState() => _SEOKeywordsPanelState();
}

class _SEOKeywordsPanelState extends State<SEOKeywordsPanel> {
  final keywordsEN = TextEditingController();
  final keywordsHU = TextEditingController();
  final organizationName = TextEditingController();
  final organizationURL = TextEditingController();

  String defaultOGImage = '';
  List<Map<String, dynamic>> auditItems = <Map<String, dynamic>>[];
  Map<String, dynamic> auditSummary = <String, dynamic>{};
  bool loading = true;
  bool saving = false;
  String? error;
  int version = 0;
  DateTime? publishedAt;

  @override
  void initState() {
    super.initState();
    load();
  }

  @override
  void dispose() {
    keywordsEN.dispose();
    keywordsHU.dispose();
    organizationName.dispose();
    organizationURL.dispose();
    super.dispose();
  }

  List<String> parseKeywords(String value) => value
      .split(RegExp(r'[,;\n]'))
      .map((item) => item.trim())
      .where((item) => item.isNotEmpty)
      .toSet()
      .take(30)
      .toList();

  String joinKeywords(dynamic value) {
    if (value is! List) return '';
    return value
        .map((item) => item.toString().trim())
        .where((item) => item.isNotEmpty)
        .join(', ');
  }

  Future<void> load() async {
    if (mounted) {
      setState(() {
        loading = true;
        error = null;
      });
    }

    try {
      final responses = await Future.wait<Map<String, dynamic>>([
        widget.api.get('/api/v1/cms/seo', force: true),
        widget.api.get('/api/v1/cms/seo/audit', force: true),
      ]);
      if (!mounted) return;

      final settings = responses[0];
      final audit = responses[1];
      final draft = settings['draft'] is Map
          ? Map<String, dynamic>.from(settings['draft'] as Map)
          : <String, dynamic>{};

      setState(() {
        keywordsEN.text = joinKeywords(draft['global_keywords_en']);
        keywordsHU.text = joinKeywords(draft['global_keywords_hu']);
        organizationName.text =
            (draft['organization_name'] ?? 'HIMATE System').toString();
        organizationURL.text =
            (draft['organization_url'] ?? 'https://www.himate.com').toString();
        defaultOGImage =
            (draft['default_og_image_asset_id'] ?? '').toString();
        version = (settings['version'] as num?)?.toInt() ?? 0;
        publishedAt =
            DateTime.tryParse((settings['published_at'] ?? '').toString())
                ?.toLocal();
        auditItems = items(audit);
        auditSummary = audit['summary'] is Map
            ? Map<String, dynamic>.from(audit['summary'] as Map)
            : <String, dynamic>{};
      });
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Map<String, dynamic> payload() => <String, dynamic>{
        'global_keywords_en': parseKeywords(keywordsEN.text),
        'global_keywords_hu': parseKeywords(keywordsHU.text),
        'organization_name': organizationName.text.trim(),
        'organization_url': organizationURL.text.trim(),
        'default_og_image_asset_id': defaultOGImage,
      };

  Future<void> refreshAudit() async {
    try {
      final response =
          await widget.api.get('/api/v1/cms/seo/audit', force: true);
      if (!mounted) return;
      setState(() {
        auditItems = items(response);
        auditSummary = response['summary'] is Map
            ? Map<String, dynamic>.from(response['summary'] as Map)
            : <String, dynamic>{};
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: LText(e.toString()),
            backgroundColor: brandDanger,
          ),
        );
      }
    }
  }

  Future<bool> saveDraft({bool quiet = false}) async {
    if (mounted) setState(() => saving = true);
    try {
      final response =
          await widget.api.put('/api/v1/cms/seo/draft', payload());
      if (!mounted) return true;

      setState(() {
        version = (response['version'] as num?)?.toInt() ?? version;
      });

      if (!quiet) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: LText('SEO settings draft saved.')),
        );
      }
      await refreshAudit();
      return true;
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: LText(e.toString()),
            backgroundColor: brandDanger,
          ),
        );
      }
      return false;
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> publish() async {
    if (!await saveDraft(quiet: true)) return;
    if (mounted) setState(() => saving = true);

    try {
      final response = await widget.api.post('/api/v1/cms/seo/publish');
      if (!mounted) return;

      setState(() {
        version = (response['version'] as num?)?.toInt() ?? version;
        publishedAt =
            DateTime.tryParse((response['published_at'] ?? '').toString())
                ?.toLocal();
      });

      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: LText('SEO settings published to the public website.'),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: LText(e.toString()),
            backgroundColor: brandDanger,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Color scoreTone(int score) {
    if (score >= 80) return brandSuccess;
    if (score >= 60) return brandGold;
    return brandDanger;
  }

  Widget auditCard(Map<String, dynamic> item, double width) {
    final score = (item['score'] as num?)?.toInt() ?? 0;
    final locale = (item['locale'] ?? 'en_US').toString();
    final rawIssues = item['issues'];
    final issues = rawIssues is List
        ? rawIssues
            .whereType<Map>()
            .map((value) => Map<String, dynamic>.from(value))
            .toList()
        : <Map<String, dynamic>>[];
    final rawSuggestions = item['suggested_keywords'];
    final suggestions = rawSuggestions is List
        ? rawSuggestions
            .map((value) => value.toString().trim())
            .where((value) => value.isNotEmpty)
            .toList()
        : <String>[];

    return SizedBox(
      width: width,
      child: Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: scoreTone(score).withOpacity(.10),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: LText(
                      score.toString(),
                      style: TextStyle(
                        color: scoreTone(score),
                        fontWeight: FontWeight.w800,
                        fontSize: 14,
                      ),
                    ),
                  ),
                  const SizedBox(width: 11),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        LText(
                          (item['name'] ?? item['page_key'] ?? '').toString(),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: brandNavy,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: 2),
                        LText(
                          locale == 'hu_HU'
                              ? 'Magyar · SEO audit'
                              : 'English (US) · SEO audit',
                          style: const TextStyle(
                            color: brandTextSoft,
                            fontSize: 10,
                          ),
                        ),
                      ],
                    ),
                  ),
                  _StatusPill(
                    label: (item['status'] ?? 'UNKNOWN').toString(),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              _DefinitionRow(
                label: 'SEO title length',
                value: (item['title_length'] ?? 0).toString(),
              ),
              _DefinitionRow(
                label: 'Meta description length',
                value: (item['meta_length'] ?? 0).toString(),
              ),
              _DefinitionRow(
                label: 'Visible content characters',
                value: (item['content_characters'] ?? 0).toString(),
              ),
              if (issues.isNotEmpty) ...[
                const SizedBox(height: 10),
                for (final issue in issues.take(4))
                  Padding(
                    padding: const EdgeInsets.only(bottom: 5),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(
                          (issue['severity'] ?? '').toString() == 'ERROR'
                              ? Icons.error_outline_rounded
                              : Icons.info_outline_rounded,
                          size: 15,
                          color:
                              (issue['severity'] ?? '').toString() == 'ERROR'
                                  ? brandDanger
                                  : brandGold,
                        ),
                        const SizedBox(width: 7),
                        Expanded(
                          child: LText(
                            (issue['message'] ?? '').toString(),
                            style: const TextStyle(
                              color: brandCharcoal,
                              fontSize: 10.5,
                              height: 1.35,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
              if (suggestions.isNotEmpty) ...[
                const SizedBox(height: 8),
                LText(
                  'Suggested keywords',
                  style: Theme.of(context).textTheme.titleSmall,
                ),
                const SizedBox(height: 6),
                Wrap(
                  spacing: 6,
                  runSpacing: 6,
                  children: [
                    for (final keyword in suggestions.take(8))
                      Chip(
                        visualDensity: VisualDensity.compact,
                        label: LText(
                          keyword,
                          style: const TextStyle(fontSize: 9.5),
                        ),
                      ),
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (loading) return const _BrandLoading();

    final imageMedia = widget.media
        .where(
          (asset) =>
              (asset['mime_type'] ?? '').toString().startsWith('image/'),
        )
        .toList();
    final knownImage = defaultOGImage.isEmpty ||
        imageMedia.any(
          (asset) => (asset['id'] ?? '').toString() == defaultOGImage,
        );
    final average =
        (auditSummary['average_score'] as num?)?.toInt() ?? 0;
    final ready = (auditSummary['ready'] as num?)?.toInt() ?? 0;
    final attention =
        (auditSummary['needs_attention'] as num?)?.toInt() ?? 0;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'SEO & Keywords',
          subtitle:
              'Global bilingual keywords, structured SEO defaults and automatic page-content analysis.',
          trailing: _MiniCounter(label: 'v$version'),
        ),
        const SizedBox(height: 12),
        if (error != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: _MessageCard(
              icon: Icons.warning_amber_rounded,
              title: 'SEO settings unavailable',
              message: error!,
            ),
          ),
        const _RuleStrip(
          items: [
            _RuleItem(
              Icons.language_rounded,
              'Bilingual keywords',
              'English + Hungarian',
            ),
            _RuleItem(
              Icons.auto_graph_rounded,
              'Automatic audit',
              'Title, meta, content and keyword fit',
            ),
            _RuleItem(
              Icons.data_object_rounded,
              'Structured data',
              'Schema.org WebPage JSON-LD',
            ),
            _RuleItem(
              Icons.alt_route_rounded,
              'hreflang',
              'Published EN/HU alternates',
            ),
          ],
        ),
        const SizedBox(height: 16),
        ResponsiveFieldPair(
          first: TextField(
            controller: keywordsEN,
            minLines: 2,
            maxLines: 4,
            decoration: InputDecoration(
              labelText: uiLiteral('Global keywords · English'),
              hintText: uiLiteral(
                'arts, culture, cultural organizations, digital platform',
              ),
              helperText: uiLiteral(
                'Comma-separated · up to 30 unique keywords',
              ),
            ),
          ),
          second: TextField(
            controller: keywordsHU,
            minLines: 2,
            maxLines: 4,
            decoration: InputDecoration(
              labelText: uiLiteral('Global keywords · Hungarian'),
              hintText: uiLiteral(
                'művészet, kultúra, kulturális szervezetek, digitális platform',
              ),
              helperText: uiLiteral(
                'Vesszővel elválasztva · legfeljebb 30 egyedi kulcsszó',
              ),
            ),
          ),
        ),
        const SizedBox(height: 12),
        ResponsiveFieldPair(
          first: TextField(
            controller: organizationName,
            decoration: InputDecoration(
              labelText: uiLiteral('Schema.org organization name'),
            ),
          ),
          second: TextField(
            controller: organizationURL,
            decoration: InputDecoration(
              labelText: uiLiteral('Schema.org organization HTTPS URL'),
              hintText: 'https://www.himate.com',
            ),
          ),
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          value: knownImage ? defaultOGImage : '',
          decoration: InputDecoration(
            labelText: uiLiteral('Default Open Graph image'),
          ),
          items: <DropdownMenuItem<String>>[
            const DropdownMenuItem(
              value: '',
              child: LText('No global default image'),
            ),
            for (final asset in imageMedia)
              DropdownMenuItem(
                value: (asset['id'] ?? '').toString(),
                child: LText(
                  (asset['original_filename'] ?? asset['id'] ?? '').toString(),
                ),
              ),
          ],
          onChanged: (value) {
            setState(() => defaultOGImage = value ?? '');
          },
        ),
        const SizedBox(height: 14),
        ResponsiveActionBar(
          leading: LText(
            publishedAt == null
                ? 'SEO settings have not been published yet'
                : 'SEO published ${HimateI18n.dateTime(HimateI18n.activeLocale, publishedAt!)}',
            style: const TextStyle(
              color: brandTextSoft,
              fontSize: 10.5,
            ),
          ),
          actions: [
            OutlinedButton.icon(
              onPressed: saving ? null : () => saveDraft(),
              icon: const Icon(Icons.save_outlined),
              label: const LText('Save SEO draft'),
            ),
            FilledButton.icon(
              onPressed: saving ? null : publish,
              icon: const Icon(Icons.publish_outlined),
              label: const LText('Publish SEO settings'),
            ),
          ],
        ),
        const SizedBox(height: 24),
        _SectionHeader(
          title: 'Automatic SEO Audit',
          subtitle:
              'Draft-aware analysis of every CMS page using its language-specific content and keywords.',
          trailing: OutlinedButton.icon(
            onPressed: refreshAudit,
            icon: const Icon(Icons.refresh_rounded),
            label: const LText('Refresh audit'),
          ),
        ),
        const SizedBox(height: 10),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _MiniCounter(label: 'Average $average/100'),
            _MiniCounter(label: '$ready ready'),
            _MiniCounter(label: '$attention need attention'),
          ],
        ),
        const SizedBox(height: 12),
        if (auditItems.isEmpty)
          const _MessageCard(
            icon: Icons.auto_graph_rounded,
            title: 'No pages available for SEO audit',
            message:
                'Create a CMS page draft to begin automatic SEO analysis.',
          )
        else
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 760
                  ? constraints.maxWidth
                  : (constraints.maxWidth - 12) / 2;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final item in auditItems)
                    auditCard(item, width),
                ],
              );
            },
          ),
      ],
    );
  }
}
