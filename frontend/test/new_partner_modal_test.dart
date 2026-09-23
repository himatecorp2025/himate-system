import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

class _NewPartnerApi extends Api {
  final List<String> gets = <String>[];

  @override
  Future<Map<String, dynamic>> get(
    String path, {
    Duration maxAge = const Duration(seconds: 30),
    bool force = false,
  }) async {
    gets.add(path);
    if (path == '/api/v1/partner-categories') {
      return <String, dynamic>{
        'items': <Map<String, dynamic>>[
          <String, dynamic>{
            'id': 'cat_006',
            'name': 'Other',
            'name_en': 'Other',
            'name_hu': 'Egyéb',
          },
        ],
      };
    }
    if (path.startsWith('/api/v1/partners')) {
      return <String, dynamic>{
        'items': <Map<String, dynamic>>[],
        'count': 0,
        'total': 0,
        'has_more': false,
        'lifecycle_counts': <String, int>{},
        'reference_count': 0,
      };
    }
    if (path.startsWith('/api/v1/modules')) {
      throw ApiError(502, 'Catalog unavailable');
    }
    return <String, dynamic>{'items': <Map<String, dynamic>>[]};
  }
}

void main() {
  testWidgets('START-23.11.3e New Partner button opens the master-data modal without Catalog', (tester) async {
    HimateI18n.activeLocale = 'en_US';
    await tester.binding.setSurfaceSize(const Size(1440, 1100));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final api = _NewPartnerApi();

    try {
      await tester.pumpWidget(
        MaterialApp(
          theme: buildBrandTheme(),
          home: PartnersPage(api: api),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 250));

      final initialException = tester.takeException();
      if (initialException != null) {
        fail('Partners page threw before New Partner interaction: $initialException');
      }

      final button = find.byKey(const Key('partners-new-partner-button'));
      if (button.evaluate().length != 1) {
        fail('New Partner button was not rendered. GET calls: ${api.gets}');
      }

      await tester.ensureVisible(button);
      await tester.tap(button, warnIfMissed: false);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));

      final openException = tester.takeException();
      if (openException != null) {
        fail('New Partner modal threw while opening: $openException');
      }

      final dialog = find.byKey(const Key('new-partner-dialog'));
      if (dialog.evaluate().length != 1) {
        fail('New Partner modal did not render after click. GET calls: ${api.gets}');
      }

      expect(
        api.gets.where((path) => path.startsWith('/api/v1/modules')),
        isEmpty,
        reason: 'Opening New Partner must never depend on Catalog/module availability.',
      );

      final cancel = find.widgetWithText(TextButton, 'Cancel');
      expect(cancel, findsOneWidget);
      await tester.tap(cancel);
      await tester.pumpAndSettle();

      final closeException = tester.takeException();
      if (closeException != null) {
        fail('New Partner modal threw while closing: $closeException');
      }
    } catch (error, stack) {
      fail('New Partner regression stage failed: $error\n$stack');
    }
  });
}
