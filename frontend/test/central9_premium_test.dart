import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('Central-16 approved light navy-gold visual contract supersedes Central-9 dark presentation', () {
    expect(brandNavyDeep, const Color(0xFF071A2E));
    expect(brandSurface, const Color(0xFFFFFFFF));
    expect(brandSurfaceRaised, const Color(0xFFFFFFFF));
    expect(brandIonBlue, const Color(0xFF1769E0));
    expect(buildBrandTheme().brightness, Brightness.light);
    expect(buildBrandTheme().scaffoldBackgroundColor, const Color(0xFFF6F8FC));
  });

  test('Central responsive presentation helpers remain deterministic', () {
    expect(responsiveGridColumnsForWidth(500), 1);
    expect(responsiveGridColumnsForWidth(800), 2);
    expect(responsiveGridColumnsForWidth(1400), 4);
    expect(shellLayoutForWidth(500), ShellLayoutMode.mobile);
    expect(shellLayoutForWidth(800), ShellLayoutMode.tablet);
    expect(shellLayoutForWidth(1400), ShellLayoutMode.desktop);
  });
}
