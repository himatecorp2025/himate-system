class DashboardSummary {
  const DashboardSummary({
    required this.totalPartners,
    required this.livePartners,
    required this.stagingPartners,
    required this.activeModules,
    required this.notLicensedModules,
    required this.maintenanceModules,
    required this.verifiedMetrics,
    required this.impactStatus,
    required this.systemStatus,
    required this.environment,
    required this.version,
  });

  final int totalPartners;
  final int livePartners;
  final int stagingPartners;
  final int activeModules;
  final int notLicensedModules;
  final int maintenanceModules;
  final int verifiedMetrics;
  final String impactStatus;
  final String systemStatus;
  final String environment;
  final String version;

  factory DashboardSummary.fromJson(Map<String, dynamic> json) {
    final partners = _map(json['partners']);
    final modules = _map(json['modules']);
    final impact = _map(json['impact']);
    final system = _map(json['system']);
    return DashboardSummary(
      totalPartners: _int(partners['total']),
      livePartners: _int(partners['live']),
      stagingPartners: _int(partners['staging']),
      activeModules: _int(modules['active']),
      notLicensedModules: _int(modules['not_licensed']),
      maintenanceModules: _int(modules['maintenance']),
      verifiedMetrics: _int(impact['verified_metrics']),
      impactStatus: impact['status']?.toString() ?? 'not_collected',
      systemStatus: system['status']?.toString() ?? 'unknown',
      environment: system['environment']?.toString() ?? 'unknown',
      version: system['version']?.toString() ?? 'unknown',
    );
  }

  static Map<String, dynamic> _map(dynamic value) =>
      value is Map<String, dynamic> ? value : <String, dynamic>{};

  static int _int(dynamic value) => value is num ? value.toInt() : 0;
}
