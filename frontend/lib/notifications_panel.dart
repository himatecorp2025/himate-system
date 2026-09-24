part of 'main.dart';

class NotificationCenterButton extends StatefulWidget {
  const NotificationCenterButton({
    required this.api,
    this.endpointPrefix = '/api/v1/notifications',
    this.panelSubtitle = 'Control-plane events that match your permissions.',
    this.iconColor = brandNavy,
    super.key,
  });
  final Api api;
  final String endpointPrefix;
  final String panelSubtitle;
  final Color iconColor;

  @override
  State<NotificationCenterButton> createState() => _NotificationCenterButtonState();
}

class _NotificationCenterButtonState extends State<NotificationCenterButton> {
  int unread = 0;
  Timer? timer;

  @override
  void initState() {
    super.initState();
    refreshCount();
    timer = Timer.periodic(const Duration(seconds: 45), (_) => refreshCount());
  }

  @override
  void dispose() {
    timer?.cancel();
    super.dispose();
  }

  Future<void> refreshCount() async {
    try {
      final data = await widget.api.get('${widget.endpointPrefix}?limit=100', force: true);
      if (mounted) setState(() => unread = (data['unread_count'] as num?)?.toInt() ?? 0);
    } catch (_) {
      // Notification availability must never block core navigation.
    }
  }

  Future<void> openCenter() async {
    await showGeneralDialog<void>(
      context: context,
      barrierDismissible: true,
      barrierLabel: 'Notifications',
      barrierColor: Colors.black.withOpacity(.18),
      transitionDuration: const Duration(milliseconds: 180),
      pageBuilder: (context, _, __) => Align(
        alignment: Alignment.centerRight,
        child: Material(
          color: brandWhite,
          elevation: 18,
          child: SafeArea(
            child: SizedBox(
              width: MediaQuery.sizeOf(context).width < 620
                  ? MediaQuery.sizeOf(context).width
                  : 430,
              child: NotificationCenterPanel(
                api: widget.api,
                endpointPrefix: widget.endpointPrefix,
                subtitle: widget.panelSubtitle,
              ),
            ),
          ),
        ),
      ),
      transitionBuilder: (context, animation, _, child) {
        return SlideTransition(
          position: Tween<Offset>(begin: const Offset(1, 0), end: Offset.zero).animate(
            CurvedAnimation(parent: animation, curve: Curves.easeOutCubic),
          ),
          child: child,
        );
      },
    );
    await refreshCount();
  }

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: unread > 0 ? 'Notifications · ' + unread.toString() + ' unread' : 'Notifications',
      child: InkWell(
        onTap: openCenter,
        borderRadius: BorderRadius.circular(12),
        child: SizedBox(
          width: 44,
          height: 44,
          child: Stack(
            clipBehavior: Clip.none,
            children: [
              Center(child: Icon(Icons.notifications_none_rounded, color: widget.iconColor, size: 22)),
              if (unread > 0)
                Positioned(
                  top: 5,
                  right: 3,
                  child: Container(
                    constraints: const BoxConstraints(minWidth: 17, minHeight: 17),
                    padding: const EdgeInsets.symmetric(horizontal: 4),
                    decoration: BoxDecoration(
                      color: brandDanger,
                      borderRadius: BorderRadius.circular(99),
                      border: Border.all(color: brandWhite, width: 1.5),
                    ),
                    child: Center(
                      child: LText(
                        unread > 99 ? '99+' : unread.toString(),
                        style: const TextStyle(color: brandWhite, fontSize: 8, fontWeight: FontWeight.w800),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class NotificationCenterPanel extends StatefulWidget {
  const NotificationCenterPanel({
    required this.api,
    this.endpointPrefix = '/api/v1/notifications',
    this.subtitle = 'Control-plane events that match your permissions.',
    super.key,
  });
  final Api api;
  final String endpointPrefix;
  final String subtitle;

  @override
  State<NotificationCenterPanel> createState() => _NotificationCenterPanelState();
}

class _NotificationCenterPanelState extends State<NotificationCenterPanel> {
  List<Map<String, dynamic>> notifications = <Map<String, dynamic>>[];
  bool loading = true;
  bool unreadOnly = false;
  String? error;
  int unreadCount = 0;

  String s(dynamic value) => value == null ? '' : value.toString();

  @override
  void initState() {
    super.initState();
    load();
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final suffix = unreadOnly ? '&unread_only=true' : '';
      final data = await widget.api.get('${widget.endpointPrefix}?limit=60' + suffix, force: true);
      if (!mounted) return;
      setState(() {
        notifications = items(data);
        unreadCount = (data['unread_count'] as num?)?.toInt() ?? 0;
        loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { error = e.toString(); loading = false; });
    }
  }

  Future<void> markRead(Map<String, dynamic> item) async {
    if (item['read'] == true) return;
    try {
      await widget.api.post('${widget.endpointPrefix}/' + s(item['id']) + '/read');
      await load();
    } catch (_) {}
  }

  Future<void> markAllRead() async {
    try {
      await widget.api.post('${widget.endpointPrefix}/read-all');
      await load();
    } catch (_) {}
  }

  Color severityColor(String value) {
    switch (value.toUpperCase()) {
      case 'CRITICAL':
        return brandDanger;
      case 'WARNING':
        return brandWarning;
      default:
        return brandSteel;
    }
  }

  IconData severityIcon(String value) {
    switch (value.toUpperCase()) {
      case 'CRITICAL':
        return Icons.error_outline_rounded;
      case 'WARNING':
        return Icons.warning_amber_rounded;
      default:
        return Icons.notifications_outlined;
    }
  }

  String timestamp(dynamic value) {
    final parsed = DateTime.tryParse(s(value))?.toLocal();
    if (parsed == null) return s(value);
    return intl.DateFormat('MMM d · HH:mm').format(parsed);
  }

  Widget itemCard(Map<String, dynamic> item) {
    final severity = s(item['severity']).isEmpty ? 'INFO' : s(item['severity']);
    final tone = severityColor(severity);
    final read = item['read'] == true;
    final partner = s(item['partner_id']);
    final category = s(item['category']).isEmpty ? 'SYSTEM' : s(item['category']).toUpperCase();
    return InkWell(
      onTap: () => markRead(item),
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: read ? brandWhite : tone.withOpacity(.045),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: read ? brandMist : tone.withOpacity(.22)),
        ),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(color: tone.withOpacity(.10), borderRadius: BorderRadius.circular(10)),
            child: Icon(severityIcon(severity), color: tone, size: 19),
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Row(children: [
                Expanded(child: LText(s(item['title']), style: const TextStyle(color: brandNavy, fontSize: 12, fontWeight: FontWeight.w700))),
                if (!read)
                  Container(width: 7, height: 7, decoration: const BoxDecoration(color: brandDanger, shape: BoxShape.circle)),
              ]),
              const SizedBox(height: 4),
              LText(s(item['message']), style: const TextStyle(color: brandTextSoft, fontSize: 9.8, height: 1.4)),
              const SizedBox(height: 7),
              Wrap(spacing: 7, runSpacing: 4, children: [
                LText(timestamp(item['created_at']), style: const TextStyle(color: brandTextSoft, fontSize: 8.8)),
                if (partner.isNotEmpty && widget.endpointPrefix == '/api/v1/notifications') _MiniCounter(label: partner),
                _MiniCounter(label: category),
                _MiniCounter(label: severity),
              ]),
            ]),
          ),
        ]),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Container(
        padding: const EdgeInsets.fromLTRB(20, 17, 12, 15),
        decoration: const BoxDecoration(border: Border(bottom: BorderSide(color: brandMist))),
        child: Row(children: [
          Expanded(
            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              const LText('Notifications', style: TextStyle(color: brandNavy, fontWeight: FontWeight.w800, fontSize: 18)),
              const SizedBox(height: 3),
              LText(widget.subtitle, style: const TextStyle(color: brandTextSoft, fontSize: 9.5)),
            ]),
          ),
          IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close_rounded)),
        ]),
      ),
      Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Wrap(
          alignment: WrapAlignment.end,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: 8,
          runSpacing: 8,
          children: [
            _MiniCounter(label: unreadCount.toString() + ' unread'),
            FilterChip(
              selected: unreadOnly,
              label: const LText('Unread only'),
              onSelected: (value) {
                setState(() => unreadOnly = value);
                load();
              },
            ),
            TextButton(onPressed: unreadCount > 0 ? markAllRead : null, child: const LText('Mark all read')),
          ],
        ),
      ),
      Expanded(
        child: loading
            ? const Center(child: CircularProgressIndicator())
            : error != null
                ? Padding(
                    padding: const EdgeInsets.all(16),
                    child: _MessageCard(icon: Icons.cloud_off_outlined, title: 'Notifications unavailable', message: error!),
                  )
                : notifications.isEmpty
                    ? const Padding(
                        padding: EdgeInsets.all(16),
                        child: _MessageCard(icon: Icons.notifications_none_rounded, title: 'Nothing to review', message: 'New events that match your access will appear here.'),
                      )
                    : ListView.separated(
                        padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
                        itemCount: notifications.length,
                        separatorBuilder: (_, __) => const SizedBox(height: 9),
                        itemBuilder: (_, index) => itemCard(notifications[index]),
                      ),
      ),
    ]);
  }
}
