import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  testWidgets('Central-2 KPI cards execute navigation callbacks', (tester) async {
    var taps = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 320,
            child: Kpi(
              label: 'Active Partners',
              value: '12',
              note: '12 partner records',
              icon: Icons.groups_2_outlined,
              onTap: () => taps++,
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Active Partners'));
    await tester.pump();
    expect(taps, 1);
    expect(find.byIcon(Icons.arrow_forward_rounded), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('Central-2 non-navigable KPI remains informational', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 320,
            child: Kpi(
              label: 'Restricted KPI',
              value: '—',
              note: 'Permission required',
            ),
          ),
        ),
      ),
    );

    expect(find.byIcon(Icons.arrow_forward_rounded), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
