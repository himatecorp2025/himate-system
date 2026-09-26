part of 'main.dart';

class DesignGuidePanel extends StatefulWidget {
  const DesignGuidePanel({required this.api, required this.media, super.key});
  final Api api;
  final List<Map<String, dynamic>> media;

  @override
  State<DesignGuidePanel> createState() => _DesignGuidePanelState();
}

class _DesignGuidePanelState extends State<DesignGuidePanel> {
  final navy = TextEditingController();
  final gold = TextEditingController();
  final background = TextEditingController();
  final textColor = TextEditingController();
  final radius = TextEditingController();

  String logoMediaAssetId = '';
  Map<String, String> assetSlots = <String, String>{
    'header_wordmark': '',
    'footer_wordmark': '',
    'favicon': '',
    'app_icon': '',
    'login_logo': '',
    'email_logo': '',
  };
  String layoutKey = 'classic_editorial';
  String headingFont = 'Cormorant Garamond';
  String bodyFont = 'Inter';
  List<Map<String, dynamic>> navigation = <Map<String, dynamic>>[];
  bool loading = true;
  bool saving = false;
  String? error;
  int version = 0;
  DateTime? publishedAt;

  static const fonts = <String>['Cormorant Garamond', 'Inter', 'Georgia', 'Arial'];
  static const layouts = <String>['classic_editorial', 'modern_grid', 'minimal'];

  @override
  void initState() {
    super.initState();
    load();
  }

  @override
  void dispose() {
    navy.dispose();
    gold.dispose();
    background.dispose();
    textColor.dispose();
    radius.dispose();
    super.dispose();
  }

  Map<String, dynamic> defaultDesign() => <String, dynamic>{
        'logo_media_asset_id': '',
        'assets': <String, String>{
          'header_wordmark': '',
          'footer_wordmark': '',
          'favicon': '',
          'app_icon': '',
          'login_logo': '',
          'email_logo': '',
        },
        'layout_key': 'classic_editorial',
        'navy': '#06172C',
        'gold': '#D7AE62',
        'background': '#F8F9FB',
        'text_color': '#1F2937',
        'heading_font': 'Cormorant Garamond',
        'body_font': 'Inter',
        'button_radius': 6,
        'navigation': <Map<String, dynamic>>[
          {'label_en': 'Platform', 'label_hu': 'Platform', 'url': '/platform', 'visible': true, 'sort_order': 10},
          {'label_en': 'Modules', 'label_hu': 'Modulok', 'url': '/modules', 'visible': true, 'sort_order': 20},
          {'label_en': 'Programs', 'label_hu': 'Programok', 'url': '/programs', 'visible': true, 'sort_order': 30},
          {'label_en': 'Impact', 'label_hu': 'Hatás', 'url': '/impact', 'visible': true, 'sort_order': 40},
          {'label_en': 'Partners', 'label_hu': 'Partnerek', 'url': '/partners', 'visible': true, 'sort_order': 50},
          {'label_en': 'Contact', 'label_hu': 'Kapcsolat', 'url': '/contact', 'visible': true, 'sort_order': 60},
        ],
      };

  void applyDesign(Map<String, dynamic> raw) {
    final design = raw.isEmpty ? defaultDesign() : raw;
    navy.text = (design['navy'] ?? '#06172C').toString();
    gold.text = (design['gold'] ?? '#D7AE62').toString();
    background.text = (design['background'] ?? '#F8F9FB').toString();
    textColor.text = (design['text_color'] ?? '#1F2937').toString();
    radius.text = ((design['button_radius'] as num?)?.toInt() ?? 6).toString();
    logoMediaAssetId = (design['logo_media_asset_id'] ?? '').toString();
    final rawAssets = design['assets'];
    final nextAssets = <String, String>{
      'header_wordmark': '',
      'footer_wordmark': '',
      'favicon': '',
      'app_icon': '',
      'login_logo': '',
      'email_logo': '',
    };
    if (rawAssets is Map) {
      for (final entry in rawAssets.entries) {
        if (nextAssets.containsKey(entry.key.toString())) {
          nextAssets[entry.key.toString()] = entry.value?.toString() ?? '';
        }
      }
    }
    if ((nextAssets['header_wordmark'] ?? '').isEmpty && logoMediaAssetId.isNotEmpty) {
      nextAssets['header_wordmark'] = logoMediaAssetId;
    }
    assetSlots = nextAssets;
    logoMediaAssetId = assetSlots['header_wordmark'] ?? logoMediaAssetId;
    final requestedLayout = (design['layout_key'] ?? 'classic_editorial').toString();
    layoutKey = layouts.contains(requestedLayout) ? requestedLayout : 'classic_editorial';
    headingFont = fonts.contains((design['heading_font'] ?? '').toString()) ? (design['heading_font'] ?? '').toString() : 'Cormorant Garamond';
    bodyFont = fonts.contains((design['body_font'] ?? '').toString()) ? (design['body_font'] ?? '').toString() : 'Inter';
    final rawNav = design['navigation'];
    navigation = rawNav is List
        ? rawNav.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList()
        : List<Map<String, dynamic>>.from(defaultDesign()['navigation'] as List);
    navigation.sort((a, b) => ((a['sort_order'] as num?)?.toInt() ?? 0).compareTo((b['sort_order'] as num?)?.toInt() ?? 0));
  }

  Future<void> load() async {
    if (mounted) setState(() {
      loading = true;
      error = null;
    });
    try {
      final response = await widget.api.get('/api/v1/cms/design', force: true);
      if (!mounted) return;
      final draft = response['draft'];
      applyDesign(draft is Map ? Map<String, dynamic>.from(draft) : defaultDesign());
      version = (response['version'] as num?)?.toInt() ?? 0;
      publishedAt = DateTime.tryParse((response['published_at'] ?? '').toString())?.toLocal();
    } catch (e) {
      if (!mounted) return;
      error = e.toString();
      applyDesign(defaultDesign());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  bool validHex(String value) => RegExp(r'^#[0-9A-Fa-f]{6}$').hasMatch(value.trim());

  String? validate() {
    for (final value in <String>[navy.text, gold.text, background.text, textColor.text]) {
      if (!validHex(value)) return uiLiteral('Use six-digit hexadecimal colors, for example #06172C.');
    }
    final r = int.tryParse(radius.text.trim());
    if (r == null || r < 0 || r > 40) return uiLiteral('Button radius must be between 0 and 40.');
    if (navigation.length > 12) return uiLiteral('Navigation supports at most 12 items.');
    final orders = <int>{};
    for (var i = 0; i < navigation.length; i++) {
      final item = navigation[i];
      if ((item['label_en'] ?? '').toString().trim().isEmpty || (item['label_hu'] ?? '').toString().trim().isEmpty) {
        return uiLiteral('Every navigation item requires English and Hungarian labels.');
      }
      final url = (item['url'] ?? '').toString().trim();
      if (!(url.startsWith('/') && !url.startsWith('//')) && !url.startsWith('https://') && !url.startsWith('http://')) {
        return uiLiteral('Navigation URLs must be internal paths or absolute HTTP(S) URLs.');
      }
      final order = i * 10;
      if (orders.contains(order)) return uiLiteral('Navigation order values must be unique.');
      orders.add(order);
    }
    return null;
  }

  Map<String, dynamic> payload() => <String, dynamic>{
        'logo_media_asset_id': assetSlots['header_wordmark'] ?? logoMediaAssetId,
        'assets': <String, String>{
          for (final entry in assetSlots.entries)
            if (entry.value.trim().isNotEmpty) entry.key: entry.value.trim(),
        },
        'layout_key': layoutKey,
        'navy': navy.text.trim().toUpperCase(),
        'gold': gold.text.trim().toUpperCase(),
        'background': background.text.trim().toUpperCase(),
        'text_color': textColor.text.trim().toUpperCase(),
        'heading_font': headingFont,
        'body_font': bodyFont,
        'button_radius': int.tryParse(radius.text.trim()) ?? 6,
        'navigation': <Map<String, dynamic>>[
          for (var i = 0; i < navigation.length; i++)
            <String, dynamic>{
              'label_en': (navigation[i]['label_en'] ?? '').toString().trim(),
              'label_hu': (navigation[i]['label_hu'] ?? '').toString().trim(),
              'url': (navigation[i]['url'] ?? '').toString().trim(),
              'visible': navigation[i]['visible'] != false,
              'sort_order': i * 10,
            },
        ],
      };

  Future<bool> saveDraft({bool quiet = false}) async {
    final validation = validate();
    if (validation != null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: LText(validation), backgroundColor: brandDanger));
      }
      return false;
    }
    if (mounted) setState(() => saving = true);
    try {
      final response = await widget.api.put('/api/v1/cms/design/draft', payload());
      if (!mounted) return true;
      final draft = response['draft'];
      if (draft is Map) applyDesign(Map<String, dynamic>.from(draft));
      version = (response['version'] as num?)?.toInt() ?? version;
      if (!quiet) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: LText('Design draft saved.')));
      }
      return true;
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: LText(e.toString()), backgroundColor: brandDanger));
      }
      return false;
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> createPreview(String viewport) async {
    if (!await saveDraft(quiet: true)) return;
    if (mounted) setState(() => saving = true);
    try {
      final response = await widget.api.post('/api/v1/cms/design/preview');
      final pathKey = switch (viewport) {
        'tablet' => 'tablet_path',
        'mobile' => 'mobile_path',
        _ => 'desktop_path',
      };
      final path = (response[pathKey] ?? response['preview_path'] ?? '').toString();
      if (path.isEmpty) throw StateError('Design preview returned no preview path.');
      openBrowserDownload(path);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: LText('${viewport[0].toUpperCase()}${viewport.substring(1)} website preview opened.')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: LText(e.toString()), backgroundColor: brandDanger),
        );
      }
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> publish() async {
    if (!await saveDraft(quiet: true)) return;
    if (mounted) setState(() => saving = true);
    try {
      final response = await widget.api.post('/api/v1/cms/design/publish');
      if (!mounted) return;
      setState(() {
        version = (response['version'] as num?)?.toInt() ?? version;
        publishedAt = DateTime.tryParse((response['published_at'] ?? '').toString())?.toLocal();
      });
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: LText('Design Guide published to the public website.')));
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: LText(e.toString()), backgroundColor: brandDanger));
      }
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  void addNavigation() {
    if (navigation.length >= 12) return;
    setState(() {
      navigation.add(<String, dynamic>{
        'label_en': 'New item',
        'label_hu': 'Új elem',
        'url': '/',
        'visible': true,
        'sort_order': navigation.length * 10,
      });
    });
  }

  void moveNavigation(int index, int delta) {
    final target = index + delta;
    if (target < 0 || target >= navigation.length) return;
    setState(() {
      final item = navigation.removeAt(index);
      navigation.insert(target, item);
    });
  }

  Color parseColor(String value, Color fallback) {
    final clean = value.trim().replaceFirst('#', '');
    final parsed = int.tryParse(clean, radix: 16);
    return parsed == null || clean.length != 6 ? fallback : Color(0xFF000000 | parsed);
  }

  Widget colorField(String label, TextEditingController controller) => TextField(
        controller: controller,
        onChanged: (_) => setState(() {}),
        decoration: InputDecoration(labelText: uiLiteral(label), hintText: '#06172C'),
      );

  Widget assetField(String label, String slot, List<Map<String, dynamic>> imageMedia, {String emptyLabel = 'Use built-in asset'}) {
    final current = assetSlots[slot] ?? '';
    final known = current.isEmpty || imageMedia.any((asset) => (asset['id'] ?? '').toString() == current);
    return DropdownButtonFormField<String>(
      value: known ? current : '',
      decoration: InputDecoration(labelText: uiLiteral(label)),
      items: <DropdownMenuItem<String>>[
        DropdownMenuItem(value: '', child: LText(emptyLabel)),
        for (final asset in imageMedia)
          DropdownMenuItem(
            value: (asset['id'] ?? '').toString(),
            child: LText((asset['original_filename'] ?? asset['id'] ?? '').toString()),
          ),
      ],
      onChanged: (value) => setState(() {
        assetSlots[slot] = value ?? '';
        if (slot == 'header_wordmark') logoMediaAssetId = value ?? '';
      }),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (loading) return const _BrandLoading();

    final imageMedia = widget.media.where((asset) => (asset['mime_type'] ?? '').toString().startsWith('image/')).toList();
    final previewNavy = parseColor(navy.text, brandNavyDeep);
    final previewGold = parseColor(gold.text, brandGold);
    final previewBackground = parseColor(background.text, brandIvory);
    final previewText = parseColor(textColor.text, brandCharcoal);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Design Guide',
          subtitle: 'Global brand controls with independent header/footer logos, favicon, app/login/email assets, typography, layout family and bilingual navigation.',
          trailing: _MiniCounter(label: 'v$version'),
        ),
        const SizedBox(height: 12),
        if (error != null)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: _MessageCard(icon: Icons.warning_amber_rounded, title: 'Design settings loaded from defaults', message: error!),
          ),
        const _RuleStrip(items: [
          _RuleItem(Icons.palette_outlined, 'Brand', 'Published globally'),
          _RuleItem(Icons.language_outlined, 'Languages', 'English + Hungarian labels'),
          _RuleItem(Icons.visibility_outlined, 'Preview-safe', 'Draft before publish'),
          _RuleItem(Icons.history_rounded, 'Audit', 'Every save/publish recorded'),
        ]),
        const SizedBox(height: 16),
        LayoutBuilder(
          builder: (context, constraints) {
            final preview = Container(
              constraints: const BoxConstraints(minHeight: 220),
              padding: const EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: previewBackground,
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: brandMist),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    height: 54,
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    decoration: BoxDecoration(color: previewNavy, borderRadius: BorderRadius.circular(8)),
                    child: Row(children: [
                      Container(width: 24, height: 24, decoration: BoxDecoration(color: previewGold, shape: BoxShape.circle)),
                      const SizedBox(width: 12),
                      Expanded(child: LText('HIMATE website preview', style: TextStyle(color: previewBackground, fontWeight: FontWeight.w700))),
                    ]),
                  ),
                  const SizedBox(height: 20),
                  LText('Culture Fuels Tomorrow.', style: TextStyle(color: previewNavy, fontSize: 26, fontWeight: FontWeight.w700)),
                  const SizedBox(height: 8),
                  LText('The published Design Guide is applied without editing source code.', style: TextStyle(color: previewText)),
                  const SizedBox(height: 16),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                    decoration: BoxDecoration(color: previewGold, borderRadius: BorderRadius.circular(double.tryParse(radius.text) ?? 6)),
                    child: LText('Primary action', style: TextStyle(color: previewNavy, fontWeight: FontWeight.w700)),
                  ),
                ],
              ),
            );

            final controls = Column(
              children: [
                ResponsiveFieldPair(
                  first: assetField('Header wordmark / logo', 'header_wordmark', imageMedia, emptyLabel: 'Default HIMATE wordmark'),
                  second: assetField('Footer wordmark / logo', 'footer_wordmark', imageMedia, emptyLabel: 'Use header wordmark'),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: assetField('Browser favicon', 'favicon', imageMedia, emptyLabel: 'Default favicon'),
                  second: assetField('App / touch icon', 'app_icon', imageMedia, emptyLabel: 'Default app icon'),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: assetField('Login surface logo', 'login_logo', imageMedia, emptyLabel: 'Use header wordmark'),
                  second: assetField('Email / document logo', 'email_logo', imageMedia, emptyLabel: 'Use header wordmark'),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: layoutKey,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Layout family'),
                    helperText: uiLiteral('Content and system logic remain unchanged when the layout family changes.'),
                  ),
                  items: const [
                    DropdownMenuItem(value: 'classic_editorial', child: LText('Classic editorial')),
                    DropdownMenuItem(value: 'modern_grid', child: LText('Modern grid')),
                    DropdownMenuItem(value: 'minimal', child: LText('Minimal')),
                  ],
                  onChanged: (value) {
                    if (value != null) setState(() => layoutKey = value);
                  },
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: colorField('Primary navy', navy),
                  second: colorField('Brand gold', gold),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: colorField('Page background', background),
                  second: colorField('Body text color', textColor),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: DropdownButtonFormField<String>(
                    value: headingFont,
                    decoration: InputDecoration(labelText: uiLiteral('Heading font')),
                    items: [for (final font in fonts) DropdownMenuItem(value: font, child: LText(font))],
                    onChanged: (value) {
                      if (value != null) setState(() => headingFont = value);
                    },
                  ),
                  second: DropdownButtonFormField<String>(
                    value: bodyFont,
                    decoration: InputDecoration(labelText: uiLiteral('Body font')),
                    items: [for (final font in fonts) DropdownMenuItem(value: font, child: LText(font))],
                    onChanged: (value) {
                      if (value != null) setState(() => bodyFont = value);
                    },
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: radius,
                  keyboardType: TextInputType.number,
                  onChanged: (_) => setState(() {}),
                  decoration: InputDecoration(labelText: uiLiteral('Button corner radius'), helperText: uiLiteral('0–40 pixels')),
                ),
              ],
            );

            if (constraints.maxWidth < 960) {
              return Column(children: [controls, const SizedBox(height: 16), preview]);
            }
            return Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(child: controls),
                const SizedBox(width: 18),
                Expanded(child: preview),
              ],
            );
          },
        ),
        const SizedBox(height: 22),
        _SectionHeader(
          title: 'Website Navigation',
          subtitle: 'Control visible public menu items, bilingual labels and order.',
          trailing: OutlinedButton.icon(
            onPressed: navigation.length >= 12 ? null : addNavigation,
            icon: const Icon(Icons.add_rounded),
            label: const LText('Add navigation item'),
          ),
        ),
        const SizedBox(height: 12),
        if (navigation.isEmpty)
          const _MessageCard(icon: Icons.menu_open_rounded, title: 'No navigation items', message: 'Add at least one item for public navigation.')
        else
          Column(
            children: [
              for (var index = 0; index < navigation.length; index++)
                Padding(
                  padding: const EdgeInsets.only(bottom: 10),
                  child: Card(
                    child: Padding(
                      padding: const EdgeInsets.all(14),
                      child: Column(
                        children: [
                          Row(children: [
                            Expanded(child: LText('Navigation item ${index + 1}', style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700))),
                            IconButton(
                              onPressed: index > 0 ? () => moveNavigation(index, -1) : null,
                              tooltip: uiLiteral('Move up'),
                              icon: const Icon(Icons.arrow_upward_rounded),
                            ),
                            IconButton(
                              onPressed: index < navigation.length - 1 ? () => moveNavigation(index, 1) : null,
                              tooltip: uiLiteral('Move down'),
                              icon: const Icon(Icons.arrow_downward_rounded),
                            ),
                            IconButton(
                              onPressed: () => setState(() => navigation.removeAt(index)),
                              tooltip: uiLiteral('Remove'),
                              icon: const Icon(Icons.delete_outline_rounded, color: brandDanger),
                            ),
                          ]),
                          const SizedBox(height: 8),
                          ResponsiveFieldPair(
                            first: TextFormField(
                              initialValue: (navigation[index]['label_en'] ?? '').toString(),
                              decoration: InputDecoration(labelText: uiLiteral('English label')),
                              onChanged: (value) => navigation[index]['label_en'] = value,
                            ),
                            second: TextFormField(
                              initialValue: (navigation[index]['label_hu'] ?? '').toString(),
                              decoration: InputDecoration(labelText: uiLiteral('Hungarian label')),
                              onChanged: (value) => navigation[index]['label_hu'] = value,
                            ),
                          ),
                          const SizedBox(height: 10),
                          ResponsiveFieldPair(
                            first: TextFormField(
                              initialValue: (navigation[index]['url'] ?? '').toString(),
                              decoration: InputDecoration(labelText: uiLiteral('URL')),
                              onChanged: (value) => navigation[index]['url'] = value,
                            ),
                            second: SwitchListTile(
                              contentPadding: EdgeInsets.zero,
                              title: const LText('Visible in navigation'),
                              value: navigation[index]['visible'] != false,
                              onChanged: (value) => setState(() => navigation[index]['visible'] = value),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
            ],
          ),
        const SizedBox(height: 12),
        ResponsiveActionBar(
          leading: LText(
            publishedAt == null
                ? 'No Design Guide publication yet'
                : 'Published ${HimateI18n.dateTime(HimateI18n.activeLocale, publishedAt!)}',
            style: const TextStyle(color: brandTextSoft, fontSize: 10.5),
          ),
          actions: [
            OutlinedButton.icon(
              onPressed: saving ? null : () => saveDraft(),
              icon: const Icon(Icons.save_outlined),
              label: const LText('Save design draft'),
            ),
            OutlinedButton.icon(
              onPressed: saving ? null : () => createPreview('desktop'),
              icon: const Icon(Icons.desktop_windows_outlined),
              label: const LText('Desktop preview'),
            ),
            OutlinedButton.icon(
              onPressed: saving ? null : () => createPreview('tablet'),
              icon: const Icon(Icons.tablet_mac_outlined),
              label: const LText('Tablet preview'),
            ),
            OutlinedButton.icon(
              onPressed: saving ? null : () => createPreview('mobile'),
              icon: const Icon(Icons.phone_iphone_outlined),
              label: const LText('Mobile preview'),
            ),
            FilledButton.icon(
              onPressed: saving ? null : publish,
              icon: const Icon(Icons.publish_outlined),
              label: const LText('Publish design'),
            ),
          ],
        ),
      ],
    );
  }
}
