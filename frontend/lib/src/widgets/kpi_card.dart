import 'package:flutter/material.dart';

import '../theme/himate_theme.dart';

class KpiCard extends StatelessWidget {
  const KpiCard({
    required this.label,
    required this.value,
    required this.icon,
    this.caption,
    super.key,
  });

  final String label;
  final String value;
  final IconData icon;
  final String? caption;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: HimateColors.navy.withOpacity(0.07),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Icon(icon, color: HimateColors.navy),
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(label, style: const TextStyle(color: HimateColors.muted)),
                  const SizedBox(height: 6),
                  Text(
                    value,
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                          color: HimateColors.text,
                          fontWeight: FontWeight.w700,
                        ),
                  ),
                  if (caption != null) ...[
                    const SizedBox(height: 6),
                    Text(
                      caption!,
                      style: const TextStyle(
                        color: HimateColors.muted,
                        fontSize: 12,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
