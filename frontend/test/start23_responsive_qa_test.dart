import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

Future<void> pumpAt(
  WidgetTester tester,
  Size size,
  Widget child, {
  double textScale = 1,
}) async {
  await tester.binding.setSurfaceSize(size);
  await tester.pumpWidget(
    MaterialApp(
      home: MediaQuery(
        data: MediaQueryData(
          size: size,
          textScaler: TextScaler.linear(textScale),
        ),
        child: Scaffold(
          body: SizedBox.expand(child: child),
        ),
      ),
    ),
  );
  await tester.pump();
  expect(tester.takeException(), isNull);
}

void main() {
  test('START-23 breakpoints cover phone tablet laptop and desktop', () {
    expect(shellLayoutForWidth(320), ShellLayoutMode.mobile);
    expect(shellLayoutForWidth(390), ShellLayoutMode.mobile);
    expect(shellLayoutForWidth(768), ShellLayoutMode.tablet);
    expect(shellLayoutForWidth(1024), ShellLayoutMode.desktop);
    expect(shellLayoutForWidth(1440), ShellLayoutMode.desktop);

    expect(responsiveGridColumnsForWidth(320), 1);
    expect(responsiveGridColumnsForWidth(619), 1);
    expect(responsiveGridColumnsForWidth(620), 2);
    expect(responsiveGridColumnsForWidth(900), 2);
    expect(responsiveGridColumnsForWidth(980), 4);
    expect(responsiveGridColumnsForWidth(1440), 4);
  });

  test('short desktop login falls back to compact composition', () {
    expect(useCompactLoginForSize(const Size(390, 844)), isTrue);
    expect(useCompactLoginForSize(const Size(1280, 650)), isTrue);
    expect(useCompactLoginForSize(const Size(1280, 800)), isFalse);
  });

  test('content actions stack before they can crowd the title', () {
    expect(shouldStackContentActions(360, 1), isTrue);
    expect(shouldStackContentActions(768, 2), isTrue);
    expect(shouldStackContentActions(1024, 3), isTrue);
    expect(shouldStackContentActions(1180, 3), isFalse);
    expect(shouldStackContentActions(1000, 1), isFalse);
  });

  testWidgets('responsive action bar stacks on a small phone', (tester) async {
    const firstKey = Key('start23-action-1');
    const secondKey = Key('start23-action-2');

    await pumpAt(
      tester,
      const Size(320, 568),
      const Padding(
        padding: EdgeInsets.all(12),
        child: ResponsiveActionBar(
          actions: [
            SizedBox(key: firstKey, height: 44, child: Text('Primary action with a long label')),
            SizedBox(key: secondKey, height: 44, child: Text('Secondary action with a long label')),
          ],
        ),
      ),
      textScale: 1.3,
    );
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final first = tester.getTopLeft(find.byKey(firstKey));
    final second = tester.getTopLeft(find.byKey(secondKey));
    expect(second.dy, greaterThan(first.dy));
    expect(tester.takeException(), isNull);
  });

  testWidgets('responsive action bar stays horizontal on desktop', (tester) async {
    const firstKey = Key('start23-action-wide-1');
    const secondKey = Key('start23-action-wide-2');

    await pumpAt(
      tester,
      const Size(1440, 900),
      const Padding(
        padding: EdgeInsets.all(24),
        child: ResponsiveActionBar(
          actions: [
            SizedBox(key: firstKey, width: 180, height: 44),
            SizedBox(key: secondKey, width: 180, height: 44),
          ],
        ),
      ),
    );
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final first = tester.getTopLeft(find.byKey(firstKey));
    final second = tester.getTopLeft(find.byKey(secondKey));
    expect(second.dy, first.dy);
    expect(second.dx, greaterThan(first.dx));
    expect(tester.takeException(), isNull);
  });

  testWidgets('brand dialog survives phone height long copy and text scaling', (tester) async {
    await pumpAt(
      tester,
      const Size(320, 568),
      BrandDialog(
        title: 'Very long configuration dialog title that must remain readable',
        subtitle: 'Long explanatory copy must wrap without forcing the dialog outside the viewport.',
        icon: Icons.tune_rounded,
        primaryLabel: 'Save configuration changes',
        onPrimary: () {},
        child: Column(
          children: [
            for (var i = 0; i < 10; i++) ...[
              TextField(
                decoration: InputDecoration(
                  labelText: 'Long field label number $i',
                  helperText: 'Long helper text used by the responsive QA viewport matrix.',
                ),
              ),
              const SizedBox(height: 10),
            ],
          ],
        ),
      ),
      textScale: 1.3,
    );
    addTearDown(() => tester.binding.setSurfaceSize(null));

    expect(find.text('Save configuration changes'), findsOneWidget);
    expect(find.text('Cancel'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('content header and action matrix has no overflow', (tester) async {
    for (final size in <Size>[
      const Size(320, 568),
      const Size(390, 844),
      const Size(768, 1024),
      const Size(1024, 768),
      const Size(1366, 768),
      const Size(1440, 900),
    ]) {
      await pumpAt(
        tester,
        size,
        Content(
          title: 'A deliberately long workspace title for responsive quality assurance',
          subtitle: 'Long subtitles, long partner names and action labels must wrap without horizontal overflow.',
          actions: [
            OutlinedButton(onPressed: () {}, child: const Text('Refresh all records')),
            OutlinedButton(onPressed: () {}, child: const Text('Export current view')),
            FilledButton(onPressed: () {}, child: const Text('Create a new record')),
          ],
          child: const SizedBox(
            height: 120,
            child: Center(child: Text('Functional state placeholder')),
          ),
        ),
        textScale: size.width <= 390 ? 1.3 : 1,
      );
      expect(tester.takeException(), isNull, reason: 'overflow at $size');
    }
    addTearDown(() => tester.binding.setSurfaceSize(null));
  });

  testWidgets('responsive field pair boundary remains deterministic', (tester) async {
    const firstKey = Key('start23-field-first');
    const secondKey = Key('start23-field-second');

    await pumpAt(
      tester,
      const Size(600, 800),
      const Padding(
        padding: EdgeInsets.all(10),
        child: ResponsiveFieldPair(
          first: SizedBox(key: firstKey, height: 48),
          second: SizedBox(key: secondKey, height: 48),
        ),
      ),
    );
    var first = tester.getTopLeft(find.byKey(firstKey));
    var second = tester.getTopLeft(find.byKey(secondKey));
    expect(second.dy, greaterThan(first.dy));
    expect(tester.takeException(), isNull);

    await pumpAt(
      tester,
      const Size(900, 800),
      const Padding(
        padding: EdgeInsets.all(10),
        child: ResponsiveFieldPair(
          first: SizedBox(key: firstKey, height: 48),
          second: SizedBox(key: secondKey, height: 48),
        ),
      ),
    );
    first = tester.getTopLeft(find.byKey(firstKey));
    second = tester.getTopLeft(find.byKey(secondKey));
    expect(second.dy, first.dy);
    expect(second.dx, greaterThan(first.dx));
    expect(tester.takeException(), isNull);
    addTearDown(() => tester.binding.setSurfaceSize(null));
  });
}
