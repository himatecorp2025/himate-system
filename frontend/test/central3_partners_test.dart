import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  test('Central-3 partner portfolio literals are bilingual', () {
    expect(HimateI18n.literal('hu_HU', 'Partner records'), 'Partnerrekordok');
    expect(HimateI18n.literal('hu_HU', 'Live partners'), 'Élő partnerek');
    expect(HimateI18n.literal('hu_HU', 'Prospects'), 'Érdeklődők');
    expect(HimateI18n.literal('hu_HU', 'Reference partners'), 'Referenciapartnerek');
    expect(HimateI18n.literal('hu_HU', 'Reference partners only'), 'Csak referenciapartnerek');
    expect(HimateI18n.literal('hu_HU', 'Back to Partners'), 'Vissza a Partnerekhez');
    expect(HimateI18n.literal('hu_HU', 'Ready For Launch'), 'Indításra kész');
    expect(HimateI18n.literal('hu_HU', 'TEST DATA · excluded from platform aggregates'),
        'TESZTADAT · kizárva a platform összesítéseiből');
  });
}
