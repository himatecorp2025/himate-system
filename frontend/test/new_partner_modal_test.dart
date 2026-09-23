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
    await tester.binding.setSurfaceSize(const Size(1440, 900));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final api = _NewPartnerApi();

    await tester.pumpWidget(
      MaterialApp(
        theme: buildBrandTheme(),
        home: PartnersPage(api: api),
      ),
    );
    await tester.pumpAndSettle();

    final button = find.byKey(const Key('partners-new-partner-button'));
    expect(button, findsOneWidget);

    await tester.tap(button);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('new-partner-dialog')), findsOneWidget);
    expect(find.text('Create partner'), findsOneWidget);
    expect(
      api.gets.where((path) => path.startsWith('/api/v1/modules')),
      isEmpty,
      reason: 'Opening New Partner must never depend on Catalog/module availability.',
    );
    expect(tester.takeException(), isNull);
  });
}
