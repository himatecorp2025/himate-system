import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:himate_frontend/main.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('Central-9 premium dark visual contract remains active', () {
    expect(brandNavyDeep, const Color(0xFF020914));
    expect(brandSurface, const Color(0xFF07182A));
    expect(brandSurfaceRaised, const Color(0xFF0B2540));
    expect(brandIonBlue, const Color(0xFF19B5FF));
    expect(buildBrandTheme().brightness, Brightness.dark);
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
