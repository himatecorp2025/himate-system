part of 'main.dart';

class PlatformSecretsPanel extends StatefulWidget {
  const PlatformSecretsPanel({required this.api, required this.currentUser, super.key});
  final Api api;
  final Map<String, dynamic> currentUser;

  @override
  State<PlatformSecretsPanel> createState() => _PlatformSecretsPanelState();
}

class _PlatformSecretsPanelState extends State<PlatformSecretsPanel> {
  List<Map<String, dynamic>> secrets = <Map<String, dynamic>>[];
  bool loading = true;
  String? error;

  bool get canManage => widget.currentUser['system_owner'] == true;

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final response = await widget.api.get('/api/v1/admin/secrets', force: true);
      if (!mounted) return;
      setState(() {
        secrets = items(response);
        loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        error = e.toString();
        loading = false;
      });
    }
  }

  Future<void> configure(Map<String, dynamic> item) async {
    if (!canManage) return;
    final controller = TextEditingController();
    bool obscure = true;
    final saved = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: item['configured'] == true ? 'Replace secret' : 'Configure secret',
          subtitle: '${item['provider']} · ${item['environment_key']}',
          icon: Icons.key_rounded,
          width: 680,
          primaryLabel: item['configured'] == true ? 'Replace securely' : 'Store securely',
          onPrimary: () {
            if (controller.text.trim().isNotEmpty) Navigator.pop(dialogContext, true);
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              LText('${item['label']}', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 6),
              LText('${item['description']}', style: const TextStyle(color: brandTextSoft)),
              const SizedBox(height: 16),
              TextField(
                controller: controller,
                obscureText: obscure,
                autocorrect: false,
                enableSuggestions: false,
                decoration: InputDecoration(
                  labelText: uiLiteral('Secret value'),
                  hintText: uiLiteral('Paste the provider credential'),
                  prefixIcon: const Icon(Icons.password_rounded),
                  suffixIcon: IconButton(
                    tooltip: obscure ? 'Show while editing' : 'Hide',
                    onPressed: () => setLocal(() => obscure = !obscure),
                    icon: Icon(obscure ? Icons.visibility_outlined : Icons.visibility_off_outlined),
                  ),
                ),
              ),
              const SizedBox(height: 14),
              const _MessageCard(
                icon: Icons.shield_outlined,
                title: 'Write-only secret',
                message: 'HIMATE encrypts this value before storage. After saving, the raw value is never returned by the API or shown in the administration interface.',
              ),
            ],
          ),
        ),
      ),
    );
    if (saved == true) {
      try {
        await widget.api.put('/api/v1/admin/secrets/${item['key']}', {'secret': controller.text});
        await load();
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: LText('Secret stored securely.'), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess),
          );
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: LText('Secret could not be stored: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
          );
        }
      }
    }
    controller.dispose();
  }

  Future<void> remove(Map<String, dynamic> item) async {
    if (!canManage || item['configured'] != true) return;
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const LText('Remove stored secret?'),
        content: LText('Remove ${item['environment_key']} from the HIMATE encrypted vault? Dependent provider operations will become unavailable until a new value is configured.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const LText('Cancel')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const LText('Remove secret')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await widget.api.delete('/api/v1/admin/secrets/${item['key']}');
      await load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: LText('Secret removed.'), behavior: SnackBarBehavior.floating),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: LText('Secret could not be removed: $e'), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final configured = secrets.where((e) => e['configured'] == true).length;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Secrets & API Keys',
          subtitle: 'Write-only encrypted provider credentials. Missing credentials disable only the dependent provider operation; they do not prevent HIMATE from starting.',
          trailing: _MiniCounter(label: '$configured / ${secrets.length} configured'),
        ),
        const SizedBox(height: 12),
        if (!canManage)
          const _MessageCard(
            icon: Icons.admin_panel_settings_outlined,
            title: 'System owner control',
            message: 'Secret values can only be created, replaced or removed by the HIMATE system owner.',
          ),
        if (!canManage) const SizedBox(height: 12),
        if (loading && secrets.isEmpty)
          const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist)
        else if (error != null)
          _MessageCard(icon: Icons.error_outline_rounded, title: 'Secret status unavailable', message: error!)
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
                  for (final item in secrets)
                    SizedBox(
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
                                    decoration: BoxDecoration(
                                      color: (item['configured'] == true ? brandSuccess : brandWarning).withOpacity(.10),
                                      borderRadius: BorderRadius.circular(10),
                                    ),
                                    child: Icon(
                                      item['configured'] == true ? Icons.lock_rounded : Icons.lock_open_rounded,
                                      color: item['configured'] == true ? brandSuccess : brandWarning,
                                      size: 20,
                                    ),
                                  ),
                                  const SizedBox(width: 11),
                                  Expanded(child: LText('${item['label']}', style: Theme.of(context).textTheme.titleMedium)),
                                ],
                              ),
                              const SizedBox(height: 12),
                              _DefinitionRow(label: 'Provider', value: '${item['provider']}'),
                              _DefinitionRow(label: 'Consumer', value: '${item['consumer']}'),
                              _DefinitionRow(label: 'Key', value: '${item['environment_key']}'),
                              _DefinitionRow(label: 'Status', value: item['configured'] == true ? 'CONFIGURED' : 'CONFIGURATION REQUIRED'),
                              const SizedBox(height: 10),
                              LText('${item['description']}', style: const TextStyle(color: brandTextSoft, fontSize: 11, height: 1.45)),
                              const SizedBox(height: 14),
                              Wrap(
                                spacing: 8,
                                runSpacing: 8,
                                children: [
                                  FilledButton.icon(
                                    onPressed: canManage ? () => configure(item) : null,
                                    icon: const Icon(Icons.key_rounded, size: 18),
                                    label: LText(item['configured'] == true ? 'Replace' : 'Configure'),
                                  ),
                                  if (item['configured'] == true)
                                    OutlinedButton.icon(
                                      onPressed: canManage ? () => remove(item) : null,
                                      icon: const Icon(Icons.delete_outline_rounded, size: 18),
                                      label: const LText('Remove'),
                                    ),
                                ],
                              ),
                            ],
                          ),
                        ),
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
