import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

class _PresetHarness extends StatefulWidget {
  const _PresetHarness({required this.rows});
  final List<Map<String,dynamic>> rows;

  @override
  State<_PresetHarness> createState() => _PresetHarnessState();
}

class _PresetHarnessState extends State<_PresetHarness> {
  late List<Map<String,dynamic>> visible;

  @override
  void initState() {
    super.initState();
    visible = List<Map<String,dynamic>>.from(widget.rows);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        children: [
          Kpi(
            label: 'Live partners',
            value: '1',
            note: 'Operational partner environments',
            icon: Icons.public_outlined,
            onTap: () => setState(() {
              visible = central8PartnerPresetRows(widget.rows, lifecycle: 'LIVE');
            }),
          ),
          for (final row in visible) Text('${row['id']}'),
        ],
      ),
    );
  }
}

void main() {
  test('Central-8 weekly Dashboard window shows the latest four real weeks', () {
    final rows = <Map<String,dynamic>>[
      for (var i = 1; i <= 6; i++)
        <String,dynamic>{
          'week': i,
          'week_start': '2026-0${i}-01',
          'value': i * 10,
        },
    ];
    final window = central8LatestWeeklyWindow(rows);
    expect(window.length, 4);
    expect(window.map((row) => row['week']).toList(), <dynamic>[3, 4, 5, 6]);
    expect(window.every((row) => '${row['label']}'.contains('/')), isTrue);
  });

  test('Central-8 partner preset produces an immediate authoritative-intent snapshot', () {
    final rows = <Map<String,dynamic>>[
      {'id': 'a', 'lifecycle': 'LIVE', 'reference_partner': false},
      {'id': 'b', 'lifecycle': 'PROSPECT', 'reference_partner': true},
      {'id': 'c', 'lifecycle': 'LIVE', 'reference_partner': true},
    ];
    expect(
      central8PartnerPresetRows(rows, lifecycle: 'LIVE').map((row) => row['id']).toList(),
      <dynamic>['a', 'c'],
    );
    expect(
      central8PartnerPresetRows(rows, reference: true).map((row) => row['id']).toList(),
      <dynamic>['b', 'c'],
    );
  });

  testWidgets('Central-8 KPI applies partner filtering on the first click', (tester) async {
    final rows = <Map<String,dynamic>>[
      {'id': 'live', 'lifecycle': 'LIVE', 'reference_partner': false},
      {'id': 'prospect', 'lifecycle': 'PROSPECT', 'reference_partner': false},
    ];

    await tester.pumpWidget(MaterialApp(home: _PresetHarness(rows: rows)));

    expect(find.text('live'), findsOneWidget);
    expect(find.text('prospect'), findsOneWidget);

    await tester.tap(find.text('Live partners'));
    await tester.pump();

    expect(find.text('live'), findsOneWidget);
    expect(find.text('prospect'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
