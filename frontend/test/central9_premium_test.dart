import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  test('Central-9 weekly window excludes future weeks and keeps latest four elapsed weeks', () {
    final now = DateTime.now().toUtc();
    final currentMonday = DateTime.utc(now.year, now.month, now.day)
        .subtract(Duration(days: now.weekday - DateTime.monday));

    final rows = <Map<String,dynamic>>[
      for (var delta = -6; delta <= 3; delta++)
        <String,dynamic>{
          'week': delta,
          'week_start': currentMonday.add(Duration(days: delta * 7)).toIso8601String().substring(0, 10),
          'value': delta + 10,
        },
    ];

    final window = central8LatestWeeklyWindow(rows);

    expect(window.length, 4);
    final starts = window
        .map((row) => DateTime.parse('${row['week_start']}').toUtc())
        .toList();
    expect(starts.every((date) => !date.isAfter(currentMonday)), isTrue);
    expect(starts.last, currentMonday);
    expect(starts.first, currentMonday.subtract(const Duration(days: 21)));
  });

  test('Central-9 canonical packages never regress to legacy pricing or limits', () {
    expect(central9CanonicalPackage('STARTER'), containsPair('price', 990));
    expect(central9CanonicalPackage('STARTER'), containsPair('entitlement', '10 modules'));

    expect(central9CanonicalPackage('BUSINESS'), containsPair('price', 1490));
    expect(central9CanonicalPackage('BUSINESS'), containsPair('entitlement', '20 modules'));

    expect(central9CanonicalPackage('FLEX'), containsPair('price', 2490));
    expect(central9CanonicalPackage('FLEX'), containsPair('entitlement', 'Unlimited'));

    expect(central9CanonicalPackagePrice('STARTER'), r'$990 + VAT');
    expect(central9CanonicalPackagePrice('BUSINESS'), r'$1,490 + VAT');
    expect(central9CanonicalPackagePrice('FLEX'), r'$2,490 + VAT');
  });
}
