import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  test('Central-4 module workspace literals are bilingual', () {
    expect(HimateI18n.literal('hu_HU', 'Module Topics'), 'Modultémák');
    expect(HimateI18n.literal('hu_HU', 'Commercial Matrix'), 'Kereskedelmi mátrix');
    expect(HimateI18n.literal('hu_HU', 'Back to module topics'), 'Vissza a modultémákhoz');
    expect(HimateI18n.literal('hu_HU', 'Runtime uses · 7 days'), 'Runtime használat · 7 nap');
    expect(HimateI18n.literal('hu_HU', 'Runtime uses · 30 days'), 'Runtime használat · 30 nap');
    expect(HimateI18n.literal('hu_HU', 'active partner assignments'), 'aktív partner-hozzárendelés');
  });
}
