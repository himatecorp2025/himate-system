import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  test('workspace route slugs are stable', () {
    expect(workspaceRouteSlug('Pricing & Subscription'), 'pricing-and-subscription');
    expect(workspaceRouteSlug('Finance & Documents'), 'finance-and-documents');
    expect(workspaceRouteSlug('System & Environment'), 'system-and-environment');
  });

  testWidgets('responsive field pair stacks on phone width', (tester) async {
    await tester.binding.setSurfaceSize(const Size(360, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    const firstKey = Key('first-field');
    const secondKey = Key('second-field');
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 340,
            child: ResponsiveFieldPair(
              first: SizedBox(key: firstKey, height: 48),
              second: SizedBox(key: secondKey, height: 48),
            ),
          ),
        ),
      ),
    );

    final first = tester.getTopLeft(find.byKey(firstKey));
    final second = tester.getTopLeft(find.byKey(secondKey));
    expect(second.dy, greaterThan(first.dy));
    expect(tester.takeException(), isNull);
  });

  testWidgets('responsive field pair stays horizontal on desktop width', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1200, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    const firstKey = Key('first-wide');
    const secondKey = Key('second-wide');
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 900,
            child: ResponsiveFieldPair(
              first: SizedBox(key: firstKey, height: 48),
              second: SizedBox(key: secondKey, height: 48),
            ),
          ),
        ),
      ),
    );

    final first = tester.getTopLeft(find.byKey(firstKey));
    final second = tester.getTopLeft(find.byKey(secondKey));
    expect(second.dy, first.dy);
    expect(second.dx, greaterThan(first.dx));
    expect(tester.takeException(), isNull);
  });
}
