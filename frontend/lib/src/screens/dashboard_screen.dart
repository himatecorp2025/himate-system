import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/dashboard_summary.dart';
import '../theme/himate_theme.dart';
import '../widgets/kpi_card.dart';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen({required this.api, super.key});

  final ApiClient api;

  Future<DashboardSummary> _load() async {
    final json = await api.getJson('/api/v1/dashboard/summary');
    return DashboardSummary.fromJson(json);
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<DashboardSummary>(
      future: _load(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }
        if (snapshot.hasError || !snapshot.hasData) {
          return _ErrorState(onRetry: () {
            (context as Element).markNeedsBuild();
          });
        }
        final data = snapshot.data!;
        return LayoutBuilder(
          builder: (context, constraints) {
            final columns = constraints.maxWidth >= 1250
                ? 4
                : constraints.maxWidth >= 760
                    ? 2
                    : 1;
            final cardWidth = (constraints.maxWidth - ((columns - 1) * 16)) / columns;

            return SingleChildScrollView(
              padding: const EdgeInsets.all(28),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'HIMATE overview',
                              style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                                    color: HimateColors.text,
                                    fontWeight: FontWeight.w700,
                                  ),
                            ),
                            const SizedBox(height: 6),
                            const Text(
                              'Verified data only. Empty values remain zero until real partner data is collected.',
                              style: TextStyle(color: HimateColors.muted),
                            ),
                          ],
                        ),
                      ),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                        decoration: BoxDecoration(
                          color: HimateColors.navy.withOpacity(0.06),
                          borderRadius: BorderRadius.circular(999),
                        ),
                        child: Text(
                          '${data.environment.toUpperCase()} · ${data.version}',
                          style: const TextStyle(
                            color: HimateColors.navy,
                            fontSize: 12,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 28),
                  Wrap(
                    spacing: 16,
                    runSpacing: 16,
                    children: [
                      SizedBox(
                        width: cardWidth,
                        child: KpiCard(
                          label: 'Partners',
                          value: '${data.totalPartners}',
                          icon: Icons.business_outlined,
                          caption: '${data.livePartners} live · ${data.stagingPartners} staging',
                        ),
                      ),
                      SizedBox(
                        width: cardWidth,
                        child: KpiCard(
                          label: 'Active modules',
                          value: '${data.activeModules}',
                          icon: Icons.grid_view_rounded,
                          caption: '${data.notLicensedModules} not licensed · ${data.maintenanceModules} maintenance',
                        ),
                      ),
                      SizedBox(
                        width: cardWidth,
                        child: KpiCard(
                          label: 'Verified impact metrics',
                          value: '${data.verifiedMetrics}',
                          icon: Icons.insights_outlined,
                          caption: data.impactStatus == 'not_collected'
                              ? 'No impact data collected yet'
                              : data.impactStatus,
                        ),
                      ),
                      SizedBox(
                        width: cardWidth,
                        child: KpiCard(
                          label: 'System health',
                          value: data.systemStatus.toUpperCase(),
                          icon: Icons.health_and_safety_outlined,
                          caption: 'START 01-03 control plane foundation',
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 26),
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(24),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Current development scope',
                            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                                  fontWeight: FontWeight.w700,
                                ),
                          ),
                          const SizedBox(height: 16),
                          const _ScopeRow(
                            icon: Icons.check_circle_outline,
                            title: 'START-01',
                            text: 'Project skeleton, Go service foundation, Render-ready container layout.',
                          ),
                          const _ScopeRow(
                            icon: Icons.check_circle_outline,
                            title: 'START-02',
                            text: 'Bootstrap administration authentication, secure session cookie and admin shell.',
                          ),
                          const _ScopeRow(
                            icon: Icons.check_circle_outline,
                            title: 'START-03',
                            text: 'Responsive sidebar/card navigation and verified-zero dashboard foundation.',
                          ),
                          const Divider(height: 32),
                          const Text(
                            'Partner registry, licensing, provisioning, impact evidence and reporting are intentionally reserved for later START blocks.',
                            style: TextStyle(color: HimateColors.muted),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            );
          },
        );
      },
    );
  }
}

class _ScopeRow extends StatelessWidget {
  const _ScopeRow({required this.icon, required this.title, required this.text});

  final IconData icon;
  final String title;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 20, color: HimateColors.success),
          const SizedBox(width: 12),
          SizedBox(
            width: 86,
            child: Text(title, style: const TextStyle(fontWeight: FontWeight.w700)),
          ),
          Expanded(child: Text(text)),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off_outlined, size: 48, color: HimateColors.muted),
            const SizedBox(height: 16),
            const Text('Dashboard data could not be loaded.'),
            const SizedBox(height: 12),
            OutlinedButton(onPressed: onRetry, child: const Text('Retry')),
          ],
        ),
      ),
    );
  }
}
