import 'package:flutter/material.dart';

import '../theme/himate_theme.dart';

class PlaceholderSectionScreen extends StatelessWidget {
  const PlaceholderSectionScreen({
    required this.title,
    required this.description,
    required this.futureStartBlock,
    super.key,
  });

  final String title;
  final String description;
  final String futureStartBlock;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(28),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                  color: HimateColors.text,
                  fontWeight: FontWeight.w700,
                ),
          ),
          const SizedBox(height: 8),
          Text(description, style: const TextStyle(color: HimateColors.muted)),
          const SizedBox(height: 24),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(28),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Icon(Icons.architecture_outlined, color: HimateColors.gold),
                  const SizedBox(width: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text(
                          'Navigation shell ready',
                          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
                        ),
                        const SizedBox(height: 8),
                        Text(
                          'This menu is part of the START-03 information architecture. Functional implementation is reserved for $futureStartBlock so the first development round does not silently expand scope.',
                          style: const TextStyle(color: HimateColors.muted, height: 1.45),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
