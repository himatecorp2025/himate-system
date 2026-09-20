import 'package:flutter_test/flutter_test.dart';
import 'package:himate_frontend/main.dart';

void main() {
  test('CMS section serialization preserves content and deterministic order', () {
    final section = CMSSectionDraft(
      id: 'hero',
      componentType: 'HERO',
      heading: 'Heading',
      body: 'Body',
      mediaAssetId: 'cms_media_1',
      ctaLabel: 'Contact',
      ctaUrl: '/contact',
      visible: false,
      sortOrder: 90,
    );

    final json = section.toJson(2);
    expect(json['id'], 'hero');
    expect(json['component_type'], 'HERO');
    expect(json['heading'], 'Heading');
    expect(json['body'], 'Body');
    expect(json['media_asset_id'], 'cms_media_1');
    expect(json['cta_label'], 'Contact');
    expect(json['cta_url'], '/contact');
    expect(json['visible'], isFalse);
    expect(json['sort_order'], 20);
    expect(json['settings'], isA<Map<String, dynamic>>());
  });
}
