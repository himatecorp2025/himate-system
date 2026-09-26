import 'dart:js_interop';
import 'dart:typed_data';

import 'package:web/web.dart' as web;

class BrowserFile {
  const BrowserFile(this._file);

  final web.File _file;

  String get name => _file.name;
}

Future<BrowserFile?> pickBrowserFile(String accept) async {
  final input = web.HTMLInputElement()
    ..type = 'file'
    ..accept = accept;
  input.click();
  await input.onChange.first;

  final files = input.files;
  if (files == null || files.length == 0) return null;
  final file = files.item(0);
  return file == null ? null : BrowserFile(file);
}

Future<Uint8List> readBrowserFile(BrowserFile file) async {
  final jsBuffer = await file._file.arrayBuffer().toDart;
  return Uint8List.view(jsBuffer.toDart);
}

void openBrowserDownload(String path) {
  web.window.open(path, '_blank');
}

String? browserStorageGet(String key) =>
    web.window.localStorage.getItem(key);

void browserStorageSet(String key, String value) {
  web.window.localStorage.setItem(key, value);
}

void browserStorageRemove(String key) {
  web.window.localStorage.removeItem(key);
}

void replaceBrowserHistory(String title, String path) {
  web.window.history.replaceState(null, title, path);
}
