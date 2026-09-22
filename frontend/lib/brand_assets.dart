part of 'main.dart';

const String himateWordmarkUrl = '/brand/himate_identity_wordmark_2026.webp';
const String himateIconUrl = '/brand/himate_identity_icon_192.webp';

String himateRuntimeWordmarkUrl = himateWordmarkUrl;
String himateRuntimeIconUrl = himateIconUrl;
String himateLoginWordmarkUrl = himateWordmarkUrl;

String _publishedDesignMediaUrl(dynamic mediaID, String fallback) {
  final id = mediaID?.toString().trim() ?? '';
  return id.isEmpty ? fallback : '/public/v1/cms/media/' + Uri.encodeComponent(id);
}

void applyPublishedBrandAssets(Map<String, dynamic> design) {
  final rawAssets = design['assets'];
  final assets = rawAssets is Map ? Map<String, dynamic>.from(rawAssets) : <String, dynamic>{};
  final legacy = (design['logo_media_asset_id'] ?? '').toString().trim();
  final header = (assets['header_wordmark'] ?? legacy).toString().trim();
  final login = (assets['login_logo'] ?? header).toString().trim();
  final appIcon = (assets['app_icon'] ?? '').toString().trim();

  himateRuntimeWordmarkUrl = _publishedDesignMediaUrl(header, himateWordmarkUrl);
  himateLoginWordmarkUrl = _publishedDesignMediaUrl(login, himateRuntimeWordmarkUrl);
  himateRuntimeIconUrl = _publishedDesignMediaUrl(appIcon, himateIconUrl);
}
