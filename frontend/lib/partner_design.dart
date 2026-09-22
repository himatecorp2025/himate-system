part of 'main.dart';

extension PartnerDesignUI on _PartnerPortalShellState {
  Color partnerThemeColor(dynamic value, Color fallback) {
    final text = value?.toString().trim().replaceFirst('#', '') ?? '';
    final parsed = int.tryParse(text, radix: 16);
    return parsed == null || text.length != 6 ? fallback : Color(0xFF000000 | parsed);
  }

  Future<void> uploadDesignMedia() async {
    if (!can('design.write')) return;
    final file = await pickBrowserFile('image/png,image/jpeg,image/webp');
    if (file == null) return;
    final alt = TextEditingController(text: file.name);
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'Upload brand asset',
        subtitle: 'Upload a partner-owned image for a logo, favicon, app icon, login logo or email/document logo.',
        icon: Icons.upload_file_outlined,
        primaryLabel: 'Upload asset',
        onPrimary: () => Navigator.pop(dialogContext, true),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _DefinitionRow(label: 'File', value: file.name),
            const SizedBox(height: 12),
            TextField(
              controller: alt,
              decoration: InputDecoration(labelText: uiLiteral('Description / alt text')),
            ),
          ],
        ),
      ),
    );
    if (ok == true) {
      try {
        final bytes = await readBrowserFile(file);
        await widget.api.multipart(
          '/partner/api/v1/design/media',
          <String, String>{'alt_text': alt.text.trim()},
          bytes,
          file.name,
        );
        await load();
        if (mounted) toast('Brand asset uploaded.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }
    alt.dispose();
  }

  Future<void> activateDesignProfile(Map<String, dynamic> profile) async {
    if (!can('design.write')) return;
    final id = (profile['id'] ?? '').toString();
    if (id.isEmpty) return;
    final name = (profile['name'] ?? id).toString();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: LText('Activate ' + name + '?'),
        content: const LText(
          'Only the active visual theme changes. CMS content, partner data, modules, billing, workflows and system mechanics remain unchanged.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const LText('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const LText('Activate design'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.post('/partner/api/v1/design/profiles/' + id + '/activate');
      await load();
      if (mounted) toast('Design activated. Content and system logic were preserved.');
    } catch (e) {
      if (mounted) toast(e.toString(), failure: true);
    }
  }

  Future<void> editDesignProfile([Map<String, dynamic>? existing]) async {
    if (!can('design.write')) return;
    final sourceTheme = existing?['theme'] is Map
        ? Map<String, dynamic>.from(existing!['theme'] as Map)
        : <String, dynamic>{
            'layout_key': 'classic_editorial',
            'navy': '#06172C',
            'gold': '#D7AE62',
            'background': '#F8F9FB',
            'text_color': '#1F2937',
            'heading_font': 'Cormorant Garamond',
            'body_font': 'Inter',
            'button_radius': 6,
            'assets': <String, dynamic>{},
          };

    final name = TextEditingController(
      text: existing?['name']?.toString() ?? 'My Brand Theme',
    );
    final description = TextEditingController(
      text: existing?['description']?.toString() ?? '',
    );
    final navy = TextEditingController(text: (sourceTheme['navy'] ?? '#06172C').toString());
    final gold = TextEditingController(text: (sourceTheme['gold'] ?? '#D7AE62').toString());
    final background = TextEditingController(text: (sourceTheme['background'] ?? '#F8F9FB').toString());
    final textColor = TextEditingController(text: (sourceTheme['text_color'] ?? '#1F2937').toString());
    final radius = TextEditingController(text: (sourceTheme['button_radius'] ?? 6).toString());

    String layout = (sourceTheme['layout_key'] ?? 'classic_editorial').toString();
    String headingFont = (sourceTheme['heading_font'] ?? 'Cormorant Garamond').toString();
    String bodyFont = (sourceTheme['body_font'] ?? 'Inter').toString();

    final assets = <String, String>{
      'header_wordmark': '',
      'footer_wordmark': '',
      'favicon': '',
      'app_icon': '',
      'login_logo': '',
      'email_logo': '',
    };
    final rawAssets = sourceTheme['assets'];
    if (rawAssets is Map) {
      for (final entry in rawAssets.entries) {
        final key = entry.key.toString();
        if (assets.containsKey(key)) assets[key] = entry.value?.toString() ?? '';
      }
    }

    final ok = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) {
          Widget assetField(String label, String slot) {
            final current = assets[slot] ?? '';
            final known = current.isEmpty ||
                designMedia.any((item) => (item['id'] ?? '').toString() == current);
            return DropdownButtonFormField<String>(
              value: known ? current : '',
              decoration: InputDecoration(labelText: uiLiteral(label)),
              items: <DropdownMenuItem<String>>[
                const DropdownMenuItem(
                  value: '',
                  child: LText('No custom asset'),
                ),
                for (final item in designMedia)
                  DropdownMenuItem(
                    value: (item['id'] ?? '').toString(),
                    child: LText(
                      (item['original_filename'] ?? item['id'] ?? '').toString(),
                    ),
                  ),
              ],
              onChanged: (value) => setLocal(() => assets[slot] = value ?? ''),
            );
          }

          return BrandDialog(
            title: existing == null ? 'Create custom design' : 'Edit custom design',
            subtitle: 'A design profile contains only visual tokens and brand assets. Content, data and business logic are deliberately outside the theme.',
            icon: Icons.palette_outlined,
            width: 880,
            primaryLabel: existing == null ? 'Create design' : 'Save design',
            onPrimary: () {
              final hex = RegExp(r'^#[0-9A-Fa-f]{6}$');
              final validColors = <String>[
                navy.text,
                gold.text,
                background.text,
                textColor.text,
              ].every((value) => hex.hasMatch(value.trim()));
              final parsedRadius = int.tryParse(radius.text.trim());
              if (name.text.trim().length < 2 ||
                  !validColors ||
                  parsedRadius == null ||
                  parsedRadius < 0 ||
                  parsedRadius > 40) {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(
                    content: LText(
                      'Complete the name, valid six-digit colors and a button radius from 0 to 40.',
                    ),
                  ),
                );
                return;
              }
              Navigator.pop(dialogContext, true);
            },
            child: Column(
              children: [
                ResponsiveFieldPair(
                  first: TextField(
                    controller: name,
                    decoration: InputDecoration(labelText: uiLiteral('Design name')),
                  ),
                  second: DropdownButtonFormField<String>(
                    value: layout,
                    decoration: InputDecoration(labelText: uiLiteral('Layout family')),
                    items: const [
                      DropdownMenuItem(
                        value: 'classic_editorial',
                        child: LText('Classic editorial'),
                      ),
                      DropdownMenuItem(
                        value: 'modern_grid',
                        child: LText('Modern grid'),
                      ),
                      DropdownMenuItem(
                        value: 'minimal',
                        child: LText('Minimal'),
                      ),
                    ],
                    onChanged: (value) {
                      if (value != null) setLocal(() => layout = value);
                    },
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: description,
                  maxLines: 2,
                  decoration: InputDecoration(labelText: uiLiteral('Description')),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: TextField(
                    controller: navy,
                    decoration: InputDecoration(labelText: uiLiteral('Primary color')),
                  ),
                  second: TextField(
                    controller: gold,
                    decoration: InputDecoration(labelText: uiLiteral('Accent color')),
                  ),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: TextField(
                    controller: background,
                    decoration: InputDecoration(labelText: uiLiteral('Background color')),
                  ),
                  second: TextField(
                    controller: textColor,
                    decoration: InputDecoration(labelText: uiLiteral('Text color')),
                  ),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: DropdownButtonFormField<String>(
                    value: headingFont,
                    decoration: InputDecoration(labelText: uiLiteral('Heading font')),
                    items: const [
                      DropdownMenuItem(value: 'Cormorant Garamond', child: LText('Cormorant Garamond')),
                      DropdownMenuItem(value: 'Inter', child: LText('Inter')),
                      DropdownMenuItem(value: 'Georgia', child: LText('Georgia')),
                      DropdownMenuItem(value: 'Arial', child: LText('Arial')),
                    ],
                    onChanged: (value) {
                      if (value != null) setLocal(() => headingFont = value);
                    },
                  ),
                  second: DropdownButtonFormField<String>(
                    value: bodyFont,
                    decoration: InputDecoration(labelText: uiLiteral('Body font')),
                    items: const [
                      DropdownMenuItem(value: 'Cormorant Garamond', child: LText('Cormorant Garamond')),
                      DropdownMenuItem(value: 'Inter', child: LText('Inter')),
                      DropdownMenuItem(value: 'Georgia', child: LText('Georgia')),
                      DropdownMenuItem(value: 'Arial', child: LText('Arial')),
                    ],
                    onChanged: (value) {
                      if (value != null) setLocal(() => bodyFont = value);
                    },
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: radius,
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Button radius'),
                    helperText: uiLiteral('0–40 pixels'),
                  ),
                ),
                const SizedBox(height: 18),
                const _SectionHeader(
                  title: 'Brand asset slots',
                  subtitle: 'Each visual surface can use a different partner-owned image.',
                ),
                const SizedBox(height: 10),
                ResponsiveFieldPair(
                  first: assetField('Header wordmark / logo', 'header_wordmark'),
                  second: assetField('Footer wordmark / logo', 'footer_wordmark'),
                ),
                const SizedBox(height: 10),
                ResponsiveFieldPair(
                  first: assetField('Browser favicon', 'favicon'),
                  second: assetField('App / touch icon', 'app_icon'),
                ),
                const SizedBox(height: 10),
                ResponsiveFieldPair(
                  first: assetField('Login surface logo', 'login_logo'),
                  second: assetField('Email / document logo', 'email_logo'),
                ),
              ],
            ),
          );
        },
      ),
    );

    if (ok == true) {
      final payload = <String, dynamic>{
        'name': name.text.trim(),
        'description': description.text.trim(),
        'theme': <String, dynamic>{
          'layout_key': layout,
          'navy': navy.text.trim().toUpperCase(),
          'gold': gold.text.trim().toUpperCase(),
          'background': background.text.trim().toUpperCase(),
          'text_color': textColor.text.trim().toUpperCase(),
          'heading_font': headingFont,
          'body_font': bodyFont,
          'button_radius': int.tryParse(radius.text.trim()) ?? 6,
          'assets': <String, String>{
            for (final entry in assets.entries)
              if (entry.value.trim().isNotEmpty) entry.key: entry.value.trim(),
          },
        },
      };
      try {
        if (existing == null) {
          await widget.api.post('/partner/api/v1/design/profiles', payload);
        } else {
          await widget.api.put(
            '/partner/api/v1/design/profiles/' + (existing['id'] ?? '').toString(),
            payload,
          );
        }
        await load();
        if (mounted) {
          toast(existing == null ? 'Custom design created.' : 'Custom design updated.');
        }
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }

    for (final controller in <TextEditingController>[
      name,
      description,
      navy,
      gold,
      background,
      textColor,
      radius,
    ]) {
      controller.dispose();
    }
  }

  Widget designPage() {
    final activeID = (designState['active_profile_id'] ?? '').toString();
    return Content(
      eyebrow: 'BRAND & EXPERIENCE',
      title: 'Design',
      subtitle: 'Choose a visual family or create your own. Theme changes never modify content, modules, billing, workflows or system mechanics.',
      actions: [
        if (can('design.write'))
          OutlinedButton.icon(
            onPressed: uploadDesignMedia,
            icon: const Icon(Icons.upload_file_outlined),
            label: const LText('Upload brand asset'),
          ),
        if (can('design.write'))
          FilledButton.icon(
            onPressed: () => editDesignProfile(),
            icon: const Icon(Icons.add_rounded),
            label: const LText('New custom design'),
          ),
      ],
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _RuleStrip(items: [
            _RuleItem(Icons.layers_outlined, 'Theme only', 'Content stays unchanged'),
            _RuleItem(Icons.settings_suggest_outlined, 'Mechanics', 'System logic stays unchanged'),
            _RuleItem(Icons.lock_person_outlined, 'Tenant isolated', 'Your assets only'),
            _RuleItem(Icons.palette_outlined, 'Brand slots', 'Logo · favicon · app · login · email'),
          ]),
          const SizedBox(height: 22),
          _SectionHeader(
            title: 'Available design profiles',
            subtitle: 'Catalog designs and your organization’s own profiles. Commercial package rules can later restrict which catalog profiles are selectable without changing this data model.',
            trailing: _MiniCounter(label: designProfiles.length.toString() + ' designs'),
          ),
          const SizedBox(height: 12),
          if (designProfiles.isEmpty)
            const _MessageCard(
              icon: Icons.palette_outlined,
              title: 'No designs available',
              message: 'Design profiles will appear when the design catalog is available.',
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 660
                    ? constraints.maxWidth
                    : constraints.maxWidth < 1080
                        ? (constraints.maxWidth - 12) / 2
                        : (constraints.maxWidth - 24) / 3;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final profile in designProfiles)
                      SizedBox(
                        width: width,
                        child: Builder(
                          builder: (context) {
                            final theme = profile['theme'] is Map
                                ? Map<String, dynamic>.from(profile['theme'] as Map)
                                : <String, dynamic>{};
                            final active = (profile['id'] ?? '').toString() == activeID;
                            final primary = partnerThemeColor(theme['navy'], brandNavyDeep);
                            final accent = partnerThemeColor(theme['gold'], brandGold);
                            final bg = partnerThemeColor(theme['background'], brandIvory);
                            final radiusValue = number(theme['button_radius']).clamp(0, 20).toDouble();
                            return Card(
                              child: Padding(
                                padding: const EdgeInsets.all(16),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Container(
                                      height: 92,
                                      decoration: BoxDecoration(
                                        color: bg,
                                        borderRadius: BorderRadius.circular(10),
                                        border: Border.all(color: brandMist),
                                      ),
                                      padding: const EdgeInsets.all(12),
                                      child: Column(
                                        crossAxisAlignment: CrossAxisAlignment.start,
                                        children: [
                                          Container(
                                            height: 22,
                                            decoration: BoxDecoration(
                                              color: primary,
                                              borderRadius: BorderRadius.circular(4),
                                            ),
                                          ),
                                          const SizedBox(height: 10),
                                          Row(
                                            children: [
                                              Expanded(
                                                child: Container(
                                                  height: 20,
                                                  decoration: BoxDecoration(
                                                    color: primary.withOpacity(.15),
                                                    borderRadius: BorderRadius.circular(4),
                                                  ),
                                                ),
                                              ),
                                              const SizedBox(width: 8),
                                              Container(
                                                width: 54,
                                                height: 20,
                                                decoration: BoxDecoration(
                                                  color: accent,
                                                  borderRadius: BorderRadius.circular(radiusValue),
                                                ),
                                              ),
                                            ],
                                          ),
                                        ],
                                      ),
                                    ),
                                    const SizedBox(height: 12),
                                    Row(
                                      children: [
                                        Expanded(
                                          child: LText(
                                            (profile['name'] ?? profile['id'] ?? '').toString(),
                                            style: const TextStyle(
                                              color: brandNavy,
                                              fontWeight: FontWeight.w800,
                                            ),
                                          ),
                                        ),
                                        if (active) const _StatusPill(label: 'ACTIVE'),
                                      ],
                                    ),
                                    const SizedBox(height: 5),
                                    LText(
                                      (profile['description'] ?? '').toString(),
                                      maxLines: 2,
                                      overflow: TextOverflow.ellipsis,
                                      style: const TextStyle(color: brandTextSoft, fontSize: 9.5),
                                    ),
                                    const SizedBox(height: 10),
                                    _DefinitionRow(
                                      label: 'Layout',
                                      value: _humanize((theme['layout_key'] ?? 'classic_editorial').toString()),
                                    ),
                                    _DefinitionRow(
                                      label: 'Owner',
                                      value: profile['owner_type'] == 'CATALOG'
                                          ? 'HIMATE catalog'
                                          : 'Your organization',
                                    ),
                                    const SizedBox(height: 10),
                                    Wrap(
                                      spacing: 8,
                                      runSpacing: 8,
                                      children: [
                                        if (!active && can('design.write'))
                                          FilledButton.icon(
                                            onPressed: () => activateDesignProfile(profile),
                                            icon: const Icon(Icons.check_circle_outline_rounded),
                                            label: const LText('Activate'),
                                          ),
                                        if (profile['owner_type'] == 'PARTNER' && can('design.write'))
                                          OutlinedButton.icon(
                                            onPressed: () => editDesignProfile(profile),
                                            icon: const Icon(Icons.edit_outlined),
                                            label: const LText('Edit'),
                                          ),
                                      ],
                                    ),
                                  ],
                                ),
                              ),
                            );
                          },
                        ),
                      ),
                  ],
                );
              },
            ),
          const SizedBox(height: 24),
          _SectionHeader(
            title: 'Brand asset library',
            subtitle: 'Partner-owned images available to your theme profiles.',
            trailing: _MiniCounter(label: designMedia.length.toString() + ' assets'),
          ),
          const SizedBox(height: 12),
          if (designMedia.isEmpty)
            const _MessageCard(
              icon: Icons.image_outlined,
              title: 'No brand assets uploaded',
              message: 'Upload a logo, favicon or other image to assign it to a design slot.',
            )
          else
            LayoutBuilder(
              builder: (context, constraints) {
                final width = constraints.maxWidth < 620
                    ? constraints.maxWidth
                    : (constraints.maxWidth - 12) / 2;
                return Wrap(
                  spacing: 12,
                  runSpacing: 12,
                  children: [
                    for (final asset in designMedia)
                      SizedBox(
                        width: width,
                        child: _InfoCard(
                          title: (asset['original_filename'] ?? asset['id'] ?? '').toString(),
                          icon: Icons.image_outlined,
                          children: [
                            _DefinitionRow(
                              label: 'Asset ID',
                              value: (asset['id'] ?? '').toString(),
                            ),
                            _DefinitionRow(
                              label: 'Type',
                              value: (asset['mime_type'] ?? '').toString(),
                            ),
                            _DefinitionRow(
                              label: 'SHA-256',
                              value: _partnerShortHash((asset['sha256'] ?? '').toString()),
                            ),
                          ],
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
}

String _partnerShortHash(String value) {
  if (value.isEmpty) return '—';
  return value.length > 18 ? value.substring(0, 18) + '…' : value;
}
