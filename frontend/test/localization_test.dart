import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:himate_frontend/main.dart';
import 'package:intl/date_symbol_data_local.dart' show initializeDateFormatting;

void main() {
  setUpAll(() async {
    await initializeDateFormatting('en_US');
    await initializeDateFormatting('hu_HU');
  });

  test('HIMATE locale mapping supports en_US and hu_HU', () {
    expect(himateLocaleFromCode('en_US'), const Locale('en', 'US'));
    expect(himateLocaleFromCode('hu_HU'), const Locale('hu', 'HU'));
    expect(himateLocaleFromCode('unexpected'), const Locale('en', 'US'));
    expect(himateLocaleCode(const Locale('hu', 'HU')), 'hu_HU');
    expect(himateLocaleCode(const Locale('en', 'US')), 'en_US');
  });

  test('central translation keys have English and Hungarian values', () {
    expect(HimateI18n.text('en_US', 'profile'), 'Profile');
    expect(HimateI18n.text('hu_HU', 'profile'), 'Profil');
    expect(HimateI18n.text('en_US', 'nav.admin'), 'Administration');
    expect(HimateI18n.text('hu_HU', 'nav.admin'), 'Adminisztráció');
    expect(HimateI18n.text('hu_HU', 'passwordSecurity'), 'Jelszó és biztonság');
  });

  test('locale-aware formatting produces values', () {
    final value = DateTime.utc(2026, 9, 21, 12, 30);
    expect(HimateI18n.dateTime('en_US', value), isNotEmpty);
    expect(HimateI18n.dateTime('hu_HU', value), isNotEmpty);
    expect(HimateI18n.currency('en_US', 1234.5), isNotEmpty);
    expect(HimateI18n.currency('hu_HU', 1234.5), isNotEmpty);
  });
}
