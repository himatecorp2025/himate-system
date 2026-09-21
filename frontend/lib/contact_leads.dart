part of 'main.dart';

class ContactLeadsPanel extends StatefulWidget {
  const ContactLeadsPanel({required this.api, super.key});
  final Api api;

  @override
  State<ContactLeadsPanel> createState() => _ContactLeadsPanelState();
}

class _ContactLeadsPanelState extends State<ContactLeadsPanel> {
  final search = TextEditingController();
  Timer? debounce;
  List<Map<String, dynamic>> leads = <Map<String, dynamic>>[];
  String status = 'ALL';
  bool loading = true;
  String? error;
  int total = 0;

  static const statuses = <String>['ALL', 'NEW', 'IN_PROGRESS', 'CONTACTED', 'CLOSED'];

  @override
  void initState() {
    super.initState();
    load();
  }

  @override
  void dispose() {
    debounce?.cancel();
    search.dispose();
    super.dispose();
  }

  Uri endpoint() {
    final params = <String, String>{'limit': '100', 'offset': '0'};
    final q = search.text.trim();
    if (q.isNotEmpty) params['q'] = q;
    if (status != 'ALL') params['status'] = status;
    return Uri(path: '/api/v1/contact/inquiries', queryParameters: params);
  }

  Future<void> load() async {
    if (mounted) setState(() { loading = true; error = null; });
    try {
      final response = await widget.api.get(endpoint().toString(), force: true);
      if (!mounted) return;
      setState(() {
        leads = items(response);
        total = (response['total'] as num?)?.toInt() ?? leads.length;
      });
    } catch (e) {
      if (mounted) setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  void onSearch(String _) {
    debounce?.cancel();
    debounce = Timer(const Duration(milliseconds: 280), load);
  }

  Color tone(String value) {
    switch (value) {
      case 'NEW': return brandGold;
      case 'IN_PROGRESS': return brandSteel;
      case 'CONTACTED': return brandSuccess;
      case 'CLOSED': return brandTextSoft;
      default: return brandNavy;
    }
  }

  Future<void> editLead(Map<String, dynamic> lead) async {
    var nextStatus = (lead['lead_status'] ?? 'NEW').toString();
    final assignedTo = TextEditingController(text: (lead['assigned_to'] ?? '').toString());
    final note = TextEditingController(text: (lead['admin_note'] ?? '').toString());

    final result = await showDialog<Map<String, dynamic>?>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => BrandDialog(
          title: 'Contact lead',
          subtitle: (lead['organization'] ?? '').toString().isEmpty
              ? (lead['email'] ?? '').toString()
              : '${lead['organization']} · ${lead['email']}',
          icon: Icons.manage_search_outlined,
          width: 760,
          primaryLabel: 'Save lead',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ResponsiveFieldPair(
                first: DropdownButtonFormField<String>(
                  value: nextStatus,
                  decoration: const InputDecoration(labelText: 'Lead status'),
                  items: [
                    for (final item in statuses.where((value) => value != 'ALL'))
                      DropdownMenuItem(value: item, child: Text(_humanize(item))),
                  ],
                  onChanged: (value) {
                    if (value != null) setDialogState(() => nextStatus = value);
                  },
                ),
                second: TextField(
                  controller: assignedTo,
                  decoration: const InputDecoration(labelText: 'Assigned to', hintText: 'Name or team'),
                ),
              ),
              const SizedBox(height: 14),
              _DefinitionRow(label: 'Name', value: (lead['name'] ?? '—').toString()),
              _DefinitionRow(label: 'Organization', value: (lead['organization'] ?? '—').toString().isEmpty ? '—' : (lead['organization'] ?? '').toString()),
              _DefinitionRow(label: 'Email', value: (lead['email'] ?? '—').toString()),
              _DefinitionRow(label: 'Notification', value: _humanize((lead['notification_status'] ?? 'stored').toString())),
              const SizedBox(height: 12),
              Text('Inquiry', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 7),
              SelectableText(
                (lead['message'] ?? '').toString(),
                style: const TextStyle(color: brandCharcoal, height: 1.5),
              ),
              const SizedBox(height: 14),
              TextField(
                controller: note,
                minLines: 3,
                maxLines: 7,
                decoration: const InputDecoration(
                  labelText: 'Internal follow-up note',
                  hintText: 'Record next step, outcome or context',
                ),
              ),
            ],
          ),
          onPrimary: () => Navigator.pop(dialogContext, <String, dynamic>{
            'lead_status': nextStatus,
            'assigned_to': assignedTo.text.trim(),
            'admin_note': note.text.trim(),
          }),
        ),
      ),
    );

    assignedTo.dispose();
    note.dispose();
    if (result == null) return;
    try {
      await widget.api.patch('/api/v1/contact/inquiries/${lead['id']}', result);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Contact lead updated.')),
        );
      }
      await load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString()), backgroundColor: brandDanger),
        );
      }
    }
  }

  Widget leadCard(Map<String, dynamic> lead, double width) {
    final state = (lead['lead_status'] ?? 'NEW').toString();
    final created = DateTime.tryParse((lead['created_at'] ?? '').toString())?.toLocal();
    final createdLabel = created == null
        ? (lead['created_at'] ?? '').toString()
        : HimateI18n.dateTime(himateLocaleCode(Localizations.localeOf(context)), created);
    final organization = (lead['organization'] ?? '').toString().trim();
    final assigned = (lead['assigned_to'] ?? '').toString().trim();
    return SizedBox(
      width: width,
      child: Card(
        child: InkWell(
          borderRadius: BorderRadius.circular(10),
          onTap: () => editLead(lead),
          child: Padding(
            padding: const EdgeInsets.all(17),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Container(
                    width: 38,
                    height: 38,
                    decoration: BoxDecoration(color: tone(state).withOpacity(.10), borderRadius: BorderRadius.circular(9)),
                    child: Icon(Icons.mail_outline_rounded, color: tone(state), size: 19),
                  ),
                  const SizedBox(width: 11),
                  Expanded(
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text((lead['name'] ?? '—').toString(), maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandNavy, fontWeight: FontWeight.w700)),
                      const SizedBox(height: 2),
                      Text(organization.isEmpty ? (lead['email'] ?? '').toString() : organization, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandTextSoft, fontSize: 10)),
                    ]),
                  ),
                  _StatusPill(label: _humanize(state)),
                ]),
                const SizedBox(height: 12),
                Text((lead['message'] ?? '').toString(), maxLines: 3, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandCharcoal, fontSize: 11, height: 1.45)),
                const SizedBox(height: 12),
                const Divider(height: 1),
                const SizedBox(height: 8),
                Row(children: [
                  Expanded(child: Text(createdLabel, style: const TextStyle(color: brandTextSoft, fontSize: 9.5))),
                  if (assigned.isNotEmpty)
                    Flexible(child: Text('Assigned: $assigned', textAlign: TextAlign.right, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: brandSteel, fontSize: 9.5, fontWeight: FontWeight.w600))),
                  IconButton(
                    tooltip: 'Email contact',
                    visualDensity: VisualDensity.compact,
                    onPressed: () => html.window.open('mailto:${Uri.encodeComponent((lead['email'] ?? '').toString())}', '_self'),
                    icon: const Icon(Icons.outgoing_mail, size: 18),
                  ),
                ]),
              ],
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionHeader(
          title: 'Contact Leads',
          subtitle: 'Website inquiries stored in HIMATE with follow-up status, owner and internal notes.',
          trailing: _MiniCounter(label: '$total leads'),
        ),
        const SizedBox(height: 12),
        ResponsiveFieldPair(
          first: TextField(
            controller: search,
            onChanged: onSearch,
            decoration: const InputDecoration(
              labelText: 'Search leads',
              hintText: 'Name, organization, email or message',
              prefixIcon: Icon(Icons.search_rounded),
            ),
          ),
          second: DropdownButtonFormField<String>(
            value: status,
            decoration: const InputDecoration(labelText: 'Lead status'),
            items: [for (final item in statuses) DropdownMenuItem(value: item, child: Text(item == 'ALL' ? 'All statuses' : _humanize(item)))],
            onChanged: (value) {
              if (value == null) return;
              setState(() => status = value);
              load();
            },
          ),
        ),
        const SizedBox(height: 12),
        if (loading && leads.isEmpty)
          const _BrandLoading()
        else if (error != null && leads.isEmpty)
          _MessageCard(icon: Icons.error_outline_rounded, title: 'Contact Leads unavailable', message: error!)
        else if (leads.isEmpty)
          const _MessageCard(icon: Icons.mark_email_unread_outlined, title: 'No contact leads', message: 'New website inquiries will appear here automatically.')
        else
          LayoutBuilder(
            builder: (context, constraints) {
              final width = constraints.maxWidth < 760 ? constraints.maxWidth : (constraints.maxWidth - 12) / 2;
              return Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [for (final lead in leads) leadCard(lead, width)],
              );
            },
          ),
      ],
    );
  }
}
