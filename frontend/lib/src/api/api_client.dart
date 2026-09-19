// ignore_for_file: avoid_web_libraries_in_flutter

import 'dart:convert';
import 'dart:html' as html;

class ApiException implements Exception {
  ApiException(this.statusCode, this.message, {this.code});

  final int statusCode;
  final String message;
  final String? code;

  @override
  String toString() => message;
}

class ApiClient {
  ApiClient({this.baseUrl = ''});

  final String baseUrl;

  Future<Map<String, dynamic>> getJson(String path) async {
    final response = await html.HttpRequest.request(
      '$baseUrl$path',
      method: 'GET',
      withCredentials: true,
    );
    return _decode(response);
  }

  Future<Map<String, dynamic>> postJson(
    String path, [
    Map<String, dynamic>? body,
  ]) async {
    final response = await html.HttpRequest.request(
      '$baseUrl$path',
      method: 'POST',
      withCredentials: true,
      requestHeaders: const {'Content-Type': 'application/json'},
      sendData: jsonEncode(body ?? <String, dynamic>{}),
    );
    if (response.status == 204) {
      return <String, dynamic>{};
    }
    return _decode(response);
  }

  Map<String, dynamic> _decode(html.HttpRequest response) {
    final raw = response.responseText ?? '';
    Map<String, dynamic> decoded = <String, dynamic>{};
    if (raw.trim().isNotEmpty) {
      final value = jsonDecode(raw);
      if (value is Map<String, dynamic>) {
        decoded = value;
      }
    }

    final status = response.status ?? 0;
    if (status >= 200 && status < 300) {
      return decoded;
    }

    final error = decoded['error'];
    if (error is Map<String, dynamic>) {
      throw ApiException(
        status,
        error['message']?.toString() ?? 'Request failed',
        code: error['code']?.toString(),
      );
    }
    throw ApiException(status, 'Request failed');
  }
}
