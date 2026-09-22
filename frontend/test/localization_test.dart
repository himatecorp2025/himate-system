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

  test('admin literal translation covers key business surfaces', () {
    expect(HimateI18n.literal('hu_HU', 'Contact Leads'), 'Kapcsolati érdeklődők');
    expect(HimateI18n.literal('hu_HU', 'Design Guide'), 'Arculati útmutató');
    expect(HimateI18n.literal('hu_HU', '6 reports'), '6 jelentés');
    expect(HimateI18n.literal('en_US', 'Contact Leads'), 'Contact Leads');
  });

  test('START-23 bilingual QA covers critical control-plane surfaces', () {
    for (final value in <String>[
      'Create role',
      'Add administrator',
      'Commercial agreement reference *',
      'Partner Website Adapter',
      'Backup storage',
      'Notifications',
      'Partner Portal',
      'Activate module',
      'Edit CMS draft · Example page',
      'CMS audit · Example page',
      'Version history · Example page',
    ]) {
      expect(
        HimateI18n.literal('hu_HU', value),
        isNot(value),
        reason: 'Missing Hungarian START-23 translation for $value',
      );
    }
  });

  test('START-23.2 commercial control-plane labels are localized', () {
    for (final value in <String>[
      'Partner × Module Commercial Matrix',
      'View by partner',
      'View by module',
      'Commercial history',
      'Activation date',
      'Next billing date',
      'Next billing price',
      'Price source',
      'Activation fee source',
      'Partner activation fee',
    ]) {
      expect(
        HimateI18n.literal('hu_HU', value),
        isNot(value),
        reason: 'Missing Hungarian START-23.2 translation for $value',
      );
    }
  });

  test('START-23.3 subscription lifecycle labels are localized', () {
    for (final value in <String>[
      'Subscription lifecycle',
      'Cancellation effective',
      'Cancel pending',
      'Paid-period deactivation is Billing-managed. Use Cancel at period end; access remains active until the current 30-day period closes.',
    ]) {
      expect(
        HimateI18n.literal('hu_HU', value),
        isNot(value),
        reason: 'Missing Hungarian START-23.3 translation for $value',
      );
    }
  });

  test('START-23.4 provider payment labels are localized', () {
    for (final value in <String>[
      'Provider customer ID',
      'Payment method ID',
      'Provider reference',
      'Automatic recurring collection',
      'Collect activation license after save',
      'PAID is set only after the signed provider webhook is verified.',
      'Signed provider webhook only',
      'Automatic 30-day collection',
    ]) {
      expect(
        HimateI18n.literal('hu_HU', value),
        isNot(value),
        reason: 'Missing Hungarian START-23.4 translation for $value',
      );
    }
  });

  test('START-23 dynamic CMS and commercial messages are localized', () {
    expect(HimateI18n.literal('hu_HU', 'Section 4'), 'Szekció 4');
    expect(HimateI18n.literal('hu_HU', 'Version 12'), 'Verzió 12');
    expect(
      HimateI18n.literal('hu_HU', 'Agreement could not be updated: timeout'),
      startsWith('A megállapodás nem frissíthető:'),
    );
    expect(
      HimateI18n.literal('hu_HU', 'Website adapter could not be updated: timeout'),
      startsWith('A weboldal-adapter nem frissíthető:'),
    );
  });

  test('password policy requires every complexity class', () {
    expect(himatePasswordMeetsPolicy('Strong-Password1!'), isTrue);
    expect(himatePasswordMeetsPolicy('alllowercase123!'), isFalse);
    expect(himatePasswordMeetsPolicy('ALLUPPERCASE123!'), isFalse);
    expect(himatePasswordMeetsPolicy('NoNumberPassword!'), isFalse);
    expect(himatePasswordMeetsPolicy('NoSpecial123456'), isFalse);
    expect(himatePasswordPolicyMessage('hu_HU', 'weak'), isNotNull);
  });

  test('locale-aware formatting produces values', () {
    final value = DateTime.utc(2026, 9, 21, 12, 30);
    expect(HimateI18n.dateTime('en_US', value), isNotEmpty);
    expect(HimateI18n.dateTime('hu_HU', value), isNotEmpty);
    expect(HimateI18n.currency('en_US', 1234.5), isNotEmpty);
    expect(HimateI18n.currency('hu_HU', 1234.5), isNotEmpty);
  });
}
