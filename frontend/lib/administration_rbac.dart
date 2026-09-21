part of 'main.dart';

class AccessControlPanel extends StatefulWidget {
  const AccessControlPanel({required this.api, required this.currentUser, super.key});
  final Api api;
  final Map<String, dynamic> currentUser;

  @override
  State<AccessControlPanel> createState() => _AccessControlPanelState();
}

class _AccessControlPanelState extends State<AccessControlPanel> {
  List<Map<String, dynamic>> roles = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> users = <Map<String, dynamic>>[];
  bool loading = false;
  String? error;

  bool get canManage => widget.currentUser['system_owner'] == true;

  static const permissionCatalog = <String>[
    'dashboard.read',
    'partners.read','partners.write','partners.approve',
    'catalog.read','catalog.write','catalog.approve',
    'billing.read','billing.write','billing.approve',
    'impact.read','impact.write','impact.approve',
    'evidence.read','evidence.write','evidence.approve',
    'reports.read','reports.write','reports.approve',
    'cms.read','cms.write','cms.approve',
    'contact.read','contact.write',
    'provisioning.read','provisioning.write','provisioning.approve',
    'environments.read','environments.write','environments.approve',
    'connectors.read','connectors.write','connectors.approve',
    'backups.read','backups.write','backups.approve',
    'health.read','notifications.read','audit.read','administration.read',
  ];

  @override
  void initState() {
    super.initState();
    if (canManage) {
      load();
    } else {
      loading = false;
    }
  }

  Future<void> load() async {
    if (mounted) setState(() => error = null);
    final failures = <String>[];

    Future<void> fetch(String path, void Function(Map<String, dynamic>) apply) async {
      try {
        final data = await widget.api.get(path);
        if (mounted) setState(() => apply(data));
      } catch (e) {
        failures.add(e.toString());
      }
    }

    await Future.wait<void>([
      fetch('/api/v1/admin/roles', (data) => roles = items(data)),
      fetch('/api/v1/admin/users', (data) => users = items(data)),
    ]);

    if (mounted && failures.length == 2) {
      setState(() => error = failures.first);
    }
  }

  void notify(String message, {bool failure = false}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: LText(message),
        behavior: SnackBarBehavior.floating,
        backgroundColor: failure ? brandDanger : brandSuccess,
      ),
    );
  }

  List<String> _roleKeys(dynamic raw) {
    if (raw is! List) return <String>[];
    return raw.map((e) => e.toString()).where((e) => e.isNotEmpty).toList();
  }

  String _roleLabel(String key) {
    for (final role in roles) {
      if ('${role['key']}' == key) return '${role['label']}';
    }
    return _humanize(key);
  }

  Widget _permissionChip(String value) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
      decoration: BoxDecoration(
        color: value == '*' ? brandGold.withOpacity(.12) : brandNavy.withOpacity(.045),
        borderRadius: BorderRadius.circular(99),
        border: Border.all(color: value == '*' ? brandGold.withOpacity(.28) : brandMist),
      ),
      child: LText(
        value == '*' ? 'ALL PERMISSIONS' : value,
        style: TextStyle(
          color: value == '*' ? brandWarning : brandNavy,
          fontSize: 8.7,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }

  Widget _roleCard(Map<String, dynamic> role) {
    final permissions = _roleKeys(role['permissions']);
    final system = role['system'] == true;
    final active = role['active'] != false;
    return _InfoCard(
      title: '${role['label'] ?? _humanize('${role['key']}')}',
      icon: '${role['key']}' == 'platform_admin' ? Icons.shield_outlined : Icons.badge_outlined,
      children: [
        Row(
          children: [
            Expanded(
              child: LText(
                '${role['description'] ?? ''}',
                style: const TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
              ),
            ),
            const SizedBox(width: 8),
            _StatusPill(label: system ? 'SYSTEM' : (active ? 'CUSTOM' : 'INACTIVE')),
          ],
        ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 6,
          runSpacing: 6,
          children: [for (final permission in permissions) _permissionChip(permission)],
        ),
        if (!system) ...[
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: () => editRole(role),
              icon: const Icon(Icons.tune_rounded),
              label: const LText('Edit role'),
            ),
          ),
        ],
      ],
    );
  }

  Widget _userCard(Map<String, dynamic> user) {
    final userRoles = _roleKeys(user['roles']);
    final active = user['active'] == true;
    final current = '${user['id']}' == '${widget.currentUser['id']}';
    final permissions = _roleKeys(user['permissions']);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(17),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _Avatar(name: '${user['name'] ?? 'Admin User'}'),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Flexible(
                            child: LText(
                              '${user['name'] ?? 'Administrator'}',
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(color: brandNavy, fontSize: 14, fontWeight: FontWeight.w700),
                            ),
                          ),
                          if (current) ...[
                            const SizedBox(width: 6),
                            const _MiniCounter(label: 'YOU'),
                          ],
                          if (user['system_owner'] == true) ...[
                            const SizedBox(width: 6),
                            const _MiniCounter(label: 'OWNER'),
                          ],
                        ],
                      ),
                      const SizedBox(height: 3),
                      LText(
                        '${user['email'] ?? ''}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(color: brandTextSoft, fontSize: 10),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                _StatusPill(label: active ? 'ACTIVE' : 'SUSPENDED'),
              ],
            ),
            const SizedBox(height: 14),
            Wrap(
              spacing: 6,
              runSpacing: 6,
              children: [
                for (final role in userRoles)
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
                    decoration: BoxDecoration(
                      color: brandGold.withOpacity(.09),
                      borderRadius: BorderRadius.circular(99),
                      border: Border.all(color: brandGold.withOpacity(.20)),
                    ),
                    child: LText(
                      _roleLabel(role),
                      style: const TextStyle(color: brandNavy, fontSize: 8.8, fontWeight: FontWeight.w700),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 10),
            _DefinitionRow(label: 'Effective permissions', value: permissions.contains('*') ? 'All' : '${permissions.length}'),
            _DefinitionRow(label: 'User ID', value: '${user['id'] ?? '—'}'),
            const SizedBox(height: 8),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: () => editUser(user),
                icon: const Icon(Icons.manage_accounts_outlined),
                label: const LText('Edit access'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<Map<String, dynamic>?> _userDialog({Map<String, dynamic>? user}) async {
    final editing = user != null;
    final name = TextEditingController(text: editing ? '${user['name'] ?? ''}' : '');
    final email = TextEditingController(text: editing ? '${user['email'] ?? ''}' : '');
    final password = TextEditingController();
    final selected = <String>{..._roleKeys(user?['roles'])};
    final editingUser = user;
    final editingSystemOwner = editingUser != null && editingUser['system_owner'] == true;
    if (!editing && selected.isEmpty) selected.add('operations_admin');
    var active = editing ? user['active'] == true : true;
    String? dialogError;

    final result = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: editing ? 'Edit administrator access' : 'Create administrator',
          subtitle: editing
              ? 'Changes are enforced by the backend on the next API request.'
              : 'Create an administrator account and assign one or more backend roles.',
          icon: Icons.manage_accounts_outlined,
          width: 700,
          primaryLabel: editing ? 'Save access' : 'Create administrator',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ResponsiveFieldPair(
                first: TextField(
                  controller: name,
                  decoration: InputDecoration(labelText: uiLiteral('Full name')),
                ),
                second: TextField(
                  controller: email,
                  keyboardType: TextInputType.emailAddress,
                  decoration: InputDecoration(labelText: uiLiteral('Email address')),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: password,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: editing ? 'New password (optional)' : 'Temporary password',
                  helperText: editing
                      ? 'Leave blank to keep the current password. Use 12+ characters with lowercase, uppercase, number and special character.'
                      : 'Use 12+ characters with lowercase, uppercase, number and special character.',
                ),
              ),
              const SizedBox(height: 18),
              LText('Roles', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 5),
              const LText(
                'Permissions are additive when multiple roles are assigned.',
                style: TextStyle(color: brandTextSoft, fontSize: 10.5),
              ),
              const SizedBox(height: 8),
              for (final role in roles.where((role) =>
                  role['active'] != false &&
                  ('${role['key']}' != 'platform_admin' || editingSystemOwner)))
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  dense: true,
                  value: selected.contains('${role['key']}'),
                  title: LText('${role['label']}', style: const TextStyle(color: brandNavy, fontSize: 12, fontWeight: FontWeight.w700)),
                  subtitle: LText(
                    '${role['key']}' == 'platform_admin' && editingSystemOwner
                        ? 'Reserved system-owner role'
                        : '${role['description']}',
                    style: const TextStyle(color: brandTextSoft, fontSize: 9.5),
                  ),
                  onChanged: '${role['key']}' == 'platform_admin' && editingSystemOwner
                      ? null
                      : (value) {
                          setDialogState(() {
                            final key = '${role['key']}';
                            if (value == true) {
                              selected.add(key);
                            } else {
                              selected.remove(key);
                            }
                            dialogError = null;
                          });
                        },
                ),
              if (editing) ...[
                const SizedBox(height: 8),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const LText('Active account', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700)),
                  subtitle: const LText('Suspended administrators cannot sign in or use an existing session.', style: TextStyle(color: brandTextSoft, fontSize: 9.5)),
                  value: active,
                  onChanged: (value) => setDialogState(() { active = value; dialogError = null; }),
                ),
              ],
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontSize: 10.5, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
          onPrimary: () {
            final cleanName = name.text.trim();
            final cleanEmail = email.text.trim().toLowerCase();
            final cleanPassword = password.text;
            String? validation;
            if (cleanName.length < 2) {
              validation = 'Enter the administrator name.';
            } else if (!cleanEmail.contains('@') || !cleanEmail.contains('.')) {
              validation = 'Enter a valid email address.';
            } else if (!editing && !himatePasswordMeetsPolicy(cleanPassword)) {
              validation = HimateI18n.text(himateLocaleCode(Localizations.localeOf(context)), 'passwordPolicy');
            } else if (editing && cleanPassword.isNotEmpty && !himatePasswordMeetsPolicy(cleanPassword)) {
              validation = HimateI18n.text(himateLocaleCode(Localizations.localeOf(context)), 'passwordPolicy');
            } else if (selected.isEmpty) {
              validation = 'Assign at least one role.';
            }
            if (validation != null) {
              setDialogState(() => dialogError = validation);
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              'name': cleanName,
              'email': cleanEmail,
              if (cleanPassword.isNotEmpty) 'password': cleanPassword,
              'roles': selected.toList(),
              if (editing) 'active': active,
            });
          },
        ),
      ),
    );

    name.dispose();
    email.dispose();
    password.dispose();
    return result;
  }

  Future<Map<String, dynamic>?> _roleDialog({Map<String, dynamic>? role}) async {
    final editing = role != null;
    final key = TextEditingController(text: editing ? '${role['key'] ?? ''}' : '');
    final label = TextEditingController(text: editing ? '${role['label'] ?? ''}' : '');
    final description = TextEditingController(text: editing ? '${role['description'] ?? ''}' : '');
    final selected = <String>{..._roleKeys(role?['permissions'])};
    var active = editing ? role['active'] != false : true;
    String? dialogError;

    final result = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setLocal) => BrandDialog(
          title: editing ? 'Edit custom role' : 'Create custom role',
          subtitle: 'Custom roles are additive and remain subordinate to the protected System Owner boundary.',
          icon: Icons.rule_folder_outlined,
          width: 820,
          primaryLabel: editing ? 'Save role' : 'Create role',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ResponsiveFieldPair(
                first: TextField(controller: label, decoration: InputDecoration(labelText: uiLiteral('Role name *'))),
                second: TextField(controller: key, readOnly: editing, decoration: InputDecoration(labelText: uiLiteral('Stable role key *'), hintText: uiLiteral('finance_assistant'))),
              ),
              const SizedBox(height: 12),
              TextField(controller: description, maxLines: 2, decoration: InputDecoration(labelText: uiLiteral('Description'))),
              const SizedBox(height: 18),
              const _DialogSectionLabel('PERMISSION MATRIX'),
              const SizedBox(height: 8),
              const LText(
                'Read, write and approval permissions are enforced by the backend. administration.approve and System Owner authority cannot be delegated through a custom role.',
                style: TextStyle(color: brandTextSoft, fontSize: 10, height: 1.45),
              ),
              const SizedBox(height: 10),
              ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 360),
                child: SingleChildScrollView(
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      for (final permission in permissionCatalog)
                        FilterChip(
                          selected: selected.contains(permission),
                          label: LText(permission),
                          onSelected: (value) => setLocal(() {
                            if (value) { selected.add(permission); } else { selected.remove(permission); }
                            dialogError = null;
                          }),
                        ),
                    ],
                  ),
                ),
              ),
              if (editing) ...[
                const SizedBox(height: 12),
                SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  value: active,
                  title: const LText('Active role', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700)),
                  subtitle: const LText('Deactivate only after the role is removed from all administrators.', style: TextStyle(color: brandTextSoft, fontSize: 9.5)),
                  onChanged: (value) => setLocal(() => active = value),
                ),
              ],
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                LText(dialogError!, style: const TextStyle(color: brandDanger, fontSize: 10.5, fontWeight: FontWeight.w600)),
              ],
            ],
          ),
          onPrimary: () {
            final cleanKey = key.text.trim().toLowerCase();
            final cleanLabel = label.text.trim();
            String? validation;
            if (cleanLabel.length < 2) {
              validation = 'Enter a role name.';
            } else if (!editing && !RegExp(r'^[a-z][a-z0-9_]{2,63}    final id = '${updated['id']}';
    final next = <Map<String, dynamic>>[
      for (final user in users)
        if ('${user['id']}' == id) updated else user,
    ];
    next.sort((a, b) {
      final activeA = a['active'] == true ? 0 : 1;
      final activeB = b['active'] == true ? 0 : 1;
      if (activeA != activeB) return activeA.compareTo(activeB);
      return '${a['name']}'.toLowerCase().compareTo('${b['name']}'.toLowerCase());
    });
    setState(() => users = next);
  }

  Future<void> createUser() async {
    final payload = await _userDialog();
    if (payload == null) return;
    try {
      final created = await widget.api.post('/api/v1/admin/users', payload);
      if (!mounted) return;
      setState(() {
        users = <Map<String, dynamic>>[created, ...users];
      });
      notify('Administrator created.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> editUser(Map<String, dynamic> user) async {
    final payload = await _userDialog(user: user);
    if (payload == null) return;
    try {
      final updated = await widget.api.patch('/api/v1/admin/users/${user['id']}', payload);
      if (!mounted) return;
      _replaceUser(updated);
      notify('Administrator access updated.');
      if ('${user['id']}' == '${widget.currentUser['id']}') {
        notify('Your own permissions are now enforced immediately. Reload the page to refresh navigation if your roles changed.');
      }
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!canManage) {
      return const _MessageCard(
        icon: Icons.lock_outline_rounded,
        title: 'Platform Admin access required',
        message: 'Role assignment and administrator lifecycle are protected by backend Administration permissions.',
      );
    }
    if (loading && roles.isEmpty && users.isEmpty) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 28),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (error != null && roles.isEmpty && users.isEmpty) {
      return _MessageCard(
        icon: Icons.error_outline_rounded,
        title: 'Roles & Permissions could not be loaded',
        message: error!,
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Roles & Permissions',
          subtitle: 'Backend-enforced RBAC. Read, write and approval rights are checked before each protected administration API request.',
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _MiniCounter(label: '${users.length} admins'),
              const SizedBox(width: 8),
              OutlinedButton.icon(
                onPressed: createRole,
                icon: const Icon(Icons.rule_folder_outlined),
                label: const LText('Create role'),
              ),
              const SizedBox(width: 8),
              FilledButton.icon(
                onPressed: roles.isEmpty ? null : createUser,
                icon: const Icon(Icons.person_add_alt_1_rounded),
                label: const LText('Add administrator'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        const _RuleStrip(items: [
          _RuleItem(Icons.admin_panel_settings_outlined, 'Platform', 'Full control'),
          _RuleItem(Icons.settings_suggest_outlined, 'Operations', 'Technical operations'),
          _RuleItem(Icons.account_balance_wallet_outlined, 'Finance', 'Commercial control'),
          _RuleItem(Icons.rule_folder_outlined, 'Custom roles', 'Permission matrix'),
        ]),
        const SizedBox(height: 18),
        _SectionHeader(
          title: 'Role Matrix',
          subtitle: 'Roles are additive. Approval permissions protect sensitive actions such as publishing, verification and provisioning execution.',
          trailing: _MiniCounter(label: '${roles.length} roles'),
        ),
        const SizedBox(height: 10),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth < 760
                ? constraints.maxWidth
                : constraints.maxWidth < 1180
                    ? (constraints.maxWidth - 12) / 2
                    : (constraints.maxWidth - 36) / 4;
            return Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                for (final role in roles) SizedBox(width: width, child: _roleCard(role)),
              ],
            );
          },
        ),
        const SizedBox(height: 24),
        _SectionHeader(
          title: 'Administrators',
          subtitle: 'Accounts are never hard-deleted from administration. Suspend access to preserve audit identity and historical attribution.',
          trailing: _MiniCounter(label: '${users.where((u) => u['active'] == true).length} active'),
        ),
        const SizedBox(height: 10),
        if (users.isEmpty)
          const _MessageCard(
            icon: Icons.person_off_outlined,
            title: 'No administrators found',
            message: 'Create an administrator to assign a scoped HIMATE role.',
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
                  for (final user in users) SizedBox(width: width, child: _userCard(user)),
                ],
              );
            },
          ),
        if (loading) ...[
          const SizedBox(height: 12),
          const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
        ],
      ],
    );
  }
}
).hasMatch(cleanKey)) {
              validation = 'Use a stable key such as finance_assistant.';
            } else if (selected.isEmpty) {
              validation = 'Select at least one permission.';
            }
            if (validation != null) {
              setLocal(() => dialogError = validation);
              return;
            }
            Navigator.pop(dialogContext, <String, dynamic>{
              if (!editing) 'key': cleanKey,
              'label': cleanLabel,
              'description': description.text.trim(),
              'permissions': selected.toList()..sort(),
              if (editing) 'active': active,
            });
          },
        ),
      ),
    );

    key.dispose();
    label.dispose();
    description.dispose();
    return result;
  }

  Future<void> createRole() async {
    final payload = await _roleDialog();
    if (payload == null) return;
    try {
      await widget.api.post('/api/v1/admin/roles', payload);
      await load();
      if (mounted) notify('Custom role created.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> editRole(Map<String, dynamic> role) async {
    final payload = await _roleDialog(role: role);
    if (payload == null) return;
    try {
      await widget.api.patch('/api/v1/admin/roles/${role['key']}', payload);
      await load();
      if (mounted) notify('Custom role updated.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  void _replaceUser(Map<String, dynamic> updated) {
    final id = '${updated['id']}';
    final next = <Map<String, dynamic>>[
      for (final user in users)
        if ('${user['id']}' == id) updated else user,
    ];
    next.sort((a, b) {
      final activeA = a['active'] == true ? 0 : 1;
      final activeB = b['active'] == true ? 0 : 1;
      if (activeA != activeB) return activeA.compareTo(activeB);
      return '${a['name']}'.toLowerCase().compareTo('${b['name']}'.toLowerCase());
    });
    setState(() => users = next);
  }

  Future<void> createUser() async {
    final payload = await _userDialog();
    if (payload == null) return;
    try {
      final created = await widget.api.post('/api/v1/admin/users', payload);
      if (!mounted) return;
      setState(() {
        users = <Map<String, dynamic>>[created, ...users];
      });
      notify('Administrator created.');
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  Future<void> editUser(Map<String, dynamic> user) async {
    final payload = await _userDialog(user: user);
    if (payload == null) return;
    try {
      final updated = await widget.api.patch('/api/v1/admin/users/${user['id']}', payload);
      if (!mounted) return;
      _replaceUser(updated);
      notify('Administrator access updated.');
      if ('${user['id']}' == '${widget.currentUser['id']}') {
        notify('Your own permissions are now enforced immediately. Reload the page to refresh navigation if your roles changed.');
      }
    } catch (e) {
      if (mounted) notify(e.toString(), failure: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (!canManage) {
      return const _MessageCard(
        icon: Icons.lock_outline_rounded,
        title: 'Platform Admin access required',
        message: 'Role assignment and administrator lifecycle are protected by backend Administration permissions.',
      );
    }
    if (loading && roles.isEmpty && users.isEmpty) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 28),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (error != null && roles.isEmpty && users.isEmpty) {
      return _MessageCard(
        icon: Icons.error_outline_rounded,
        title: 'Roles & Permissions could not be loaded',
        message: error!,
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Roles & Permissions',
          subtitle: 'Backend-enforced RBAC. Read, write and approval rights are checked before each protected administration API request.',
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _MiniCounter(label: '${users.length} admins'),
              const SizedBox(width: 8),
              FilledButton.icon(
                onPressed: roles.isEmpty ? null : createUser,
                icon: const Icon(Icons.person_add_alt_1_rounded),
                label: const LText('Add administrator'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        const _RuleStrip(items: [
          _RuleItem(Icons.admin_panel_settings_outlined, 'Platform', 'Full control'),
          _RuleItem(Icons.settings_suggest_outlined, 'Operations', 'Technical operations'),
          _RuleItem(Icons.account_balance_wallet_outlined, 'Finance', 'Commercial control'),
          _RuleItem(Icons.analytics_outlined, 'Reporting', 'Impact & reports'),
        ]),
        const SizedBox(height: 18),
        _SectionHeader(
          title: 'Role Matrix',
          subtitle: 'Roles are additive. Approval permissions protect sensitive actions such as publishing, verification and provisioning execution.',
          trailing: _MiniCounter(label: '${roles.length} roles'),
        ),
        const SizedBox(height: 10),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth < 760
                ? constraints.maxWidth
                : constraints.maxWidth < 1180
                    ? (constraints.maxWidth - 12) / 2
                    : (constraints.maxWidth - 36) / 4;
            return Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                for (final role in roles) SizedBox(width: width, child: _roleCard(role)),
              ],
            );
          },
        ),
        const SizedBox(height: 24),
        _SectionHeader(
          title: 'Administrators',
          subtitle: 'Accounts are never hard-deleted from administration. Suspend access to preserve audit identity and historical attribution.',
          trailing: _MiniCounter(label: '${users.where((u) => u['active'] == true).length} active'),
        ),
        const SizedBox(height: 10),
        if (users.isEmpty)
          const _MessageCard(
            icon: Icons.person_off_outlined,
            title: 'No administrators found',
            message: 'Create an administrator to assign a scoped HIMATE role.',
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
                  for (final user in users) SizedBox(width: width, child: _userCard(user)),
                ],
              );
            },
          ),
        if (loading) ...[
          const SizedBox(height: 12),
          const LinearProgressIndicator(minHeight: 2, color: brandGold, backgroundColor: brandMist),
        ],
      ],
    );
  }
}

class CompanySettingsPanel extends StatefulWidget {
  const CompanySettingsPanel({required this.api, required this.currentUser, super.key});
  final Api api;
  final Map<String, dynamic> currentUser;
  @override
  State<CompanySettingsPanel> createState() => _CompanySettingsPanelState();
}

class _CompanySettingsPanelState extends State<CompanySettingsPanel> {
  Map<String, dynamic>? profile;
  bool loading = false;
  String? error;
  bool get canManage => widget.currentUser['system_owner'] == true;

  @override
  void initState() { super.initState(); if (canManage) load(); }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final data = await widget.api.get('/api/v1/billing/profile', force: true);
      if (mounted) setState(() { profile = data; loading = false; });
    } catch (e) {
      if (mounted) setState(() { error = e.toString(); loading = false; });
    }
  }

  Future<void> edit() async {
    final current = profile ?? <String, dynamic>{};
    final legal = TextEditingController(text: '${current['legal_name'] ?? ''}');
    final registration = TextEditingController(text: '${current['registration_number'] ?? ''}');
    final address = TextEditingController(text: '${current['address'] ?? ''}');
    final tax = TextEditingController(text: '${current['tax_id'] ?? ''}');
    final contact = TextEditingController(text: '${current['contact_name'] ?? ''}');
    final email = TextEditingController(text: '${current['email'] ?? ''}');
    final phone = TextEditingController(text: '${current['phone'] ?? ''}');
    final bank = TextEditingController(text: '${current['bank_name'] ?? ''}');
    final bankAddress = TextEditingController(text: '${current['bank_address'] ?? ''}');
    final account = TextEditingController(text: '${current['account_number'] ?? ''}');
    final iban = TextEditingController(text: '${current['iban'] ?? ''}');
    final swift = TextEditingController(text: '${current['swift'] ?? ''}');

    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => BrandDialog(
        title: 'HIMATE company settings',
        subtitle: 'Authoritative issuer and company identity used across billing and commercial records.',
        icon: Icons.corporate_fare_outlined,
        width: 820,
        child: Column(mainAxisSize: MainAxisSize.min, children: [
          ResponsiveFieldPair(
            first: TextField(controller: legal, decoration: InputDecoration(labelText: uiLiteral('Legal company name'))),
            second: TextField(controller: registration, decoration: InputDecoration(labelText: uiLiteral('Registration number'))),
          ),
          const SizedBox(height: 12),
          ResponsiveFieldPair(
            first: TextField(controller: tax, decoration: InputDecoration(labelText: uiLiteral('Tax / VAT ID'))),
            second: TextField(controller: contact, decoration: InputDecoration(labelText: uiLiteral('Billing contact'))),
          ),
          const SizedBox(height: 12),
          TextField(controller: address, decoration: InputDecoration(labelText: uiLiteral('Registered address'))),
          const SizedBox(height: 12),
          ResponsiveFieldPair(
            first: TextField(controller: email, keyboardType: TextInputType.emailAddress, decoration: InputDecoration(labelText: uiLiteral('Company / billing email'))),
            second: TextField(controller: phone, decoration: InputDecoration(labelText: uiLiteral('Phone'))),
          ),
          const SizedBox(height: 18),
          const _DialogSectionLabel('BANKING'),
          const SizedBox(height: 10),
          ResponsiveFieldPair(
            first: TextField(controller: bank, decoration: InputDecoration(labelText: uiLiteral('Bank name'))),
            second: TextField(controller: bankAddress, decoration: InputDecoration(labelText: uiLiteral('Bank address'))),
          ),
          const SizedBox(height: 12),
          TextField(controller: account, decoration: InputDecoration(labelText: uiLiteral('Account number'))),
          const SizedBox(height: 12),
          ResponsiveFieldPair(
            first: TextField(controller: iban, decoration: InputDecoration(labelText: uiLiteral('IBAN'))),
            second: TextField(controller: swift, decoration: InputDecoration(labelText: uiLiteral('SWIFT / BIC'))),
          ),
        ]),
        primaryLabel: 'Save company settings',
        onPrimary: () => Navigator.pop(dialogContext, true),
      ),
    );

    if (ok == true) {
      try {
        final updated = await widget.api.put('/api/v1/billing/profile', {
          'legal_name': legal.text.trim(), 'registration_number': registration.text.trim(),
          'address': address.text.trim(), 'tax_id': tax.text.trim(),
          'contact_name': contact.text.trim(), 'email': email.text.trim(), 'phone': phone.text.trim(),
          'bank_name': bank.text.trim(), 'bank_address': bankAddress.text.trim(),
          'account_number': account.text.trim(), 'iban': iban.text.trim(), 'swift': swift.text.trim(),
        });
        if (mounted) {
          setState(() => profile = updated);
          ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: LText('HIMATE company settings updated.'), behavior: SnackBarBehavior.floating, backgroundColor: brandSuccess));
        }
      } catch (e) {
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: LText(e.toString()), behavior: SnackBarBehavior.floating, backgroundColor: brandDanger));
      }
    }
    for (final controller in [legal,registration,address,tax,contact,email,phone,bank,bankAddress,account,iban,swift]) { controller.dispose(); }
  }

  @override
  Widget build(BuildContext context) {
    if (!canManage) return const SizedBox.shrink();
    if (loading && profile == null) return const Padding(padding: EdgeInsets.symmetric(vertical: 22), child: Center(child: CircularProgressIndicator()));
    if (error != null && profile == null) return _MessageCard(icon: Icons.error_outline_rounded, title: 'Company settings unavailable', message: error!);
    final data = profile ?? <String, dynamic>{};
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      _SectionHeader(
        title: 'HIMATE Company',
        subtitle: 'Issuer identity, billing contact and banking details used by the HIMATE control plane.',
        trailing: FilledButton.icon(onPressed: edit, icon: const Icon(Icons.edit_outlined), label: const LText('Edit company')),
      ),
      const SizedBox(height: 10),
      LayoutBuilder(builder: (context, constraints) {
        final width = constraints.maxWidth < 760 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
        return Wrap(spacing: 12, runSpacing: 12, children: [
          SizedBox(width: width, child: _InfoCard(title: 'Corporate identity', icon: Icons.corporate_fare_outlined, children: [
            _DefinitionRow(label: 'Legal name', value: '${data['legal_name'] ?? '—'}'),
            _DefinitionRow(label: 'Registration', value: '${data['registration_number'] ?? '—'}'),
            _DefinitionRow(label: 'Tax / VAT', value: '${data['tax_id'] ?? '—'}'),
            _DefinitionRow(label: 'Address', value: '${data['address'] ?? '—'}'),
            _DefinitionRow(label: 'Contact', value: '${data['contact_name'] ?? '—'}'),
            _DefinitionRow(label: 'Email', value: '${data['email'] ?? '—'}'),
          ])),
          SizedBox(width: width, child: _InfoCard(title: 'Banking', icon: Icons.account_balance_outlined, children: [
            _DefinitionRow(label: 'Bank', value: '${data['bank_name'] ?? '—'}'),
            _DefinitionRow(label: 'Bank address', value: '${data['bank_address'] ?? '—'}'),
            _DefinitionRow(label: 'Account', value: '${data['account_number'] ?? '—'}'),
            _DefinitionRow(label: 'IBAN', value: '${data['iban'] ?? '—'}'),
            _DefinitionRow(label: 'SWIFT / BIC', value: '${data['swift'] ?? '—'}'),
          ])),
        ]);
      }),
    ]);
  }
}
