import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  testWidgets('START-23.11.3d Partner Login password can be explicitly cleared', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: buildBrandTheme(),
        home: PartnerPortalLoginPage(
          localeCode: 'en_US',
          onLocaleChanged: (_) {},
          onLogin: (_, __, ___) async {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    final fields = find.byType(TextField);
    expect(fields, findsNWidgets(2));

    final passwordField = fields.at(1);
    await tester.enterText(passwordField, 'Temporary!Password123');
    await tester.pump();

    var passwordWidget = tester.widget<TextField>(passwordField);
    expect(passwordWidget.controller?.text, 'Temporary!Password123');

    await tester.tap(find.byTooltip('Clear password'));
    await tester.pump();

    passwordWidget = tester.widget<TextField>(passwordField);
    expect(passwordWidget.controller?.text, isEmpty);
    expect(tester.takeException(), isNull);
  });
}
