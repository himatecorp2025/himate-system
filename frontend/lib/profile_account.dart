// ignore_for_file: deprecated_member_use
part of 'main.dart';

Future<void> showAccountProfileDialog(
  BuildContext context, {
  required Api api,
  required Map<String, dynamic> currentUser,
  required ValueChanged<Map<String, dynamic>> onUserChanged,
}) async {
  await showDialog<void>(
    context: context,
    builder: (_) => _AccountProfileDialog(
      api: api,
      currentUser: currentUser,
      onUserChanged: onUserChanged,
    ),
  );
}

class _AccountProfileDialog extends StatefulWidget {
  const _AccountProfileDialog({
    required this.api,
    required this.currentUser,
    required this.onUserChanged,
  });

  final Api api;
  final Map<String, dynamic> currentUser;
  final ValueChanged<Map<String, dynamic>> onUserChanged;

  @override
  State<_AccountProfileDialog> createState() => _AccountProfileDialogState();
}

class _AccountProfileDialogState extends State<_AccountProfileDialog> {
  late final TextEditingController name;
  late final TextEditingController jobTitle;
  late final TextEditingController phone;
  late final TextEditingController timezone;
  final currentPassword = TextEditingController();
  final newPassword = TextEditingController();
  final confirmPassword = TextEditingController();

  late String locale;
  bool saving = false;
  bool changingPassword = false;
  String? error;
  String? success;

  @override
  void initState() {
    super.initState();
    name = TextEditingController(text: '${widget.currentUser['name'] ?? ''}');
    jobTitle = TextEditingController(text: '${widget.currentUser['job_title'] ?? ''}');
    phone = TextEditingController(text: '${widget.currentUser['phone'] ?? ''}');
    timezone = TextEditingController(text: '${widget.currentUser['timezone'] ?? 'UTC'}');
    locale = '${widget.currentUser['preferred_locale'] ?? 'en_US'}' == 'hu_HU' ? 'hu_HU' : 'en_US';
  }

  @override
  void dispose() {
    name.dispose();
    jobTitle.dispose();
    phone.dispose();
    timezone.dispose();
    currentPassword.dispose();
    newPassword.dispose();
    confirmPassword.dispose();
    super.dispose();
  }

  Future<void> save() async {
    setState(() { saving = true; error = null; success = null; });
    try {
      final updated = await widget.api.patch('/api/v1/profile', {
        'name': name.text.trim(),
        'job_title': jobTitle.text.trim(),
        'phone': phone.text.trim(),
        'timezone': timezone.text.trim(),
        'preferred_locale': locale,
      });
      widget.onUserChanged(updated);
      if (mounted) setState(() => success = HimateI18n.text(locale, 'profileUpdated'));
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => saving = false);
    }
  }

  Future<void> changePassword() async {
    if (newPassword.text != confirmPassword.text) {
      setState(() => error = HimateI18n.text(locale, 'passwordsMismatch'));
      return;
    }
    final policyMessage = himatePasswordPolicyMessage(locale, newPassword.text);
    if (policyMessage != null) {
      setState(() => error = policyMessage);
      return;
    }
    setState(() { changingPassword = true; error = null; success = null; });
    try {
      final updated = await widget.api.post('/api/v1/profile/password', {
        'current_password': currentPassword.text,
        'new_password': newPassword.text,
      });
      widget.onUserChanged(updated);
      currentPassword.clear();
      newPassword.clear();
      confirmPassword.clear();
      if (mounted) setState(() => success = HimateI18n.text(locale, 'passwordChanged'));
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => changingPassword = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    String t(String key) => HimateI18n.text(locale, key);
    final roles = widget.currentUser['roles'] is List
        ? (widget.currentUser['roles'] as List).map((e) => e.toString()).toList()
        : <String>[];

    return AlertDialog(
      constraints: const BoxConstraints(maxWidth: 720),
      title: Row(
        children: [
          const Icon(Icons.account_circle_outlined, color: brandNavy),
          const SizedBox(width: 10),
          Expanded(child: Text(t('profile'))),
          IconButton(
            onPressed: () => Navigator.pop(context),
            tooltip: t('close'),
            icon: const Icon(Icons.close_rounded),
          ),
        ],
      ),
      content: SizedBox(
        width: 680,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(t('profileIntro'), style: const TextStyle(color: brandTextSoft)),
              if (widget.currentUser['system_owner'] == true) ...[
                const SizedBox(height: 14),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: brandGold.withOpacity(.10),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: brandGold.withOpacity(.28)),
                  ),
                  child: Row(
                    children: [
                      const Icon(Icons.verified_user_outlined, color: brandNavy),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(t('systemOwner'), style: const TextStyle(fontWeight: FontWeight.w700, color: brandNavy)),
                            Text(t('systemOwnerHint'), style: const TextStyle(color: brandTextSoft, fontSize: 11)),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
              const SizedBox(height: 18),
              ResponsiveFieldPair(
                first: TextField(controller: name, decoration: InputDecoration(labelText: t('fullName'))),
                second: TextFormField(
                  initialValue: '${widget.currentUser['email'] ?? ''}',
                  readOnly: true,
                  decoration: InputDecoration(labelText: t('emailAddress')),
                ),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: TextField(controller: jobTitle, decoration: InputDecoration(labelText: t('jobTitle'))),
                second: TextField(controller: phone, decoration: InputDecoration(labelText: t('phone'))),
              ),
              const SizedBox(height: 12),
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: locale,
                  decoration: InputDecoration(labelText: t('language')),
                  items: [
                    DropdownMenuItem(value: 'en_US', child: Text(t('englishUS'))),
                    DropdownMenuItem(value: 'hu_HU', child: Text(t('hungarian'))),
                  ],
                  onChanged: (value) {
                    if (value != null) setState(() => locale = value);
                  },
                ),
                second: TextField(controller: timezone, decoration: InputDecoration(labelText: t('timezone'))),
              ),
              const SizedBox(height: 10),
              Text('${t('roles')}: ${roles.join(', ')}', style: const TextStyle(color: brandTextSoft, fontSize: 11)),
              const SizedBox(height: 14),
              FilledButton.icon(
                onPressed: saving ? null : save,
                icon: saving
                    ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2, color: brandWhite))
                    : const Icon(Icons.save_outlined),
                label: Text(t('saveChanges')),
              ),
              const Padding(padding: EdgeInsets.symmetric(vertical: 22), child: Divider()),
              Text(t('passwordSecurity'), style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 12),
              TextField(
                controller: currentPassword,
                obscureText: true,
                decoration: InputDecoration(labelText: t('currentPassword')),
              ),
              const SizedBox(height: 10),
              ResponsiveFieldPair(
                first: TextField(controller: newPassword, obscureText: true, decoration: InputDecoration(labelText: t('newPassword'), helperText: t('passwordPolicy'))),
                second: TextField(controller: confirmPassword, obscureText: true, decoration: InputDecoration(labelText: t('confirmPassword'))),
              ),
              const SizedBox(height: 10),
              OutlinedButton.icon(
                onPressed: changingPassword ? null : changePassword,
                icon: const Icon(Icons.password_rounded),
                label: Text(t('changePassword')),
              ),
              if (error != null) ...[
                const SizedBox(height: 12),
                Text(error!, style: const TextStyle(color: brandDanger)),
              ],
              if (success != null) ...[
                const SizedBox(height: 12),
                Text(success!, style: const TextStyle(color: brandSuccess)),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: Text(t('close'))),
      ],
    );
  }
}
