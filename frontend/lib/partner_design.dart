part of 'main.dart';

extension PartnerDesignUI on _PartnerPortalShellState {
  Color partnerThemeColor(dynamic value, Color fallback) {
    final text = value?.toString().trim().replaceFirst('#', '') ?? '';
    final parsed = int.tryParse(text, radix: 16);
    return parsed == null || text.length != 6 ? fallback : Color(0xFF000000 | parsed);
  }


  Map<String, dynamic> get workspaceSettings {
    final raw = designState['workspace'];
    return raw is Map ? Map<String, dynamic>.from(raw) : <String, dynamic>{};
  }

  List<Map<String, dynamic>> get modulePresentations {
    final raw = designState['module_presentations'];
    return raw is List
        ? raw.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList()
        : <Map<String, dynamic>>[];
  }

  String get workspaceDisplayName {
    final custom = (workspaceSettings['workspace_name'] ?? '').toString().trim();
    if (custom.isNotEmpty) return custom;
    final companyName = (company['display_name'] ?? '').toString().trim();
    return companyName.isEmpty ? 'Partner Workspace' : companyName;
  }

  String get workspaceLogoUrl => (workspaceSettings['logo_url'] ?? '').toString().trim();
  String get workspaceDefaultModuleKey => (workspaceSettings['default_module_key'] ?? '').toString().trim();

  Color get workspacePrimaryColor =>
      partnerThemeColor(workspaceSettings['primary_color'], brandNavy);
  Color get workspaceSidebarColor =>
      partnerThemeColor(workspaceSettings['sidebar_color'], brandNavyDeep);
  Color get workspaceBackgroundColor =>
      partnerThemeColor(workspaceSettings['background_color'], brandIvory);
  Color get workspaceAccentColor =>
      partnerThemeColor(workspaceSettings['accent_color'], brandGold);
  Color get workspaceTextColor =>
      partnerThemeColor(workspaceSettings['text_color'], brandCharcoal);

  Color workspaceReadableForeground(Color background) =>
      background.computeLuminance() > .42 ? brandNavyDeep : brandWhite;

  Map<String, dynamic>? modulePresentationFor(String moduleKey) {
    for (final item in modulePresentations) {
      if ((item['module_key'] ?? '').toString() == moduleKey) return item;
    }
    return null;
  }

  String moduleDisplayName(Map<String, dynamic> module) {
    final presentation = modulePresentationFor((module['key'] ?? '').toString());
    final custom = (presentation?['display_name'] ?? '').toString().trim();
    return custom.isNotEmpty ? custom : (module['label'] ?? module['key'] ?? '').toString();
  }

  String moduleDisplayDescription(Map<String, dynamic> module) {
    final presentation = modulePresentationFor((module['key'] ?? '').toString());
    final custom = (presentation?['description'] ?? '').toString().trim();
    if (custom.isNotEmpty) return custom;
    return (module['marketplace_summary'] ?? module['description'] ?? '').toString().trim();
  }

  Color? modulePresentationCardColor(Map<String, dynamic> module) {
    final presentation = modulePresentationFor((module['key'] ?? '').toString());
    final raw = (presentation?['card_color'] ?? '').toString().trim();
    return raw.isEmpty ? null : partnerThemeColor(raw, brandWhite);
  }

  IconData workspaceIconData(String key) {
    switch (key) {
      case 'finance':
        return Icons.account_balance_wallet_outlined;
      case 'workflow':
        return Icons.account_tree_outlined;
      case 'inventory':
        return Icons.inventory_2_outlined;
      case 'calendar':
        return Icons.calendar_month_outlined;
      case 'crm':
        return Icons.people_alt_outlined;
      case 'marketing':
        return Icons.campaign_outlined;
      case 'events':
        return Icons.event_outlined;
      case 'analytics':
        return Icons.insights_outlined;
      case 'documents':
        return Icons.folder_copy_outlined;
      case 'settings':
        return Icons.settings_outlined;
      case 'integrations':
        return Icons.hub_outlined;
      case 'users':
        return Icons.group_outlined;
      case 'support':
        return Icons.support_agent_outlined;
      default:
        return Icons.extension_outlined;
    }
  }

  Widget workspaceLogo({double height = 42}) {
    if (workspaceLogoUrl.isEmpty) {
      return HimateLogo(
        onDark: workspaceSidebarColor.computeLuminance() < .45,
        width: 174,
      );
    }
    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: 190, maxHeight: height),
      child: Image.network(
        Uri.base.resolve(workspaceLogoUrl).toString(),
        height: height,
        fit: BoxFit.contain,
        alignment: Alignment.centerLeft,
        errorBuilder: (_, __, ___) => HimateLogo(
          onDark: workspaceSidebarColor.computeLuminance() < .45,
          width: 174,
        ),
      ),
    );
  }

  Widget modulePresentationIcon(
    Map<String, dynamic> module, {
    double size = 23,
  }) {
    final presentation = modulePresentationFor((module['key'] ?? '').toString());
    final customUrl = (presentation?['custom_icon_url'] ?? '').toString().trim();
    if (customUrl.isNotEmpty) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(7),
        child: Image.network(
          Uri.base.resolve(customUrl).toString(),
          width: size,
          height: size,
          fit: BoxFit.cover,
          errorBuilder: (_, __, ___) =>
              Icon(Icons.extension_outlined, size: size),
        ),
      );
    }
    return Icon(
      workspaceIconData((presentation?['icon_key'] ?? '').toString()),
      size: size,
      color: workspacePrimaryColor,
    );
  }

  bool _workspaceHexValid(String value) =>
      RegExp(r'^#[0-9A-Fa-f]{6}$').hasMatch(value.trim());

  double _workspaceContrastRatio(String first, String second) {
    Color parse(String value) => partnerThemeColor(value, brandWhite);
    final a = parse(first).computeLuminance();
    final b = parse(second).computeLuminance();
    final high = a > b ? a : b;
    final low = a > b ? b : a;
    return (high + .05) / (low + .05);
  }

  Future<void> editWorkspacePersonalization() async {
    if (!can('design.write')) return;
    final workspace = workspaceSettings;
    final name = TextEditingController(
      text: (workspace['workspace_name'] ?? company['display_name'] ?? '')
          .toString(),
    );
    final primary = TextEditingController(
      text: (workspace['primary_color'] ?? '#0B1F3B').toString(),
    );
    final sidebar = TextEditingController(
      text: (workspace['sidebar_color'] ?? '#06172C').toString(),
    );
    final background = TextEditingController(
      text: (workspace['background_color'] ?? '#F8F9FB').toString(),
    );
    final accent = TextEditingController(
      text: (workspace['accent_color'] ?? '#D4AF6B').toString(),
    );
    final textColor = TextEditingController(
      text: (workspace['text_color'] ?? '#1F2937').toString(),
    );
    var logoMediaID = (workspace['logo_media_id'] ?? '').toString();
    var defaultModuleKey = (workspace['default_module_key'] ?? '').toString();
    String? validationError;

    final activeModules = modules
        .where(
          (module) =>
              (module['access_state'] ?? '').toString() == 'ACTIVE' &&
              module['executable'] == true,
        )
        .toList();

    final ok = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) {
          final knownLogo = logoMediaID.isEmpty ||
              designMedia.any(
                (item) => (item['id'] ?? '').toString() == logoMediaID,
              );
          final knownDefault = defaultModuleKey.isEmpty ||
              activeModules.any(
                (item) => (item['key'] ?? '').toString() == defaultModuleKey,
              );
          return BrandDialog(
            title: 'Workspace identity',
            subtitle:
                'Customize presentation only. HIMATE module keys, APIs, routes, billing, permissions, entitlements and workflows remain unchanged.',
            icon: Icons.dashboard_customize_outlined,
            width: 900,
            primaryLabel: 'Save workspace',
            onPrimary: () {
              final colors = [
                primary.text,
                sidebar.text,
                background.text,
                accent.text,
                textColor.text,
              ];
              String? problem;
              if (name.text.trim().length > 100) {
                problem = 'Workspace name must be at most 100 characters.';
              } else if (!colors.every(_workspaceHexValid)) {
                problem = 'Every brand color must use the #RRGGBB format.';
              } else if (_workspaceContrastRatio(
                    background.text,
                    textColor.text,
                  ) <
                  4.5) {
                problem =
                    'Background and text colors need at least 4.5:1 contrast.';
              }
              if (problem != null) {
                setLocal(() => validationError = problem);
                return;
              }
              Navigator.pop(dialogContext, true);
            },
            child: Column(
              children: [
                if (validationError != null) ...[
                  _MessageCard(
                    icon: Icons.contrast_outlined,
                    title: 'Workspace colors need attention',
                    message: validationError!,
                  ),
                  const SizedBox(height: 12),
                ],
                TextField(
                  controller: name,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Workspace company name'),
                    helperText: uiLiteral(
                      'Changes the tenant workspace label, not the HIMATE system name.',
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: knownLogo ? logoMediaID : '',
                  decoration: InputDecoration(
                    labelText: uiLiteral('Workspace logo'),
                  ),
                  items: <DropdownMenuItem<String>>[
                    const DropdownMenuItem(
                      value: '',
                      child: LText('HIMATE / company default'),
                    ),
                    for (final item in designMedia)
                      DropdownMenuItem(
                        value: (item['id'] ?? '').toString(),
                        child: LText(
                          (item['original_filename'] ?? item['id'] ?? '')
                              .toString(),
                        ),
                      ),
                  ],
                  onChanged: (value) =>
                      setLocal(() => logoMediaID = value ?? ''),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: TextField(
                    controller: primary,
                    decoration: InputDecoration(
                      labelText: uiLiteral('Primary color'),
                    ),
                  ),
                  second: TextField(
                    controller: accent,
                    decoration: InputDecoration(
                      labelText: uiLiteral('Accent color'),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: TextField(
                    controller: sidebar,
                    decoration: InputDecoration(
                      labelText: uiLiteral('Sidebar color'),
                    ),
                  ),
                  second: TextField(
                    controller: background,
                    decoration: InputDecoration(
                      labelText: uiLiteral('Background color'),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: textColor,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Text color'),
                    helperText: uiLiteral(
                      'HIMATE enforces readable text/background contrast.',
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: knownDefault ? defaultModuleKey : '',
                  decoration: InputDecoration(
                    labelText: uiLiteral('Default module'),
                    helperText: uiLiteral(
                      'Only an ACTIVE module owned by your organization can be selected.',
                    ),
                  ),
                  items: <DropdownMenuItem<String>>[
                    const DropdownMenuItem(
                      value: '',
                      child: LText('Overview'),
                    ),
                    for (final module in activeModules)
                      DropdownMenuItem(
                        value: (module['key'] ?? '').toString(),
                        child: LText(moduleDisplayName(module)),
                      ),
                  ],
                  onChanged: (value) =>
                      setLocal(() => defaultModuleKey = value ?? ''),
                ),
              ],
            ),
          );
        },
      ),
    );

    if (ok == true) {
      try {
        await widget.api.put('/partner/api/v1/design/workspace', {
          'workspace_name': name.text.trim(),
          'logo_media_id': logoMediaID,
          'primary_color': primary.text.trim().toUpperCase(),
          'sidebar_color': sidebar.text.trim().toUpperCase(),
          'background_color': background.text.trim().toUpperCase(),
          'accent_color': accent.text.trim().toUpperCase(),
          'text_color': textColor.text.trim().toUpperCase(),
          'default_module_key': defaultModuleKey,
        });
        await load();
        if (mounted) toast('Workspace personalization saved.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }

    for (final controller in [
      name,
      primary,
      sidebar,
      background,
      accent,
      textColor,
    ]) {
      controller.dispose();
    }
  }

  Future<void> editModulePresentation(Map<String, dynamic> module) async {
    if (!can('design.write')) return;
    final moduleKey = (module['key'] ?? '').toString();
    final existing = modulePresentationFor(moduleKey);
    final displayName = TextEditingController(
      text: (existing?['display_name'] ?? '').toString(),
    );
    final description = TextEditingController(
      text: (existing?['description'] ?? '').toString(),
    );
    final cardColor = TextEditingController(
      text: (existing?['card_color'] ?? '').toString(),
    );
    var iconKey = (existing?['icon_key'] ?? '').toString();
    var customIconMediaID =
        (existing?['custom_icon_media_id'] ?? '').toString();
    String? validationError;

    final rawLibrary = designState['icon_library'];
    final iconLibrary = rawLibrary is List
        ? rawLibrary
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList()
        : <Map<String, dynamic>>[];

    final saved = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) {
          final knownIcon = iconKey.isEmpty ||
              iconLibrary.any(
                (item) => (item['key'] ?? '').toString() == iconKey,
              );
          final knownMedia = customIconMediaID.isEmpty ||
              designMedia.any(
                (item) =>
                    (item['id'] ?? '').toString() == customIconMediaID,
              );
          return BrandDialog(
            title: 'Module presentation',
            subtitle:
                'Rename and restyle the card for your team. The canonical HIMATE module identity and behavior cannot be changed.',
            icon: Icons.edit_outlined,
            width: 820,
            primaryLabel: 'Save presentation',
            onPrimary: () {
              String? problem;
              if (displayName.text.trim().length > 100) {
                problem = 'Display name must be at most 100 characters.';
              } else if (description.text.trim().length > 800) {
                problem = 'Description must be at most 800 characters.';
              } else if (cardColor.text.trim().isNotEmpty &&
                  !_workspaceHexValid(cardColor.text)) {
                problem = 'Card color must use the #RRGGBB format.';
              } else if (iconKey.isNotEmpty &&
                  customIconMediaID.isNotEmpty) {
                problem =
                    'Choose an HIMATE icon or a custom icon, not both.';
              }
              if (problem != null) {
                setLocal(() => validationError = problem);
                return;
              }
              Navigator.pop(dialogContext, true);
            },
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (validationError != null) ...[
                  _MessageCard(
                    icon: Icons.error_outline_rounded,
                    title: 'Presentation needs attention',
                    message: validationError!,
                  ),
                  const SizedBox(height: 12),
                ],
                _DefinitionRow(
                  label: 'Official HIMATE name',
                  value: (module['label'] ?? moduleKey).toString(),
                  emphasis: true,
                ),
                _DefinitionRow(
                  label: 'Canonical module key',
                  value: moduleKey,
                ),
                const SizedBox(height: 14),
                TextField(
                  controller: displayName,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Your display name'),
                    helperText: uiLiteral(
                      'Leave blank to use the official HIMATE name.',
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: description,
                  minLines: 3,
                  maxLines: 5,
                  decoration: InputDecoration(
                    labelText: uiLiteral('What this module does'),
                    helperText: uiLiteral(
                      'Use a short 3–4 sentence description for your team.',
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                ResponsiveFieldPair(
                  first: DropdownButtonFormField<String>(
                    value: knownIcon ? iconKey : '',
                    decoration: InputDecoration(
                      labelText: uiLiteral('HIMATE icon library'),
                    ),
                    items: <DropdownMenuItem<String>>[
                      const DropdownMenuItem(
                        value: '',
                        child: LText('HIMATE default'),
                      ),
                      for (final item in iconLibrary)
                        DropdownMenuItem(
                          value: (item['key'] ?? '').toString(),
                          child: LText(
                            (item['label'] ?? item['key'] ?? '').toString(),
                          ),
                        ),
                    ],
                    onChanged: (value) => setLocal(() {
                      iconKey = value ?? '';
                      if (iconKey.isNotEmpty) customIconMediaID = '';
                    }),
                  ),
                  second: DropdownButtonFormField<String>(
                    value: knownMedia ? customIconMediaID : '',
                    decoration: InputDecoration(
                      labelText: uiLiteral('Custom icon asset'),
                    ),
                    items: <DropdownMenuItem<String>>[
                      const DropdownMenuItem(
                        value: '',
                        child: LText('No custom icon'),
                      ),
                      for (final item in designMedia)
                        DropdownMenuItem(
                          value: (item['id'] ?? '').toString(),
                          child: LText(
                            (item['original_filename'] ?? item['id'] ?? '')
                                .toString(),
                          ),
                        ),
                    ],
                    onChanged: (value) => setLocal(() {
                      customIconMediaID = value ?? '';
                      if (customIconMediaID.isNotEmpty) iconKey = '';
                    }),
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: cardColor,
                  decoration: InputDecoration(
                    labelText: uiLiteral('Card color'),
                    helperText: uiLiteral(
                      'Optional #RRGGBB color; leave blank for HIMATE default.',
                    ),
                  ),
                ),
                if (existing != null) ...[
                  const SizedBox(height: 18),
                  OutlinedButton.icon(
                    onPressed: () async {
                      final confirm = await showDialog<bool>(
                        context: dialogContext,
                        builder: (confirmContext) => AlertDialog(
                          title: const LText('Reset to HIMATE default?'),
                          content: const LText(
                            'Your custom module name, description, icon and card color will be removed. The module itself is unchanged.',
                          ),
                          actions: [
                            TextButton(
                              onPressed: () =>
                                  Navigator.pop(confirmContext, false),
                              child: const LText('Cancel'),
                            ),
                            FilledButton(
                              onPressed: () =>
                                  Navigator.pop(confirmContext, true),
                              child: const LText('Reset'),
                            ),
                          ],
                        ),
                      );
                      if (confirm != true) return;
                      try {
                        await widget.api.delete(
                          '/partner/api/v1/design/modules/$moduleKey',
                        );
                        if (dialogContext.mounted) {
                          Navigator.pop(dialogContext, false);
                        }
                        await load();
                        if (mounted) {
                          toast(
                            'Module presentation reset to HIMATE default.',
                          );
                        }
                      } catch (e) {
                        if (mounted) toast(e.toString(), failure: true);
                      }
                    },
                    icon: const Icon(Icons.restart_alt_rounded),
                    label: const LText('Reset to HIMATE default'),
                  ),
                ],
              ],
            ),
          );
        },
      ),
    );

    if (saved == true) {
      try {
        await widget.api.put('/partner/api/v1/design/modules/$moduleKey', {
          'display_name': displayName.text.trim(),
          'description': description.text.trim(),
          'icon_key': iconKey,
          'custom_icon_media_id': customIconMediaID,
          'card_color': cardColor.text.trim().toUpperCase(),
        });
        await load();
        if (mounted) toast('Module presentation updated.');
      } catch (e) {
        if (mounted) toast(e.toString(), failure: true);
      }
    }

    displayName.dispose();
    description.dispose();
    cardColor.dispose();
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
            _RuleItem(Icons.layers_outlined, 'Presentation only', 'Content stays unchanged'),
            _RuleItem(Icons.settings_suggest_outlined, 'Canonical modules', 'System logic stays unchanged'),
            _RuleItem(Icons.lock_person_outlined, 'Tenant isolated', 'Your assets only'),
            _RuleItem(Icons.contrast_outlined, 'Readable branding', 'Contrast protected'),
          ]),
          const SizedBox(height: 22),
          _SectionHeader(
            title: 'Workspace identity',
            subtitle: 'Your company label, workspace logo, brand colors and default module.',
            trailing: can('design.write')
                ? OutlinedButton.icon(
                    onPressed: editWorkspacePersonalization,
                    icon: const Icon(Icons.edit_outlined),
                    label: const LText('Edit workspace'),
                  )
                : null,
          ),
          const SizedBox(height: 12),
          _InfoCard(
            title: workspaceDisplayName,
            icon: Icons.dashboard_customize_outlined,
            children: [
              _DefinitionRow(
                label: 'Default module',
                value: workspaceDefaultModuleKey.isEmpty
                    ? 'Overview'
                    : workspaceDefaultModuleKey,
              ),
              _DefinitionRow(
                label: 'Primary color',
                value: (workspaceSettings['primary_color'] ?? '#0B1F3B').toString(),
              ),
              _DefinitionRow(
                label: 'Sidebar color',
                value: (workspaceSettings['sidebar_color'] ?? '#06172C').toString(),
              ),
              _DefinitionRow(
                label: 'Background color',
                value: (workspaceSettings['background_color'] ?? '#F8F9FB').toString(),
              ),
              _DefinitionRow(
                label: 'Accent color',
                value: (workspaceSettings['accent_color'] ?? '#D4AF6B').toString(),
              ),
              _DefinitionRow(
                label: 'Customized modules',
                value: modulePresentations.length.toString(),
                emphasis: modulePresentations.isNotEmpty,
              ),
              const _DefinitionRow(
                label: 'System contract',
                value: 'Presentation only · canonical HIMATE mechanics unchanged',
              ),
            ],
          ),
          const SizedBox(height: 24),
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
