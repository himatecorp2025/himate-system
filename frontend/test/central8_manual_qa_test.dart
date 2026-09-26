import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

class _KpiHarness extends StatefulWidget {
  const _KpiHarness();

  @override
  State<_KpiHarness> createState() => _KpiHarnessState();
}

class _KpiHarnessState extends State<_KpiHarness> {
  String state = 'ALL';

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      home: Scaffold(
        body: Column(
          children: [
            Kpi(
              label: 'Live partners',
              value: '1',
              note: 'Operational partner environments',
              icon: Icons.public_outlined,
              onTap: () => setState(() => state = 'LIVE'),
            ),
            Text(state),
          ],
        ),
      ),
    );
  }
}

void main() {
  test('Central-8 responsive presentation grid remains deterministic', () {
    expect(responsiveGridColumnsForWidth(500), 1);
    expect(responsiveGridColumnsForWidth(800), 2);
    expect(responsiveGridColumnsForWidth(1200), 4);
  });

  testWidgets('Central-8 KPI action is applied on the first click', (tester) async {
    await tester.pumpWidget(const _KpiHarness());

    expect(find.text('ALL'), findsOneWidget);
    await tester.tap(find.text('Live partners'));
    await tester.pump();

    expect(find.text('LIVE'), findsOneWidget);
    expect(find.text('ALL'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
