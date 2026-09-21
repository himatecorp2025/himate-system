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
        content: Text(message),
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
      child: Text(
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
    return _InfoCard(
      title: '${role['label'] ?? _humanize('${role['key']}')}',
      icon: '${role['key']}' == 'platform_admin' ? Icons.shield_outlined : Icons.badge_outlined,
      children: [
        Text(
          '${role['description'] ?? ''}',
          style: const TextStyle(color: brandTextSoft, fontSize: 10.5, height: 1.45),
        ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 6,
          runSpacing: 6,
          children: [for (final permission in permissions) _permissionChip(permission)],
        ),
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
                            child: Text(
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
                      Text(
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
                    child: Text(
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
                label: const Text('Edit access'),
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
    final editingSystemOwner = editing && user?['system_owner'] == true;
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
                  decoration: const InputDecoration(labelText: 'Full name'),
                ),
                second: TextField(
                  controller: email,
                  keyboardType: TextInputType.emailAddress,
                  decoration: const InputDecoration(labelText: 'Email address'),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: password,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: editing ? 'New password (optional)' : 'Temporary password',
                  helperText: editing ? 'Leave blank to keep the current password. Minimum 12 characters if changed.' : 'Minimum 12 characters.',
                ),
              ),
              const SizedBox(height: 18),
              Text('Roles', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 5),
              const Text(
                'Permissions are additive when multiple roles are assigned.',
                style: TextStyle(color: brandTextSoft, fontSize: 10.5),
              ),
              const SizedBox(height: 8),
              for (final role in roles.where((role) =>
                  '${role['key']}' != 'platform_admin' || editingSystemOwner))
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  dense: true,
                  value: selected.contains('${role['key']}'),
                  title: Text('${role['label']}', style: const TextStyle(color: brandNavy, fontSize: 12, fontWeight: FontWeight.w700)),
                  subtitle: Text(
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
                  title: const Text('Active account', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w700)),
                  subtitle: const Text('Suspended administrators cannot sign in or use an existing session.', style: TextStyle(color: brandTextSoft, fontSize: 9.5)),
                  value: active,
                  onChanged: (value) => setDialogState(() { active = value; dialogError = null; }),
                ),
              ],
              if (dialogError != null) ...[
                const SizedBox(height: 8),
                Text(dialogError!, style: const TextStyle(color: brandDanger, fontSize: 10.5, fontWeight: FontWeight.w600)),
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
            } else if (!editing && cleanPassword.length < 12) {
              validation = 'Temporary password must be at least 12 characters.';
            } else if (editing && cleanPassword.isNotEmpty && cleanPassword.length < 12) {
              validation = 'New password must be at least 12 characters.';
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
                label: const Text('Add administrator'),
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
