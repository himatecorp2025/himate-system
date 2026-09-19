import 'package:flutter/material.dart';

import '../services/auth_controller.dart';
import '../theme/himate_theme.dart';
import '../widgets/brand_mark.dart';
import 'dashboard_screen.dart';
import 'placeholder_section_screen.dart';

class ShellScreen extends StatefulWidget {
  const ShellScreen({required this.auth, super.key});

  final AuthController auth;

  @override
  State<ShellScreen> createState() => _ShellScreenState();
}

class _ShellScreenState extends State<ShellScreen> {
  int _selectedIndex = 0;

  static const _items = <_NavItem>[
    _NavItem(Icons.dashboard_outlined, 'Dashboard'),
    _NavItem(Icons.business_outlined, 'Partners'),
    _NavItem(Icons.account_balance_wallet_outlined, 'Licensing & Finance'),
    _NavItem(Icons.insights_outlined, 'Impact & Reports'),
    _NavItem(Icons.public_outlined, 'Website & Marketing'),
    _NavItem(Icons.settings_suggest_outlined, 'System & Operations'),
    _NavItem(Icons.admin_panel_settings_outlined, 'Administration'),
  ];

  Widget _screenFor(int index) {
    switch (index) {
      case 0:
        return DashboardScreen(api: widget.auth.api);
      case 1:
        return const PlaceholderSectionScreen(
          title: 'Partners',
          description: 'Partner cards, onboarding and partner workspace.',
          futureStartBlock: 'START-04 and START-05',
        );
      case 2:
        return const PlaceholderSectionScreen(
          title: 'Licensing & Finance',
          description: 'License evidence, partner pricing, subscriptions and finance documents.',
          futureStartBlock: 'START-07 and START-08',
        );
      case 3:
        return const PlaceholderSectionScreen(
          title: 'Impact & Reports',
          description: 'Verified metrics, manual evidence, impact history and PDF reporting.',
          futureStartBlock: 'START-13 to START-15',
        );
      case 4:
        return const PlaceholderSectionScreen(
          title: 'Website & Marketing',
          description: 'HIMATE CMS, SEO and the public arts technology website.',
          futureStartBlock: 'START-16 and START-17',
        );
      case 5:
        return const PlaceholderSectionScreen(
          title: 'System & Operations',
          description: 'Provisioning, environments, domains, health, releases and backups.',
          futureStartBlock: 'START-09 to START-12 and START-20 to START-21',
        );
      default:
        return const PlaceholderSectionScreen(
          title: 'Administration',
          description: 'HIMATE users, permissions, security controls and audit.',
          futureStartBlock: 'START-18 and START-19',
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final wide = constraints.maxWidth >= 980;
        final extended = constraints.maxWidth >= 1240;
        final body = _screenFor(_selectedIndex);

        if (!wide) {
          return Scaffold(
            appBar: AppBar(
              backgroundColor: HimateColors.navy,
              foregroundColor: Colors.white,
              title: Text(_items[_selectedIndex].label),
              actions: [
                IconButton(
                  tooltip: 'Sign out',
                  onPressed: widget.auth.logout,
                  icon: const Icon(Icons.logout),
                ),
              ],
            ),
            drawer: Drawer(
              backgroundColor: HimateColors.navy,
              child: SafeArea(
                child: Column(
                  children: [
                    const Padding(
                      padding: EdgeInsets.all(22),
                      child: HimateBrandMark(),
                    ),
                    const Divider(color: Colors.white24),
                    Expanded(
                      child: ListView.builder(
                        itemCount: _items.length,
                        itemBuilder: (context, index) {
                          final item = _items[index];
                          final selected = index == _selectedIndex;
                          return ListTile(
                            selected: selected,
                            selectedTileColor: Colors.white.withOpacity(0.08),
                            leading: Icon(item.icon, color: selected ? HimateColors.gold : Colors.white70),
                            title: Text(item.label, style: const TextStyle(color: Colors.white)),
                            onTap: () {
                              setState(() => _selectedIndex = index);
                              Navigator.of(context).pop();
                            },
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
            ),
            body: body,
          );
        }

        return Scaffold(
          body: Row(
            children: [
              NavigationRail(
                extended: extended,
                minExtendedWidth: 270,
                backgroundColor: HimateColors.navy,
                selectedIndex: _selectedIndex,
                selectedIconTheme: const IconThemeData(color: HimateColors.gold),
                unselectedIconTheme: const IconThemeData(color: Colors.white60),
                selectedLabelTextStyle: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700),
                unselectedLabelTextStyle: const TextStyle(color: Colors.white70),
                leading: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 22),
                  child: HimateBrandMark(compact: !extended),
                ),
                trailing: Expanded(
                  child: Align(
                    alignment: Alignment.bottomCenter,
                    child: Padding(
                      padding: const EdgeInsets.only(bottom: 20),
                      child: IconButton(
                        tooltip: 'Sign out',
                        onPressed: widget.auth.logout,
                        color: Colors.white70,
                        icon: const Icon(Icons.logout),
                      ),
                    ),
                  ),
                ),
                destinations: [
                  for (final item in _items)
                    NavigationRailDestination(
                      icon: Icon(item.icon),
                      label: Text(item.label),
                    ),
                ],
                onDestinationSelected: (index) => setState(() => _selectedIndex = index),
              ),
              Expanded(
                child: Column(
                  children: [
                    Container(
                      height: 68,
                      color: Colors.white,
                      padding: const EdgeInsets.symmetric(horizontal: 24),
                      child: Row(
                        children: [
                          Text(
                            _items[_selectedIndex].label,
                            style: const TextStyle(
                              color: HimateColors.text,
                              fontSize: 18,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          const Spacer(),
                          Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            crossAxisAlignment: CrossAxisAlignment.end,
                            children: [
                              Text(
                                widget.auth.user?.name ?? '',
                                style: const TextStyle(fontWeight: FontWeight.w700),
                              ),
                              Text(
                                widget.auth.user?.email ?? '',
                                style: const TextStyle(color: HimateColors.muted, fontSize: 12),
                              ),
                            ],
                          ),
                          const SizedBox(width: 12),
                          const CircleAvatar(
                            backgroundColor: HimateColors.navy,
                            child: Icon(Icons.person_outline, color: Colors.white),
                          ),
                        ],
                      ),
                    ),
                    const Divider(height: 1),
                    Expanded(child: body),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _NavItem {
  const _NavItem(this.icon, this.label);

  final IconData icon;
  final String label;
}
