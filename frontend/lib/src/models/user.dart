class HimateUser {
  const HimateUser({
    required this.id,
    required this.name,
    required this.email,
    required this.roles,
  });

  final String id;
  final String name;
  final String email;
  final List<String> roles;

  factory HimateUser.fromJson(Map<String, dynamic> json) {
    final rawRoles = json['roles'];
    return HimateUser(
      id: json['id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      email: json['email']?.toString() ?? '',
      roles: rawRoles is List
          ? rawRoles.map((value) => value.toString()).toList(growable: false)
          : const <String>[],
    );
  }
}
